package store

import (
	"context"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func TestArchiveTreatsSameSecondBuyBeforeClose(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	const at = int64(1_700_000_000)
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行",
		TradeType: domain.TradeOpen, TradeTime: at,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行",
		TradeType: domain.TradeClose, TradeTime: at,
		Fills: []repository.StockFill{{PriceMilli: 6_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.ArchiveStockAccount(ctx, account.ID, "同日归档"); err != nil {
		t.Fatalf("same-second close left an open round: %v", err)
	}
}
