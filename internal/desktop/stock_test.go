package desktop

import (
	"context"
	"testing"
	"time"

	"transactions/internal/domain"
	"transactions/internal/repository"
	"transactions/internal/store/storetest"
)

func TestStockAccountPageAddsPrincipal(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "股票")
	if model.stock.ActiveAccount() == "" {
		t.Fatal("stock page has no active account")
	}
	if !shows(driver, "总资产") || !shows(driver, "默认股票账户") {
		t.Fatal("stock account screen did not paint")
	}
	click(t, driver, "追加本金")
	if !shows(driver, "金额（元）") {
		t.Fatal("amount dialog did not open")
	}
	clickLast(t, driver, "金额（元）")
	driver.Type("1000")
	clickLast(t, driver, "追加本金")
	if !shows(driver, "追加本金已记录") {
		t.Fatal("principal toast missing")
	}
	if !shows(driver, "¥1000.00") {
		t.Fatal("principal not reflected on the page")
	}
}

func TestSettingsManagesStockAccounts(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "设置")
	click(t, driver, "股票设置")
	if !shows(driver, "默认股票账户") {
		t.Fatal("default stock account missing")
	}
	click(t, driver, "新增股票账户")
	clickLast(t, driver, "名称")
	driver.Type("短线")
	click(t, driver, "创建")
	if !shows(driver, "短线") {
		t.Fatal("created stock account missing")
	}
	if len(model.stock.Accounts()) != 2 {
		t.Fatalf("accounts = %d", len(model.stock.Accounts()))
	}
	if model.stock.ActiveAccount() == "" {
		t.Fatal("created account was not selected")
	}
	var defaultID string
	for _, account := range model.stock.Accounts() {
		if account.Name == "默认股票账户" {
			defaultID = account.ID
		}
	}
	click(t, driver, "设为当前")
	if model.stock.ActiveAccount() != defaultID {
		t.Fatalf("active account = %s, want %s", model.stock.ActiveAccount(), defaultID)
	}
}

// TestLessonStockFieldSearches checks the 心得 form's stock field is a Combobox,
// not a menu: an account can have dozens of stocks, so the choices are searched
// by name or code and the list stays a few rows tall instead of growing past
// the dialog.
func TestLessonStockFieldSearches(t *testing.T) {
	fixture := storetest.Open(t)
	ctx := context.Background()
	created, err := fixture.Workspace.CreateStockAccount(ctx, "默认股票账户", "")
	if err != nil {
		t.Fatal(err)
	}
	account := created.ID
	if err := fixture.Workspace.AddStockPrincipal(ctx, account, 100_000_00, domain.Today()); err != nil {
		t.Fatal(err)
	}
	fee := domain.RoundFeeOfSetting(domain.DefaultStockFeeSetting())
	for _, stock := range []struct{ code, name string }{
		{"600000", "浦发银行"},
		{"601398", "工商银行"},
	} {
		if _, err := fixture.Workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
			AccountID: account, StockCode: stock.code, StockName: stock.name,
			TradeType: domain.TradeOpen, TradeTime: time.Now().Unix(),
			Fills:    []repository.StockFill{{PriceMilli: 10_000, Lots: 1}},
			RoundFee: &fee,
		}); err != nil {
			t.Fatalf("seed %s: %v", stock.code, err)
		}
		if _, err := fixture.Workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
			AccountID: account, StockCode: stock.code, StockName: stock.name,
			TradeType: domain.TradeClose, TradeTime: time.Now().Unix() + 1,
			Fills: []repository.StockFill{{PriceMilli: 11_000, Lots: 1}},
		}); err != nil {
			t.Fatalf("close %s: %v", stock.code, err)
		}
	}
	model := New()
	if err := model.adoptWorkspace(fixture.Workspace); err != nil {
		t.Fatal(err)
	}
	driver, _ := openModel(t, model)
	click(t, driver, "股票")
	click(t, driver, "心得")
	click(t, driver, "新建心得")
	// The field is an input, not a menu: clicking it opens a searchable list.
	clickLast(t, driver, "股票")
	if !hasControl(driver, "浦发银行 600000") || !hasControl(driver, "工商银行 601398") {
		t.Fatal("the stock field did not list the account's stocks")
	}
	driver.Type("601398")
	if hasControl(driver, "浦发银行 600000") {
		t.Fatal("the stock field did not filter by the code typed into it")
	}
	click(t, driver, "工商银行 601398")
	if !shows(driver, "工商银行 601398") {
		t.Fatal("the chosen stock did not reach the field")
	}
}
