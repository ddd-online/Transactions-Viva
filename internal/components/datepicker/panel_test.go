package datepicker

import (
	"fmt"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// TestPanelPicksInsideADialog drives the dialog's date field where Day cannot
// work: inside a ui.Modal. Viva's calendar opens over the dialog, takes the
// press on a day, writes it back and closes, and the dialog stays up — the
// framework's own popover would have dismissed it instead.
func TestPanelPicksInsideADialog(t *testing.T) {
	open := true
	value := ""
	var picked []string
	panel := Panel{}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Modal(c, &open, "新增记录", ui.ModalSpec{Width: 640}, func() {
			panel.Field(c, &value, PanelSpec{
				Placeholder: "选择日期", Fill: true, Viewport: c.Bounds(),
				OnPick: func(day string) { picked = append(picked, day) },
			})
		}, nil)
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	// The field opens Viva's calendar on the month of today.
	clickLabel(t, driver, "选择日期")
	now := time.Now()
	if !painted(driver, fmt.Sprintf("%d月 %d", int(now.Month()), now.Year())) {
		t.Fatal("the panel did not open on this month")
	}
	if !named(driver, "上个月") || !named(driver, "下个月") {
		t.Fatal("the month's arrows are not in Chinese")
	}
	if !painted(driver, "一") || !painted(driver, "日") {
		t.Fatal("the week does not start on Monday in Chinese")
	}

	// The arrows move the month the grid shows, not the dialog.
	next := now.AddDate(0, 1, 0)
	clickLabel(t, driver, "下个月")
	if !painted(driver, fmt.Sprintf("%d月 %d", int(next.Month()), next.Year())) {
		t.Fatal("下个月 did not move the panel a month forward")
	}
	clickLabel(t, driver, "上个月")

	// A press on a day belongs to the dialog: it picks the day, closes the panel
	// and leaves the dialog open.
	want := now.Format("2006-01") + "-15"
	clickLabel(t, driver, fmt.Sprintf("%d 年 %d 月 15 日", now.Year(), int(now.Month())))
	if value != want {
		t.Fatalf("value = %q, want %q", value, want)
	}
	if len(picked) != 1 || picked[0] != want {
		t.Fatalf("OnPick = %q, want [%q]", picked, want)
	}
	if !open {
		t.Fatal("picking a day dismissed the dialog")
	}
	if named(driver, "关闭日期选择") {
		t.Fatal("picking a day left the panel open")
	}
	if !painted(driver, want) {
		t.Fatalf("the field did not paint the picked date %q", want)
	}
}

// TestPanelDismissesWithoutPicking closes the panel with a press elsewhere in
// the dialog: the press belongs to the panel's backdrop, so the day keeps its
// value and the dialog stays open.
func TestPanelDismissesWithoutPicking(t *testing.T) {
	open := true
	value := "2026-11-20"
	panel := Panel{}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Modal(c, &open, "编辑记录", ui.ModalSpec{Width: 640}, func() {
			panel.Field(c, &value, PanelSpec{Placeholder: "选择日期", Fill: true, Viewport: c.Bounds()})
		}, nil)
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	clickLabel(t, driver, "2026-11-20")
	if !named(driver, "关闭日期选择") {
		t.Fatal("the panel did not open")
	}
	// The dialog's title is over the panel's backdrop and under no control: the
	// press closes the panel and nothing else.
	var title struct{ X, Y float32 }
	found := false
	for _, text := range driver.Snapshot().Texts {
		if text.Value == "编辑记录" {
			title.X, title.Y, found = text.Bounds.X+text.Bounds.Width/2, text.Bounds.Y+text.Bounds.Height/2, true
			break
		}
	}
	if !found {
		t.Fatal("the dialog painted no title")
	}
	if !open {
		t.Fatal("the dialog closed before the press")
	}
	if err := driver.Click(title.X, title.Y); err != nil {
		t.Fatal(err)
	}
	if named(driver, "关闭日期选择") {
		t.Fatal("a press elsewhere in the dialog left the panel open")
	}
	if !open {
		t.Fatal("a press elsewhere in the dialog closed it")
	}
	if value != "2026-11-20" {
		t.Fatalf("value = %q after dismissing, want it unchanged", value)
	}
}
