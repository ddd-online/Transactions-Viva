package layout

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestPagePutsToolbarAboveContentAndCentersIt(t *testing.T) {
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		Page(c, func(c ui.C) {
			Toolbar(c, func() {
				ui.Button(c, "left").Outline()
				ui.Button(c, "right").Outline()
			})
		}, func(c ui.C) {
			ui.Row(c).FillWidth().Height(20).Hit(ui.ID(c), "body").
				Children(func() { ui.Text(c, "body") })
		})
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 600, Height: 400}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	hits := map[string]gfx.Rect{}
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "left" || hit.Label == "right" || hit.Label == "body" {
			hits[hit.Label] = hit.Bounds
		}
	}
	left, right, body := hits["left"], hits["right"], hits["body"]
	if left.Width == 0 || right.Width == 0 || body.Width == 0 {
		t.Fatalf("missing hits: %+v", hits)
	}
	if left.Y+left.Height/2 != right.Y+right.Height/2 {
		t.Fatalf("toolbar controls are not vertically centered: %+v vs %+v", left, right)
	}
	if body.Y < ToolbarHeight+1 {
		t.Fatalf("content starts at %.1f, want below the %.0fpx toolbar", body.Y, ToolbarHeight)
	}
}

func TestTableHeaderKeepsLabelsInsideTheBand(t *testing.T) {
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		Page(c, nil, func(c ui.C) {
			TableHeader(c, TableHeaderSpec{PadX: 16, Gap: 8}, func(c ui.C) {
				ui.Button(c, "标头").Ghost()
			})
		})
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 600, Height: 400}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != "标头" {
			continue
		}
		center := hit.Bounds.Y + hit.Bounds.Height/2
		if center < 0 || center > TableHeaderHeight {
			t.Fatalf("table header label center %.1f is outside the %.0fpx band", center, TableHeaderHeight)
		}
		return
	}
	t.Fatal("table header label not painted")
}

func TestSectionHeaderKeepsItsTitleOnThePageInset(t *testing.T) {
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		Page(c, nil, func(c ui.C) {
			SectionHeader(c, Inset).Children(func() {
				ui.Button(c, "标题").Ghost()
				SectionCount(c, "3 只")
			})
			ui.Row(c).Height(20).FillWidth().PadX(Inset).Children(func() {
				ui.Button(c, "内容").Ghost()
			})
		})
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 600, Height: 400}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	hits := map[string]gfx.Rect{}
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "标题" || hit.Label == "内容" {
			hits[hit.Label] = hit.Bounds
		}
	}
	title, content := hits["标题"], hits["内容"]
	if title.Width == 0 || content.Width == 0 {
		t.Fatalf("missing hits: %+v", hits)
	}
	if title.X != content.X {
		t.Fatalf("header title starts at %.1f, content at %.1f", title.X, content.X)
	}
	if center := title.Y + title.Height/2; center != SectionHeaderHeight/2 {
		t.Fatalf("header title center is %.1f, want %.1f", center, SectionHeaderHeight/2)
	}
}

func TestMetricGridKeepsOneColumnWidthInEveryRow(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e", "f"}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		Page(c, nil, func(c ui.C) {
			MetricGrid(c, len(names), 4, func(c ui.C, index int) {
				ui.Row(c).Grow(1).Basis(0).Height(20).
					Hit(ui.ID(c), names[index])
			})
		})
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 600, Height: 400}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	hits := map[string]gfx.Rect{}
	for _, hit := range driver.Snapshot().Hits {
		hits[hit.Label] = hit.Bounds
	}
	first, last := hits["a"], hits["d"]
	if first.Width == 0 || last.Width != first.Width {
		t.Fatalf("first row cells are not equal: %+v vs %+v", first, last)
	}
	second := hits["e"]
	if second.Width != first.Width {
		t.Fatalf("short row cell is %.1f wide, want %.1f", second.Width, first.Width)
	}
	if second.X != first.X {
		t.Fatalf("short row starts at %.1f, first row at %.1f", second.X, first.X)
	}
}
