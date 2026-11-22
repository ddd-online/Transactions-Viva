package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"transactions/internal/repository"
)

func TestStockAccountsCreateUpdateAndList(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	first, err := workspace.CreateStockAccount(ctx, " 主账户 ", " 长期 ")
	if err != nil {
		t.Fatalf("CreateStockAccount: %v", err)
	}
	if first.Name != "主账户" || first.Description != "长期" || first.SortOrder != 0 {
		t.Fatalf("account = %+v", first)
	}
	second, err := workspace.CreateStockAccount(ctx, "短线", "")
	if err != nil {
		t.Fatalf("CreateStockAccount: %v", err)
	}
	if second.SortOrder != 1 {
		t.Fatalf("second sort order = %d", second.SortOrder)
	}
	accounts, err := workspace.StockAccounts(ctx)
	if err != nil || len(accounts) != 2 || accounts[0].ID != first.ID || accounts[1].ID != second.ID {
		t.Fatalf("accounts = %+v, err = %v", accounts, err)
	}

	if _, err := workspace.CreateStockAccount(ctx, " ", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("blank name error = %v", err)
	}
	if _, err := workspace.CreateStockAccount(ctx, "主账户", ""); !errors.Is(err, repository.ErrDuplicate) {
		t.Fatalf("duplicate name error = %v", err)
	}
	if err := workspace.UpdateStockAccount(ctx, first.ID, "主账户", "改了描述"); err != nil {
		t.Fatalf("UpdateStockAccount: %v", err)
	}
	accounts, _ = workspace.StockAccounts(ctx)
	if accounts[0].Description != "改了描述" {
		t.Fatalf("updated account = %+v", accounts[0])
	}
	if err := workspace.UpdateStockAccount(ctx, second.ID, "主账户", ""); !errors.Is(err, repository.ErrDuplicate) {
		t.Fatalf("duplicate rename error = %v", err)
	}
	if err := workspace.UpdateStockAccount(ctx, "missing", "其他", ""); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("missing rename error = %v", err)
	}
}

func TestDeleteStockAccountCascadesAndKeepsTheLast(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	first, err := workspace.CreateStockAccount(ctx, "主账户", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := workspace.CreateStockAccount(ctx, "短线", "")
	if err != nil {
		t.Fatal(err)
	}
	db, err := workspace.dbHandle()
	if err != nil {
		t.Fatal(err)
	}
	now := nowUnix()
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO stock_fund_record (id, account_id, record_date, event_type, event_text, amount_change_cents, cash_balance_cents, remark, created_at) VALUES (?, ?, '2026-01-01', 'add_principal', '追加本金', 1000, 1000, '', ?)", []any{"f1", second.ID, now}},
		{"INSERT INTO stock_position (id, account_id, stock_code, stock_name, quantity, total_cost_cents, realized_pnl_cents, review, created_at, updated_at, tag) VALUES (?, ?, '600000', '浦发银行', 100, 500, 0, '', ?, ?, '')", []any{"p1", second.ID, now, now}},
		{"INSERT INTO stock_trade (id, account_id, stock_code, trade_type, created_at) VALUES (?, ?, '600000', 'open', ?)", []any{"t1", second.ID, now}},
		{"INSERT INTO stock_lesson (id, account_id, text, created_at, updated_at) VALUES (?, ?, '一句话', ?, ?)", []any{"l1", second.ID, now, now}},
	} {
		if _, err := db.ExecContext(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := workspace.DeleteStockAccount(ctx, second.ID); err != nil {
		t.Fatalf("DeleteStockAccount: %v", err)
	}
	for _, table := range stockAccountChildTables {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE account_id = ?", second.ID).Scan(&count); err != nil {
			t.Fatalf("%s count: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s still has %d rows", table, count)
		}
	}
	accounts, _ := workspace.StockAccounts(ctx)
	if len(accounts) != 1 || accounts[0].ID != first.ID {
		t.Fatalf("remaining accounts = %+v", accounts)
	}
	if err := workspace.DeleteStockAccount(ctx, first.ID); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("last account delete error = %v", err)
	}
}

func TestSchemaCreatesIdentityStockAndTodoTables(t *testing.T) {
	workspace := testWorkspace(t)
	db := workspace.db
	version, exists, err := schemaVersion(db)
	if err != nil || !exists || version != targetVersion() {
		t.Fatalf("version = %d exists=%v err=%v", version, exists, err)
	}
	if !hasAppID(db) {
		t.Fatal("app_id missing")
	}
	for _, table := range []string{
		"stock_account", "stock_fee_setting", "stock_fund_record", "stock_position",
		"stock_trade", "stock_trade_history", "stock_trade_round",
		"stock_trade_tag_setting", "stock_review_setting", "stock_operation",
		"stock_day_review", "stock_lesson", "stock_ledger",
		"stock_limit_day", "stock_limit_pool", "stock_limit_quote",
	} {
		ok, err := tableExists(db, table)
		if err != nil || !ok {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

func TestV6DatabaseIsNotMistakenForTheOldApplication(t *testing.T) {
	directory := t.TempDir()
	workspace, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	db := workspace.db
	if _, err := db.Exec("UPDATE app_meta SET value = '99' WHERE key = 'schema_version'"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(directory); !errors.Is(err, ErrVersionTooNew) {
		t.Fatalf("v6 database with a future version = %v", err)
	}
}

func TestV5DatabaseMigratesToStockAndTodoTables(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "transactions.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(initSchema); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{chartSchema, diarySchema, diaryBookSchema, keyEventSchema} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT OR REPLACE INTO app_meta(key, value) VALUES ('schema_version', '5')"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	workspace, err := Open(directory)
	if err != nil {
		t.Fatalf("Open v5: %v", err)
	}
	defer workspace.Close()
	version, _, _ := schemaVersion(workspace.db)
	if version != targetVersion() {
		t.Fatalf("migrated version = %d", version)
	}
	if !hasAppID(workspace.db) {
		t.Fatal("app_id missing after migration")
	}
	for _, table := range []string{"todo_card", "todo_item", "todo_progress"} {
		if ok, err := tableExists(workspace.db, table); err != nil || !ok {
			t.Fatalf("migrated table %s missing: %v", table, err)
		}
	}
}
