package stock

import (
	"context"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/domain"
	"transactions/internal/repository"
	"transactions/internal/store/storetest"
)

// TestPositionsPanelRendersAndSaves drives the 持仓 screen against a real
// workspace: the rail, the detail metrics and the two write dialogs that do not
// need a live quote source.
func TestPositionsPanelRendersAndSaves(t *testing.T) {
	fixture := storetest.Open(t)
	ctx := context.Background()
	account, err := fixture.Workspace.CreateStockAccount(ctx, "主账户", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.Workspace.AddStockPrincipal(ctx, account.ID, 100000, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.Workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "600000", StockName: "浦发银行",
		TradeType: domain.TradeOpen,
		Fills:     []repository.StockFill{{PriceMilli: 10000, Lots: 2}},
	}); err != nil {
		t.Fatal(err)
	}

	accounts := newAccountStore()
	accounts.reset([]domain.StockAccount{account})
	panel := newPositionsPanel(accounts, nil)
	panel.toaster = &ui.Toaster{}
	panel.setRepository(fixture.Workspace)

	driver := positionsTestDriver(t, panel)
	frame := driver.Frame()
	if frame.Width != 960 || frame.Height != 620 {
		t.Fatalf("frame = %v", frame)
	}
	for _, want := range []string{
		"浦发银行", "600000", "2 手", "现价", "浮动盈亏", "本轮费率",
		"交易台账", "每日复盘", "本轮复盘", "成交记录",
	} {
		if !positionsTestShows(driver, want) {
			t.Fatalf("%q did not paint", want)
		}
	}

	// The order dialog opens from 建仓 and closes with 取消.
	positionsTestClick(t, driver, "建仓")
	if !positionsTestShows(driver, "委托建仓") {
		t.Fatal("the order dialog did not open")
	}
	positionsTestClick(t, driver, "取消")
	if positionsTestShows(driver, "委托建仓") {
		t.Fatal("the order dialog did not close")
	}

	// 填写台账 writes the five text fields and rereads the page.
	positionsTestClick(t, driver, "填写台账")
	if !panel.ledgerEdit.open {
		t.Fatalf("the ledger dialog state did not open; hits=%v", positionsTestHitNames(driver))
	}
	if !positionsTestShows(driver, "当时为什么决定买这一轮") {
		t.Fatal("the ledger dialog did not open")
	}
	positionsTestType(t, driver, "为什么买", "首板放量")
	positionsTestType(t, driver, "止损", "9.00")
	positionsTestType(t, driver, "止盈", "12.00")
	if panel.ledgerEdit.draft.stopLoss != "9.00" || panel.ledgerEdit.draft.takeProfit != "12.00" {
		t.Fatalf("ledger draft levels = %+v", panel.ledgerEdit.draft)
	}
	positionsTestClick(t, driver, "保存")
	if !positionsTestShows(driver, "台账已保存") {
		t.Fatal("the ledger save toast is missing")
	}
	if !positionsTestShows(driver, "首板放量") || !positionsTestShows(driver, "盈亏比") {
		t.Fatal("the saved ledger did not paint as a sheet")
	}

	// 写今日 creates the current round's day review. The ledger sheet is
	// taller than the 620 px viewport, so bring 每日复盘 into view first.
	if err := driver.Scroll(gfx.ScrollEvent{X: 700, Y: 400, DeltaY: 600}); err != nil {
		t.Fatal(err)
	}
	positionsTestClick(t, driver, "写今日")
	positionsTestType(t, driver, "复盘", "今天放量")
	positionsTestClick(t, driver, "保存")
	if !positionsTestShows(driver, "复盘已保存") {
		t.Fatal("the daily review toast is missing")
	}
	reviews, err := fixture.Workspace.StockDayReviews(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 || reviews[0].RoundID != "" || !strings.Contains(reviews[0].Review, "今天放量") {
		t.Fatalf("day reviews = %+v", reviews)
	}

	// Back to the top for the header's 加仓 action.
	if err := driver.Scroll(gfx.ScrollEvent{X: 700, Y: 400, DeltaY: -600}); err != nil {
		t.Fatal(err)
	}
	// 加仓 records a second order through the dialog; its submit button is the
	// last control named 加仓 (the header has one too).
	positionsTestClick(t, driver, "加仓")
	if !positionsTestShows(driver, "委托加仓") {
		t.Fatal("the add-position dialog did not open")
	}
	positionsTestType(t, driver, "成交价（元）", "10.5")
	positionsTestType(t, driver, "手数", "1")
	positionsTestClickLast(t, driver, "加仓")
	if !positionsTestShows(driver, "成交已记录") {
		t.Fatal("the order toast is missing")
	}
	positions, err := fixture.Workspace.StockPositions(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 1 || positions[0].Quantity != 300 {
		t.Fatalf("positions after 加仓 = %+v", positions)
	}
}

// TestTradeDialogKeepsItsFieldsOffTheScrollClip measures the 委托 form: its two
// fields span the dialog's body less one ClipInset on each side. A field that
// starts exactly on the edge of the area that scrolls the form has its hairline
// border on that edge, where the window does not paint it, so the form would show
// without a left border.
func TestTradeDialogKeepsItsFieldsOffTheScrollClip(t *testing.T) {
	fixture := storetest.Open(t)
	account, err := fixture.Workspace.CreateStockAccount(context.Background(), "主账户", "")
	if err != nil {
		t.Fatal(err)
	}
	accounts := newAccountStore()
	accounts.reset([]domain.StockAccount{account})
	panel := newPositionsPanel(accounts, nil)
	panel.toaster = &ui.Toaster{}
	panel.setRepository(fixture.Workspace)
	driver := positionsTestDriver(t, panel)

	positionsTestClick(t, driver, "建仓")
	if !positionsTestShows(driver, "成交明细") {
		t.Fatal("the order dialog did not open")
	}
	name := positionsTestBounds(t, driver, "股票名称")
	code := positionsTestBounds(t, driver, "股票代码")

	metrics := gfx.DefaultMetrics()
	// The dialog is 640 wide; its body is that less the surface padding on each
	// side, and the form keeps one ClipInset clear of the scroll area's edges.
	body := float32(640) - 2*(metrics.PadSurface+metrics.GapItem)
	if got, want := code.X+code.Width-name.X, body-2*layout.ClipInset; got != want {
		t.Fatalf("the form spans %v of the %v body; want %v, one ClipInset off each clip edge", got, body, want)
	}
}

func positionsTestClickLast(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	var last *gfx.Hit
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != name || hit.IsLabel {
			continue
		}
		hit := hit
		last = &hit
	}
	if last == nil {
		t.Fatalf("no control named %q", name)
	}
	b := last.Bounds
	if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
		t.Fatal(err)
	}
}

func positionsTestDriver(t *testing.T, panel *positionsPanel) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		panel.View(c)
		ui.Toasts(c, panel.toaster)
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 960, Height: 620}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.Close() })
	return driver
}

func positionsTestShows(driver *app.Driver, want string) bool {
	snapshot := driver.Snapshot()
	for _, label := range snapshot.Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	for _, text := range snapshot.Texts {
		if strings.Contains(text.Value, want) {
			return true
		}
	}
	return false
}

func positionsTestClick(t *testing.T, driver *app.Driver, name string) {
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

func positionsTestType(t *testing.T, driver *app.Driver, name, value string) {
	t.Helper()
	positionsTestClick(t, driver, name)
	if err := driver.Type(value); err != nil {
		t.Fatal(err)
	}
}

func positionsTestBounds(t *testing.T, driver *app.Driver, name string) gfx.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return hit.Bounds
		}
	}
	t.Fatalf("no control named %q", name)
	return gfx.Rect{}
}

func positionsTestHitNames(driver *app.Driver) []string {
	names := make([]string, 0, 16)
	for _, hit := range driver.Snapshot().Hits {
		if !hit.IsLabel {
			names = append(names, hit.Label)
		}
	}
	return names
}
