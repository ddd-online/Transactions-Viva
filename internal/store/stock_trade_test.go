package store

import (
	"context"
	"errors"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func seedStockCash(t *testing.T, workspace *Workspace, accountID string, cents int64) {
	t.Helper()
	if err := workspace.AddStockPrincipal(context.Background(), accountID, cents, ""); err != nil {
		t.Fatalf("AddStockPrincipal: %v", err)
	}
}

func TestCreateStockOrderBuyCloseAndRound(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)

	trades, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "600000", StockName: "浦发银行",
		TradeType: domain.TradeOpen, Tag: "打板",
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	})
	if err != nil {
		t.Fatalf("CreateStockOrder open: %v", err)
	}
	if len(trades) != 1 || trades[0].AmountCents != 50_000 || trades[0].FeeCents != 501 {
		t.Fatalf("open trades = %+v", trades)
	}
	summary, _ := workspace.StockAccountSummary(ctx, account.ID)
	if summary.PositionCost != 50_501 || summary.AvailableCash != 49_499 {
		t.Fatalf("summary after open = %+v", summary)
	}

	trades, err = workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "600000", StockName: "浦发银行",
		TradeType: domain.TradeClose, Tag: "打板",
		Fills: []repository.StockFill{{PriceMilli: 6_000, Lots: 1}},
	})
	if err != nil {
		t.Fatalf("CreateStockOrder close: %v", err)
	}
	if len(trades) != 1 || trades[0].RealizedPnl == nil || *trades[0].RealizedPnl != 8_968 {
		t.Fatalf("close trades = %+v", trades)
	}
	summary, _ = workspace.StockAccountSummary(ctx, account.ID)
	if summary.PositionCost != 0 || summary.AvailableCash != 108_968 {
		t.Fatalf("summary after close = %+v", summary)
	}

	var rounds int
	var roundTag string
	var roundReview string
	db, _ := workspace.dbHandle()
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*), COALESCE(MAX(tag), ''), COALESCE(MAX(review), '') FROM stock_trade_round WHERE account_id = ?",
		account.ID).Scan(&rounds, &roundTag, &roundReview); err != nil {
		t.Fatal(err)
	}
	if rounds != 1 || roundTag != "打板" || roundReview != "" {
		t.Fatalf("round = %d tag=%q review=%q", rounds, roundTag, roundReview)
	}
	var positionTag string
	if err := db.QueryRowContext(ctx, "SELECT tag FROM stock_position WHERE account_id = ? AND stock_code = '600000'", account.ID).Scan(&positionTag); err != nil {
		t.Fatal(err)
	}
	if positionTag != "" {
		t.Fatalf("position tag after close = %q", positionTag)
	}
}

func TestCreateStockOrderRejectsOversell(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "600000", StockName: "浦发银行", TradeType: domain.TradeOpen,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "600000", StockName: "浦发银行", TradeType: domain.TradeReduce,
		Fills: []repository.StockFill{{PriceMilli: 6_000, Lots: 2}},
	})
	if !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("oversell error = %v", err)
	}
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "600000", StockName: "浦发银行", TradeType: domain.TradeClose,
		Fills: []repository.StockFill{{PriceMilli: 6_000, Lots: 1}},
	}); err != nil {
		t.Fatalf("close after failed reduce: %v", err)
	}
}

func TestCreateStockOrderArchiveOnly(t *testing.T) {
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
	// Reduce all shares without closing: the round stays open at 0 shares.
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行", TradeType: domain.TradeReduce,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	trades, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行", TradeType: domain.TradeClose,
	})
	if err != nil {
		t.Fatalf("archive-only close: %v", err)
	}
	if len(trades) != 1 || trades[0].Shares != 0 || trades[0].AmountCents != 0 {
		t.Fatalf("archive marker = %+v", trades)
	}
	var rounds int
	db, _ := workspace.dbHandle()
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_trade_round WHERE account_id = ?", account.ID).Scan(&rounds); err != nil {
		t.Fatal(err)
	}
	if rounds != 1 {
		t.Fatalf("rounds = %d", rounds)
	}
}
