package desktop

import "testing"

func TestStockSettingsMovedIntoTheSettingsPage(t *testing.T) {
	driver, model := open(t)
	if got := model.stock.Tabs(); len(got) != 6 {
		t.Fatalf("stock tabs = %v, want six", got)
	}
	click(t, driver, "设置")
	click(t, driver, "股票设置")
	if !shows(driver, "交易标签") || !shows(driver, "交易费用") ||
		!shows(driver, "最近操作") || !shows(driver, "归档到新账户") || !shows(driver, "重置股票数据") {
		t.Fatal("stock settings groups are missing")
	}
	if !shows(driver, "默认股票账户") || !shows(driver, "新增股票账户") {
		t.Fatal("stock account list is missing")
	}
	click(t, driver, "新增交易标签")
	driver.Type("低吸")
	click(t, driver, "添加标签")
	if !shows(driver, "低吸") {
		t.Fatal("added stock tag is not shown")
	}
	tags, _, _, _, err := model.stock.StockSettings()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tag := range tags {
		if tag == "低吸" {
			found = true
		}
	}
	if !found {
		t.Fatalf("stock tag was not stored: %v", tags)
	}
}
