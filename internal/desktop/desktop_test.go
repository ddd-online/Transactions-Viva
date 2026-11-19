package desktop

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// open runs the shell with no window: the real painter, input and focus rules.
func open(t *testing.T) (*app.Driver, *Model) {
	t.Helper()
	model := New()
	page := ui.NewPage(ui.PageConfig{Tree: &model.tree, View: model.page, Theme: &model.theme})
	driver, err := app.NewDriver(page.Config(newConfig(model)), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver, model
}

// click presses the middle of the control with the given name, as a user would.
func click(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q on screen", name)
}

func shows(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func hasControl(driver *app.Driver, name string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return true
		}
	}
	return false
}

func TestNavigationSwitchesSections(t *testing.T) {
	driver, _ := open(t)
	if !shows(driver, "记一笔") {
		t.Fatal("the accounting page is not the one shown first")
	}

	click(t, driver, "股票")
	if !shows(driver, "资金账户") {
		t.Fatal("股票 did not show the stock account screen")
	}
	if shows(driver, "记一笔") {
		t.Fatal("股票 left the accounting screen on screen")
	}

	click(t, driver, "待办")
	if !shows(driver, "四象限") {
		t.Fatal("待办 did not show the todo board screen")
	}

	click(t, driver, "日记")
	if !shows(driver, "日记目录树") {
		t.Fatal("日记 did not show the diary screen")
	}
}

func TestSubFunctionTabs(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "分析")
	if !shows(driver, "分组合计") {
		t.Fatal("the 分析 tab did not show its screen")
	}
	click(t, driver, "标签")
	if !shows(driver, "分类与标签") {
		t.Fatal("the 标签 tab did not show its screen")
	}
}

func TestRecordsComposer(t *testing.T) {
	t.Run("saves a mock row", func(t *testing.T) {
		driver, _ := open(t)
		click(t, driver, "记一笔")
		if err := driver.Type("88.50"); err != nil {
			t.Fatal(err)
		}
		click(t, driver, "分类")
		if err := driver.Type("测试分类"); err != nil {
			t.Fatal(err)
		}
		click(t, driver, "保存")
		if !shows(driver, "测试分类") {
			t.Fatal("the composer did not add its row")
		}
	})
}

func TestRecordsPagination(t *testing.T) {
	driver, _ := open(t)
	if !shows(driver, "共 5 条") {
		t.Fatal("the pager did not show the today count")
	}

	click(t, driver, "今天")
	click(t, driver, "全部时间")
	if !shows(driver, "共 12 条") {
		t.Fatal("all time did not leave all twelve rows")
	}
	if shows(driver, "超市采购") {
		t.Fatal("the first page showed a row from the second page")
	}

	click(t, driver, "下一页")
	if !shows(driver, "超市采购") {
		t.Fatal("下一页 did not show the second page")
	}
	click(t, driver, "上一页")
	if shows(driver, "超市采购") {
		t.Fatal("上一页 did not return to the first page")
	}
}

func TestSidebarExpandsOneSectionAtATime(t *testing.T) {
	driver, _ := open(t)

	// 记账 starts expanded, so its sub-functions are on the rail.
	if !hasControl(driver, "分析") {
		t.Fatal("记账 is not expanded on start")
	}

	click(t, driver, "股票")
	if !hasControl(driver, "持仓") {
		t.Fatal("股票 did not expand its sub-functions")
	}
	if hasControl(driver, "分析") {
		t.Fatal("股票 did not collapse 记账")
	}
	if !shows(driver, "资金账户") {
		t.Fatal("股票 did not show its account screen")
	}

	click(t, driver, "待办")
	if !hasControl(driver, "历史") {
		t.Fatal("待办 did not expand its sub-functions")
	}
	if hasControl(driver, "持仓") {
		t.Fatal("待办 did not collapse 股票")
	}

	// A section without sub-functions is a standalone item and closes the open
	// list.
	click(t, driver, "事件")
	if hasControl(driver, "历史") {
		t.Fatal("事件 did not collapse 待办")
	}
	if !shows(driver, "按日期的事件画廊") {
		t.Fatal("事件 did not show its screen")
	}

	click(t, driver, "记账")
	if !hasControl(driver, "分析") {
		t.Fatal("记账 did not expand again")
	}
	if !shows(driver, "记一笔") {
		t.Fatal("记账 did not return to its selected sub-function")
	}
}

func TestSettingsCategoriesSwitchThePage(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "应用设置")

	for _, name := range []string{"通用设置", "功能开关", "数据备份", "日记配置", "关于软件"} {
		if !hasControl(driver, name) {
			t.Fatalf("settings list has no %q entry", name)
		}
	}

	click(t, driver, "数据备份")
	if !shows(driver, "手动备份") || !shows(driver, "从备份恢复") {
		t.Fatal("数据备份 did not show its settings rows")
	}

	click(t, driver, "关于软件")
	if !shows(driver, "检查更新") {
		t.Fatal("关于软件 did not show its settings rows")
	}
}

func TestWindowBorderFollowsTheme(t *testing.T) {
	model := New()
	config := newConfig(model)
	if config.WindowBorderColor == nil {
		t.Fatal("the window has no border color")
	}
	light := config.WindowBorderColor()
	if light != config.ClearColor {
		t.Fatalf("border %v does not match the cleared edge %v", light, config.ClearColor)
	}
	model.toggleTheme()
	if dark := config.WindowBorderColor(); dark == light {
		t.Fatal("the window border did not follow the appearance toggle")
	}
}

func TestSettingsPageAndThemeToggle(t *testing.T) {
	driver, model := open(t)

	click(t, driver, "应用设置")
	if !shows(driver, "窗口关闭行为") {
		t.Fatal("应用设置 did not show the settings page")
	}
	if model.section != SectionSettings {
		t.Fatalf("section while settings is shown = %v", model.section)
	}

	click(t, driver, "功能开关")
	if !shows(driver, "五个开关") {
		t.Fatal("功能开关 did not switch the settings section")
	}

	// Settings is a section like any other: another section replaces it, and
	// the chosen settings section stays for the next visit.
	click(t, driver, "记账")
	if !shows(driver, "记一笔") {
		t.Fatal("记账 did not replace the settings page")
	}
	if model.section != SectionAccounting {
		t.Fatalf("section after leaving settings = %v", model.section)
	}
	click(t, driver, "应用设置")
	if !shows(driver, "五个开关") {
		t.Fatal("settings did not return to its chosen section")
	}

	dark := model.dark
	click(t, driver, "切换到深色模式")
	if !model.dark || model.dark == dark {
		t.Fatal("the appearance toggle did not switch the theme")
	}
	if !hasControl(driver, "切换到浅色模式") {
		t.Fatal("the toggle does not offer the light theme after switching")
	}
}
