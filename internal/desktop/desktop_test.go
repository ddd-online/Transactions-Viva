package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
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

// inDialog reports whether a control with the name sits in the dialog area,
// which starts under the toolbar.
func inDialog(driver *app.Driver, name string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && hit.Bounds.Y > 150 {
			return true
		}
	}
	return false
}

// clickLast presses the last control with the given name: a dialog paints after
// the page, so the field inside it is the last one.
func clickLast(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	var last *gfx.Hit
	for index := range driver.Snapshot().Hits {
		hit := driver.Snapshot().Hits[index]
		if hit.Label == name && !hit.IsLabel {
			last = &hit
		}
	}
	if last == nil {
		t.Fatalf("no control named %q on screen", name)
	}
	b := last.Bounds
	if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
		t.Fatal(err)
	}
}

// clickContains presses the middle of the first control whose name contains
// want, for a control whose name is its value.
func clickContains(t *testing.T, driver *app.Driver, want string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if strings.Contains(hit.Label, want) && !hit.IsLabel {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control containing %q on screen", want)
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

// rightClickText presses the secondary button on the first painted text that
// contains want, which opens the context menu of the row it belongs to.
func rightClickText(t *testing.T, driver *app.Driver, want string) {
	t.Helper()
	for _, label := range driver.Snapshot().Labels {
		if !strings.Contains(label.Value, want) {
			continue
		}
		b := label.Bounds
		if err := driver.ClickButton(b.X+b.Width/2, b.Y+b.Height/2, gfx.PointerButtonSecondary); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no text %q on screen", want)
}

func clickRole(t *testing.T, driver *app.Driver, label string, role gfx.Role) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != label || hit.Role != role {
			continue
		}
		b := hit.Bounds
		if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no %s named %q on screen", role, label)
}

func shows(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

// settle lets the window's motion finish, as a user waiting a moment would.
func settle(t *testing.T, driver *app.Driver) {
	t.Helper()
	if err := driver.Advance(300 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

// labelBoundsOf is where a text sits, failing when the page does not paint it.
func labelBoundsOf(t *testing.T, driver *app.Driver, value string) gfx.Rect {
	t.Helper()
	bounds, ok := labelBounds(driver, value)
	if !ok {
		t.Fatalf("no text %q on screen", value)
	}
	return bounds
}

// labelBounds is where the page paints a text, for a test that checks a control
// did not move it.
func labelBounds(driver *app.Driver, value string) (gfx.Rect, bool) {
	for _, label := range driver.Snapshot().Labels {
		if label.Value == value {
			return label.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// controlBounds is where a control named for assistive technology sits, for a
// test that checks what it shows.
func controlBounds(driver *app.Driver, name string) (gfx.Rect, bool) {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name {
			return hit.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// showsIn reports whether the text is painted inside a rectangle.
func showsIn(driver *app.Driver, value string, within gfx.Rect) bool {
	for _, label := range driver.Snapshot().Labels {
		b := label.Bounds
		if label.Value != value || b.X < within.X || b.Y < within.Y ||
			b.X+b.Width > within.X+within.Width || b.Y+b.Height > within.Y+within.Height {
			continue
		}
		return true
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

func TestRecordsPage(t *testing.T) {
	t.Run("marks show an outlier and a linked event", func(t *testing.T) {
		driver, _ := open(t)
		if !shows(driver, "离群值") {
			t.Fatal("the outlier mark is missing")
		}
		if !shows(driver, "★") {
			t.Fatal("the linked-event mark is missing")
		}
	})

	t.Run("saves a mock row", func(t *testing.T) {
		driver, _ := open(t)
		click(t, driver, "记一笔")
		click(t, driver, "金额")
		if err := driver.Type("88.50"); err != nil {
			t.Fatal(err)
		}
		click(t, driver, "分类")
		click(t, driver, "娱乐休闲")
		click(t, driver, "保存")
		if !shows(driver, "-88.50") {
			t.Fatal("the composer did not add its row")
		}
	})
}

// TestRecordCategorySelect checks the 分类 field is a single choice among the
// categories of the chosen type, and that a category already chosen survives a
// change of type.
func TestRecordCategorySelect(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "记一笔")
	click(t, driver, "分类")
	if !hasControl(driver, "餐饮美食") {
		t.Fatal("the 分类 list offers no 支出 category")
	}
	if hasControl(driver, "工资奖金") {
		t.Fatal("the 分类 list offered an 收入 category under 支出")
	}
	click(t, driver, "餐饮美食")
	field, ok := controlBounds(driver, "分类")
	if !ok || !showsIn(driver, "餐饮美食", field) {
		t.Fatal("the chosen category is not shown in the field")
	}

	click(t, driver, "收入")
	field, _ = controlBounds(driver, "分类")
	if !showsIn(driver, "餐饮美食", field) {
		t.Fatal("changing the type blanked the chosen category")
	}
	click(t, driver, "分类")
	if !hasControl(driver, "工资奖金") {
		t.Fatal("the 分类 list offers no 收入 category")
	}
	if !hasControl(driver, "餐饮美食") {
		t.Fatal("the kept category is missing from the list")
	}
}

func TestRecordTagsMultiSelect(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "记一笔")
	clickRole(t, driver, "标签", gfx.RoleCombobox)
	if !hasControl(driver, "日常") {
		t.Fatal("the tags multi-select did not open its options")
	}
	click(t, driver, "日常")
	if !shows(driver, "日常") {
		t.Fatal("the chosen tag is not shown")
	}
}

// TestRecordMarkSelectKeepsTheDialogStill guards the framework gap recorded in
// docs/architecture.md: the open list of a Select must not drag the dialog's
// fields along with its own slide.
func TestRecordMarkSelectKeepsTheDialogStill(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "记一笔")
	if err := driver.Advance(300 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	before, ok := labelBounds(driver, "取消")
	if !ok {
		t.Fatal("the dialog has no 取消")
	}
	click(t, driver, "选择标记")
	if !hasControl(driver, "离群值") {
		t.Fatal("the mark select did not open its options")
	}
	if after, _ := labelBounds(driver, "取消"); after != before {
		t.Fatalf("opening 标记 moved the dialog: 取消 %v -> %v", before, after)
	}
	click(t, driver, "离群值")
	if hasControl(driver, "离群值") {
		t.Fatal("choosing a mark left its list open")
	}
}

// TestRecordDatePicker drives the composer's date panel: it opens over the
// form, floats without moving it, writes the picked day and closes. The field
// is the same DatePicker as the toolbar's, fixed to days and without the grain
// choice and the period arrows.
func TestRecordDatePicker(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "记一笔")
	settle(t, driver)
	still := labelBoundsOf(t, driver, "取消")
	clickLast(t, driver, time.Now().Format("2006-01-02"))
	if !hasControl(driver, "关闭日期选择") {
		t.Fatal("the date panel did not open")
	}
	if inDialog(driver, "上一周期") || inDialog(driver, "月") {
		t.Fatal("a single date kept the range's grain choice or period arrows")
	}
	if moved := labelBoundsOf(t, driver, "取消"); moved != still {
		t.Fatalf("opening the panel moved the form: %v -> %v", still, moved)
	}
	click(t, driver, "15")
	want := time.Now().Format("2006-01") + "-15"
	if !shows(driver, want) {
		t.Fatalf("the date picker did not write %s", want)
	}
	if hasControl(driver, "关闭日期选择") {
		t.Fatal("picking a day left the panel open")
	}
}

// TestTimeRangePicker drives the toolbar's range picker: its presets, its
// period arrows and the grain choice.
func TestTimeRangePicker(t *testing.T) {
	driver, _ := open(t)
	settle(t, driver)
	still := labelBoundsOf(t, driver, "记录")
	click(t, driver, time.Now().Format("2006-01-02"))
	if !hasControl(driver, "关闭日期选择") {
		t.Fatal("the range panel did not open")
	}
	if moved := labelBoundsOf(t, driver, "记录"); moved != still {
		t.Fatalf("opening the range panel moved the page: %v -> %v", still, moved)
	}
	click(t, driver, "近 7 天")
	if !shows(driver, "共 11 条") {
		t.Fatal("近 7 天 did not leave the week's rows")
	}

	click(t, driver, "上一周期")
	if shows(driver, "共 11 条") {
		t.Fatal("上一周期 did not move the range")
	}

	clickContains(t, driver, " ~ ")
	click(t, driver, "月")
	if !shows(driver, time.Now().Format("2006-01")) {
		t.Fatal("the 月 grain did not show the month in the field")
	}
}

func TestRecordsRowContextMenu(t *testing.T) {
	driver, _ := open(t)
	rightClickText(t, driver, "早餐 · 豆浆油条")
	for _, item := range []string{"编辑记录", "同步到其他账本", "删除记录"} {
		if !hasControl(driver, item) {
			t.Fatalf("the row menu has no %q", item)
		}
	}

	click(t, driver, "删除记录")
	if !shows(driver, "删除这条消费记录？") {
		t.Fatal("删除记录 did not open the confirmation")
	}
	click(t, driver, "删除")
	if shows(driver, "早餐 · 豆浆油条") {
		t.Fatal("删除 left the row on the page")
	}
	if !shows(driver, "记录已删除") {
		t.Fatal("删除 did not report the result")
	}
}

func TestRecordsPagination(t *testing.T) {
	driver, _ := open(t)
	if !shows(driver, "共 5 条") {
		t.Fatal("the pager did not show the today count")
	}

	// Six months of records is more than one page of fifteen.
	click(t, driver, time.Now().Format("2006-01-02"))
	click(t, driver, "月")
	click(t, driver, time.Now().Format("2006-01"))
	click(t, driver, "近 6 个月")
	if !shows(driver, "共 19 条") {
		t.Fatal("近 6 个月 did not leave nineteen rows")
	}
	if shows(driver, "年终奖") {
		t.Fatal("the first page showed a row from the second page")
	}

	click(t, driver, "下一页")
	if !shows(driver, "年终奖") {
		t.Fatal("下一页 did not show the second page")
	}
	click(t, driver, "上一页")
	if shows(driver, "年终奖") {
		t.Fatal("上一页 did not return to the first page")
	}

	// The page-size Select changes how many rows one page holds.
	click(t, driver, "每页条数")
	click(t, driver, "30")
	if hasControl(driver, "下一页") {
		t.Fatal("30 per page should leave a single page for twenty rows")
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
