package store

import (
	"context"
	"testing"
)

func TestRefreshStockNameKeepsHistoryOrder(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	closeStockRound(t, workspace, account.ID, "000001", "平安银行", 5_000, 6_000)
	db, _ := workspace.dbHandle()
	var before int64
	if err := db.QueryRowContext(ctx,
		"SELECT updated_at FROM stock_trade_history WHERE account_id = ? AND stock_code = '000001'",
		account.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := workspace.RefreshStockName(ctx, account.ID, "000001", "平安银行XD"); err != nil {
		t.Fatalf("RefreshStockName: %v", err)
	}
	var positionName, tradeName, historyName string
	if err := db.QueryRowContext(ctx,
		"SELECT stock_name FROM stock_position WHERE account_id = ? AND stock_code = '000001'",
		account.ID).Scan(&positionName); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx,
		"SELECT stock_name FROM stock_trade WHERE account_id = ? AND stock_code = '000001' LIMIT 1",
		account.ID).Scan(&tradeName); err != nil {
		t.Fatal(err)
	}
	var after int64
	if err := db.QueryRowContext(ctx,
		"SELECT stock_name, updated_at FROM stock_trade_history WHERE account_id = ? AND stock_code = '000001'",
		account.ID).Scan(&historyName, &after); err != nil {
		t.Fatal(err)
	}
	if positionName != "平安银行XD" || tradeName != "平安银行XD" || historyName != "平安银行XD" {
		t.Fatalf("names = %q %q %q", positionName, tradeName, historyName)
	}
	if after != before {
		t.Fatalf("history updated_at changed: %d -> %d", before, after)
	}
}
