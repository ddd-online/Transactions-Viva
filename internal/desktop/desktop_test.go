package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/buildinfo"
	"transactions/internal/config"
	"transactions/internal/store/storetest"
)

// open runs the shell with no window: the real painter, input and focus rules.
func open(t *testing.T) (*app.Driver, *Model) {
	t.Helper()
	fixture := storetest.Open(t)
	model := New()
	if err := model.adoptWorkspace(fixture.Workspace); err != nil {
		t.Fatalf("adopt workspace: %v", err)
	}
	return openModel(t, model)
}

// openModel is open over an already configured model.
func openModel(t *testing.T, model *Model) (*app.Driver, *Model) {
	t.Helper()
	t.Cleanup(model.closeWorkspace)
	page := ui.NewPage(ui.PageConfig{Tree: &model.tree, View: model.page, Theme: &model.theme})
	driver, err := app.NewDriver(ui.Configure(page, newConfig(model)), nil)
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

func controlTop(driver *app.Driver, name string) float32 {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return hit.Bounds.Y
		}
	}
	return 0
}

func TestNavigationSwitchesSections(t *testing.T) {
	driver, _ := open(t)
	if !shows(driver, "记一笔") {
		t.Fatal("the accounting page is not the one shown first")
	}

	click(t, driver, "股票")
	if !shows(driver, "总资产") {
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
	if !shows(driver, "暂无日记") || !shows(driver, "今天") {
		t.Fatal("日记 did not show the diary screen")
	}
}

func TestSubFunctionTabs(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "分析")
	if !shows(driver, "名称") || !hasControl(driver, "新增图表") {
		t.Fatal("the 分析 tab did not show its screen")
	}
	click(t, driver, "标签")
	if !hasControl(driver, "新增分类") || !shows(driver, "餐饮美食") {
		t.Fatal("the 标签 tab did not show its screen")
	}
	click(t, driver, "模板")
	if !hasControl(driver, "新建模板") {
		t.Fatal("the 模板 tab did not show its screen")
	}
}

