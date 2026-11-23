package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

const recordColumns = "transaction_id, ledger_id, amount_cents, transaction_type, category, description, " +
	"flags, key_event_date, transaction_at, created_at, updated_at"

// Records returns one ledger's records in a local-date range. The range is
// handled by SQL; the page keeps its own multi-column sorting and pagination.
func (w *Workspace) Records(ctx context.Context, query repository.RecordQuery) ([]repository.RecordEntry, error) {
	if strings.TrimSpace(query.LedgerID) == "" {
		return nil, invalidf("没有指定账本")
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	where := []string{"ledger_id = ?"}
	args := []any{query.LedgerID}
	if strings.TrimSpace(query.StartDate) != "" {
		start, err := localDayStart(query.StartDate)
		if err != nil {
			return nil, err
		}
		where = append(where, "transaction_at >= ?")
		args = append(args, start)
	}
	if strings.TrimSpace(query.EndDate) != "" {
		end, err := localDayEnd(query.EndDate)
		if err != nil {
			return nil, err
		}
		where = append(where, "transaction_at <= ?")
		args = append(args, end)
	}
	orderBy, err := recordOrderBy(query.Sorts)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+recordColumns+" FROM transaction_record WHERE "+strings.Join(where, " AND ")+
			" ORDER BY "+orderBy, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []repository.RecordEntry
	var ids []string
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		ids = append(ids, record.TransactionID)
		records = append(records, repository.RecordEntry{Record: record})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tags, err := tagsForRecords(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for index := range records {
		records[index].Tags = tags[records[index].Record.TransactionID]
	}
	return records, nil
}

// Record returns one record and its tags.
func (w *Workspace) Record(ctx context.Context, id string) (repository.RecordEntry, error) {
	db, err := w.dbHandle()
	if err != nil {
		return repository.RecordEntry{}, err
	}
	row := db.QueryRowContext(ctx, "SELECT "+recordColumns+" FROM transaction_record WHERE transaction_id = ?", id)
	record, err := scanRecord(row)
	if err == sql.ErrNoRows {
		return repository.RecordEntry{}, notFoundf("找不到这条记录")
	}
	if err != nil {
		return repository.RecordEntry{}, err
	}
	tags, err := tagsForRecords(ctx, db, []string{id})
	if err != nil {
		return repository.RecordEntry{}, err
	}
	return repository.RecordEntry{Record: record, Tags: tags[id]}, nil
}

// CreateRecord inserts a record and its tag links in one transaction.
func (w *Workspace) CreateRecord(ctx context.Context, record domain.TransactionRecord, tags []string) (string, error) {
	if err := validateRecord(record); err != nil {
		return "", err
	}
	db, err := w.dbHandle()
	if err != nil {
		return "", err
	}
	if record.TransactionID == "" {
		record.TransactionID = newID()
	}
	now := nowUnix()
	record.CreatedAt, record.UpdatedAt = now, now
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		if err := requireLedger(ctx, tx, record.LedgerID); err != nil {
			return err
		}
		if err := insertRecord(ctx, tx, record); err != nil {
			return err
		}
		return insertRecordTags(ctx, tx, record.LedgerID, record.TransactionID, tags)
	})
	if err != nil {
		return "", err
	}
	return record.TransactionID, nil
}

// UpdateRecord keeps the record id and replaces its tag links; keeping the id
// is deliberate and invisible in the UI.
func (w *Workspace) UpdateRecord(ctx context.Context, record domain.TransactionRecord, tags []string) error {
	if strings.TrimSpace(record.TransactionID) == "" {
		return invalidf("没有指定记录")
	}
	if err := validateRecord(record); err != nil {
		return err
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	record.UpdatedAt = nowUnix()
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if err := requireLedger(ctx, tx, record.LedgerID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx,
			"UPDATE transaction_record SET ledger_id = ?, amount_cents = ?, transaction_type = ?, category = ?, "+
				"description = ?, flags = ?, key_event_date = ?, transaction_at = ?, updated_at = ? "+
				"WHERE transaction_id = ?",
			record.LedgerID, record.AmountCents, record.TransactionType, record.Category,
			record.Description, record.Flags, record.KeyEventDate, record.TransactionAt, record.UpdatedAt,
			record.TransactionID,
		)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return notFoundf("找不到这条记录")
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM transaction_record_tag WHERE transaction_id = ?", record.TransactionID); err != nil {
			return err
		}
		return insertRecordTags(ctx, tx, record.LedgerID, record.TransactionID, tags)
	})
}

// DeleteRecord removes the record and its tag links; a missing row is fine.
func (w *Workspace) DeleteRecord(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return invalidf("没有指定记录")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM transaction_record_tag WHERE transaction_id = ?", id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM transaction_record WHERE transaction_id = ?", id)
		return err
	})
}

