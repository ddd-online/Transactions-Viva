package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func testWorkspace(t *testing.T) *Workspace {
	t.Helper()
	workspace, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Close() })
	return workspace
}

func testLedger(t *testing.T, workspace *Workspace) domain.Ledger {
	t.Helper()
	ledger, err := workspace.CreateLedger(context.Background(), "家庭账本", "")
	if err != nil {
		t.Fatalf("CreateLedger: %v", err)
	}
	return ledger
}

func day(year int, month time.Month, dayOfMonth, hour, minute, second int) int64 {
	return time.Date(year, month, dayOfMonth, hour, minute, second, 0, time.Local).Unix()
}

func TestOpenCreatesSchemaAndReopens(t *testing.T) {
	directory := t.TempDir()
	workspace, err := Open(directory)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if workspace.DatabasePath() != filepath.Join(directory, "transactions.db") {
		t.Fatalf("database path = %q", workspace.DatabasePath())
	}
	version, exists, err := schemaVersion(workspace.db)
	if err != nil || !exists || version != targetVersion() {
		t.Fatalf("schema version = %d, exists=%v, err=%v", version, exists, err)
	}
	db := workspace.db
	for _, table := range []string{"app_meta", "ledger", "transaction_record", "transaction_record_tag", "category", "tag", "transaction_template"} {
		ok, err := tableExists(db, table)
		if err != nil || !ok {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
	if err := workspace.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	ledgers, err := reopened.Ledgers(context.Background())
	if err != nil || len(ledgers) != 0 {
		t.Fatalf("ledgers = %v, err=%v", ledgers, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "transactions.db")); err != nil {
		t.Fatalf("database file: %v", err)
	}
}

func TestCreateLedgerSeedsDefaults(t *testing.T) {
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)
	snapshot, err := workspace.Labels(context.Background(), ledger.ID)
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if len(snapshot.Categories) != 19 || len(snapshot.Tags) != 57 {
		t.Fatalf("defaults = %d categories, %d tags; want 19/57", len(snapshot.Categories), len(snapshot.Tags))
	}
	counts := map[string]int{}
	for _, category := range snapshot.Categories {
		counts[category.TransactionType]++
	}
	if counts["expense"] != 9 || counts["income"] != 8 || counts["transfer"] != 2 {
		t.Fatalf("category types = %v", counts)
	}
	if _, err := workspace.CreateLedger(context.Background(), "  ", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty name error = %v", err)
	}
}

func TestRecordCRUDTagsAndRange(t *testing.T) {
	workspace := testWorkspace(t)
	home := testLedger(t, workspace)
	travel, err := workspace.CreateLedger(context.Background(), "旅行账本", "")
	if err != nil {
		t.Fatalf("travel ledger: %v", err)
	}
	ctx := context.Background()

	at := day(2026, time.November, 20, 12, 0, 0)
	record := domain.TransactionRecord{
		TransactionID:   "rec-1",
		LedgerID:        home.ID,
		AmountCents:     3200,
		TransactionType: "expense",
		Category:        "餐饮美食",
		Description:     "午餐",
		Flags:           `{"outlier":false}`,
		KeyEventDate:    "2026-11-20",
		TransactionAt:   at,
	}
	id, err := workspace.CreateRecord(ctx, record, []string{"三餐", "外卖", "三餐"})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if id != "rec-1" {
		t.Fatalf("id = %q", id)
	}
	entry, err := workspace.Record(ctx, id)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if entry.Record.Description != "午餐" || entry.Record.KeyEventDate != "2026-11-20" {
		t.Fatalf("record = %+v", entry.Record)
	}
	if len(entry.Tags) != 2 || entry.Tags[0] != "三餐" || entry.Tags[1] != "外卖" {
		t.Fatalf("tags = %v", entry.Tags)
	}

	// The end date includes 23:59:59 and the next day is outside the range.
	if _, err := workspace.CreateRecord(ctx, domain.TransactionRecord{
		TransactionID: "rec-2", LedgerID: home.ID, AmountCents: 100, TransactionType: "expense",
		Category: "餐饮美食", TransactionAt: day(2026, time.November, 20, 23, 59, 59),
	}, nil); err != nil {
		t.Fatalf("end-of-day record: %v", err)
	}
	if _, err := workspace.CreateRecord(ctx, domain.TransactionRecord{
		TransactionID: "rec-3", LedgerID: home.ID, AmountCents: 200, TransactionType: "income",
		Category: "工资奖金", TransactionAt: day(2026, time.November, 21, 0, 0, 0),
	}, nil); err != nil {
		t.Fatalf("next-day record: %v", err)
	}
	entries, err := workspace.Records(ctx, repository.RecordQuery{
		LedgerID: home.ID, StartDate: "2026-11-20", EndDate: "2026-11-20",
	})
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("range records = %d, want 2", len(entries))
	}
	if entries[0].Record.TransactionID != "rec-2" {
		t.Fatalf("order = %v", entries)
	}

	// Updating keeps the id and the original created_at, and replaces tags.
	before := entry.Record.CreatedAt
	record.TransactionID = id
	record.Description, record.Category = "晚餐", "购物消费"
	if err := workspace.UpdateRecord(ctx, record, []string{"衣物"}); err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	updated, err := workspace.Record(ctx, id)
	if err != nil {
		t.Fatalf("Record after update: %v", err)
	}
	if updated.Record.Description != "晚餐" || updated.Record.Category != "购物消费" {
		t.Fatalf("updated = %+v", updated.Record)
	}
	if updated.Record.CreatedAt != before {
		t.Fatalf("created_at = %d, want %d", updated.Record.CreatedAt, before)
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "衣物" {
		t.Fatalf("updated tags = %v", updated.Tags)
	}

	// Syncing copies fields and tags with a new id, without the event date.
	if err := workspace.SyncRecord(ctx, id, travel.ID); err != nil {
		t.Fatalf("SyncRecord: %v", err)
	}
	travelEntries, err := workspace.Records(ctx, repository.RecordQuery{LedgerID: travel.ID})
	if err != nil {
		t.Fatalf("travel records: %v", err)
	}
	if len(travelEntries) != 1 {
		t.Fatalf("travel records = %d", len(travelEntries))
	}
	copied := travelEntries[0]
	if copied.Record.TransactionID == id || copied.Record.LedgerID != travel.ID || copied.Record.KeyEventDate != "" {
		t.Fatalf("copied = %+v", copied)
	}
	if len(copied.Tags) != 1 || copied.Tags[0] != "衣物" {
		t.Fatalf("copied tags = %v", copied.Tags)
	}
	if err := workspace.SyncRecord(ctx, id, home.ID); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("sync to same ledger error = %v", err)
	}

	if err := workspace.DeleteRecord(ctx, id); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	if _, err := workspace.Record(ctx, id); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("deleted Record error = %v", err)
	}
	// A delete is idempotent.
	if err := workspace.DeleteRecord(ctx, id); err != nil {
		t.Fatalf("second DeleteRecord: %v", err)
	}
}

