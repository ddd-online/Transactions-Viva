package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/domain"
)

// Ledgers lists the workspace's ledgers in creation order.
func (w *Workspace) Ledgers(ctx context.Context) ([]domain.Ledger, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT id, name, description, created_at, updated_at FROM ledger ORDER BY created_at, rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ledgers []domain.Ledger
	for rows.Next() {
		var ledger domain.Ledger
		if err := rows.Scan(&ledger.ID, &ledger.Name, &ledger.Description, &ledger.CreatedAt, &ledger.UpdatedAt); err != nil {
			return nil, err
		}
		ledgers = append(ledgers, ledger)
	}
	return ledgers, rows.Err()
}

// CreateLedger inserts a ledger and seeds its default categories and tags in the
// same transaction: a half-seeded ledger is never left behind.
func (w *Workspace) CreateLedger(ctx context.Context, name, description string) (domain.Ledger, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Ledger{}, invalidf("账本名称不能为空")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.Ledger{}, err
	}
	now := nowUnix()
	ledger := domain.Ledger{
		ID: newID(), Name: name, Description: strings.TrimSpace(description),
		CreatedAt: now, UpdatedAt: now,
	}
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO ledger (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			ledger.ID, ledger.Name, ledger.Description, ledger.CreatedAt, ledger.UpdatedAt,
		); err != nil {
			return fmt.Errorf("创建账本失败：%w", err)
		}
		return seedDefaults(ctx, tx, ledger.ID)
	})
	if err != nil {
		return domain.Ledger{}, err
	}
	return ledger, nil
}

// UpdateLedger renames a ledger and replaces its description. The name is the
// only required field; an empty description clears it. As in CreateLedger the
// name is trimmed, and a missing ledger is reported rather than silently
// ignored: the settings page would otherwise show a name that never saved.
func (w *Workspace) UpdateLedger(ctx context.Context, id, name, description string) error {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" {
		return invalidf("没有指定账本")
	}
	if name == "" {
		return invalidf("账本名称不能为空")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx,
		"UPDATE ledger SET name = ?, description = ?, updated_at = ? WHERE id = ?",
		name, strings.TrimSpace(description), nowUnix(), id)
	if err != nil {
		return fmt.Errorf("重命名账本失败：%w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return notFoundf("找不到该账本")
	}
	return nil
}

// DeleteLedger removes the ledger and its data. The schema has no foreign keys,
// so the cascade is explicit and ordered.
func (w *Workspace) DeleteLedger(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return invalidf("没有指定账本")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		// The application's list, minus the stock, event and diary tables
		// this schema does not create. The charts belong to the ledger too:
		// without this line they would outlive it as orphan rows.
		statements := []string{
			"DELETE FROM transaction_record_tag WHERE ledger_id = ?",
			"DELETE FROM transaction_record WHERE ledger_id = ?",
			"DELETE FROM category WHERE ledger_id = ?",
			"DELETE FROM tag WHERE ledger_id = ?",
			"DELETE FROM chart WHERE ledger_id = ?",
			"DELETE FROM transaction_template WHERE ledger_id = ?",
			"DELETE FROM ledger WHERE id = ?",
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// seedDefaults writes the 19 categories and their 57 tags. The default
// categories start at sort_order 0; the page orders by sort_order then name.
func seedDefaults(ctx context.Context, tx *sql.Tx, ledgerID string) error {
	categoryStatement, err := tx.PrepareContext(ctx,
		"INSERT INTO category (ledger_id, name, transaction_type, sort_order) VALUES (?, ?, ?, 0)")
	if err != nil {
		return err
	}
	defer categoryStatement.Close()
	tagStatement, err := tx.PrepareContext(ctx,
		"INSERT INTO tag (ledger_id, name, category_transaction_type, sort_order) VALUES (?, ?, ?, 0)")
	if err != nil {
		return err
	}
	defer tagStatement.Close()
	for _, group := range defaultGroups {
		for _, category := range group.categories {
			if _, err := categoryStatement.ExecContext(ctx, ledgerID, category.name, group.transactionType); err != nil {
				return fmt.Errorf("写入默认分类失败：%w", err)
			}
			key := category.name + ":" + group.transactionType
			for _, tag := range category.tags {
				if _, err := tagStatement.ExecContext(ctx, ledgerID, tag, key); err != nil {
					return fmt.Errorf("写入默认标签失败：%w", err)
				}
			}
		}
	}
	return nil
}

// ledgerExists reports whether a ledger id is in the workspace, for writes that
// need a valid target (record sync).
func ledgerExists(ctx context.Context, tx *sql.Tx, id string) (bool, error) {
	var found string
	err := tx.QueryRowContext(ctx, "SELECT id FROM ledger WHERE id = ?", id).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
