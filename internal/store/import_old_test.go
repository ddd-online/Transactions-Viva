package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func oldFixture(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	path := filepath.Join(directory, "transactions.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy, err := os.ReadFile(filepath.Join("testdata", "legacy_schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(legacy)); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		"INSERT INTO ledger (id, name, description, created_at, updated_at) VALUES ('l1', '家庭账本', '', 100, 200)",
		"INSERT INTO ledger (id, name, description, created_at, updated_at) VALUES ('l2', '旅行账本', '', 300, 400)",
		"INSERT INTO category (ledger_id, name, transaction_type, sort_order) VALUES ('l1', '餐饮美食', 'expense', 0)",
		"INSERT INTO tag (ledger_id, name, category_transaction_type, sort_order) VALUES ('l1', '三餐', '餐饮美食:expense', 0)",
		"INSERT INTO transaction_record (transaction_id, ledger_id, amount_cents, transaction_type, category, description, flags, key_event_date, transaction_at, created_at, updated_at) " +
			"VALUES ('r1', 'l1', 3200, 'expense', '餐饮美食', NULL, NULL, '', 500, 600, 700)",
		"INSERT INTO transaction_record_tag (ledger_id, transaction_id, tag) VALUES ('l1', 'r1', '三餐')",
		"INSERT INTO transaction_template (template_id, ledger_id, template_name, transaction_type, category, tags, flags, description, sort_order, created_at, updated_at) " +
			"VALUES ('t1', 'l1', '午餐', 'expense', '餐饮美食', '[\"三餐\"]', 'outlier', NULL, 0, 800, 900)",
		"INSERT INTO chart (chart_id, ledger_id, title, granularity, chart_lines, chart_type, group_field, group_transaction_type, group_category, is_preset, sort_order, created_at, updated_at) " +
			"VALUES ('c1', 'l1', '月度消费趋势', 'month', '[]', 'line', '', '', '', 1, 0, 1000, 1000)",
		"INSERT INTO app_meta (key, value) VALUES ('schema_version', '12')",
		"CREATE TABLE stock_account (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', principal_cents INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)",
		"INSERT INTO stock_account (id, ledger_id, principal_cents, created_at, updated_at) VALUES ('s1', 'l1', 100000, 100, 200)",
		"INSERT INTO todo_card (id, ledger_id, title, created_at, updated_at, sort_order) VALUES ('tc1', 'l1', '本周', 100, 200, 1)",
		"INSERT INTO todo_item (id, ledger_id, card_id, title, start_date, due_date, urgency, importance, status, completed_at, created_at, updated_at) " +
			"VALUES ('ti1', 'l1', 'tc1', '写周报', '2026-01-01', '2026-01-02', 3, 4, 'done', 500, 100, 500)",
		"INSERT INTO todo_progress (id, ledger_id, item_id, content, created_at, done) VALUES ('tp1', 'l1', 'ti1', '列大纲', 300, 1)",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	return path
}

