package desktop

import "testing"

func TestStockSettingsMovedIntoTheSettingsPage(t *testing.T) {
	driver, model := open(t)
	if got := model.stock.Tabs(); len(got) != 6 {
		t.Fatalf("stock tabs = %v, want six", got)
	}
	click(t, driver, "设置")
	click(t, driver, "股票设置")
	if !shows(driver, "交易标签") || !shows(driver, "交易费用设置") ||
		!shows(driver, "操作记录") || !shows(driver, "归档") || !shows(driver, "重置") {
		t.Fatal("stock settings groups are missing")
	}
	if !shows(driver, "设置作用账户") {
		t.Fatal("stock settings account row is missing")
	}
	click(t, driver, "新增交易标签")
	driver.Type("低吸")
	click(t, driver, "添加")
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
