package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vivaui/viva/app"

	"transactions/internal/domain"
)

// advance writes the diary draft: the editor waits saveDelay after the last
// change, so a test moves the driver's clock past it and pumps the frame the
// page asked for.
func advance(t *testing.T, driver *app.Driver) {
	t.Helper()
	if err := driver.Advance(700 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
}

// TestDiaryPageWritesTheDay drives 日记: the empty book's notice, the word count
// that follows typing, the write that lands after the pause, the tree row the
// write creates, the mood the row shows, and the delete confirmation.
func TestDiaryPageWritesTheDay(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "日记")
	if !shows(driver, "暂无日记") {
		t.Fatal("an empty book should say it has no diary")
	}
	today := domain.DiaryToday()
	if !shows(driver, domain.DiaryDateCn(today)) {
		t.Fatalf("the editor does not name today (%s)", today)
	}

	click(t, driver, "日记正文")
	if err := driver.Type("今天很好"); err != nil {
		t.Fatal(err)
	}
	if !shows(driver, "4 字") {
		t.Fatal("the word count did not follow the typing")
	}
	if !shows(driver, "未保存") {
		t.Fatal("a draft that has not been written should say 未保存")
	}
	advance(t, driver)
	if !shows(driver, "已保存") {
		t.Fatal("the draft was not written after the pause")
	}

	bookID := model.diary.ActiveBook()
	entry, err := model.workspace.DiaryEntry(context.Background(), bookID, today)
	if err != nil {
		t.Fatalf("the diary was not written: %v", err)
	}
	if entry.Content != "今天很好" || entry.WordCount != 4 {
		t.Fatalf("written entry = %+v", entry)
	}

	// The tree now carries today: the year, the month and the day number.
	if !containsLabel(driver, fmt.Sprintf("%d", dayOf(today))) {
		t.Fatal("the written day is not in the tree")
	}

	// The mood is written too, and the picker shows the chosen tier.
	click(t, driver, "开心")
	advance(t, driver)
	entry, err = model.workspace.DiaryEntry(context.Background(), bookID, today)
	if err != nil {
		t.Fatalf("read the entry: %v", err)
	}
	if entry.Mood != "😊" {
		t.Fatalf("mood = %q, want 😊", entry.Mood)
	}

	// The delete key is the day row's own: it is showing because today is the day
	// the page has selected.
	click(t, driver, "删除日记 "+domain.DiaryDateCn(today))
	if !shows(driver, "删除日记「") {
		t.Fatal("the delete confirmation did not open")
	}
	click(t, driver, "删除")
	if _, err := model.workspace.DiaryEntry(context.Background(), bookID, today); err == nil {
		t.Fatal("the diary survived the confirmation")
	}
	if shows(driver, "4 字") {
		t.Fatal("the editor still shows the deleted text")
	}
	if !shows(driver, "暂无日记") {
		t.Fatal("the empty notice did not come back")
	}
}

// dayOf is the day number of a stored date.
func dayOf(date string) int {
	parsed, ok := domain.ParseDiaryDate(date)
	if !ok {
		return 0
	}
	return parsed.Day()
}

// containsLabel reports whether a label with exactly this text is painted (a day
// number, a count), where shows checks one label alone.
func containsLabel(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if label.Value == want {
			return true
		}
	}
	return false
}

