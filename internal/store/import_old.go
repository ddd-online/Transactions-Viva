package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ImportReport is what an old-workspace import copied and what it noticed.
type ImportReport struct {
	Ledgers      int
	Categories   int
	Tags         int
	Records      int
	RecordTags   int
	Templates    int
	Charts       int
	Orphans      []string
	SourceSchema int
}

// ImportOld copies the five accounting tables from an old application database
// into a new workspace. The old database is opened read-only; the destination
// must be a new directory or an empty one. The old stock/event/diary/todo
// tables are deliberately ignored; saved charts are copied when present.
func ImportOld(destinationDirectory, sourceDatabase string, strict bool) (ImportReport, error) {
	var report ImportReport
	if strings.TrimSpace(sourceDatabase) == "" {
		return report, errors.New("导入：旧数据库路径为空")
	}
	sourcePath, err := filepath.Abs(sourceDatabase)
	if err != nil {
		return report, err
	}
	if info, err := os.Stat(sourcePath); err != nil || info.IsDir() {
		return report, fmt.Errorf("导入：旧数据库不存在（%s）", sourcePath)
	}
	if err := os.MkdirAll(destinationDirectory, 0o755); err != nil {
		return report, err
	}
	destinationPath := databasePathOf(destinationDirectory)
	if _, err := os.Stat(destinationPath); err == nil {
		return report, fmt.Errorf("导入：目标目录已存在 transactions.db（%s）", destinationPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return report, err
	}

	source, err := openOldDatabase(sourcePath)
	if err != nil {
		return report, err
	}
	defer source.Close()
	if err := quickCheck(source); err != nil {
		return report, fmt.Errorf("导入：旧数据库已损坏（%w）", err)
	}
	if legacy, err := tableExists(source, "tbl_billadm_schema_migration"); err != nil {
		return report, err
	} else if legacy {
		return report, ErrLegacyWorkspace
	}
	if err := validateOldTables(source); err != nil {
		return report, err
	}
	if version, hasMeta, err := schemaVersion(source); err == nil && hasMeta {
		report.SourceSchema = version
	}
	report.Orphans, err = oldOrphans(source)
	if err != nil {
		return report, err
	}
	if strict && len(report.Orphans) > 0 {
		return report, fmt.Errorf("导入：旧库有 %d 处孤儿数据（-strict），%s", len(report.Orphans), strings.Join(report.Orphans, "；"))
	}

	workspace, err := Open(destinationDirectory)
	if err != nil {
		return report, err
	}
	defer func() {
		_ = workspace.Close()
		if err != nil {
			cleanupDatabase(destinationPath)
		}
	}()
	db, err := workspace.dbHandle()
	if err != nil {
		return report, err
	}

	err = inTx(context.Background(), db, func(tx *sql.Tx) error {
		if report.Ledgers, err = copyLedgers(source, tx); err != nil {
			return err
		}
		if report.Categories, err = copyCategories(source, tx); err != nil {
			return err
		}
		if report.Tags, err = copyTags(source, tx); err != nil {
			return err
		}
		if report.Records, err = copyRecords(source, tx); err != nil {
			return err
		}
		if report.RecordTags, err = copyRecordTags(source, tx); err != nil {
			return err
		}
		if report.Templates, err = copyTemplates(source, tx); err != nil {
			return err
		}
		if hasCharts, err := tableExists(source, "chart"); err != nil {
			return err
		} else if hasCharts {
			if report.Charts, err = copyCharts(source, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return report, fmt.Errorf("导入：写入目标库失败（%w）", err)
	}
	return report, nil
}

// openOldDatabase opens the source read-only so an import never creates a WAL
// or changes the old application's data.
func openOldDatabase(path string) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	slash := filepath.ToSlash(absolute)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	uri := url.URL{Scheme: "file", Path: slash}
	query := uri.Query()
	query.Set("mode", "ro")
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("导入：打开旧数据库失败（%w）", err)
	}
	return db, nil
}

func validateOldTables(db *sql.DB) error {
	required := []string{"ledger", "category", "tag", "transaction_record", "transaction_record_tag", "transaction_template"}
	for _, table := range required {
		exists, err := tableExists(db, table)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("导入：旧数据库缺少表 %s", table)
		}
	}
	return nil
}

func oldOrphans(db *sql.DB) ([]string, error) {
	checks := []struct {
		label string
		query string
	}{
		{"记录指向不存在的账本", "SELECT COUNT(*) FROM transaction_record WHERE ledger_id NOT IN (SELECT id FROM ledger)"},
		{"模板指向不存在的账本", "SELECT COUNT(*) FROM transaction_template WHERE ledger_id NOT IN (SELECT id FROM ledger)"},
		{"分类指向不存在的账本", "SELECT COUNT(*) FROM category WHERE ledger_id NOT IN (SELECT id FROM ledger)"},
		{"标签指向不存在的账本", "SELECT COUNT(*) FROM tag WHERE ledger_id NOT IN (SELECT id FROM ledger)"},
		{"记录标签指向不存在的记录", "SELECT COUNT(*) FROM transaction_record_tag WHERE transaction_id NOT IN (SELECT transaction_id FROM transaction_record)"},
		{"记录标签指向不存在的账本", "SELECT COUNT(*) FROM transaction_record_tag WHERE ledger_id NOT IN (SELECT id FROM ledger)"},
	}
	var orphans []string
	for _, check := range checks {
		var count int
		if err := db.QueryRow(check.query).Scan(&count); err != nil {
			return nil, err
		}
		if count > 0 {
			orphans = append(orphans, fmt.Sprintf("%s（%d）", check.label, count))
		}
	}
	return orphans, nil
}

func copyLedgers(source *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := source.Query("SELECT id, name, description, created_at, updated_at FROM ledger")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, name, description string
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &name, &description, &createdAt, &updatedAt); err != nil {
			return count, err
		}
		if _, err := tx.Exec("INSERT INTO ledger (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			id, name, description, createdAt, updatedAt); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyCategories(source *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := source.Query("SELECT ledger_id, name, transaction_type, sort_order FROM category")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var ledgerID, name, transactionType string
		var sortOrder int
		if err := rows.Scan(&ledgerID, &name, &transactionType, &sortOrder); err != nil {
			return count, err
		}
		if _, err := tx.Exec("INSERT INTO category (ledger_id, name, transaction_type, sort_order) VALUES (?, ?, ?, ?)",
			ledgerID, name, transactionType, sortOrder); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyTags(source *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := source.Query("SELECT ledger_id, name, category_transaction_type, sort_order FROM tag")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var ledgerID, name, categoryTransactionType string
		var sortOrder int
		if err := rows.Scan(&ledgerID, &name, &categoryTransactionType, &sortOrder); err != nil {
			return count, err
		}
		if _, err := tx.Exec("INSERT INTO tag (ledger_id, name, category_transaction_type, sort_order) VALUES (?, ?, ?, ?)",
			ledgerID, name, categoryTransactionType, sortOrder); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyRecords(source *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := source.Query("SELECT transaction_id, ledger_id, amount_cents, transaction_type, category, " +
		"description, flags, key_event_date, transaction_at, created_at, updated_at FROM transaction_record")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var transactionID, ledgerID, transactionType, category, keyEventDate string
		var amountCents, transactionAt, createdAt, updatedAt int64
		var description, flags sql.NullString
		if err := rows.Scan(&transactionID, &ledgerID, &amountCents, &transactionType, &category,
			&description, &flags, &keyEventDate, &transactionAt, &createdAt, &updatedAt); err != nil {
			return count, err
		}
		if _, err := tx.Exec("INSERT INTO transaction_record (transaction_id, ledger_id, amount_cents, transaction_type, category, "+
			"description, flags, key_event_date, transaction_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			transactionID, ledgerID, amountCents, transactionType, category, nullable(description), nullable(flags),
			keyEventDate, transactionAt, createdAt, updatedAt); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func tableHasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func copyCharts(source *sql.DB, tx *sql.Tx) (int, error) {
	// chart_type and the grouping columns were added by later old migrations;
	// a chart table from an early workspace still imports as line charts with no
	// grouping instead of failing the whole import.
	optional := func(column, fallback string) (string, error) {
		exists, err := tableHasColumn(source, "chart", column)
		if err != nil {
			return "", err
		}
		if exists {
			return column, nil
		}
		return fallback + " AS " + column, nil
	}
	chartType, err := optional("chart_type", "'line'")
	if err != nil {
		return 0, err
	}
	groupField, err := optional("group_field", "''")
	if err != nil {
		return 0, err
	}
	groupTransactionType, err := optional("group_transaction_type", "''")
	if err != nil {
		return 0, err
	}
	groupCategory, err := optional("group_category", "''")
	if err != nil {
		return 0, err
	}
	rows, err := source.Query("SELECT chart_id, ledger_id, title, granularity, chart_lines, " + chartType + ", " +
		groupField + ", " + groupTransactionType + ", " + groupCategory + ", is_preset, sort_order, created_at, updated_at FROM chart")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var chartID, ledgerID, title, granularity, chartLines, chartTypeValue string
		var groupFieldValue, groupTransactionTypeValue, groupCategoryValue string
		var isPreset, sortOrder, createdAt, updatedAt int64
		if err := rows.Scan(&chartID, &ledgerID, &title, &granularity, &chartLines, &chartTypeValue,
			&groupFieldValue, &groupTransactionTypeValue, &groupCategoryValue, &isPreset, &sortOrder, &createdAt, &updatedAt); err != nil {
			return count, err
		}
		if chartTypeValue == "" {
			chartTypeValue = "line"
		}
		if _, err := tx.Exec("INSERT INTO chart (chart_id, ledger_id, title, granularity, chart_lines, chart_type, "+
			"group_field, group_transaction_type, group_category, is_preset, sort_order, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			chartID, ledgerID, title, granularity, chartLines, chartTypeValue, groupFieldValue,
			groupTransactionTypeValue, groupCategoryValue, isPreset, sortOrder, createdAt, updatedAt); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyRecordTags(source *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := source.Query("SELECT ledger_id, transaction_id, tag FROM transaction_record_tag")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var ledgerID, transactionID, tag string
		if err := rows.Scan(&ledgerID, &transactionID, &tag); err != nil {
			return count, err
		}
		if _, err := tx.Exec("INSERT INTO transaction_record_tag (ledger_id, transaction_id, tag) VALUES (?, ?, ?)",
			ledgerID, transactionID, tag); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyTemplates(source *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := source.Query("SELECT template_id, ledger_id, template_name, transaction_type, category, " +
		"tags, flags, description, sort_order, created_at, updated_at FROM transaction_template")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var templateID, ledgerID, templateName, transactionType, category string
		var sortOrder, createdAt, updatedAt int64
		var tags, flags, description sql.NullString
		if err := rows.Scan(&templateID, &ledgerID, &templateName, &transactionType, &category,
			&tags, &flags, &description, &sortOrder, &createdAt, &updatedAt); err != nil {
			return count, err
		}
		if _, err := tx.Exec("INSERT INTO transaction_template (template_id, ledger_id, template_name, transaction_type, category, "+
			"tags, flags, description, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			templateID, ledgerID, templateName, transactionType, category, nullable(tags), nullable(flags),
			nullable(description), sortOrder, createdAt, updatedAt); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func nullable(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func cleanupDatabase(path string) {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(path + suffix)
	}
}
