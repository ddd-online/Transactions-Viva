package datepicker

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestDayGrainYearArrowsSwitchAYear pins the pair of double chevrons beside the
// month arrows: they move the day grid a whole year, while the single arrows
// keep moving a month.
func TestDayGrainYearArrowsSwitchAYear(t *testing.T) {
	start, end := "2026-11-20", "2026-11-20"
	state := NewState()
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			Picker(c, &state, Spec{
				Label: "日期范围", Start: start, End: end,
				Grain: ByDay, Grains: true, Periods: true,
				Viewport: gfx.Rect{Width: 1200, Height: 800},
				OnRange:  func(from, to string) { start, end = from, to },
			})
		})
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	clickLabel(t, driver, "2026-11-20")
	if !painted(driver, "2026 年 11 月") {
		t.Fatal("the day panel did not open on the chosen month")
	}

	clickLabel(t, driver, "上一年")
	if !painted(driver, "2025 年 11 月") {
		t.Fatal("上一年 did not move the day grid a year back")
	}
	clickLabel(t, driver, "下一年")
	clickLabel(t, driver, "下一年")
	if !painted(driver, "2027 年 11 月") {
		t.Fatal("下一年 did not move the day grid a year forward")
	}

	clickLabel(t, driver, "上个月")
	if !painted(driver, "2027 年 10 月") {
		t.Fatal("上个月 stopped moving the day grid a month")
	}
}

func TestDayCellPicksDate(t *testing.T) {
	start := ""
	state := NewState()
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			Picker(c, &state, Spec{
				Label: "开始时间", Start: start, Grain: ByDay,
				Placeholder: "不填", Fill: true, CloseOnPick: true,
				Viewport: gfx.Rect{Width: 1200, Height: 800},
				OnRange:  func(from, _ string) { start = from },
			})
		})
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	clickLabel(t, driver, "不填")
	want := time.Now().Format("2006-01") + "-15"
	clickLabel(t, driver, "15")
	if start != want {
		t.Fatalf("picked = %q, want %q", start, want)
	}
	if !painted(driver, want) {
		t.Fatalf("the field did not paint the picked date %q", want)
	}
}

// painted reports whether the frame paints a text, the panel's title included.
func painted(driver *app.Driver, want string) bool {
	for _, text := range driver.Snapshot().Texts {
		if text.Value == want {
			return true
		}
	}
	return false
}

// clickLabel presses the middle of the first control with the name, as a user
// would; the picker's field is named by its value, the panel's arrows by their
// action.
func clickLabel(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != name || hit.IsLabel {
			continue
		}
		bounds := hit.Bounds
		if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no control named %q on screen", name)
}
