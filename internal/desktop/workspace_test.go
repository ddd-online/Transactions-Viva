package desktop

import (
	"sync/atomic"
	"testing"

	"github.com/vivaui/viva/app"

	storepkg "transactions/internal/store"
	"transactions/internal/store/storetest"
)

// TestEmptyWorkspaceCreatesDefaultLedgerAndPersists drives the first-run flow:
// an empty workspace gets a 默认账本 with the default labels, a record written
// through the dialog is still there after the workspace is closed and reopened.
func TestEmptyWorkspaceCreatesDefaultLedgerAndPersists(t *testing.T) {
	directory := t.TempDir()
	workspace, err := storepkg.Open(directory)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	model := New()
	if err := model.adoptWorkspace(workspace); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	driver, _ := openModel(t, model)
	if !shows(driver, "默认账本") {
		t.Fatal("an empty workspace did not create 默认账本")
	}

	click(t, driver, "记一笔")
	click(t, driver, "金额")
	if err := driver.Type("88.50"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "分类")
	click(t, driver, "娱乐休闲")
	click(t, driver, "创建")
	if !shows(driver, "-88.50") {
		t.Fatal("the composer did not save its record")
	}

	driver.Close()
	model.closeWorkspace()
	reopened, err := storepkg.Open(directory)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	model = New()
	if err := model.adoptWorkspace(reopened); err != nil {
		t.Fatalf("adopt reopened: %v", err)
	}
	driver, _ = openModel(t, model)
	if !shows(driver, "-88.50") {
		t.Fatal("the record did not survive closing and reopening the workspace")
	}
	if !shows(driver, "默认账本") {
		t.Fatal("the reopened workspace lost its ledger")
	}
}

// TestWorkspaceGateOpensADirectory checks the no-workspace gate: the accounting
// pages stay empty until the user picks a directory, then the default ledger
// and its labels appear.
func TestWorkspaceGateOpensADirectory(t *testing.T) {
	directory := t.TempDir()
	model := New()
	driver, _ := openModel(t, model)
	if !shows(driver, "打开工作空间") {
		t.Fatal("the no-workspace gate is missing")
	}
	if shows(driver, "记一笔") {
		t.Fatal("the records page was usable without a workspace")
	}

	model.pickWorkspace = func() (string, error) { return directory, nil }
	click(t, driver, "选择工作空间")
	if !shows(driver, "默认账本") {
		t.Fatal("opening a directory did not create the default ledger")
	}
	click(t, driver, "标签")
	if !shows(driver, "餐饮美食") {
		t.Fatal("the new workspace did not seed its default categories")
	}
}

// TestWorkspaceSwitchLoadsTheOtherLedgers checks that switching workspaces
// closes the database and shows the new workspace's own data.
func TestWorkspaceSwitchLoadsTheOtherLedgers(t *testing.T) {
	fixture := storetest.Open(t)
	model := New()
	if err := model.adoptWorkspace(fixture.Workspace); err != nil {
		t.Fatalf("adopt fixture: %v", err)
	}
	driver, _ := openModel(t, model)
	click(t, driver, "账本")
	click(t, driver, "旅行账本")
	if !shows(driver, "去程机票") {
		t.Fatal("the fixture's travel ledger did not load")
	}

	second := t.TempDir()
	model.pickWorkspace = func() (string, error) { return second, nil }
	click(t, driver, "设置")
	click(t, driver, "切换")
	click(t, driver, "记账")
	if shows(driver, "去程机票") || shows(driver, "早餐 · 豆浆油条") {
		t.Fatal("the old workspace's records stayed after switching")
	}
	if !shows(driver, "这个时间范围里没有记录") {
		t.Fatal("the new empty workspace did not show its empty records")
	}
	if !shows(driver, "默认账本") {
		t.Fatal("the new empty workspace did not create 默认账本")
	}

	model.pickWorkspace = func() (string, error) { return fixture.Workspace.Directory(), nil }
	click(t, driver, "设置")
	click(t, driver, "切换")
	click(t, driver, "记账")
	if !shows(driver, "早餐 · 豆浆油条") {
		t.Fatal("switching back did not load the original workspace")
	}
}

// TestWorkspacePickerRunsOutsideThePaint checks the directory dialog opens
// after the click's event pass. A native dialog pumps the window's messages,
// so a dialog opened inside the paint lets a nested repaint deliver the same
// click again — the second dialog the user sees — and can end the window. Viva
// defers such a handler with c.After for exactly this reason.
func TestWorkspacePickerRunsOutsideThePaint(t *testing.T) {
	model := New()
	driver, _ := openModel(t, model)
	var calls atomic.Int32
	model.pickWorkspace = func() (string, error) {
		calls.Add(1)
		if err := driver.Paint(); err != nil {
			return "", err
		}
		return "", app.ErrCanceled
	}

	click(t, driver, "选择工作空间")
	if got := calls.Load(); got != 1 {
		t.Fatalf("the picker ran %d times, want one", got)
	}
}
