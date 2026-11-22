package desktop

import (
	"context"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func TestStockAccountSwitchChangesEveryTab(t *testing.T) {
	driver, model := open(t)
	ctx := context.Background()
	accountID := model.stock.ActiveAccount()
	if accountID == "" {
		t.Fatal("no active account")
	}
	if err := model.workspace.AddStockPrincipal(ctx, accountID, 100_000, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := model.workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: accountID, StockCode: "600000", StockName: "浦发银行", TradeType: domain.TradeOpen,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "股票")
	click(t, driver, "持仓")
	if !shows(driver, "浦发银行") {
		t.Fatal("first account position missing")
	}

	if err := model.stock.CreateAccount("短线", ""); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "账户")
	click(t, driver, "持仓")
	if shows(driver, "浦发银行") {
		t.Fatal("second account shows the first account's position")
	}
	if !shows(driver, "当前没有持仓") {
		t.Fatal("second account has no empty state")
	}

	model.stock.SetAccount(accountID)
	click(t, driver, "账户")
	click(t, driver, "持仓")
	if !shows(driver, "浦发银行") {
		t.Fatal("switching back lost the position")
	}
}
