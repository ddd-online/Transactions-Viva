package store

import (
	"context"
	"errors"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func closeStockRound(t *testing.T, workspace *Workspace, accountID, code, name string, openPrice, closePrice int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: accountID, StockCode: code, StockName: name, TradeType: domain.TradeOpen,
		Fills: []repository.StockFill{{PriceMilli: openPrice, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: accountID, StockCode: code, StockName: name, TradeType: domain.TradeClose,
		Fills: []repository.StockFill{{PriceMilli: closePrice, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStockStatisticsCumulativeAndGrouped(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 200_000)
	closeStockRound(t, workspace, account.ID, "000001", "平安银行", 5_000, 6_000)
	closeStockRound(t, workspace, account.ID, "000001", "平安银行", 5_000, 4_000)

	stats, err := workspace.StockStatistics(ctx, domain.StockStatisticsQuery{
		AccountID: account.ID, Strategy: domain.StockStatisticsCumulative, GroupSize: 5,
	})
	if err != nil {
		t.Fatalf("StockStatistics: %v", err)
	}
	if stats.RoundCount != 2 || len(stats.Points) != 2 {
		t.Fatalf("stats = %+v", stats)
	}
	last := stats.Points[1]
	if last.TotalPnl != -2_050 || last.WinCount != 1 || last.LossCount != 1 || last.TieCount != 0 {
		t.Fatalf("last point = %+v", last)
	}
	if last.WinRate != 50 || last.AvgWin != 8_970 || last.AvgLoss != 11_020 {
		t.Fatalf("averages = %+v", last)
	}
	if last.PnlRatio == nil || *last.PnlRatio < 0.81 || *last.PnlRatio > 0.82 {
		t.Fatalf("pnl ratio = %v", last.PnlRatio)
	}
	if last.Expectancy != -1_025 || last.MaxDrawdown != 11_020 {
		t.Fatalf("expectancy/drawdown = %+v", last)
	}

	grouped, err := workspace.StockStatistics(ctx, domain.StockStatisticsQuery{
		AccountID: account.ID, Strategy: domain.StockStatisticsGrouped, GroupSize: 2,
	})
	if err != nil {
		t.Fatalf("grouped: %v", err)
	}
	if len(grouped.Groups) != 1 || grouped.Groups[0].Span != 2 || grouped.Groups[0].TotalPnl != -2_050 {
		t.Fatalf("groups = %+v", grouped.Groups)
	}
}

func TestStockResetAndArchive(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	closeStockRound(t, workspace, account.ID, "000001", "平安银行", 5_000, 6_000)

	archived, err := workspace.ArchiveStockAccount(ctx, account.ID, "归档账户")
	if err != nil {
		t.Fatalf("ArchiveStockAccount: %v", err)
	}
	if archived.Name != "归档账户" {
		t.Fatalf("archived = %+v", archived)
	}
	oldSummary, _ := workspace.StockAccountSummary(ctx, account.ID)
	if oldSummary.Principal != 0 || oldSummary.AvailableCash != 0 {
		t.Fatalf("old account summary = %+v", oldSummary)
	}
	newSummary, _ := workspace.StockAccountSummary(ctx, archived.ID)
	if newSummary.Principal != 100_000 {
		t.Fatalf("new account summary = %+v", newSummary)
	}
	positions, _ := workspace.StockPositions(ctx, archived.ID)
	if len(positions) != 1 {
		t.Fatalf("archived positions = %+v", positions)
	}

	if err := workspace.ResetStockAccount(ctx, archived.ID); err != nil {
		t.Fatalf("ResetStockAccount: %v", err)
	}
	summary, _ := workspace.StockAccountSummary(ctx, archived.ID)
	if summary.Principal != 0 || summary.AvailableCash != 0 {
		t.Fatalf("summary after reset = %+v", summary)
	}
	trades, _ := workspace.StockTrades(ctx, archived.ID)
	if len(trades) != 0 {
		t.Fatalf("trades after reset = %d", len(trades))
	}
}

func TestStockArchiveRefusesOpenRound(t *testing.T) {
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
	if _, err := workspace.ArchiveStockAccount(ctx, account.ID, "归档账户"); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("archive open round error = %v", err)
	}
}

func TestStockLadderCache(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	entries := []domain.StockLimitEntry{{
		StockCode: "600000", StockName: "浦发银行", Level: 1, FirstSeal: 92_500,
		Industry: "银行", PriceMilli: 11_000,
	}}
	if err := workspace.SaveStockLimitEntries(ctx, "2026-01-05", entries, 100); err != nil {
		t.Fatalf("SaveStockLimitEntries: %v", err)
	}
	day, err := workspace.StockLimitDay(ctx, "2026-01-05")
	if err != nil || day == nil || day.Status != "ok" || day.Count != 1 {
		t.Fatalf("day = %+v err=%v", day, err)
	}
	got, err := workspace.StockLimitEntries(ctx, "2026-01-05")
	if err != nil || len(got) != 1 || got[0].StockCode != "600000" {
		t.Fatalf("entries = %+v err=%v", got, err)
	}
	open := 6.0
	closePct := 3.0
	closePrice := int64(11_000)
	if err := workspace.SaveStockLimitQuotes(ctx, "2026-01-05", map[string]domain.StockLimitQuote{
		"600000": {OpenPct: &open, ClosePct: &closePct, ClosePriceMilli: &closePrice},
	}, 100); err != nil {
		t.Fatalf("SaveStockLimitQuotes: %v", err)
	}
	quotes, err := workspace.StockLimitQuotes(ctx, "2026-01-05")
	if err != nil || quotes["600000"].OpenPct == nil || *quotes["600000"].OpenPct != 6 {
		t.Fatalf("quotes = %+v err=%v", quotes, err)
	}
}