func TestImportOldCopiesAccountingTables(t *testing.T) {
	source := oldFixture(t)
	destination := filepath.Join(t.TempDir(), "imported")
	report, err := ImportOld(destination, source, false)
	if err != nil {
		t.Fatalf("ImportOld: %v", err)
	}
	if report.Ledgers != 2 || report.Categories != 1 || report.Tags != 1 ||
		report.Records != 1 || report.RecordTags != 1 || report.Templates != 1 || report.Charts != 1 ||
		report.TodoCards != 1 || report.TodoItems != 1 || report.TodoProgress != 1 {
		t.Fatalf("report = %+v", report)
	}
	if report.SourceSchema != 12 {
		t.Fatalf("source schema = %d", report.SourceSchema)
	}

	workspace, err := Open(destination)
	if err != nil {
		t.Fatalf("open destination: %v", err)
	}
	defer workspace.Close()
	if version, _, err := schemaVersion(workspace.db); err != nil || version != 1 {
		t.Fatalf("destination schema = %d, err=%v, want v1", version, err)
	}
	ledgers, err := workspace.Ledgers(t.Context())
	if err != nil || len(ledgers) != 2 || ledgers[0].ID != "l1" {
		t.Fatalf("ledgers = %+v, err=%v", ledgers, err)
	}
	labels, err := workspace.Labels(t.Context(), "l1")
	if err != nil || len(labels.Categories) != 1 || len(labels.Tags) != 1 || labels.TagCounts["三餐"] != 1 {
		t.Fatalf("labels = %+v, err=%v", labels, err)
	}
	entry, err := workspace.Record(t.Context(), "r1")
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if entry.Record.Description != "" || entry.Record.Flags != "" || entry.Record.AmountCents != 3200 {
		t.Fatalf("record = %+v", entry.Record)
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != "三餐" {
		t.Fatalf("tags = %v", entry.Tags)
	}
	templates, err := workspace.Templates(t.Context(), "l1")
	if err != nil || len(templates) != 1 || templates[0].Flags != "outlier" || templates[0].Description != "" {
		t.Fatalf("templates = %+v, err=%v", templates, err)
	}
	if cards, err := workspace.TodoCards(t.Context()); err != nil || len(cards) != 1 || cards[0].Title != "本周" {
		t.Fatalf("todo cards = %+v, err=%v", cards, err)
	}
	if history, err := workspace.TodoHistory(t.Context()); err != nil || len(history) != 1 ||
		history[0].Item.Importance != 5 || len(history[0].Item.Progress) != 1 || !history[0].Item.Progress[0].Done {
		t.Fatalf("todo history = %+v, err=%v", history, err)
	}
}

func TestImportOldStrictRejectsOrphans(t *testing.T) {
	source := oldFixture(t)
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO transaction_record (transaction_id, ledger_id, amount_cents, transaction_type, category, transaction_at, created_at, updated_at) " +
		"VALUES ('orphan', 'missing', 100, 'expense', '餐饮美食', 100, 100, 100)"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	destination := filepath.Join(t.TempDir(), "rejected")
	if _, err := ImportOld(destination, source, true); err == nil {
		t.Fatal("strict import accepted an orphan record")
	}
	if _, err := os.Stat(filepath.Join(destination, "transactions.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed import left a database: %v", err)
	}
}

func TestImportOldNonStrictReportsOrphans(t *testing.T) {
	source := oldFixture(t)
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO transaction_record (transaction_id, ledger_id, amount_cents, transaction_type, category, transaction_at, created_at, updated_at) " +
		"VALUES ('orphan', 'missing', 100, 'expense', '餐饮美食', 100, 100, 100)"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	destination := filepath.Join(t.TempDir(), "imported")
	report, err := ImportOld(destination, source, false)
	if err != nil {
		t.Fatalf("ImportOld: %v", err)
	}
	if len(report.Orphans) == 0 || report.Records != 2 {
		t.Fatalf("report = %+v", report)
	}
}

func TestImportOldRefusesExistingDatabase(t *testing.T) {
	source := oldFixture(t)
	destination := t.TempDir()
	workspace, err := Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Close()
	if _, err := ImportOld(destination, source, false); err == nil {
		t.Fatal("import wrote over an existing workspace")
	}
}

// addOldEvents gives an old fixture the event tables the old application had,
// two ledgers' events on the same day and image rows with real files. The
// second image's thumbnail is deliberately absent, to exercise the warning.
func addOldEvents(t *testing.T, source string) {
	t.Helper()
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	statements := []string{
		`CREATE TABLE key_event (
			id TEXT PRIMARY KEY, date TEXT NOT NULL, title TEXT, content TEXT, color TEXT,
			created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, ledger_id TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE key_event_image (
			id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', event_date TEXT NOT NULL,
			file_path TEXT NOT NULL DEFAULT '', thumb_path TEXT NOT NULL DEFAULT '',
			original_path TEXT NOT NULL DEFAULT '', motion_path TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL)`,
		"INSERT INTO key_event (id, date, title, content, color, created_at, updated_at, ledger_id) VALUES " +
			"('e1', '2026-01-15', '年会', '旧正文', '#C6963A', 100, 200, 'l1')",
		"INSERT INTO key_event (id, date, title, content, color, created_at, updated_at, ledger_id) VALUES " +
			"('e2', '2026-01-15', '聚餐', '另一份', '#D9705A', 150, 300, 'l2')",
		"INSERT INTO key_event_image (id, ledger_id, event_date, file_path, thumb_path, original_path, motion_path, sort_order, created_at) VALUES " +
			"('i1', 'l1', '2026-01-15', 'key_events/2026-01-15/i1.png', 'key_events/2026-01-15/thumb_i1.jpg', '', '', 1, 100)",
		"INSERT INTO key_event_image (id, ledger_id, event_date, file_path, thumb_path, original_path, motion_path, sort_order, created_at) VALUES " +
			"('i2', 'l2', '2026-01-15', 'key_events/2026-01-15/i2.jpg', 'key_events/2026-01-15/thumb_i2.jpg', '', '', 1, 150)",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	assets := filepath.Join(filepath.Dir(source), "data", "assets", "key_events", "2026-01-15")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"i1.png": "display-one", "thumb_i1.jpg": "thumb-one", "i2.jpg": "display-two",
	} {
		if err := os.WriteFile(filepath.Join(assets, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestImportOldMergesEventsAndCopiesImages(t *testing.T) {
	source := oldFixture(t)
	addOldEvents(t, source)
	destination := filepath.Join(t.TempDir(), "imported")
	report, err := ImportOld(destination, source, false)
	if err != nil {
		t.Fatalf("ImportOld: %v", err)
	}
	if report.Events != 1 || report.EventImages != 2 || report.EventImageFiles != 3 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.MergedEvents) != 1 {
		t.Fatalf("merged events = %+v", report.MergedEvents)
	}
	if len(report.MissingAssets) != 1 || !strings.Contains(report.MissingAssets[0], "thumb_i2.jpg") {
		t.Fatalf("missing assets = %+v", report.MissingAssets)
	}

	workspace, err := Open(destination)
	if err != nil {
		t.Fatalf("open destination: %v", err)
	}
	defer workspace.Close()
	events, err := workspace.KeyEventsByYear(t.Context(), "2026")
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %+v, err=%v", events, err)
	}
	event := events[0]
	if event.ID != "e2" || event.Title != "聚餐" || event.Content != "另一份" || event.Color != "#D9705A" {
		t.Fatalf("merged event = %+v", event)
	}
	images, err := workspace.KeyEventImages(t.Context(), "2026-01-15")
	if err != nil || len(images) != 2 {
		t.Fatalf("images = %+v, err=%v", images, err)
	}
	if images[0].ID != "i1" || images[0].SortOrder != 1 || images[1].ID != "i2" || images[1].SortOrder != 2 {
		t.Fatalf("images = %+v", images)
	}
	for _, image := range images {
		for _, relative := range []string{image.FilePath, image.ThumbPath} {
			if relative == "key_events/2026-01-15/thumb_i2.jpg" {
				continue
			}
			if _, err := os.Stat(filepath.Join(destination, "data", "assets", filepath.FromSlash(relative))); err != nil {
				t.Fatalf("imported asset %q: %v", relative, err)
			}
		}
	}
}

func TestImportOldNormalizesTheTwoDarkenedEventColors(t *testing.T) {
	source := oldFixture(t)
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE key_event (
		id TEXT PRIMARY KEY, date TEXT NOT NULL, title TEXT, content TEXT, color TEXT,
		created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, ledger_id TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO key_event (id, date, title, content, color, created_at, updated_at, ledger_id) " +
		"VALUES ('e1', '2026-02-02', '旧色', '', '#c6963a', 1, 2, 'l1')"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	destination := filepath.Join(t.TempDir(), "imported")
	if _, err := ImportOld(destination, source, false); err != nil {
		t.Fatalf("ImportOld: %v", err)
	}
	workspace, err := Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Close()
	event, err := workspace.KeyEvent(t.Context(), "2026-02-02")
	if err != nil || event.Color != "#B08534" {
		t.Fatalf("colour = %q, err=%v", event.Color, err)
	}
}

func TestImportOldStrictRejectsOrphanEventImages(t *testing.T) {
	source := oldFixture(t)
	addOldEvents(t, source)
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO key_event_image (id, ledger_id, event_date, file_path, sort_order, created_at) " +
		"VALUES ('orphan', 'l1', '2026-03-03', 'key_events/2026-03-03/x.jpg', 1, 100)"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	destination := filepath.Join(t.TempDir(), "rejected")
	if _, err := ImportOld(destination, source, true); err == nil {
		t.Fatal("strict import accepted an image without an event")
	}
	if _, err := os.Stat(filepath.Join(destination, "transactions.db")); !os.IsNotExist(err) {
		t.Fatalf("failed import left a database: %v", err)
	}
}
