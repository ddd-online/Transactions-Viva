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

// TestDiaryPageWritesTheDay drives 日记: the empty ledger's notice, the word
// count that follows typing, the write that lands after the pause, the tree row
// the write creates, the mood the row shows, and the delete confirmation.
func TestDiaryPageWritesTheDay(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "日记")
	if !shows(driver, "还没有日记") {
		t.Fatal("an empty ledger should say it has no diary")
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

	ledgerID := model.ledgers.Active()
	entry, err := model.workspace.DiaryEntry(context.Background(), ledgerID, today)
	if err != nil {
		t.Fatalf("the diary was not written: %v", err)
	}
	if entry.Content != "今天很好" || entry.WordCount != 4 {
		t.Fatalf("written entry = %+v", entry)
	}

	// The tree now carries today: the year, the month and the day number.
	day := fmt.Sprintf("%d", at(today))
	if !shows(driver, domain.DiaryDateCn(today)) || !containsLabel(driver, day) {
		t.Fatal("the written day is not in the tree")
	}

	// The mood is written too, and the tree row shows its emoji.
	click(t, driver, "开心")
	advance(t, driver)
	entry, err = model.workspace.DiaryEntry(context.Background(), ledgerID, today)
	if err != nil {
		t.Fatalf("read the entry: %v", err)
	}
	if entry.Mood != "😊" {
		t.Fatalf("mood = %q, want 😊", entry.Mood)
	}

	click(t, driver, "删除日记")
	if !shows(driver, "删除这篇日记？") {
		t.Fatal("the delete confirmation did not open")
	}
	click(t, driver, "确认删除")
	if _, err := model.workspace.DiaryEntry(context.Background(), ledgerID, today); err == nil {
		t.Fatal("the diary survived the confirmation")
	}
	if shows(driver, "4 字") {
		t.Fatal("the editor still shows the deleted text")
	}
	if !shows(driver, "还没有日记") {
		t.Fatal("the empty notice did not come back")
	}
}

// at is the day number of a stored date.
func at(date string) int {
	parsed, ok := domain.ParseDiaryDate(date)
	if !ok {
		return 0
	}
	return parsed.Day()
}

// containsLabel reports whether any painted label contains the text (a day
// number, a count), where shows checks one label alone.
func containsLabel(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if label.Value == want {
			return true
		}
	}
	return false
}

// TestDiarySwitchKeepsEachLedgerApart checks that the diary follows the shared
// 账本 choice: switching the ledger on the records page shows that ledger's
// diary, and the day written there does not appear in the other one.
func TestDiarySwitchKeepsEachLedgerApart(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "账本")
	click(t, driver, "旅行账本")
	if model.ledgers.Active() != model.ledgers.List()[1].ID {
		t.Fatalf("the ledger did not switch: %q", model.ledgers.Active())
	}

	click(t, driver, "日记")
	click(t, driver, "日记正文")
	if err := driver.Type("路上的日记"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	travelID := model.ledgers.Active()
	if _, err := model.workspace.DiaryEntry(context.Background(), travelID, domain.DiaryToday()); err != nil {
		t.Fatalf("the travel diary was not written: %v", err)
	}

	// Back to the home ledger: its diary is empty, the travel text is not shown.
	// The sidebar shows one section's sub-functions at a time, so 记账 has to be
	// opened again before its 账本 field is on screen.
	click(t, driver, "记账")
	click(t, driver, "账本")
	click(t, driver, "家庭账本")
	click(t, driver, "日记")
	if shows(driver, "路上的日记") {
		t.Fatal("the other ledger's diary is shown")
	}
	if !shows(driver, "还没有日记") {
		t.Fatal("the home ledger should show its own empty diary")
	}
	homeID := model.ledgers.Active()
	if _, err := model.workspace.DiaryEntry(context.Background(), homeID, domain.DiaryToday()); err == nil {
		t.Fatal("the travel diary leaked into the home ledger")
	}
}

// TestSettingsDiaryImportAndExport drives 日记配置: importing a directory of
// YYYY-MM-DD.txt files, exporting the ledger again, and reading what each
// command reported.
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
	entry, err := model.workspace.DiaryEntry(context.Background(), model.ledgers.Active(), "2026-02-01")
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

	// The diary page sees the imported day without another visit: its tree has
	// the year and the month open, and choosing the day shows it in the editor.
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