// TestDiaryBooksHoldTheirOwnDays checks the point of the diary book: diaries are
// no longer tied to ledgers, a workspace keeps as many books as it wants, and the
// toolbar switches between them.
func TestDiaryBooksHoldTheirOwnDays(t *testing.T) {
	driver, model := open(t)
	today := domain.DiaryToday()
	click(t, driver, "日记")
	click(t, driver, "日记正文")
	if err := driver.Type("第一本的日记"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	first := model.diary.ActiveBook()

	// A second book through the settings page becomes the diary page's book.
	click(t, driver, "设置")
	click(t, driver, "日记配置")
	click(t, driver, "新建日记本")
	click(t, driver, "名称")
	if err := driver.Type("旅行日记"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !shows(driver, "日记本已创建") && !hasControl(driver, "编辑日记本 旅行日记") {
		t.Fatal("the new book is not in the list")
	}
	second := model.diary.ActiveBook()
	if second == first {
		t.Fatal("creating a book did not make it the diary page's book")
	}

	// The new book starts empty, and writing into it leaves the first alone.
	click(t, driver, "日记")
	if !shows(driver, "暂无日记") {
		t.Fatal("the new book should start empty")
	}
	if shows(driver, "第一本的日记") {
		t.Fatal("the other book's diary is shown")
	}
	click(t, driver, "日记正文")
	if err := driver.Type("第二本的日记"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	firstEntry, err := model.workspace.DiaryEntry(context.Background(), first, today)
	if err != nil || firstEntry.Content != "第一本的日记" {
		t.Fatalf("first book entry = %+v, err=%v", firstEntry, err)
	}
	secondEntry, err := model.workspace.DiaryEntry(context.Background(), second, today)
	if err != nil || secondEntry.Content != "第二本的日记" {
		t.Fatalf("second book entry = %+v, err=%v", secondEntry, err)
	}

	// The toolbar's 日记本 field switches back.
	click(t, driver, "日记本")
	click(t, driver, "默认日记本")
	if model.diary.ActiveBook() != first {
		t.Fatalf("the book did not switch: %q", model.diary.ActiveBook())
	}
	if !shows(driver, "第一本的日记") {
		t.Fatal("switching books did not show the first book's day")
	}
}

// TestSettingsDiaryBookManagement drives 日记本 in settings: create, rename and
// delete a book, and the last book's delete staying disabled.
func TestSettingsDiaryBookManagement(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "设置")
	click(t, driver, "日记配置")
	if !shows(driver, "默认日记本") {
		t.Fatal("the seeded book is not listed")
	}
	// The description line is always there, like a ledger's: a book that has
	// none says so rather than hiding the line.
	if !shows(driver, "还没有描述") {
		t.Fatal("a book without a description does not say so")
	}
	// The only book cannot be deleted: the diary page needs one to show.
	if !hasControl(driver, "删除日记本 默认日记本") {
		t.Fatal("the book row has no delete command")
	}

	click(t, driver, "新建日记本")
	click(t, driver, "名称")
	if err := driver.Type("随笔"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "描述")
	if err := driver.Type("想到什么写什么"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "创建")
	if !hasControl(driver, "编辑日记本 随笔") {
		t.Fatal("the created book is not in the list")
	}
	if !shows(driver, "想到什么写什么") {
		t.Fatal("the created book's description is not shown")
	}

	click(t, driver, "编辑日记本 随笔")
	click(t, driver, "名称")
	if err := driver.SelectText(0, len([]rune("随笔"))); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("读书笔记"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "保存")
	if !hasControl(driver, "编辑日记本 读书笔记") {
		t.Fatal("the rename did not reach the list")
	}
	books, err := model.workspace.DiaryBooks(context.Background())
	if err != nil || len(books) != 2 || books[1].Name != "读书笔记" {
		t.Fatalf("books = %+v, err=%v", books, err)
	}

	click(t, driver, "删除日记本 读书笔记")
	if !shows(driver, "删除日记本「读书笔记」？") {
		t.Fatal("the delete confirmation did not open")
	}
	click(t, driver, "删除")
	if hasControl(driver, "编辑日记本 读书笔记") {
		t.Fatal("the deleted book is still listed")
	}
	books, err = model.workspace.DiaryBooks(context.Background())
	if err != nil || len(books) != 1 || books[0].Name != "默认日记本" {
		t.Fatalf("books after delete = %+v, err=%v", books, err)
	}
}

// TestSettingsDiaryImportAndExport drives 导入导出: importing a directory of
// YYYY-MM-DD.txt files into the diary page's book, exporting the book again, and
// reading what each command reported.
func TestSettingsDiaryImportAndExport(t *testing.T) {
	driver, model := open(t)
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "2026-02-01.txt"), []byte("导入的日记"), 0o644); err != nil {
		t.Fatal(err)
	}
	// One file that is not a diary: the scan must skip it.
	if err := os.WriteFile(filepath.Join(source, "notes.md"), []byte("忽略"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "out")

	model.pickDirectory = func(title string) (string, error) {
		if title == "选择日记导出目录" {
			return target, nil
		}
		return source, nil
	}

	click(t, driver, "设置")
	click(t, driver, "日记配置")
	click(t, driver, "批量导入")
	if !shows(driver, "导入 1/1 篇") {
		t.Fatalf("import status = %q", statusLine(driver))
	}
	entry, err := model.workspace.DiaryEntry(context.Background(), model.diary.ActiveBook(), "2026-02-01")
	if err != nil || entry.Content != "导入的日记" {
		t.Fatalf("imported = %+v, err=%v", entry, err)
	}

	click(t, driver, "批量导出")
	if !shows(driver, "导出 1/1 篇") {
		t.Fatalf("export status = %q", statusLine(driver))
	}
	raw, err := os.ReadFile(filepath.Join(target, "2026-02-01.txt"))
	if err != nil {
		t.Fatalf("read the exported file: %v", err)
	}
	if string(raw) != "导入的日记" {
		t.Fatalf("exported = %q", raw)
	}

	// The diary page sees the imported day without another visit.
	click(t, driver, "日记")
	click(t, driver, "2026年2月1日，5 字")
	if !shows(driver, "2026年2月1日") || !shows(driver, "导入的日记") {
		t.Fatal("the imported day did not reach the diary page")
	}
}

// statusLine is the text the 日记配置 section reports after a command, for a
// failure that needs to say what it saw.
func statusLine(driver *app.Driver) string {
	lines := make([]string, 0, 4)
	for _, label := range driver.Snapshot().Labels {
		if len(label.Value) > 0 {
			lines = append(lines, label.Value)
		}
	}
	return fmt.Sprint(lines)
}
