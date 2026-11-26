package desktop

import (
	"context"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"

	"transactions/internal/domain"
)

func TestTodoTableItemThemeProgressAndHistory(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "待办")

	// The board is empty: the empty state's own 新建事项 button paints after the
	// toolbar's, so clickLast takes it. A new item starts today.
	clickLast(t, driver, "新建事项")
	if !shows(driver, domain.Today()) {
		t.Fatalf("a new item does not start today (%s)", domain.Today())
	}
	startTheme(t, driver, "本周")
	click(t, driver, "事项")
	driver.Type("写周报")
	click(t, driver, "创建")
	if !shows(driver, "写周报") || !shows(driver, "本周") {
		t.Fatal("the new item or its theme is not in the table")
	}
	cards, err := model.workspace.TodoCards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || len(cards[0].Items) != 1 {
		t.Fatalf("the board holds %+v", cards)
	}
	if cards[0].Items[0].StartDate != domain.Today() {
		t.Fatalf("the item starts %q, want today (%s)", cards[0].Items[0].StartDate, domain.Today())
	}

	// A second item under the existing theme: the table is newest first.
	click(t, driver, "新建事项")
	click(t, driver, "主题")
	click(t, driver, "本周")
	click(t, driver, "事项")
	driver.Type("写日报")
	click(t, driver, "创建")
	if !shows(driver, "写日报") {
		t.Fatal("the second item is not in the table")
	}
	if !paintedAbove(t, driver, "写日报", "写周报") {
		t.Fatal("the table is not newest first")
	}

	click(t, driver, "展开进度 写周报")
	click(t, driver, "新增进度")
	driver.Type("先列大纲")
	click(t, driver, "添加")
	if !shows(driver, "先列大纲") {
		t.Fatal("progress is not shown")
	}

	click(t, driver, "标记完成：写周报")
	click(t, driver, "历史")
	if !shows(driver, "写周报") || !shows(driver, "本周") {
		t.Fatal("completed item is not in history")
	}
	clickContains(t, driver, "0/1")
	if !shows(driver, "进度 · 写周报") || !shows(driver, "先列大纲") {
		t.Fatal("history progress dialog is missing")
	}
	click(t, driver, "关闭")
	click(t, driver, "恢复")
	clickLast(t, driver, "待办")
	if !shows(driver, "写周报") {
		t.Fatal("restored item is not back on the board")
	}

	// The row's commands are its context menu: delete the item first.
	rightClickText(t, driver, "写日报")
	click(t, driver, "删除事项")
	click(t, driver, "删除")
	if shows(driver, "写日报") {
		t.Fatal("deleted item is still shown")
	}

	// Editing can move the item to a theme that does not exist yet.
	rightClickText(t, driver, "写周报")
	click(t, driver, "编辑事项")
	startTheme(t, driver, "下周")
	click(t, driver, "保存")
	if !shows(driver, "下周") {
		t.Fatal("the edited item did not move to the new theme")
	}

	// Moving the last item out of 本周 deleted it: a theme holds items or it is
	// not a theme.
	if shows(driver, "本周") {
		t.Fatal("the theme the item left outlived it")
	}

	// The same menu deletes a theme and whatever it still holds; with the board's
	// only theme gone there is nothing left.
	rightClickText(t, driver, "写周报")
	click(t, driver, "删除主题「下周」")
	clickLast(t, driver, "删除")
	if shows(driver, "下周") || shows(driver, "写周报") {
		t.Fatal("deleted theme is still shown")
	}
	if !shows(driver, "把下一件事放到板上") {
		t.Fatal("the board is not empty after deleting every theme")
	}
}

// startTheme types a theme the board does not have yet into the form's Combobox and
// commits the row it offers for it, which is how a theme is started now that the
// field is the one control for both picking and naming.
func startTheme(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	click(t, driver, "主题")
	driver.Type(name)
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	// The field names the theme it holds on the next paint. A move over the page
	// is that paint, without the clock jump that patches which overlay a later
	// click lands on.
	if err := driver.Move(1200, 700); err != nil {
		t.Fatal(err)
	}
	if !shows(driver, name) {
		t.Fatalf("the theme field does not show %q after it was created", name)
	}
}

// paintedAbove reports whether the first exact text paints above the second.
func paintedAbove(t *testing.T, driver *app.Driver, first, second string) bool {
	t.Helper()
	y := map[string]float32{}
	for _, label := range driver.Snapshot().Labels {
		if label.Value == first || label.Value == second {
			if _, ok := y[label.Value]; !ok {
				y[label.Value] = label.Bounds.Y
			}
		}
	}
	a, hasA := y[first]
	b, hasB := y[second]
	if !hasA || !hasB {
		t.Fatalf("texts %q / %q are not both painted", first, second)
	}
	return a < b
}

// TestTodoFormKeepsItselfWhenTheDatePanelCloses: Escape belongs to the open date
// panel first. Closing the form instead would throw away the title being typed,
// which is what the dialog's own Escape does.
func TestTodoFormKeepsItselfWhenTheDatePanelCloses(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "待办")
	clickLast(t, driver, "新建事项")
	click(t, driver, "事项")
	if err := driver.Type("写周报"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, domain.Today())
	if !hasControl(driver, "关闭日期选择") {
		t.Fatal("the date panel did not open")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if hasControl(driver, "关闭日期选择") {
		t.Fatal("Escape left the date panel open")
	}
	if !inDialog(driver, "创建") {
		t.Fatal("Escape closed the whole form, not just the date panel")
	}
	if !shows(driver, "写周报") {
		t.Fatal("the typed title was lost")
	}
}