// SyncRecord copies a record to another ledger with a new id. The key-event
// date is not copied, matching the row action.
func (w *Workspace) SyncRecord(ctx context.Context, id, targetLedgerID string) error {
	source, err := w.Record(ctx, id)
	if err != nil {
		return err
	}
	if source.Record.LedgerID == targetLedgerID {
		return invalidf("记录不能同步到同一个账本")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	now := nowUnix()
	copyRecord := source.Record
	copyRecord.TransactionID = newID()
	copyRecord.LedgerID = targetLedgerID
	copyRecord.KeyEventDate = ""
	copyRecord.CreatedAt, copyRecord.UpdatedAt = now, now
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if err := requireLedger(ctx, tx, targetLedgerID); err != nil {
			return err
		}
		if err := insertRecord(ctx, tx, copyRecord); err != nil {
			return err
		}
		return insertRecordTags(ctx, tx, targetLedgerID, copyRecord.TransactionID, source.Tags)
	})
}

func validateRecord(record domain.TransactionRecord) error {
	if strings.TrimSpace(record.LedgerID) == "" {
		return invalidf("没有指定账本")
	}
	if !validTransactionType(record.TransactionType) {
		return invalidf("交易类型「%s」无法识别", record.TransactionType)
	}
	if strings.TrimSpace(record.Category) == "" {
		return invalidf("分类不能为空")
	}
	return nil
}

func requireLedger(ctx context.Context, tx *sql.Tx, ledgerID string) error {
	exists, err := ledgerExists(ctx, tx, ledgerID)
	if err != nil {
		return err
	}
	if !exists {
		return invalidf("找不到这个账本")
	}
	return nil
}

func insertRecord(ctx context.Context, tx *sql.Tx, record domain.TransactionRecord) error {
	_, err := tx.ExecContext(ctx,
		"INSERT INTO transaction_record (transaction_id, ledger_id, amount_cents, transaction_type, category, "+
			"description, flags, key_event_date, transaction_at, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		record.TransactionID, record.LedgerID, record.AmountCents, record.TransactionType, record.Category,
		record.Description, record.Flags, record.KeyEventDate, record.TransactionAt,
		record.CreatedAt, record.UpdatedAt,
	)
	return err
}

func insertRecordTags(ctx context.Context, tx *sql.Tx, ledgerID, transactionID string, tags []string) error {
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, duplicate := seen[tag]; duplicate {
			continue
		}
		seen[tag] = struct{}{}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO transaction_record_tag (ledger_id, transaction_id, tag) VALUES (?, ?, ?)",
			ledgerID, transactionID, tag); err != nil {
			return err
		}
	}
	return nil
}

// recordScanner is satisfied by *sql.Row and *sql.Rows.
type recordScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row recordScanner) (domain.TransactionRecord, error) {
	var record domain.TransactionRecord
	var description, flags sql.NullString
	err := row.Scan(
		&record.TransactionID, &record.LedgerID, &record.AmountCents, &record.TransactionType,
		&record.Category, &description, &flags, &record.KeyEventDate, &record.TransactionAt,
		&record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		return domain.TransactionRecord{}, err
	}
	record.Description = description.String
	record.Flags = flags.String
	return record, nil
}

func tagsForRecords(ctx context.Context, db *sql.DB, ids []string) (map[string][]string, error) {
	result := map[string][]string{}
	if len(ids) == 0 {
		return result, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	rows, err := db.QueryContext(ctx,
		"SELECT transaction_id, tag FROM transaction_record_tag WHERE transaction_id IN ("+placeholders+") ORDER BY tag", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, tag string
		if err := rows.Scan(&id, &tag); err != nil {
			return nil, err
		}
		result[id] = append(result[id], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for id := range result {
		sort.Strings(result[id])
	}
	return result, nil
}

// recordOrderBy turns the page's active columns into a whitelisted ORDER BY
// over the whole date range. The amount is ordered by the value the table shows:
// an expense is negative, income and transfers are positive. The final keys keep
// rows with equal sort values stable.
func recordOrderBy(sorts []repository.RecordSort) (string, error) {
	keys := make([]string, 0, len(sorts)+2)
	for _, sort := range sorts {
		direction := "ASC"
		if sort.Desc {
			direction = "DESC"
		}
		switch sort.Field {
		case repository.RecordSortDate:
			keys = append(keys, "transaction_at "+direction)
		case repository.RecordSortAmount:
			keys = append(keys, "CASE transaction_type WHEN 'expense' THEN -amount_cents ELSE amount_cents END "+direction)
		default:
			return "", invalidf("排序字段「%s」无法识别", sort.Field)
		}
	}
	keys = append(keys, "transaction_at DESC", "transaction_id ASC")
	return strings.Join(keys, ", "), nil
}

func localDayStart(value string) (int64, error) {
	day, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return 0, invalidf("日期「%s」无法识别", value)
	}
	return day.Unix(), nil
}

func localDayEnd(value string) (int64, error) {
	day, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return 0, invalidf("日期「%s」无法识别", value)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 0, time.Local).Unix(), nil
}
