package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
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
	if _, err := db.Exec(initSchema); err != nil {
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
		"INSERT INTO app_meta (key, value) VALUES ('schema_version', '12')",
		"CREATE TABLE stock_account (id TEXT)",
		"INSERT INTO stock_account (id) VALUES ('s1')",
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
		report.Records != 1 || report.RecordTags != 1 || report.Templates != 1 {
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
