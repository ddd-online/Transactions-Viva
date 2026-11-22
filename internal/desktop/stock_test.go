package desktop

import (
	"testing"
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
	click(t, driver, "追加")
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
	click(t, driver, "股票账户")
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
}
