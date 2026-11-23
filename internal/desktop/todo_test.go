package desktop

import (
	"testing"

	"github.com/vivaui/viva/app"
)

func TestTodoTableItemThemeProgressAndHistory(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "待办")

	// The board is empty: the empty state's own 新建事项 button paints after the
	// toolbar's, so clickLast takes it.
	clickLast(t, driver, "新建事项")
	click(t, driver, "主题")
	driver.Type("本周")
	click(t, driver, "事项")
	driver.Type("写周报")
	click(t, driver, "创建")
	if !shows(driver, "写周报") || !shows(driver, "本周") {
		t.Fatal("the new item or its theme is not in the table")
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
	clickLast(t, driver, "删除")
	if shows(driver, "写日报") {
		t.Fatal("deleted item is still shown")
	}

	// Editing can move the item to a theme that does not exist yet.
	rightClickText(t, driver, "写周报")
	click(t, driver, "编辑事项")
	click(t, driver, "主题")
	click(t, driver, "新建主题…")
	driver.Type("下周")
	click(t, driver, "保存")
	if !shows(driver, "下周") {
		t.Fatal("the edited item did not move to the new theme")
	}

	// The same menu deletes the theme and whatever it still holds.
	rightClickText(t, driver, "写周报")
	click(t, driver, "删除主题「下周」")
	clickLast(t, driver, "删除")
	if shows(driver, "下周") || shows(driver, "写周报") {
		t.Fatal("deleted theme is still shown")
	}

	// 本周 is now empty and has no row in the table; the theme manager is
	// where it can still be deleted.
	clickLast(t, driver, "新建事项")
	click(t, driver, "主题")
	click(t, driver, "管理主题…")
	click(t, driver, "删除")
	click(t, driver, "删除")
	if !shows(driver, "还没有主题") {
		t.Fatal("the deleted theme is still in the theme manager")
	}
	click(t, driver, "返回")
	click(t, driver, "取消")
	if !shows(driver, "把下一件事放到板上") {
		t.Fatal("the board is not empty after deleting every theme")
	}
	if !shows(driver, "把下一件事放到板上") {
		t.Fatal("the empty board is not shown after the table empties")
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
