package stock

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
	"transactions/internal/store/storetest"
)

func TestSettingsFeeSummaryMatchesTheOldScreen(t *testing.T) {
	want := "佣金 万2.354 · 最低 ¥5.00 · 印花税 0.05% · 过户费 0.001%"
	if got := feeSummary(domain.DefaultStockFeeSetting()); got != want {
		t.Fatalf("feeSummary() = %q, want %q", got, want)
	}
	custom := domain.StockFeeSetting{
		CommissionRate:  0.0003,
		MinCommission:   1000,
		StampDutyRate:   0.001,
		TransferFeeRate: 0.00002,
	}
	if got := feeSummary(custom); got != "佣金 万3 · 最低 ¥10.00 · 印花税 0.1% · 过户费 0.002%" {
		t.Fatalf("custom feeSummary() = %q", got)
	}
}

func TestSettingsErrorTextDropsTheClassifier(t *testing.T) {
	cases := map[string]string{
		"无效的数据：标签不能重复":       "标签不能重复",
		"数据已存在：股票账户名称已存在":    "股票账户名称已存在",
		"数据不存在：没有这条记录":       "没有这条记录",
		"保存交易标签失败：disk full": "保存交易标签失败：disk full",
	}
	for input, want := range cases {
		if got := settingsErrorText(errorString(input)); got != want {
			t.Fatalf("settingsErrorText(%q) = %q, want %q", input, got, want)
		}
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }

func TestSettingsPanelDrivesEverySetting(t *testing.T) {
	fixture := storetest.Open(t)
	ctx := context.Background()
	account, err := fixture.Workspace.CreateStockAccount(ctx, "默认股票账户", "")
	if err != nil {
		t.Fatal(err)
	}
	accounts := newAccountStore()
	accounts.reset([]domain.StockAccount{account})

	panel := newSettingsPanel(accounts)
	panel.toaster = &ui.Toaster{}
	panel.setRepository(fixture.Workspace)
	if err := fixture.Workspace.AddStockPrincipal(ctx, account.ID, 50000, ""); err != nil {
		t.Fatal(err)
	}
	panel.Reload()
	panel.onAccountsChanged = func() {
		items, err := fixture.Workspace.StockAccounts(ctx)
		if err == nil {
			accounts.reset(items)
		}
	}

	driver := openSettingsDriver(t, panel)
	frame := driver.Frame()
	if frame.Width != 960 || frame.Height != 620 {
		t.Fatalf("frame = %v", frame)
	}
	for _, want := range []string{
		"默认股票账户",
		"交易标签",
		"佣金 万2.354 · 最低 ¥5.00 · 印花税 0.05% · 过户费 0.001%",
		"分析",
	} {
		if !settingsShows(driver, want) {
			t.Fatalf("%q did not paint", want)
		}
	}

	// Add a tag, then delete it; 分析 has no delete control.
	settingsType(t, driver, "新增交易标签", "波段")
	settingsClick(t, driver, "添加")
	if !settingsShows(driver, "波段") {
		t.Fatal("the new tag did not paint")
	}
	if settingsFind(driver, "删除标签 分析") {
		t.Fatal("the default tag offered a delete control")
	}
	tags, err := fixture.Workspace.StockTradeTags(ctx, account.ID)
	if err != nil || !settingsContains(tags, "波段") {
		t.Fatalf("tags = %v, err = %v", tags, err)
	}
	settingsClick(t, driver, "删除标签 波段")
	if settingsShows(driver, "波段") {
		t.Fatal("the deleted tag is still painted")
	}

	// The round-template dialog: recover the default while staying open, then
	// save and close.
	settingsClickNth(t, driver, "修改", 0)
	if !settingsShows(driver, "恢复默认") {
		t.Fatal("the template dialog did not open")
	}
	settingsClick(t, driver, "恢复默认")
	review, err := fixture.Workspace.StockReviewTemplates(ctx, account.ID)
	if err != nil || review.Template != domain.StockReviewTemplateDefault {
		t.Fatalf("review after reset = %+v, err = %v", review, err)
	}
	if !settingsShows(driver, "恢复默认") {
		t.Fatal("恢复默认 closed the template dialog")
	}
	settingsClick(t, driver, "保存")
	if !settingsShows(driver, "复盘模板已保存") {
		t.Fatal("template save toast missing")
	}

	// The fee dialog opens with the four drafts.
	settingsClickNth(t, driver, "修改", 2)
	if !settingsShows(driver, "佣金费率") {
		t.Fatal("the fee dialog did not open")
	}
	settingsClick(t, driver, "取消")

	// Roll the principal operation back.
	settingsClick(t, driver, "回滚")
	if !settingsShows(driver, "已回滚：追加本金") {
		t.Fatal("rollback toast missing")
	}

	// Archive into a new account and follow it.
	settingsClick(t, driver, "归档")
	if !settingsShows(driver, "归档到新账户") {
		t.Fatal("the archive dialog did not open")
	}
	settingsType(t, driver, "新账户名称", "归档账户")
	settingsClickLast(t, driver, "归档")
	if !settingsShows(driver, "股票数据已归档") {
		t.Fatal("archive toast missing")
	}
	items, err := fixture.Workspace.StockAccounts(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("accounts = %+v, err = %v", items, err)
	}
	active := accounts.activeID()
	if active == account.ID || accounts.name(active) != "归档账户" {
		t.Fatalf("active account = %q (%q), want the archived one", active, accounts.name(active))
	}

	// Reset the new account.
	settingsClick(t, driver, "重置")
	if !settingsShows(driver, "重置全部股票数据") {
		t.Fatal("the reset dialog did not open")
	}
	settingsClickLast(t, driver, "重置")
	if !settingsShows(driver, "股票数据已重置") {
		t.Fatal("reset toast missing")
	}

	// Nothing is painted outside the 960-wide frame.
	for _, text := range driver.Snapshot().Texts {
		if text.Bounds.X < -0.5 || text.Bounds.X+text.Bounds.Width > frame.Width+0.5 {
			t.Fatalf("text %q is painted outside the frame: %v", text.Value, text.Bounds)
		}
	}
}

func TestSettingsPanelOperationsFitTheFrame(t *testing.T) {
	fixture := storetest.Open(t)
	ctx := context.Background()
	account, err := fixture.Workspace.CreateStockAccount(ctx, "默认股票账户", "")
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 10; index++ {
		if err := fixture.Workspace.AddStockPrincipal(ctx, account.ID, int64(100*(index+1)), ""); err != nil {
			t.Fatal(err)
		}
	}
	accounts := newAccountStore()
	accounts.reset([]domain.StockAccount{account})
	panel := newSettingsPanel(accounts)
	panel.toaster = &ui.Toaster{}
	panel.setRepository(fixture.Workspace)
	driver := openSettingsDriver(t, panel)
	settingsClick(t, driver, "查看记录")

	frame := driver.Frame()
	rows := 0
	for _, text := range driver.Snapshot().Texts {
		if text.Value != "追加本金" && !strings.HasPrefix(text.Value, "¥") {
			continue
		}
		rows++
		if bottom := text.Bounds.Y + text.Bounds.Height; bottom > frame.Height+0.5 {
			t.Fatalf("operation row %q is painted below the %v frame: %v", text.Value, frame.Height, text.Bounds)
		}
	}
	if rows < 10 {
		t.Fatalf("operation rows painted = %d, want 10", rows)
	}
}

func TestSettingsPanelWithoutAccounts(t *testing.T) {
	fixture := storetest.Open(t)
	accounts := newAccountStore()
	panel := newSettingsPanel(accounts)
	panel.setRepository(fixture.Workspace)
	driver := openSettingsDriver(t, panel)
	if !settingsShows(driver, "还没有股票账户") {
		t.Fatal("empty accounts did not paint the empty state")
	}
}

func openSettingsDriver(t *testing.T, panel *settingsPanel) *app.Driver {
	t.Helper()
	toaster := panel.toaster
	if toaster == nil {
		toaster = &ui.Toaster{}
	}
	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{
		View: func(c ui.C) {
			panel.View(c)
			ui.Toasts(c, toaster)
		},
		Theme: &theme,
	})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 960, Height: 620}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