// TestTagsPage drives the 记账 · 标签 screen: the type switch, choosing a
// category, adding a category and a tag, the duplicate check and deletions.
func TestTagsPage(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "标签")
	if !hasControl(driver, "新增分类") || !hasControl(driver, "新增标签") {
		t.Fatal("the tags page has no add buttons")
	}
	if !shows(driver, "选择分类查看标签") {
		t.Fatal("the tag column does not ask for a category")
	}

	click(t, driver, "餐饮美食")
	if !shows(driver, "三餐") || !shows(driver, "外卖") {
		t.Fatal("choosing 餐饮美食 did not show its tags")
	}

	// Another type clears the chosen category and its tags.
	click(t, driver, "收入")
	if !shows(driver, "选择分类查看标签") || shows(driver, "三餐") {
		t.Fatal("switching to 收入 kept the expense category or its tags")
	}

	click(t, driver, "新增分类")
	click(t, driver, "分类名称")
	if err := driver.Type("宠物"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !hasControl(driver, "宠物") {
		t.Fatal("the new category is not in the list")
	}
	if !shows(driver, "暂无标签") {
		t.Fatal("the new category is not chosen")
	}

	click(t, driver, "新增标签")
	click(t, driver, "标签名称")
	if err := driver.Type("洗澡"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !hasControl(driver, "洗澡") {
		t.Fatal("the new tag is not in the list")
	}

	// A duplicate category name is refused in the dialog.
	click(t, driver, "新增分类")
	click(t, driver, "分类名称")
	if err := driver.Type("宠物"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !shows(driver, "该分类已存在") {
		t.Fatal("the duplicate category was accepted")
	}
	click(t, driver, "取消")

	hoverRow(t, driver, "洗澡")
	click(t, driver, "删除标签 洗澡")
	click(t, driver, "删除")
	if hasControl(driver, "洗澡") {
		t.Fatal("the tag was not deleted")
	}

	hoverRow(t, driver, "宠物")
	click(t, driver, "删除分类 宠物")
	click(t, driver, "删除")
	if hasControl(driver, "宠物") {
		t.Fatal("the category was not deleted")
	}
	if !shows(driver, "选择分类查看标签") {
		t.Fatal("deleting the chosen category did not clear the tag column")
	}
}

// hoverRow puts the pointer on the middle of the row named name: the row's
// actions, such as its delete key, appear only while the row is lit.
func hoverRow(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != name || hit.IsLabel {
			continue
		}
		b := hit.Bounds
		if err := driver.Move(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no row named %q", name)
}

// TestTemplatesPage drives the 记账 · 模板 screen: the table, the required
// fields of 新建模板, and the template the page creates being offered by the
// 记一笔 dialog (they share one store).
func TestTemplatesPage(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "模板")
	if !hasControl(driver, "新建模板") || !hasControl(driver, "删除模板 午餐 · 餐饮美食") {
		t.Fatal("the template table is missing")
	}
	if !shows(driver, "模板名称") || !shows(driver, "离群值") {
		t.Fatal("the table header or the 离群值 mark is missing")
	}

	click(t, driver, "新建模板")
	click(t, driver, "创建")
	if !shows(driver, "模板名称不能为空") {
		t.Fatal("an empty name was accepted")
	}
	click(t, driver, "模板名称")
	if err := driver.Type("咖啡外带"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !shows(driver, "分类不能为空") {
		t.Fatal("a missing category was accepted")
	}

	click(t, driver, "分类")
	click(t, driver, "餐饮美食")
	settle(t, driver)
	clickRole(t, driver, "标签", gfx.RoleCombobox)
	if !hasControl(driver, "外卖") {
		t.Fatal("the tags multi-select offers no category tag")
	}
	click(t, driver, "外卖")
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "描述")
	if err := driver.Type("外带咖啡"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !hasControl(driver, "删除模板 咖啡外带") {
		t.Fatal("the new template is not in the table")
	}

	// The 记一笔 dialog offers the template the page just stored.
	click(t, driver, "记录")
	click(t, driver, "记一笔")
	click(t, driver, "选择模板")
	click(t, driver, "咖啡外带")
	if !shows(driver, "外带咖啡") {
		t.Fatal("the template did not fill the description")
	}
	field, ok := controlBounds(driver, "分类")
	if !ok || !showsIn(driver, "餐饮美食", field) {
		t.Fatal("the template did not choose its category")
	}
	if !shows(driver, "外卖") {
		t.Fatal("the template did not choose its tag")
	}
	click(t, driver, "取消")

	click(t, driver, "模板")
	click(t, driver, "删除模板 咖啡外带")
	click(t, driver, "删除")
	if hasControl(driver, "删除模板 咖啡外带") {
		t.Fatal("the template was not deleted")
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
		click(t, driver, "创建")
		if !shows(driver, "-88.50") {
			t.Fatal("the composer did not add its row")
		}
	})
}

// TestRecordsComposerStartsInTheVisibleRange checks where 记一笔 puts its date:
// the page's time range decides, not the clock.
func TestRecordsComposerStartsInTheVisibleRange(t *testing.T) {
	driver, _ := open(t)
	now := time.Now()
	stamp := func(day time.Time) string { return day.Format("2006-01-02") }
	today := stamp(now)

	// A day the page was moved to: the dialog opens on that day, so a past
	// expense does not have to be re-dated by hand.
	click(t, driver, "今天")
	click(t, driver, "上一周期")
	yesterday := stamp(now.AddDate(0, 0, -1))
	click(t, driver, "记一笔")
	if !shows(driver, yesterday) || shows(driver, today) {
		t.Fatalf("the dialog did not start on %s", yesterday)
	}
	click(t, driver, "取消")

	// The whole current month: its last day has not arrived, so the dialog keeps
	// today instead of dating the record ahead of the calendar.
	click(t, driver, "今天")
	click(t, driver, "月")
	click(t, driver, "记一笔")
	if !shows(driver, today) {
		t.Fatal("the current month did not start the dialog on today")
	}
	click(t, driver, "取消")

	// A month that is over starts on its last day, the range's nearer end.
	click(t, driver, "上一周期")
	previousEnd := stamp(time.Date(now.Year(), now.Month(), 0, 0, 0, 0, 0, time.Local))
	click(t, driver, "记一笔")
	if !shows(driver, previousEnd) {
		t.Fatalf("the closed month did not start the dialog on %s", previousEnd)
	}
}

// TestRecordsLedgerSelect switches the toolbar's 账本 Select and expects the
// table, the statistics and the pager to follow the chosen ledger.
func TestRecordsLedgerSelect(t *testing.T) {
	driver, _ := open(t)
	if !shows(driver, "早餐 · 豆浆油条") {
		t.Fatal("the family ledger's rows are not on screen")
	}
	click(t, driver, "账本")
	click(t, driver, "旅行账本")
	if !shows(driver, "去程机票") {
		t.Fatal("旅行账本 did not show its rows")
	}
	if shows(driver, "早餐 · 豆浆油条") {
		t.Fatal("the family ledger's row stayed after switching")
	}
	if !shows(driver, "共 2 条") {
		t.Fatal("the travel ledger did not show its today count")
	}
}

// TestAccountingLedgerIsSharedAcrossPages switches the ledger on each accounting
// 记账 page and checks the other pages follow it.
func TestAccountingLedgerIsSharedAcrossPages(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "账本")
	click(t, driver, "旅行账本")
	if !shows(driver, "去程机票") {
		t.Fatal("旅行账本 did not show its rows")
	}

	// The tags page shows the same choice and lets it change for everyone.
	click(t, driver, "标签")
	if !shows(driver, "旅行账本") {
		t.Fatal("the tags page did not inherit 旅行账本")
	}
	click(t, driver, "账本")
	click(t, driver, "家庭账本")
	if shows(driver, "旅行账本") {
		t.Fatal("switching back to 家庭账本 left the old name on the tags page")
	}

	// The templates page follows, and each ledger has its own templates.
	click(t, driver, "模板")
	if !shows(driver, "午餐 · 餐饮美食") {
		t.Fatal("the templates page did not inherit 家庭账本")
	}
	click(t, driver, "账本")
	click(t, driver, "旅行账本")
	if !shows(driver, "机票 · 交通出行") {
		t.Fatal("旅行账本 did not show its templates")
	}
	if shows(driver, "午餐 · 餐饮美食") {
		t.Fatal("the family ledger's template stayed after switching")
	}

	// The records page sees the ledger chosen on the templates page.
	click(t, driver, "记录")
	if !shows(driver, "去程机票") {
		t.Fatal("the records page did not inherit 旅行账本")
	}

	// A switch while the tags page has a category chosen clears the choice.
	click(t, driver, "标签")
	click(t, driver, "餐饮美食")
	if !shows(driver, "三餐") {
		t.Fatal("the chosen category did not show its tags")
	}
	click(t, driver, "账本")
	click(t, driver, "家庭账本")
	if shows(driver, "三餐") || !shows(driver, "选择分类查看标签") {
		t.Fatal("the ledger switch did not clear the chosen category")
	}
}

// TestAccountingToolbarStartsAfterTheLedgerDivider checks the ledger selector's
// trailing divider sits between it and the page's first control, at the same
// distance on the records and tags pages.
func TestAccountingToolbarStartsAfterTheLedgerDivider(t *testing.T) {
	driver, _ := open(t)
	ledger, ok := controlBounds(driver, "账本")
	if !ok {
		t.Fatal("the records page has no 账本 selector")
	}
	today, ok := controlBounds(driver, "今天")
	if !ok {
		t.Fatal("the records page has no 今天 button")
	}
	recordsGap := today.X - ledger.Right()

	click(t, driver, "标签")
	ledger, ok = controlBounds(driver, "账本")
	if !ok {
		t.Fatal("the tags page has no 账本 selector")
	}
	expense, ok := controlBounds(driver, "支出")
	if !ok {
		t.Fatal("the tags page has no 支出 type card")
	}
	tagsGap := expense.X - ledger.Right()

	if diff := recordsGap - tagsGap; diff < -0.5 || diff > 0.5 {
		t.Fatalf("records gap %.2f does not match the tags gap %.2f", recordsGap, tagsGap)
	}
	// gap + 1px divider + gap.
	if tagsGap < 16.5 || tagsGap > 17.5 {
		t.Fatalf("the ledger selector is separated by %.2f, want the 8+1+8 divider", tagsGap)
	}
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
	click(t, driver, "分类")
	click(t, driver, "餐饮美食")
	settle(t, driver)
	clickRole(t, driver, "标签", gfx.RoleCombobox)
	if !hasControl(driver, "外卖") {
		t.Fatal("the tags multi-select did not offer the category's tags")
	}
	click(t, driver, "外卖")
	if !shows(driver, "外卖") {
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
	now := time.Now()
	click(t, driver, fmt.Sprintf("%d 年 %d 月 15 日", now.Year(), int(now.Month())))
	want := now.Format("2006-01") + "-15"
	if !shows(driver, want) {
		t.Fatalf("the date picker did not write %s", want)
	}
	if hasControl(driver, "关闭日期选择") {
		t.Fatal("picking a day left the panel open")
	}
}

// TestTimeRangePickerKeepsItsFieldWidth checks the trigger has one width at
// every grain, sized for the widest range text, so the toolbar does not reflow
// when the range or the grain changes.
func TestTimeRangePickerKeepsItsFieldWidth(t *testing.T) {
	driver, _ := open(t)
	settle(t, driver)
	day, ok := controlBounds(driver, time.Now().Format("2006-01-02"))
	if !ok {
		t.Fatal("the range field is missing")
	}
	click(t, driver, time.Now().Format("2006-01-02"))
	click(t, driver, "月")
	month, ok := controlBounds(driver, time.Now().Format("2006-01"))
	if !ok {
		t.Fatal("the month field is missing")
	}
	if day.Width != month.Width {
		t.Fatalf("the field changed width with the grain: %v -> %v", day.Width, month.Width)
	}
	if day.Width < 180 {
		t.Fatalf("the field is too narrow for a range: %v", day.Width)
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
	for _, item := range []string{"编辑记录", "复制到其他账本", "删除记录"} {
		if !hasControl(driver, item) {
			t.Fatalf("the row menu has no %q", item)
		}
	}

	click(t, driver, "删除记录")
	if !shows(driver, "删除这条记录？") {
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

// TestRecordsEmptyState checks the empty state is the theme's, with the page's
// actions under it.
func TestRecordsEmptyState(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, time.Now().Format("2006-01-02"))
	click(t, driver, "年")
	click(t, driver, time.Now().Format("2006"))
	click(t, driver, "2024 年") // the start of a range
	click(t, driver, "2024 年") // and its end
	if !shows(driver, "这个时间范围里没有记录") {
		t.Fatal("a range without records showed no empty state")
	}
	for _, action := range []string{"上月", "今年", "记一笔"} {
		if !hasControl(driver, action) {
			t.Fatalf("the empty state has no %q", action)
		}
	}
	click(t, driver, "上月")
	if shows(driver, "这个时间范围里没有记录") {
		t.Fatal("上月 did not leave the empty state")
	}
}

// TestRecordPanelsCloseOnNavigation checks a picker does not follow the user to
// another screen.
func TestRecordPanelsCloseOnNavigation(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, time.Now().Format("2006-01-02"))
	if !hasControl(driver, "关闭日期选择") {
		t.Fatal("the range panel did not open")
	}
	click(t, driver, "分析")
	click(t, driver, "记录")
	if hasControl(driver, "关闭日期选择") {
		t.Fatal("the range panel followed the user to another screen")
	}
}

// TestRecordsHeaderSort checks the date and amount column titles carry a square
// icon button that cycles through unsorted, descending and ascending, and that
// the columns sort in the order they are clicked.
func TestRecordsHeaderSort(t *testing.T) {
	driver, _ := open(t)
	if hasControl(driver, "排序") {
		t.Fatal("the toolbar still has the old 排序 button")
	}
	sortButton, ok := controlBounds(driver, "按金额排序")
	if !ok || sortButton.Width != 24 || sortButton.Height != 24 {
		t.Fatalf("the amount sort control is %v, want a 24x24 square button", sortButton)
	}
	above := func(first, second string) bool {
		return labelBoundsOf(t, driver, first).Y < labelBoundsOf(t, driver, second).Y
	}

	// The amount is the first column clicked, so it leads the sort; the date
	// joins it as the second key and does not change the order.
	click(t, driver, "按金额排序")
	if !above("工资到账", "社保代扣") {
		t.Fatal("the descending amount sort did not put the largest first")
	}
	if !above("地铁通勤", "早餐 · 豆浆油条") {
		t.Fatal("the descending amount sort put the larger expense first: -4 must sort above -12")
	}
	click(t, driver, "按日期排序")
	if !above("工资到账", "社保代扣") {
		t.Fatal("the second column took over from the first")
	}
	click(t, driver, "按金额排序")
	if !above("午餐 · 公司楼下", "地铁通勤") {
		t.Fatal("the ascending amount sort did not put the largest expense first")
	}
	click(t, driver, "按金额排序")
	if !above("社保代扣", "早餐 · 豆浆油条") {
		t.Fatal("turning the amount sort off did not leave the date descending")
	}
	click(t, driver, "按日期排序")
	if !above("早餐 · 豆浆油条", "社保代扣") {
		t.Fatal("the ascending date sort did not put the earliest first")
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
	if !shows(driver, "总资产") {
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
	if !hasControl(driver, "新增事件") {
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

// TestSidebarSubItemsShareTheirSectionRowTrailingEdge pins the highlight of a
// sub-function to its section row's trailing edge: the reference application
// indents a sub-function on the leading side only, so the highlighted rows read
// as one column instead of the sub-menu stopping short on the right.
func TestSidebarSubItemsShareTheirSectionRowTrailingEdge(t *testing.T) {
	driver, _ := open(t)
	row := func(name string) gfx.Rect {
		t.Helper()
		for _, hit := range driver.Snapshot().Hits {
			if hit.Label == name && hit.Role == gfx.RoleMenuItem {
				return hit.Bounds
			}
		}
		t.Fatalf("no navigation row named %q on screen", name)
		return gfx.Rect{}
	}
	section := row("记账")
	trailing := section.X + section.Width
	for _, sub := range []string{"记录", "分析", "标签", "模板"} {
		b := row(sub)
		if got := b.X + b.Width; got != trailing {
			t.Fatalf("sub-function %q ends at x=%v, its section row ends at x=%v", sub, got, trailing)
		}
		if b.X <= section.X {
			t.Fatalf("sub-function %q starts at x=%v, not indented from its section row at x=%v", sub, b.X, section.X)
		}
	}
}

func TestSettingsCategoriesSwitchThePage(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "设置")

	for _, name := range []string{"通用设置", "数据备份", "日记配置", "关于软件"} {
		if !hasControl(driver, name) {
			t.Fatalf("settings list has no %q entry", name)
		}
	}
	last := float32(-1)
	for _, name := range []string{"通用设置", "账本管理", "股票设置", "日记配置", "数据备份", "关于软件"} {
		top := controlTop(driver, name)
		if top <= last {
			t.Fatalf("settings section %q is out of order", name)
		}
		last = top
	}
	// 功能开关 is a group of 通用设置 now, not a section of the panel: the rail
	// has no row for it, and the page the settings open on shows both groups.
	if hasControl(driver, "功能开关") {
		t.Fatal("功能开关 is still a section of the settings rail")
	}
	for _, name := range []string{"常规", "功能开关"} {
		if !shows(driver, name) {
			t.Fatalf("通用设置 has no %q group", name)
		}
	}
	// The proxy belongs to updating, so it is on 关于软件 as 更新代理, not here.
	if shows(driver, "网络代理") {
		t.Fatal("网络代理 should have moved to 关于软件 as 更新代理")
	}

	click(t, driver, "数据备份")
	if !shows(driver, "手动备份") || !shows(driver, "从备份恢复") {
		t.Fatal("数据备份 did not show its settings rows")
	}

	click(t, driver, "关于软件")
	for _, name := range []string{"检查更新", "更新代理", "检测"} {
		if !shows(driver, name) {
			t.Fatalf("关于软件 did not show its %q row", name)
		}
	}
}

func TestWindowConfigUsesTheAppIcon(t *testing.T) {
	config := newConfig(New())
	if config.Icon.Size < 128 || len(config.Icon.RGBA) != config.Icon.Size*config.Icon.Size*4 {
		t.Fatalf("window icon = size %d, %d bytes", config.Icon.Size, len(config.Icon.RGBA))
	}
}

func TestWindowMinimumMatchesMotty(t *testing.T) {
	cfg := config.Default()
	cfg.Width, cfg.Height = 400, 300
	window := newConfig(NewWithConfig(cfg, nil))
	if window.MinWidth != 960 || window.MinHeight != 620 {
		t.Fatalf("minimum = %dx%d, want motty's 960x620", window.MinWidth, window.MinHeight)
	}
	if window.Width != 960 || window.Height != 620 {
		t.Fatalf("restored frame = %dx%d, want it clamped to the minimum", window.Width, window.Height)
	}
	defaults := newConfig(New())
	if defaults.Width != 1500 || defaults.Height != 1000 {
		t.Fatalf("default frame = %dx%d, want 1500x1000", defaults.Width, defaults.Height)
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

// TestWindowControlsFollowTheTheme guards the native window buttons: Viva reads
// WindowControls on every paint, so the ink has to be a function of the current
// theme. The viva dev host never receives this callback (see the framework gap
// section of docs/architecture.md) and picks the ink once from ClearColor.
func TestWindowControlsFollowTheTheme(t *testing.T) {
	model := New()
	config := newConfig(model)
	if config.WindowControls == nil {
		t.Fatal("the window has no controls style")
	}
	light := config.WindowControls().Foreground
	if want := model.theme.Foreground; light != want {
		t.Fatalf("light control ink = %v, want the theme's foreground %v", light, want)
	}
	model.toggleTheme()
	dark := config.WindowControls().Foreground
	if dark == light {
		t.Fatal("the window buttons did not follow the appearance toggle")
	}
	if want := model.theme.Foreground; dark != want {
		t.Fatalf("dark control ink = %v, want the theme's foreground %v", dark, want)
	}
}

// TestWindowControlsHoverReadsOnTheChrome guards the pointer feedback of the
// window buttons: they sit on the 36px title bar, whose tone is Muted. The
// surface palette's Hover and Secondary are that very tone in the light
// appearance, so an opaque fill left the buttons looking dead under the
// pointer until the close button (Danger, red) was reached.
func TestWindowControlsHoverReadsOnTheChrome(t *testing.T) {
	model := New()
	config := newConfig(model)
	distance := func(a, b gfx.Color) float32 {
		channel := func(x, y float32) float32 {
			if x < y {
				x, y = y, x
			}
			return x - y
		}
		return max(channel(a.Red, b.Red), max(channel(a.Green, b.Green), channel(a.Blue, b.Blue)))
	}
	for _, dark := range []bool{false, true} {
		if model.dark != dark {
			model.toggleTheme()
		}
		style := config.WindowControls()
		chrome := model.theme.Muted
		hover := over(style.Hover, chrome)
		pressed := over(style.Pressed, chrome)
		if got := distance(hover, chrome); got < 0.02 {
			t.Errorf("hover %v is %.3f from the chrome %v: the button gives no feedback", hover, got, chrome)
		}
		if got, want := distance(pressed, chrome), distance(hover, chrome); got <= want {
			t.Errorf("pressed %v is %.3f from the chrome, hover %.3f: pressed is not stronger", pressed, got, want)
		}
	}
}

func TestSettingsPageAndThemeToggle(t *testing.T) {
	driver, model := open(t)

	click(t, driver, "设置")
	if !hasControl(driver, "直接退出") {
		t.Fatal("设置 did not show the settings page")
	}
	if model.section != SectionSettings {
		t.Fatalf("section while settings is shown = %v", model.section)
	}

	// 功能开关 is 通用设置's second group: its switches are on the page the
	// settings open on, with no section of their own to visit.
	if !hasControl(driver, "记账功能开关") {
		t.Fatal("通用设置 does not show the feature switches")
	}

	// Settings is a section like any other: another section replaces it, and
	// the chosen settings section stays for the next visit.
	click(t, driver, "数据备份")
	if !shows(driver, "从备份恢复") {
		t.Fatal("数据备份 did not replace the general settings")
	}
	click(t, driver, "记账")
	if !shows(driver, "记一笔") {
		t.Fatal("记账 did not replace the settings page")
	}
	if model.section != SectionAccounting {
		t.Fatalf("section after leaving settings = %v", model.section)
	}
	click(t, driver, "设置")
	if !shows(driver, "从备份恢复") {
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

// TestSettingsAppearanceAndCloseBehavior drives the two segmented general
// settings and checks they reach both the shell and the configuration.
func TestSettingsAppearanceAndCloseBehavior(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "设置")
	if !hasControl(driver, "直接退出") {
		t.Fatal("通用设置 is missing its close-behavior choice")
	}

	click(t, driver, "深色")
	if model.config.Appearance != config.AppearanceDark || !model.dark {
		t.Fatalf("appearance = %q, dark = %v", model.config.Appearance, model.dark)
	}
	if !hasControl(driver, "切换到浅色模式") {
		t.Fatal("the titlebar toggle did not follow the appearance")
	}
	click(t, driver, "浅色")
	if model.config.Appearance != config.AppearanceLight || model.dark {
		t.Fatalf("appearance = %q, dark = %v", model.config.Appearance, model.dark)
	}

	click(t, driver, "直接退出")
	if model.config.CloseBehavior != config.CloseQuit {
		t.Fatalf("close behavior = %q", model.config.CloseBehavior)
	}
	click(t, driver, "每次询问")
	if model.config.CloseBehavior != config.CloseAsk {
		t.Fatalf("close behavior = %q", model.config.CloseBehavior)
	}
}

// TestSettingsManagesLedgers drives 账本管理, the settings section under 通用设置:
// create a ledger with a description, rename it, and delete it. The new ledger becomes the one the
// accounting pages read, as the application's 创建账本 did.
func TestSettingsManagesLedgers(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "设置")
	// 账本管理 is its own section of the panel, under 通用设置.
	click(t, driver, "账本管理")
	if !shows(driver, "家庭账本") {
		t.Fatal("账本管理 does not list the workspace's ledgers")
	}

	click(t, driver, "新增账本")
	click(t, driver, "名称")
	if err := driver.Type("装修账本"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "描述")
	if err := driver.Type("2026 年装修支出"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !shows(driver, "装修账本") || !shows(driver, "2026 年装修支出") {
		t.Fatal("the created ledger is not in 账本管理")
	}
	created := ledgerID(t, model, "装修账本")
	if got := model.accounting.ActiveLedger(); got != created {
		t.Fatalf("active ledger = %q, want the created %q", got, created)
	}

	// Rename and rewrite the description in one dialog.
	click(t, driver, "编辑账本 装修账本")
	click(t, driver, "名称")
	if err := driver.SelectText(0, len([]rune("装修账本"))); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("装修 2026"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "描述")
	if err := driver.SelectText(0, len([]rune("2026 年装修支出"))); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("只剩水电"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "保存")
	if shows(driver, "装修账本") {
		t.Fatal("the old name is still on the page")
	}
	if !shows(driver, "装修 2026") || !shows(driver, "只剩水电") {
		t.Fatal("the rename and its description did not reach 账本管理")
	}
	renamed := ledgerID(t, model, "装修 2026")
	if renamed != created {
		t.Fatalf("the rename changed the id: %q -> %q", created, renamed)
	}

	// A name is required; the dialog stays open with the reason.
	click(t, driver, "编辑账本 装修 2026")
	click(t, driver, "名称")
	if err := driver.SelectText(0, len([]rune("装修 2026"))); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "保存")
	if !shows(driver, "账本名称不能为空") {
		t.Fatal("an empty name was accepted")
	}
	click(t, driver, "取消")

	// Delete removes it, and the accounting pages fall back to the first.
	click(t, driver, "删除账本 装修 2026")
	if !shows(driver, "删除账本「装修 2026」？") {
		t.Fatal("the delete confirmation did not open")
	}
	click(t, driver, "删除")
	if shows(driver, "装修 2026") {
		t.Fatal("the deleted ledger is still on the page")
	}
	if len(model.accounting.Ledgers()) != 2 {
		t.Fatalf("ledgers = %d after the delete, want 2", len(model.accounting.Ledgers()))
	}
	if got := model.accounting.ActiveLedger(); got == created {
		t.Fatal("the accounting pages are still reading the deleted ledger")
	}
}

// ledgerID finds a ledger by name in the shared store.
func ledgerID(t *testing.T, model *Model, name string) string {
	t.Helper()
	for _, one := range model.accounting.Ledgers() {
		if one.Name == name {
			return one.ID
		}
	}
	t.Fatalf("no ledger named %q", name)
	return ""
}

// TestAboutPageShowsTheBuildAndOpensItsLinks drives 关于软件: the page names this
// build, copies the same facts for a bug report, and opens the project's two
// links (and the framework's) in the browser.
func TestAboutPageShowsTheBuildAndOpensItsLinks(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "设置")
	click(t, driver, "关于软件")
	facts := buildinfo.Read()
	if !shows(driver, "版本信息") || !shows(driver, facts.VersionText()) {
		t.Fatal("关于软件 does not name the build's version")
	}
	if !shows(driver, facts.Platform) || !shows(driver, facts.Runtime) {
		t.Fatal("关于软件 does not name the platform and the runtime")
	}

	click(t, driver, "复制版本信息")
	if got := driver.Clipboard(); !strings.Contains(got, "Transactions "+facts.Version) {
		t.Fatalf("clipboard = %q", got)
	}

	click(t, driver, "项目主页")
	if got, ok := driver.LastOpenedURL(); !ok || got != "https://github.com/ddd-online/Transactions-Viva" {
		t.Fatalf("项目主页 opened %q", got)
	}
	click(t, driver, "问题反馈")
	if got, ok := driver.LastOpenedURL(); !ok || got != "https://github.com/ddd-online/Transactions-Viva/issues" {
		t.Fatalf("问题反馈 opened %q", got)
	}
	click(t, driver, "界面由 Viva 绘制")
	if got, ok := driver.LastOpenedURL(); !ok || got != "https://vivaui.dev" {
		t.Fatalf("the colophon opened %q", got)
	}
}

// TestFeatureSwitchHidesItsSection checks a switch removes the section from the
// rail immediately, and turning it back on restores it.
func TestFeatureSwitchHidesItsSection(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "设置")
	if !hasControl(driver, "记账功能开关") {
		t.Fatal("通用设置 did not show the five switches")
	}

	click(t, driver, "股票功能开关")
	if model.config.Features.Stock {
		t.Fatal("the switch did not turn 股票 off")
	}
	if hasControl(driver, "股票") {
		t.Fatal("股票 is still in the navigation after being switched off")
	}
	click(t, driver, "股票功能开关")
	if !model.config.Features.Stock || !hasControl(driver, "股票") {
		t.Fatal("turning 股票 back on did not restore it")
	}

	click(t, driver, "记账功能开关")
	if hasControl(driver, "记账") {
		t.Fatal("记账 is still in the navigation after being switched off")
	}
	if !hasControl(driver, "股票") {
		t.Fatal("disabling 记账 removed another section")
	}
}

// TestSettingsWritesTheConfigurationFile drives appearance, close behavior and a
// feature switch, then reloads the file to prove they were saved.
func TestSettingsWritesTheConfigurationFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.FileName)
	store := config.Load(path)
	cfg := store.Config()
	cfg.Appearance = config.AppearanceLight
	model := NewWithConfig(cfg, store)
	driver, _ := openModel(t, model)

	click(t, driver, "设置")
	click(t, driver, "深色")
	click(t, driver, "直接退出")
	click(t, driver, "日记功能开关")

	reloaded := config.Load(path).Config()
	if reloaded.Appearance != config.AppearanceDark {
		t.Fatalf("appearance in the file = %q", reloaded.Appearance)
	}
	if reloaded.CloseBehavior != config.CloseQuit {
		t.Fatalf("close behavior in the file = %q", reloaded.CloseBehavior)
	}
	if reloaded.Features.Diary {
		t.Fatal("the feature switch was not written")
	}
}

// TestSettingsFailedWriteKeepsTheOldValue points the store at a directory so
// saving fails; the setting must revert and the page must say why.
func TestSettingsFailedWriteKeepsTheOldValue(t *testing.T) {
	cfg := config.Default()
	cfg.Appearance = config.AppearanceLight
	model := NewWithConfig(cfg, config.Load(t.TempDir()))
	driver, _ := openModel(t, model)

	click(t, driver, "设置")
	click(t, driver, "深色")
	if model.config.Appearance != config.AppearanceLight || model.dark {
		t.Fatal("a failed write did not restore the appearance")
	}
	if !shows(driver, "保存外观失败") {
		t.Fatal("the failed write was silent")
	}
}

// TestSettingsWorkspacePicker adopts a directory, shows it and remembers it.
func TestSettingsWorkspacePicker(t *testing.T) {
	model := New()
	directory := filepath.Join(t.TempDir(), "books")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	model.pickWorkspace = func() (string, error) { return directory, nil }
	driver, _ := openModel(t, model)

	click(t, driver, "设置")
	if !shows(driver, "还没有选择工作空间") {
		t.Fatal("the empty workspace is not reported")
	}
	click(t, driver, "切换")
	if model.config.WorkspaceDir != directory {
		t.Fatalf("workspace = %q", model.config.WorkspaceDir)
	}
	if len(model.config.RecentWorkspaces) != 1 || model.config.RecentWorkspaces[0] != directory {
		t.Fatalf("recent = %v", model.config.RecentWorkspaces)
	}
	if !shows(driver, directory) {
		t.Fatal("the settings row does not show the chosen directory")
	}
}

// TestPrependWorkspaceKeepsFiveDistinctPaths checks the recent-list rule the
// configuration service applied.
func TestPrependWorkspaceKeepsFiveDistinctPaths(t *testing.T) {
	recent := []string{"a", "b", "c", "d", "e"}
	next := prependWorkspace("c", recent)
	want := []string{"c", "a", "b", "d", "e"}
	if len(next) != len(want) {
		t.Fatalf("recent = %v", next)
	}
	for index := range want {
		if next[index] != want[index] {
			t.Fatalf("recent = %v, want %v", next, want)
		}
	}
}

// TestNewConfigUsesTheSavedWindowFrame checks the window opens at the saved
// size and place.
func TestNewConfigUsesTheSavedWindowFrame(t *testing.T) {
	cfg := config.Default()
	cfg.Width, cfg.Height = 1200, 800
	x, y := 40, 60
	cfg.X, cfg.Y = &x, &y
	window := newConfig(NewWithConfig(cfg, nil))
	if window.Width != 1200 || window.Height != 800 {
		t.Fatalf("window = %dx%d", window.Width, window.Height)
	}
	if window.Origin == nil || window.Origin.X != 40 || window.Origin.Y != 60 {
		t.Fatalf("origin = %v", window.Origin)
	}
}

// TestClosePromptAsksBeforeClosing raises the 每次询问 dialog and checks both
// answers.
func TestClosePromptAsksBeforeClosing(t *testing.T) {
	model := New()
	model.config.WorkspaceDir = t.TempDir()
	model.config.CloseBehavior = config.CloseAsk
	closed := false
	model.requestClose(func() { closed = true })
	driver, _ := openModel(t, model)

	if !shows(driver, "关闭窗口") {
		t.Fatal("the close prompt did not appear")
	}
	click(t, driver, "取消")
	if closed || model.closeAsk {
		t.Fatal("取消 still closed the window")
	}

	model.requestClose(func() { closed = true })
	settle(t, driver)
	if !shows(driver, "关闭窗口") {
		t.Fatal("the close prompt did not appear the second time")
	}
	click(t, driver, "退出应用")
	if !closed || model.closeAsk {
		t.Fatal("关闭应用 did not close the window")
	}
}

// TestFirstEnabledSectionFollowsTheSwitches checks the fallback order when the
// current feature is off.
func TestFirstEnabledSectionFollowsTheSwitches(t *testing.T) {
	model := New()
	model.config.Features.Stock = false
	if model.firstEnabledSection() != SectionAccounting {
		t.Fatalf("fallback = %v", model.firstEnabledSection())
	}
	model.config.Features.Accounting = false
	if model.firstEnabledSection() != SectionTodo {
		t.Fatalf("fallback = %v", model.firstEnabledSection())
	}
	model.config.Features = config.FeatureFlags{}
	if model.firstEnabledSection() != SectionSettings {
		t.Fatalf("all-off fallback = %v", model.firstEnabledSection())
	}
}

// TestTrayIconCarriesTheAppMark checks the status icon is the About page's
// mark: a colored plate with the white "Tr", its corners transparent, rather
// than the template placeholder.
func TestTrayIconCarriesTheAppMark(t *testing.T) {
	icon := trayIcon()
	if icon.Size != 64 || len(icon.RGBA) != 64*64*4 {
		t.Fatalf("icon size = %d, %d bytes", icon.Size, len(icon.RGBA))
	}
	if icon.Template {
		t.Fatal("the plate is colored, not a template")
	}
	if alpha := icon.RGBA[3]; alpha != 0 {
		t.Fatalf("corner alpha = %d, want transparent", alpha)
	}
	blue, white := 0, 0
	for index := 0; index+3 < len(icon.RGBA); index += 4 {
		r, g, b, a := icon.RGBA[index], icon.RGBA[index+1], icon.RGBA[index+2], icon.RGBA[index+3]
		if a < 200 {
			continue
		}
		switch {
		case b > 150 && r < 90 && g < 130:
			blue++
		case r > 230 && g > 230 && b > 230:
			white++
		}
	}
	if blue == 0 || white == 0 {
		t.Fatalf("plate pixels = %d, mark pixels = %d", blue, white)
	}
}

// TestDevelopmentCloseSkipsThePrompt guards the reload worker's limitation: the
// development host paints on its own schedule, so a prompt raised there would
// not appear; development closes immediately.
func TestDevelopmentCloseSkipsThePrompt(t *testing.T) {
	model := New()
	model.development = true
	model.config.WorkspaceDir = t.TempDir()
	model.config.CloseBehavior = config.CloseAsk
	closed := false
	model.requestClose(func() { closed = true })
	if !closed || model.closeAsk {
		t.Fatal("development close did not close immediately")
	}
}

// TestDevModeFollowsTheReloadHost checks the configuration file split.
func TestDevModeFollowsTheReloadHost(t *testing.T) {
	t.Setenv("VIVA_RELOAD_ADDRESS", "")
	if devMode() {
		t.Fatal("development mode without a reload host")
	}
	t.Setenv("VIVA_RELOAD_ADDRESS", "127.0.0.1:1")
	if !devMode() {
		t.Fatal("release mode under a reload host")
	}
}

// TestTitlebarThemeButtonHoverChangesFill guards the titlebar's appearance
// toggle: it is a Viva ButtonBase with an explicit Hover look, because the
// titlebar's own fill is the same Muted colour Viva's Ghost button uses.
func TestTitlebarThemeButtonHoverChangesFill(t *testing.T) {
	driver, _ := open(t)
	bounds, ok := controlBounds(driver, "切换到深色模式")
	if !ok {
		t.Fatal("the titlebar has no appearance toggle")
	}
	fill := func() gfx.Color {
		for _, primitive := range driver.Snapshot().Primitives {
			if primitive.Kind == app.RectanglePrimitive && primitive.Bounds == bounds {
				return primitive.Color
			}
		}
		return gfx.Color{}
	}
	rest := fill()
	if err := driver.Move(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	settle(t, driver)
	hover := fill()
	if hover == rest {
		t.Fatalf("the appearance toggle has no hover fill: rest %v, hover %v", rest, hover)
	}
	if hover.Alpha == 0 {
		t.Fatalf("the appearance toggle's hover fill is transparent: %v", hover)
	}
}

// TestRecordsSyncSubmenuHover checks the ledger submenu of the record context
// menu: Viva paints a row's hover with theme.Accent, and the deep Accent is what
// makes it visible beside the white popover.
func TestRecordsSyncSubmenuHover(t *testing.T) {
	driver, model := open(t)
	rightClickText(t, driver, "早餐 · 豆浆油条")
	parent, ok := controlBounds(driver, "复制到其他账本")
	if !ok {
		t.Fatal("the row menu has no 复制到其他账本")
	}
	if err := driver.Move(parent.X+parent.Width/2, parent.Y+parent.Height/2); err != nil {
		t.Fatal(err)
	}
	settle(t, driver)
	item, ok := controlBounds(driver, "旅行账本")
	if !ok {
		t.Fatal("the sync submenu did not open")
	}
	if err := driver.Move(item.X+item.Width/2, item.Y+item.Height/2); err != nil {
		t.Fatal(err)
	}
	settle(t, driver)
	for _, primitive := range driver.Snapshot().Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Bounds == item {
			if primitive.Color != model.theme.Accent {
				t.Fatalf("submenu hover fill = %v, want %v", primitive.Color, model.theme.Accent)
			}
			return
		}
	}
	t.Fatal("the sync submenu item has no hover fill")
}

// TestAnalysisPage drives the 分析 screen: the two presets, the line canvas and
// summary, and creating a custom pie chart through the dialog.
func TestAnalysisPage(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "分析")
	if !shows(driver, "月度消费趋势") || !shows(driver, "名称") || !shows(driver, "包含离群值") {
		t.Fatal("the analysis presets or the curve table are missing")
	}
	if !hasControl(driver, "新增图表") {
		t.Fatal("the analysis toolbar has no 新增图表")
	}
	click(t, driver, "年度消费趋势")
	if !shows(driver, "年度消费趋势") {
		t.Fatal("selecting the yearly preset did not show it")
	}

	click(t, driver, "新增图表")
	// The name field takes the keyboard with the dialog: no click is needed.
	if err := driver.Type("支出构成"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "饼图")
	click(t, driver, "创建")
	if !shows(driver, "支出构成") || !shows(driver, "占比") {
		t.Fatal("the pie chart was not created or selected")
	}
	if !hasControl(driver, "删除图表 支出构成") {
		t.Fatal("the custom chart is not in the rail")
	}
	click(t, driver, "删除图表 支出构成")
	click(t, driver, "删除")
	if hasControl(driver, "删除图表 支出构成") {
		t.Fatal("the custom chart was not deleted")
	}
}

// TestAnalysisLineEditor creates a custom line chart and adds a filtered curve
// through the line dialog.
func TestAnalysisLineEditor(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "分析")
	click(t, driver, "新增图表")
	if err := driver.Type("餐饮趋势"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !hasControl(driver, "添加曲线") {
		t.Fatal("a custom line chart has no 添加曲线")
	}
	click(t, driver, "添加曲线")
	click(t, driver, "曲线名称")
	if err := driver.Type("餐饮"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "分类（可选）")
	click(t, driver, "餐饮美食")
	click(t, driver, "曲线标签")
	click(t, driver, "饮料")
	click(t, driver, "零食")
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "保存")
	if !hasControl(driver, "删除曲线 餐饮") {
		t.Fatal("the filtered curve was not added")
	}
	if !shows(driver, "饮料") || !shows(driver, "零食") {
		t.Fatal("the curve conditions are not shown")
	}
}

func TestRemoveRecentWorkspaceForgetsPath(t *testing.T) {
	model := New()
	model.config.RecentWorkspaces = []string{`C:\one`, `C:\two`}
	if err := model.removeRecentWorkspace(`c:\ONE`); err != nil {
		t.Fatal(err)
	}
	if len(model.config.RecentWorkspaces) != 1 || model.config.RecentWorkspaces[0] != `C:\two` {
		t.Fatalf("recent = %v", model.config.RecentWorkspaces)
	}
	if err := model.removeRecentWorkspace(`C:\missing`); err != nil {
		t.Fatal(err)
	}
	if len(model.config.RecentWorkspaces) != 1 {
		t.Fatalf("removing a missing path changed recent = %v", model.config.RecentWorkspaces)
	}
}