func TestCategoryAndTagDeletes(t *testing.T) {
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)
	ctx := context.Background()

	if err := workspace.CreateCategory(ctx, ledger.ID, "餐饮美食", "expense"); !errors.Is(err, repository.ErrDuplicate) {
		t.Fatalf("duplicate category error = %v", err)
	}
	if err := workspace.CreateCategory(ctx, ledger.ID, "自定义分类", "expense"); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if err := workspace.CreateTag(ctx, ledger.ID, "三餐", "餐饮美食:expense"); !errors.Is(err, repository.ErrDuplicate) {
		t.Fatalf("duplicate tag error = %v", err)
	}
	if err := workspace.CreateTag(ctx, ledger.ID, "深夜食堂", "自定义分类:expense"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	at := day(2026, time.November, 20, 12, 0, 0)
	id, err := workspace.CreateRecord(ctx, domain.TransactionRecord{
		TransactionID: "tagged", LedgerID: ledger.ID, AmountCents: 100, TransactionType: "expense",
		Category: "餐饮美食", TransactionAt: at,
	}, []string{"三餐"})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if err := workspace.DeleteTag(ctx, ledger.ID, "三餐", "餐饮美食:expense"); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	entry, err := workspace.Record(ctx, id)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(entry.Tags) != 0 {
		t.Fatalf("tags after DeleteTag = %v", entry.Tags)
	}

	if err := workspace.DeleteCategory(ctx, ledger.ID, "餐饮美食", "expense"); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	snapshot, err := workspace.Labels(ctx, ledger.ID)
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	for _, category := range snapshot.Categories {
		if category.Name == "餐饮美食" {
			t.Fatal("category survived delete")
		}
	}
	for _, tag := range snapshot.Tags {
		if tag.CategoryTransactionType == "餐饮美食:expense" {
			t.Fatalf("tag survived category delete: %v", tag)
		}
	}

	if err := workspace.ReorderCategories(ctx, ledger.ID, "expense", []string{"教育学习", "餐饮美食", "购物消费"}); err != nil {
		t.Fatalf("ReorderCategories: %v", err)
	}
	if err := workspace.ReorderTags(ctx, ledger.ID, "自定义分类:expense", []string{"深夜食堂"}); err != nil {
		t.Fatalf("ReorderTags: %v", err)
	}
}

