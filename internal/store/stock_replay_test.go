package store

import (
	"context"
	"errors"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func TestUpdateStockTradeRecomputesWholeOrder(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	trades, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "600000", StockName: "浦发银行", TradeType: domain.TradeOpen,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.UpdateStockTrade(ctx, repository.StockTradeUpdate{
		AccountID: account.ID, TradeID: trades[0].ID, PriceMilli: 6_000, Lots: 2,
	}); err != nil {
		t.Fatalf("UpdateStockTrade: %v", err)
	}
	db, _ := workspace.dbHandle()
	var quantity int
	var cost int64
	if err := db.QueryRowContext(ctx,
		"SELECT quantity, total_cost_cents FROM stock_position WHERE account_id = ? AND stock_code = '600000'",
		account.ID).Scan(&quantity, &cost); err != nil {
		t.Fatal(err)
	}
	if quantity != 200 || cost != 120_501 {
		t.Fatalf("position = %d shares, %d cost", quantity, cost)
	}
	summary, _ := workspace.StockAccountSummary(ctx, account.ID)
	if summary.AvailableCash != -20_501 {
		t.Fatalf("available cash = %d", summary.AvailableCash)
	}
}

func TestDeleteStockOrderRemovesRoundAndRebuilds(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行", TradeType: domain.TradeOpen,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	closeTrades, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行", TradeType: domain.TradeClose,
		Fills: []repository.StockFill{{PriceMilli: 6_000, Lots: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.DeleteStockOrder(ctx, account.ID, closeTrades[0].OrderID); err != nil {
		t.Fatalf("DeleteStockOrder: %v", err)
	}
	db, _ := workspace.dbHandle()
	var rounds, histories int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_trade_round WHERE account_id = ?", account.ID).Scan(&rounds); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_trade_history WHERE account_id = ?", account.ID).Scan(&histories); err != nil {
		t.Fatal(err)
	}
	if rounds != 0 || histories != 0 {
		t.Fatalf("rounds=%d histories=%d after deleting close", rounds, histories)
	}
	var quantity int
	if err := db.QueryRowContext(ctx, "SELECT quantity FROM stock_position WHERE account_id = ? AND stock_code = '000001'", account.ID).Scan(&quantity); err != nil {
		t.Fatal(err)
	}
	if quantity != 100 {
		t.Fatalf("quantity after deleting close = %d", quantity)
	}
	var sells int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_fund_record WHERE account_id = ? AND event_type = 'sell'", account.ID).Scan(&sells); err != nil {
		t.Fatal(err)
	}
	if sells != 0 {
		t.Fatalf("sell fund records = %d", sells)
	}
}

func TestRollbackLatestOrderAndFund(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行", TradeType: domain.TradeOpen,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	outcome, err := workspace.RollbackStockOperation(ctx, account.ID)
	if err != nil {
		t.Fatalf("rollback order: %v", err)
	}
	if outcome.Action != "建仓" || outcome.Skipped {
		t.Fatalf("outcome = %+v", outcome)
	}
	db, _ := workspace.dbHandle()
	var trades int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_trade WHERE account_id = ?", account.ID).Scan(&trades); err != nil {
		t.Fatal(err)
	}
	if trades != 0 {
		t.Fatalf("trades after rollback = %d", trades)
	}
	summary, _ := workspace.StockAccountSummary(ctx, account.ID)
	if summary.AvailableCash != 100_000 {
		t.Fatalf("cash after rollback = %d", summary.AvailableCash)
	}

	outcome, err = workspace.RollbackStockOperation(ctx, account.ID)
	if err != nil {
		t.Fatalf("rollback fund: %v", err)
	}
	if outcome.Action != "追加本金" || outcome.Skipped {
		t.Fatalf("fund outcome = %+v", outcome)
	}
	summary, _ = workspace.StockAccountSummary(ctx, account.ID)
	if summary.Principal != 0 || summary.AvailableCash != 0 {
		t.Fatalf("summary after fund rollback = %+v", summary)
	}
	if _, err := workspace.RollbackStockOperation(ctx, account.ID); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty rollback error = %v", err)
	}
}
