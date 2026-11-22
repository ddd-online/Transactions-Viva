package desktop

import "testing"

func TestTodoBoardCardItemProgressAndHistory(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "待办")
	click(t, driver, "新建主题")
	click(t, driver, "主题")
	driver.Type("本周")
	click(t, driver, "创建")
	if !shows(driver, "本周") {
		t.Fatal("new card is not shown")
	}

	click(t, driver, "添加第一件事项")
	click(t, driver, "事项")
	driver.Type("写周报")
	click(t, driver, "保存")
	if !shows(driver, "写周报") {
		t.Fatal("new item is not shown")
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

	click(t, driver, "删除事项")
	clickLast(t, driver, "删除事项")
	if shows(driver, "写周报") {
		t.Fatal("deleted item is still shown")
	}
	click(t, driver, "删除主题")
	clickLast(t, driver, "删除主题")
	if shows(driver, "本周") {
		t.Fatal("deleted card is still shown")
	}
}
