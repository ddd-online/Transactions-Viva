package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestImportOldStockMapsLedgersToAccounts(t *testing.T) {
	source := oldFixture(t)
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		"CREATE TABLE stock_fee_setting (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', commission_rate REAL NOT NULL DEFAULT 0.0002354, min_commission_cents INTEGER NOT NULL DEFAULT 500, stamp_duty_rate REAL NOT NULL DEFAULT 0.0005, transfer_fee_rate REAL NOT NULL DEFAULT 0.00001, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)",
		"INSERT INTO stock_fee_setting (id, ledger_id, created_at, updated_at) VALUES ('fee1', 'l1', 1, 1)",
		"CREATE TABLE stock_position (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', stock_code TEXT NOT NULL, stock_name TEXT NOT NULL DEFAULT '', quantity INTEGER NOT NULL DEFAULT 0, total_cost_cents INTEGER NOT NULL DEFAULT 0, realized_pnl_cents INTEGER NOT NULL DEFAULT 0, review TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, tag TEXT NOT NULL DEFAULT '')",
		"INSERT INTO stock_position (id, ledger_id, stock_code, stock_name, quantity, created_at, updated_at) VALUES ('p1', 'l1', '600000', '浦发银行', 100, 1, 1)",
		"INSERT INTO stock_position (id, ledger_id, stock_code, stock_name, quantity, created_at, updated_at) VALUES ('p2', 'l2', '000001', '平安银行', 200, 1, 1)",
		"CREATE TABLE stock_trade_history (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', stock_code TEXT NOT NULL, stock_name TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)",
		"INSERT INTO stock_trade_history (id, ledger_id, stock_code, stock_name, created_at, updated_at) VALUES ('h1', 'l1', '600000', '浦发银行', 1, 1)",
		"CREATE TABLE stock_trade_round (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', stock_code TEXT NOT NULL, history_id TEXT NOT NULL, round_no INTEGER NOT NULL, opened_at INTEGER NOT NULL DEFAULT 0, closed_at INTEGER NOT NULL DEFAULT 0, tag TEXT NOT NULL DEFAULT '分析', review TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL)",
		"INSERT INTO stock_trade_round (id, ledger_id, stock_code, history_id, round_no, created_at) VALUES ('r1', 'l1', '600000', 'h1', 1, 1)",
		"CREATE TABLE stock_trade (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', stock_code TEXT NOT NULL, stock_name TEXT NOT NULL DEFAULT '', trade_type TEXT NOT NULL DEFAULT '', round_id TEXT NOT NULL DEFAULT '', order_id TEXT NOT NULL DEFAULT '', order_seq INTEGER NOT NULL DEFAULT 1, price_milli INTEGER NOT NULL DEFAULT 0, lots INTEGER NOT NULL DEFAULT 0, shares INTEGER NOT NULL DEFAULT 0, amount_cents INTEGER NOT NULL DEFAULT 0, fee_cents INTEGER NOT NULL DEFAULT 0, commission_cents INTEGER NOT NULL DEFAULT 0, stamp_duty_cents INTEGER NOT NULL DEFAULT 0, transfer_fee_cents INTEGER NOT NULL DEFAULT 0, realized_pnl_cents INTEGER, trade_time INTEGER NOT NULL DEFAULT 0, remark TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, round_commission_rate REAL, round_min_commission_cents INTEGER, round_stamp_duty_rate REAL, round_transfer_fee_rate REAL)",
		"INSERT INTO stock_trade (id, ledger_id, stock_code, stock_name, round_id, order_id, price_milli, lots, shares, amount_cents, fee_cents, trade_time, created_at) VALUES ('t1', 'l1', '600000', '浦发银行', 'r1', 'o1', 5000, 1, 100, 50000, 501, 100, 1)",
		"CREATE TABLE stock_fund_record (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', record_date TEXT NOT NULL, event_type TEXT NOT NULL DEFAULT '', event_text TEXT NOT NULL DEFAULT '', amount_change_cents INTEGER NOT NULL DEFAULT 0, cash_balance_cents INTEGER NOT NULL DEFAULT 0, net_pnl_cents INTEGER, remark TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL)",
		"INSERT INTO stock_fund_record (id, ledger_id, record_date, event_type, event_text, amount_change_cents, cash_balance_cents, created_at) VALUES ('f1', 'l1', '2026-01-01', 'add_principal', '追加本金', 100000, 100000, 1)",
		"CREATE TABLE stock_lesson (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', text TEXT NOT NULL, round_id TEXT, stock_code TEXT NOT NULL DEFAULT '', stock_name TEXT NOT NULL DEFAULT '', round_no INTEGER NOT NULL DEFAULT 0, opened_at INTEGER NOT NULL DEFAULT 0, closed_at INTEGER NOT NULL DEFAULT 0, pnl_cents INTEGER NOT NULL DEFAULT 0, pnl_rate REAL NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)",
		"INSERT INTO stock_lesson (id, ledger_id, text, round_id, created_at, updated_at) VALUES ('l1', 'l1', '一句话', 'r1', 1, 1)",
		"CREATE TABLE stock_ledger (id TEXT PRIMARY KEY, ledger_id TEXT NOT NULL DEFAULT '', stock_code TEXT NOT NULL DEFAULT '', round_id TEXT, why_buy TEXT NOT NULL DEFAULT '', stop_loss TEXT NOT NULL DEFAULT '', take_profit TEXT NOT NULL DEFAULT '', followed_system TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)",
		"INSERT INTO stock_ledger (id, ledger_id, stock_code, round_id, why_buy, created_at, updated_at) VALUES ('le1', 'l1', '600000', 'r1', '趋势向上', 1, 1)",
		"CREATE TABLE stock_limit_day (trade_date TEXT PRIMARY KEY, status TEXT NOT NULL, count INTEGER NOT NULL DEFAULT 0, fetched_at INTEGER NOT NULL)",
		"INSERT INTO stock_limit_day (trade_date, status, count, fetched_at) VALUES ('2026-01-05', 'ok', 1, 1)",
		"CREATE TABLE stock_limit_pool (trade_date TEXT NOT NULL, stock_code TEXT NOT NULL, stock_name TEXT NOT NULL DEFAULT '', market INTEGER NOT NULL DEFAULT 0, price_milli INTEGER NOT NULL DEFAULT 0, pct REAL NOT NULL DEFAULT 0, amount_cents INTEGER NOT NULL DEFAULT 0, float_mv_cents INTEGER NOT NULL DEFAULT 0, turnover REAL NOT NULL DEFAULT 0, level INTEGER NOT NULL DEFAULT 1, first_seal INTEGER NOT NULL DEFAULT 0, last_seal INTEGER NOT NULL DEFAULT 0, broken_count INTEGER NOT NULL DEFAULT 0, seal_fund_cents INTEGER NOT NULL DEFAULT 0, industry TEXT NOT NULL DEFAULT '', stat_days INTEGER NOT NULL DEFAULT 0, stat_count INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (trade_date, stock_code))",
		"INSERT INTO stock_limit_pool (trade_date, stock_code, stock_name, level) VALUES ('2026-01-05', '600000', '浦发银行', 1)",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	db.Close()

	destination := filepath.Join(t.TempDir(), "imported")
	report, err := ImportOld(destination, source, false)
	if err != nil {
		t.Fatalf("ImportOld: %v", err)
	}
	if report.StockAccounts != 2 || report.StockPositions != 2 || report.StockTrades != 1 ||
		report.StockRounds != 1 || report.StockLessons != 1 || report.StockLadderDays != 1 {
		t.Fatalf("report = %+v", report)
	}
	workspace, err := Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Close()
	ctx := context.Background()
	accounts, err := workspace.StockAccounts(ctx)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("accounts = %+v err=%v", accounts, err)
	}
	byName := map[string]string{}
	for _, account := range accounts {
		byName[account.Name] = account.ID
	}
	if byName["家庭账本"] == "" || byName["旅行账本"] == "" {
		t.Fatalf("account names = %+v", accounts)
	}
	positions, _ := workspace.StockPositions(ctx, byName["家庭账本"])
	if len(positions) != 1 || positions[0].StockCode != "600000" {
		t.Fatalf("l1 positions = %+v", positions)
	}
	positions, _ = workspace.StockPositions(ctx, byName["旅行账本"])
	if len(positions) != 1 || positions[0].StockCode != "000001" {
		t.Fatalf("l2 positions = %+v", positions)
	}
}