func settingsShows(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func settingsFind(driver *app.Driver, name string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return true
		}
	}
	return false
}

func settingsClick(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != name || hit.IsLabel {
			continue
		}
		b := hit.Bounds
		if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no control named %q", name)
}

func settingsClickLast(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	var last *gfx.Hit
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			hit := hit
			last = &hit
		}
	}
	if last == nil {
		t.Fatalf("no control named %q", name)
	}
	b := last.Bounds
	if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
		t.Fatal(err)
	}
}

// settingsClickNth picks the nth control with the name from top to bottom; the
// three 修改 buttons belong to the two templates and the fee row.
func settingsClickNth(t *testing.T, driver *app.Driver, name string, index int) {
	t.Helper()
	hits := make([]gfx.Hit, 0, 3)
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			hits = append(hits, hit)
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Bounds.Y < hits[j].Bounds.Y })
	if index < 0 || index >= len(hits) {
		t.Fatalf("wanted control %d named %q, found %d", index, name, len(hits))
	}
	b := hits[index].Bounds
	if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
		t.Fatal(err)
	}
}

func settingsType(t *testing.T, driver *app.Driver, name, value string) {
	t.Helper()
	settingsClick(t, driver, name)
	if err := driver.Type(value); err != nil {
		t.Fatal(err)
	}
}

func settingsContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
