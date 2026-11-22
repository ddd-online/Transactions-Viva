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
