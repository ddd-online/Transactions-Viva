package datepicker

import (
	"fmt"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestDayFieldPicksAndWritesBack drives one single-date field: it opens on the
// clock's month, speaks Chinese, and hands the page the local YYYY-MM-DD string
// it stores.
func TestDayFieldPicksAndWritesBack(t *testing.T) {
	value := ""
	day := Day{}
	var picked []string
	driver := openDay(t, func(c ui.C) {
		day.Field(c, &value, DaySpec{
			Label: "开始时间", Placeholder: "不填", Fill: true,
			OnPick: func(day string) { picked = append(picked, day) },
		})
	})

	// An empty field shows its placeholder and opens on today's month.
	clickLabel(t, driver, "不填")
	now := time.Now()
	title := fmt.Sprintf("%d月 %d", int(now.Month()), now.Year())
	if !painted(driver, title) {
		t.Fatalf("the month did not open on the clock's month: no %q", title)
	}
	if !named(driver, "上个月") || !named(driver, "下个月") {
		t.Fatal("the month's arrows are not in Chinese")
	}
	if !painted(driver, "日") || !painted(driver, "一") {
		t.Fatal("the week does not start on Monday in Chinese")
	}

	// The 15th belongs to the month the panel opened on, whenever the test runs.
	want := now.Format("2006-01") + "-15"
	clickDay(t, driver, 15)
	if value != want {
		t.Fatalf("value = %q, want %q", value, want)
	}
	if len(picked) != 1 || picked[0] != want {
		t.Fatalf("OnPick = %q, want [%q]", picked, want)
	}
	if !painted(driver, want) {
		t.Fatalf("the field did not paint the picked date %q", want)
	}
}

// TestDayFieldKeepsTheDateOutOfRange checks a field's Max: a day past it is
// dimmed and takes no press, so the page's value cannot leave the range.
func TestDayFieldKeepsTheDateOutOfRange(t *testing.T) {
	now := time.Now()
	first := now.Format("2006-01") + "-01"
	value := first
	day := Day{}
	driver := openDay(t, func(c ui.C) {
		day.Field(c, &value, DaySpec{Label: "日期", Max: first})
	})

	clickLabel(t, driver, first)
	// The 2nd is in this month's grid whatever today is, and it is one day past
	// the limit.
	clickDay(t, driver, 2)
	if value != first {
		t.Fatalf("value = %q after a press past Max, want it to stay %q", value, first)
	}
}

// openDay drives one page that declares a single date field.
func openDay(t *testing.T, field func(c ui.C)) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Fill().Children(func() { field(c) })
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

// named reports whether a control is on screen under that name, painted or not.
func named(driver *app.Driver, name string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return true
		}
	}
	return false
}

// clickDay presses the day cell with a number. The day grid names each cell by
// its number alone; a cell past the field's Max is disabled and takes no press.
func clickDay(t *testing.T, driver *app.Driver, day int) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != fmt.Sprint(day) || hit.Role != gfx.RoleButton {
			continue
		}
		bounds := hit.Bounds
		if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no day cell named %d on screen", day)
}
