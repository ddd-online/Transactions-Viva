package store

import (
	"context"
	"errors"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func stockTestAccount(t *testing.T, workspace *Workspace) domain.StockAccount {
	t.Helper()
	account, err := workspace.CreateStockAccount(context.Background(), "主账户", "")
	if err != nil {
		t.Fatalf("CreateStockAccount: %v", err)
	}
	return account
}

func TestStockPrincipalWithdrawAndInterest(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)

	if err := workspace.AddStockPrincipal(ctx, account.ID, 100_000, ""); err != nil {
		t.Fatalf("AddStockPrincipal: %v", err)
	}
	summary, err := workspace.StockAccountSummary(ctx, account.ID)
	if err != nil {
		t.Fatalf("StockAccountSummary: %v", err)
	}
	if summary.Principal != 100_000 || summary.AvailableCash != 100_000 {
		t.Fatalf("summary after principal = %+v", summary)
	}

	if err := workspace.WithdrawStock(ctx, account.ID, 20_000, ""); err != nil {
		t.Fatalf("WithdrawStock: %v", err)
	}
	summary, _ = workspace.StockAccountSummary(ctx, account.ID)
	if summary.Principal != 100_000 || summary.AvailableCash != 80_000 || summary.WithdrawnTotal != 20_000 {
		t.Fatalf("summary after withdraw = %+v", summary)
	}

	if err := workspace.AddStockInterest(ctx, account.ID, 5_000, ""); err != nil {
		t.Fatalf("AddStockInterest: %v", err)
	}
	summary, _ = workspace.StockAccountSummary(ctx, account.ID)
	if summary.Principal != 100_000 || summary.AvailableCash != 85_000 || summary.InterestTotal != 5_000 {
		t.Fatalf("summary after interest = %+v", summary)
	}

	page, err := workspace.StockFundRecords(ctx, account.ID, 1, 10)
	if err != nil {
		t.Fatalf("StockFundRecords: %v", err)
	}
	if page.Total != 3 || len(page.Items) != 3 {
		t.Fatalf("fund page = %+v", page)
	}
	if page.Items[0].EventType != domain.StockEventInterestPrincipal {
		t.Fatalf("newest fund row = %+v", page.Items[0])
	}
	if page.Items[0].CashBalanceCents != 85_000 {
		t.Fatalf("latest balance = %d", page.Items[0].CashBalanceCents)
	}
}

func TestStockWithdrawLimit(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	if err := workspace.AddStockPrincipal(ctx, account.ID, 10_000, ""); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WithdrawStock(ctx, account.ID, 10_001, ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("over-withdraw error = %v", err)
	}
	if err := workspace.AddStockPrincipal(ctx, account.ID, -1, ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("negative principal error = %v", err)
	}
	if err := workspace.AddStockPrincipal(ctx, account.ID, 1, "2026/01/01"); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("bad date error = %v", err)
	}
}

func TestStockCashChainBackdatedAndClampedPaging(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	if err := workspace.AddStockPrincipal(ctx, account.ID, 10_000, "2026-02-01"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.AddStockInterest(ctx, account.ID, 1_000, "2026-01-01"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.AddStockInterest(ctx, account.ID, 2_000, "2026-03-01"); err != nil {
		t.Fatal(err)
	}
	page, err := workspace.StockFundRecords(ctx, account.ID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || page.PageSize != 10 || page.Total != 3 {
		t.Fatalf("clamped page = %+v", page)
	}
	// The backdated interest is first in the chain, so its balance is 1000;
	// principal is dated later, so the chain's final balance is 13000.
	var byDate = map[string]int64{}
	for _, row := range page.Items {
		byDate[row.RecordDate] = row.CashBalanceCents
	}
	if byDate["2026-01-01"] != 1_000 || byDate["2026-02-01"] != 11_000 || byDate["2026-03-01"] != 13_000 {
		t.Fatalf("chain balances = %+v", byDate)
	}
}

func TestStockOperationsKeepLatestTen(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	for i := 0; i < 12; i++ {
		if err := workspace.AddStockInterest(ctx, account.ID, 100, ""); err != nil {
			t.Fatal(err)
		}
	}
	operations, err := workspace.StockOperations(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 10 {
		t.Fatalf("operations = %d", len(operations))
	}
	if operations[0].Action != "利息归本" {
		t.Fatalf("newest operation = %+v", operations[0])
	}
}
