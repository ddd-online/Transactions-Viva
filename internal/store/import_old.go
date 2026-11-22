package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"transactions/internal/domain"
)

// ImportReport is what an old-workspace import copied and what it noticed.
type ImportReport struct {
	Ledgers         int
	Categories      int
	Tags            int
	Records         int
	RecordTags      int
	Templates       int
	Charts          int
	StockAccounts   int
	StockFunds      int
	StockPositions  int
	StockTrades     int
	StockRounds     int
	StockLessons    int
	StockLadderDays int
	TodoCards       int
	TodoItems       int
	TodoProgress    int
	DiaryBooks      int
	DiaryEntries    int
	Events          int
	EventImages     int
	EventImageFiles int
	Orphans         []string
	// MergedEvents explains old events that shared a date with another ledger:
	// the destination keeps one event per day, so a merge is data the user
	// should hear about (the old workspace still has the dropped copy).
	MergedEvents []string
	// MissingAssets are image files the old workspace did not have. Their rows
	// are still imported; the page shows the picture as missing.
	MissingAssets []string
	SourceSchema  int
}

// ImportOld copies the old application's data into a new v1 workspace. The old
// database is opened read-only; the destination must be a new directory or an
// empty one. The accounting tables, saved charts, key events (with their image
// files), stock tables, todo boards and diaries are copied. Old diaries are
// ledger-scoped, so each ledger with diary rows becomes a diary book; rows
// without a ledger go to 未归属日记.
//
// Events are the one place the shapes differ: the old application stored one
// event per (ledger, date), while this workspace keeps one per date. Events that
// share a date are merged into the newest row and reported, and every image of
// that date is kept.
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

	hasEvents, err := tableExists(source, "key_event")
	if err != nil {
		return report, err
	}
	if hasAppID(source) {
		return report, errors.New("导入：这是新版工作空间，不能作为旧库导入")
	}
	if exists, err := tableExists(source, "stock_account"); err != nil {
		return report, err
	} else if exists {
		hasLedger, err := tableHasColumn(source, "stock_account", "ledger_id")
		if err != nil {
			return report, err
		}
		if !hasLedger {
			return report, errors.New("导入：这是新版工作空间，不能作为旧库导入")
		}
	}
	workspace, err := Open(destinationDirectory)
	if err != nil {
		return report, err
	}
	defer func() {
		_ = workspace.Close()
		if err != nil {
			cleanupDatabase(destinationPath)
			_ = os.RemoveAll(filepath.Join(destinationDirectory, "data", "assets"))
		}
	}()
	db, err := workspace.dbHandle()
	if err != nil {
		return report, err
	}
	var eventImages []domain.KeyEventImage

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
		if hasEvents {
			events, images, merged, err := copyKeyEvents(source, tx)
			if err != nil {
				return err
			}
			report.Events, report.EventImages = events, len(images)
			report.MergedEvents = merged
			eventImages = images
		}
		stock, err := copyStock(source, tx)
		if err != nil {
			return err
		}
		report.StockAccounts = stock.Accounts
		report.StockFunds = stock.Funds
		report.StockPositions = stock.Positions
		report.StockTrades = stock.Trades
		report.StockRounds = stock.Rounds
		report.StockLessons = stock.Lessons
		report.StockLadderDays = stock.LadderDays
		todo, err := copyTodo(source, tx)
		if err != nil {
			return err
		}
		report.TodoCards = todo.Cards
		report.TodoItems = todo.Items
		report.TodoProgress = todo.Progress
		if report.DiaryBooks, report.DiaryEntries, err = copyDiary(source, tx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return report, fmt.Errorf("导入：写入目标库失败（%w）", err)
	}
	if len(eventImages) > 0 {
		report.EventImageFiles, report.MissingAssets = copyEventAssets(sourcePath, destinationDirectory, eventImages)
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

// oldDiaryEntry is one old diary row before it is assigned to a book.
type oldDiaryEntry struct {
	id        string
	date      string
	content   string
	wordCount int
	mood      string
	createdAt int64
	updatedAt int64
	ledgerID  string
	ledger    string
}

// copyDiary imports the old ledger-scoped diary as one book per old ledger.
// Keeping the ledgers separate preserves dates that occur in more than one old
// ledger; the new page supports multiple books for exactly this reason.
func copyDiary(source *sql.DB, tx *sql.Tx) (books, entries int, err error) {
	exists, err := tableExists(source, "diary_entry")
	if err != nil || !exists {
		return 0, 0, err
	}
	rows, err := source.Query(`SELECT d.id, d.date, COALESCE(d.content, ''),
		COALESCE(d.word_count, 0), COALESCE(d.mood, ''), d.created_at, d.updated_at,
		COALESCE(d.ledger_id, ''), COALESCE(l.name, '')
		FROM diary_entry d LEFT JOIN ledger l ON l.id = d.ledger_id
		ORDER BY d.ledger_id, d.date`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	var groups [][]oldDiaryEntry
	index := make(map[string]int)
	for rows.Next() {
		var entry oldDiaryEntry
		if err := rows.Scan(&entry.id, &entry.date, &entry.content, &entry.wordCount,
			&entry.mood, &entry.createdAt, &entry.updatedAt, &entry.ledgerID,
			&entry.ledger); err != nil {
			return 0, 0, err
		}
		group, ok := index[entry.ledgerID]
		if !ok {
			group = len(groups)
			index[entry.ledgerID] = group
			groups = append(groups, nil)
		}
		groups[group] = append(groups[group], entry)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	var sortOrder int
	if err := tx.QueryRow("SELECT COALESCE(MAX(sort_order), -1) FROM diary_book").Scan(&sortOrder); err != nil {
		return 0, 0, err
	}
	sortOrder++
	for _, group := range groups {
		first := group[0]
		name := strings.TrimSpace(first.ledger)
		if name == "" {
			if first.ledgerID == "" {
				name = "未归属日记"
			} else {
				name = "旧账本 " + first.ledgerID
			}
		}
		bookID := "book-import-unassigned"
		description := "旧工作空间未归属账本的日记"
		if first.ledgerID != "" {
			bookID = "book-import-" + first.ledgerID
			description = fmt.Sprintf("旧账本「%s」的日记", name)
		}
		createdAt, updatedAt := first.createdAt, first.updatedAt
		for _, entry := range group[1:] {
			if entry.createdAt < createdAt {
				createdAt = entry.createdAt
			}
			if entry.updatedAt > updatedAt {
				updatedAt = entry.updatedAt
			}
		}
		if _, err := tx.Exec(`INSERT INTO diary_book
			(book_id, name, description, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			bookID, name, description, sortOrder, createdAt, updatedAt); err != nil {
			return books, entries, err
		}
		sortOrder++
		books++
		for _, entry := range group {
			mood := domain.NormalizeDiaryMood(entry.mood)
			if _, err := tx.Exec(`INSERT INTO diary_entry
				(id, date, content, word_count, mood, created_at, updated_at, book_id)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				entry.id, entry.date, entry.content, entry.wordCount, mood,
				entry.createdAt, entry.updatedAt, bookID); err != nil {
				return books, entries, err
			}
			entries++
		}
	}
	return books, entries, nil
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
	hasImages, err := tableExists(db, "key_event_image")
	if err != nil {
		return nil, err
	}
	hasEvents, err := tableExists(db, "key_event")
	if err != nil {
		return nil, err
	}
	if hasImages && hasEvents {
		checks = append(checks, struct {
			label string
			query string
		}{"图片指向不存在的事件", "SELECT COUNT(*) FROM key_event_image i WHERE NOT EXISTS " +
			"(SELECT 1 FROM key_event e WHERE e.ledger_id = i.ledger_id AND e.date = i.event_date)"})
	}
	var orphans []string
	hasTodoCards, err := tableExists(db, "todo_card")
	if err != nil {
		return nil, err
	}
	hasTodoItems, err := tableExists(db, "todo_item")
	if err != nil {
		return nil, err
	}
	hasTodoProgress, err := tableExists(db, "todo_progress")
	if err != nil {
		return nil, err
	}
	if hasTodoCards {
		checks = append(checks,
			struct {
				label string
				query string
			}{"待办主题指向不存在的账本", "SELECT COUNT(*) FROM todo_card WHERE ledger_id NOT IN (SELECT id FROM ledger)"},
			struct {
				label string
				query string
			}{"待办事项指向不存在的卡片", "SELECT COUNT(*) FROM todo_item WHERE card_id NOT IN (SELECT id FROM todo_card)"},
		)
	}
	if hasTodoItems && hasTodoProgress {
		checks = append(checks, struct {
			label string
			query string
		}{"待办进度指向不存在的事项", "SELECT COUNT(*) FROM todo_progress WHERE item_id NOT IN (SELECT id FROM todo_item)"})
	}
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

type todoImport struct {
	Cards    int
	Items    int
	Progress int
}

// copyTodo imports the old todo board, keeping ids, timestamps, ordering and
// completed state. Old 11-stop urgency / importance values are snapped onto the
// six stops the new pages write.
func copyTodo(source *sql.DB, tx *sql.Tx) (todoImport, error) {
	var report todoImport
	hasCards, err := tableExists(source, "todo_card")
	if err != nil || !hasCards {
		return report, err
	}
	rows, err := source.Query("SELECT id, ledger_id, title, created_at, updated_at, sort_order FROM todo_card")
	if err != nil {
		return report, err
	}
	for rows.Next() {
		var id, ledgerID, title string
		var createdAt, updatedAt int64
		var sortOrder int
		if err := rows.Scan(&id, &ledgerID, &title, &createdAt, &updatedAt, &sortOrder); err != nil {
			rows.Close()
			return report, err
		}
		if _, err := tx.Exec("INSERT INTO todo_card (id, ledger_id, title, created_at, updated_at, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
			id, "", title, createdAt, updatedAt, sortOrder); err != nil {
			rows.Close()
			return report, err
		}
		report.Cards++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return report, err
	}
	rows.Close()

	hasItems, err := tableExists(source, "todo_item")
	if err != nil || !hasItems {
		return report, err
	}
	items, err := source.Query("SELECT id, ledger_id, card_id, title, start_date, due_date, urgency, importance, status, completed_at, created_at, updated_at FROM todo_item")
	if err != nil {
		return report, err
	}
	for items.Next() {
		var id, ledgerID, cardID, title, startDate, dueDate, status string
		var urgency, importance int
		var completedAt, createdAt, updatedAt int64
		if err := items.Scan(&id, &ledgerID, &cardID, &title, &startDate, &dueDate, &urgency, &importance,
			&status, &completedAt, &createdAt, &updatedAt); err != nil {
			items.Close()
			return report, err
		}
		if status != domain.TodoDone {
			status = domain.TodoDoing
			completedAt = 0
		}
		urgency = domain.SnapTodoLevel(urgency)
		importance = domain.SnapTodoLevel(importance)
		if _, err := tx.Exec("INSERT INTO todo_item ("+todoItemColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, "", cardID, title, startDate, dueDate, urgency, importance, status, completedAt, createdAt, updatedAt); err != nil {
			items.Close()
			return report, err
		}
		report.Items++
	}
	if err := items.Err(); err != nil {
		items.Close()
		return report, err
	}
	items.Close()

	hasProgress, err := tableExists(source, "todo_progress")
	if err != nil || !hasProgress {
		return report, err
	}
	progress, err := source.Query("SELECT id, ledger_id, item_id, content, created_at, done FROM todo_progress")
	if err != nil {
		return report, err
	}
	for progress.Next() {
		var id, ledgerID, itemID, content string
		var createdAt int64
		var done int
		if err := progress.Scan(&id, &ledgerID, &itemID, &content, &createdAt, &done); err != nil {
			progress.Close()
			return report, err
		}
		if _, err := tx.Exec("INSERT INTO todo_progress ("+todoProgressColumns+") VALUES (?, ?, ?, ?, ?, ?)",
			id, "", itemID, content, createdAt, done != 0); err != nil {
			progress.Close()
			return report, err
		}
		report.Progress++
	}
	if err := progress.Err(); err != nil {
		progress.Close()
		return report, err
	}
	progress.Close()
	return report, nil
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

// oldEvent is one row of the old key_event table.
type oldEvent struct {
	id        string
	date      string
	title     string
	content   string
	color     string
	createdAt int64
	updatedAt int64
	ledgerID  string
}

// copyKeyEvents copies the old per-ledger events into the workspace's one
// per-date table. The same date may exist in several ledgers; the newest row
// supplies the text and every image of that date is kept, which is reported so
// the user knows a second copy existed.
func copyKeyEvents(source *sql.DB, tx *sql.Tx) (int, []domain.KeyEventImage, []string, error) {
	rows, err := source.Query(
		"SELECT id, date, COALESCE(title, ''), COALESCE(content, ''), COALESCE(color, ''), " +
			"created_at, updated_at, COALESCE(ledger_id, '') FROM key_event")
	if err != nil {
		return 0, nil, nil, err
	}
	byDate := map[string][]oldEvent{}
	var dates []string
	for rows.Next() {
		var event oldEvent
		if err := rows.Scan(&event.id, &event.date, &event.title, &event.content, &event.color,
			&event.createdAt, &event.updatedAt, &event.ledgerID); err != nil {
			rows.Close()
			return 0, nil, nil, err
		}
		if _, exists := byDate[event.date]; !exists {
			dates = append(dates, event.date)
		}
		byDate[event.date] = append(byDate[event.date], event)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, nil, nil, err
	}
	rows.Close()
	sort.Strings(dates)
	ledgerNames := oldLedgerNames(source)

	imported := map[string]bool{}
	var merged []string
	for _, date := range dates {
		group := byDate[date]
		survivor := group[0]
		for _, event := range group[1:] {
			if event.updatedAt > survivor.updatedAt ||
				(event.updatedAt == survivor.updatedAt && event.id < survivor.id) {
				survivor = event
			}
		}
		if _, err := tx.Exec(
			"INSERT INTO key_event (id, date, title, content, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			survivor.id, survivor.date, survivor.title, survivor.content,
			normalizeOldEventColor(survivor.color), survivor.createdAt, survivor.updatedAt,
		); err != nil {
			return 0, nil, nil, err
		}
		imported[date] = true
		if len(group) > 1 {
			var dropped []string
			for _, event := range group {
				if event.id == survivor.id {
					continue
				}
				dropped = append(dropped, "「"+oldEventName(event)+"」（"+oldLedgerName(ledgerNames, event.ledgerID)+"）")
			}
			merged = append(merged, fmt.Sprintf("事件 %s：保留「%s」（%s），并入 %s",
				date, oldEventName(survivor), oldLedgerName(ledgerNames, survivor.ledgerID),
				strings.Join(dropped, "、")))
		}
	}

	hasImages, err := tableExists(source, "key_event_image")
	if err != nil {
		return 0, nil, nil, err
	}
	if !hasImages {
		return len(dates), nil, merged, nil
	}
	images, err := oldEventImages(source)
	if err != nil {
		return 0, nil, nil, err
	}
	sort.SliceStable(images, func(i, j int) bool {
		left, right := images[i], images[j]
		if left.EventDate != right.EventDate {
			return left.EventDate < right.EventDate
		}
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt < right.CreatedAt
		}
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		return left.ID < right.ID
	})
	nextOrder := 1
	lastDate := ""
	var kept []domain.KeyEventImage
	for _, image := range images {
		if !imported[image.EventDate] {
			// oldOrphans reports these; a non-strict import leaves them out.
			continue
		}
		if image.EventDate != lastDate {
			nextOrder, lastDate = 1, image.EventDate
		}
		image.SortOrder = nextOrder
		nextOrder++
		if _, err := tx.Exec(
			"INSERT INTO key_event_image (id, event_date, file_path, thumb_path, original_path, "+
				"motion_path, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			image.ID, image.EventDate, image.FilePath, image.ThumbPath, image.OriginalPath,
			image.MotionPath, image.SortOrder, image.CreatedAt,
		); err != nil {
			return 0, nil, nil, err
		}
		kept = append(kept, image)
	}
	return len(dates), kept, merged, nil
}

// oldEventImages reads the old image rows, tolerating a table from before the
// original_path / motion_path columns existed.
func oldEventImages(source *sql.DB) ([]domain.KeyEventImage, error) {
	columns := []string{
		"SELECT id, COALESCE(ledger_id, ''), event_date, file_path, thumb_path, ",
	}
	original := "''"
	if exists, err := tableHasColumn(source, "key_event_image", "original_path"); err != nil {
		return nil, err
	} else if exists {
		original = "original_path"
	}
	motion := "''"
	if exists, err := tableHasColumn(source, "key_event_image", "motion_path"); err != nil {
		return nil, err
	} else if exists {
		motion = "motion_path"
	}
	query := columns[0] + original + ", " + motion +
		", sort_order, created_at FROM key_event_image"
	rows, err := source.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var images []domain.KeyEventImage
	var ledgerID string
	for rows.Next() {
		var image domain.KeyEventImage
		if err := rows.Scan(&image.ID, &ledgerID, &image.EventDate, &image.FilePath,
			&image.ThumbPath, &image.OriginalPath, &image.MotionPath,
			&image.SortOrder, &image.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

// oldLedgerNames maps ledger ids to names for the merge report.
func oldLedgerNames(source *sql.DB) map[string]string {
	names := map[string]string{}
	rows, err := source.Query("SELECT id, name FROM ledger")
	if err != nil {
		return names
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err == nil {
			names[id] = name
		}
	}
	return names
}

// oldEventName is what a report calls an old event.
func oldEventName(event oldEvent) string {
	if title := strings.TrimSpace(event.title); title != "" {
		return title
	}
	if event.date != "" {
		return event.date
	}
	return event.id
}

// oldLedgerName names the old event's ledger; an unknown id is shown as written.
func oldLedgerName(names map[string]string, id string) string {
	if name := names[id]; name != "" {
		return name
	}
	if id == "" {
		return "未知账本"
	}
	return id
}

// normalizeOldEventColor maps the two identity colours the old application
// darkened (its migration event_tone_contrast) to the values this palette
// carries. The importer opens the old database read-only, so that migration
// never ran against a very old workspace.
func normalizeOldEventColor(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "#C6963A":
		return "#B08534"
	case "#D48838":
		return "#C17C33"
	}
	return value
}

// copyEventAssets copies the event image files the imported rows name. A file
// the old workspace no longer has is reported, not fatal: the row still tells
// the page which picture belonged there.
func copyEventAssets(sourcePath, destinationDirectory string, images []domain.KeyEventImage) (int, []string) {
	sourceRoot := filepath.Join(filepath.Dir(sourcePath), "data", "assets")
	destinationRoot := filepath.Join(destinationDirectory, "data", "assets")
	copied := 0
	var missing []string
	seen := map[string]bool{}
	for _, image := range images {
		for _, relative := range []string{image.FilePath, image.ThumbPath, image.OriginalPath, image.MotionPath} {
			if relative == "" || seen[relative] {
				continue
			}
			seen[relative] = true
			sourceFile, ok := joinAssetPath(sourceRoot, relative)
			if !ok {
				missing = append(missing, relative+"（路径非法）")
				continue
			}
			if info, err := os.Stat(sourceFile); err != nil || info.IsDir() {
				missing = append(missing, relative+"（旧工作空间里没有这个文件）")
				continue
			}
			destinationFile, ok := joinAssetPath(destinationRoot, relative)
			if !ok {
				missing = append(missing, relative+"（路径非法）")
				continue
			}
			if err := os.MkdirAll(filepath.Dir(destinationFile), 0o755); err != nil {
				missing = append(missing, relative+"（"+err.Error()+"）")
				continue
			}
			if err := copyFileBytes(sourceFile, destinationFile); err != nil {
				missing = append(missing, relative+"（"+err.Error()+"）")
				continue
			}
			copied++
		}
	}
	return copied, missing
}

// joinAssetPath resolves a stored relative path under root, refusing escapes.
func joinAssetPath(root, relative string) (string, bool) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(rootAbs, filepath.FromSlash(relative))
	rel, err := filepath.Rel(rootAbs, candidate)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return candidate, true
}

// copyFileBytes copies one file, removing a partly written destination.
func copyFileBytes(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(destination)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return nil
}