func TestTemplatesAndReorder(t *testing.T) {
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)
	ctx := context.Background()

	first, err := workspace.CreateTemplate(ctx, domain.TransactionTemplate{
		LedgerID: ledger.ID, TemplateName: "午餐", TransactionType: "expense", Category: "餐饮美食",
		Tags: `["三餐"]`, Flags: "", Description: "",
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	second, err := workspace.CreateTemplate(ctx, domain.TransactionTemplate{
		LedgerID: ledger.ID, TemplateName: "工资", TransactionType: "income", Category: "工资奖金",
		Tags: `[]`, Flags: "outlier",
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	templates, err := workspace.Templates(ctx, ledger.ID)
	if err != nil {
		t.Fatalf("Templates: %v", err)
	}
	if len(templates) != 2 || templates[0].TemplateID != first || templates[1].TemplateID != second {
		t.Fatalf("templates = %+v", templates)
	}
	if templates[1].Flags != "outlier" {
		t.Fatalf("flags = %q", templates[1].Flags)
	}
	if err := workspace.ReorderTemplates(ctx, ledger.ID, []string{second, first}); err != nil {
		t.Fatalf("ReorderTemplates: %v", err)
	}
	templates, err = workspace.Templates(ctx, ledger.ID)
	if err != nil {
		t.Fatalf("Templates after reorder: %v", err)
	}
	if templates[0].TemplateID != second {
		t.Fatalf("reordered = %+v", templates)
	}
	if err := workspace.DeleteTemplate(ctx, first); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	templates, err = workspace.Templates(ctx, ledger.ID)
	if err != nil || len(templates) != 1 {
		t.Fatalf("templates after delete = %+v, err=%v", templates, err)
	}
}

func TestMigrationBackupRollbackAndVersionChecks(t *testing.T) {
	workspace := testWorkspace(t)
	path := workspace.DatabasePath()

	bad := []migration{
		{version: 1, name: "init", sql: initSchema},
		{version: 2, name: "broken", sql: "CREATE TABLE broken (id INTEGER); INSERT INTO missing VALUES (1);"},
	}
	if err := applyMigrations(workspace.db, path, bad, 1); err == nil {
		t.Fatal("broken migration succeeded")
	}
	version, _, err := schemaVersion(workspace.db)
	if err != nil || version != 1 {
		t.Fatalf("version after rollback = %d, err=%v", version, err)
	}

	upgrade := []migration{
		{version: 1, name: "init", sql: initSchema},
		{version: 2, name: "ledger_note", sql: "ALTER TABLE ledger ADD COLUMN note TEXT NOT NULL DEFAULT '';"},
	}
	if err := applyMigrations(workspace.db, path, upgrade, 1); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("upgrade backup: %v", err)
	}
	version, _, err = schemaVersion(workspace.db)
	if err != nil || version != 2 {
		t.Fatalf("version after upgrade = %d, err=%v", version, err)
	}
	if err := applyMigrations(workspace.db, path, upgrade, 3); !errors.Is(err, ErrVersionTooNew) {
		t.Fatalf("version too new error = %v", err)
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	directory := t.TempDir()
	workspace, err := Open(directory)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := workspace.db.Exec("UPDATE app_meta SET value = '99' WHERE key = 'schema_version'"); err != nil {
		t.Fatalf("bump version: %v", err)
	}
	if err := workspace.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := Open(directory); !errors.Is(err, ErrVersionTooNew) {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenRejectsBadWorkspaces(t *testing.T) {
	t.Run("not a database", func(t *testing.T) {
		directory := t.TempDir()
		if err := os.WriteFile(filepath.Join(directory, "transactions.db"), []byte("not sqlite"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(directory); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("foreign database", func(t *testing.T) {
		directory := t.TempDir()
		db, err := sql.Open("sqlite", filepath.Join(directory, "transactions.db"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("CREATE TABLE other (id INTEGER)"); err != nil {
			t.Fatal(err)
		}
		db.Close()
		if _, err := Open(directory); !errors.Is(err, ErrNotWorkspace) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("legacy database", func(t *testing.T) {
		directory := t.TempDir()
		db, err := sql.Open("sqlite", filepath.Join(directory, "transactions.db"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("CREATE TABLE tbl_billadm_schema_migration (version INTEGER)"); err != nil {
			t.Fatal(err)
		}
		db.Close()
		if _, err := Open(directory); !errors.Is(err, ErrLegacyWorkspace) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("old accounting database", func(t *testing.T) {
		directory := t.TempDir()
		db, err := sql.Open("sqlite", filepath.Join(directory, "transactions.db"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(initSchema); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("CREATE TABLE stock_account (id TEXT)"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO app_meta(key, value) VALUES ('schema_version', '12')"); err != nil {
			t.Fatal(err)
		}
		db.Close()
		if _, err := Open(directory); !errors.Is(err, ErrOldWorkspace) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing directory", func(t *testing.T) {
		if _, err := Open(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, ErrWorkspaceMissing) {
			t.Fatalf("error = %v", err)
		}
	})
}
