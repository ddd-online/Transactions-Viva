package stock

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
)

// ladderTestEntries is one rung's stocks: enough of them that a card cannot
// show all of them in the test window.
func ladderTestEntries(prefix string, count int) []domain.StockLimitEntry {
	items := make([]domain.StockLimitEntry, count)
	for index := range items {
		items[index] = domain.StockLimitEntry{
			StockCode: fmt.Sprintf("6000%02d", index),
			StockName: fmt.Sprintf("%s%02d", prefix, index),
		}
	}
	return items
}

// ladderTestDriver paints only the rungs: the market day is set directly, so no
// effect fetches anything and the card layout is the thing under test.
func ladderTestDriver(t *testing.T, rungs []domain.StockLimitRung, width, height int) *app.Driver {
	t.Helper()
	maxLevel := 0
	for _, rung := range rungs {
		maxLevel = max(maxLevel, rung.Level)
	}
	panel := &ladderPanel{}
	panel.ladder = domain.StockLimitLadder{
		Metrics: domain.StockLimitMetrics{MaxLevel: &maxLevel},
		Rungs:   rungs,
	}
	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Fill().Children(func() { panel.rungs(c) })
	}, Theme: &theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: width, Height: height}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

func ladderTestLabelY(t *testing.T, driver *app.Driver, value string) (float32, bool) {
	t.Helper()
	for _, label := range driver.Snapshot().Labels {
		if label.Value == value {
			return label.Bounds.Y, true
		}
	}
	return 0, false
}

// TestLadderCardsFillTheirRowAndScrollTheirStocks: the rung cards share the
// window's height, and the stocks inside a card scroll while its own header and
// the card beside it stay put.
func TestLadderCardsFillTheirRowAndScrollTheirStocks(t *testing.T) {
	driver := ladderTestDriver(t, []domain.StockLimitRung{
		{Level: 1, Entries: ladderTestEntries("甲", 30)},
		{Level: 2, Entries: ladderTestEntries("乙", 30)},
	}, 1440, 900)

	header, ok := ladderTestLabelY(t, driver, "1")
	if !ok {
		t.Fatal("the first rung's header did not paint")
	}
	first, ok := ladderTestLabelY(t, driver, "甲00")
	if !ok {
		t.Fatal("the first rung's first stock did not paint")
	}
	moved, ok := ladderTestLabelY(t, driver, "甲05")
	if !ok {
		t.Fatal("a stock inside the first rung did not paint")
	}
	other, ok := ladderTestLabelY(t, driver, "乙00")
	if !ok {
		t.Fatal("the second rung's first stock did not paint")
	}

	if err := driver.Scroll(gfx.ScrollEvent{X: 100, Y: 500, DeltaY: 200}); err != nil {
		t.Fatal(err)
	}
	if first != other {
		t.Fatalf("the two cards' first stocks start at %.1f and %.1f; their headers differ in height", first, other)
	}
	if after, _ := ladderTestLabelY(t, driver, "1"); after != header {
		t.Fatalf("the rung header moved from %.1f to %.1f with its stocks", header, after)
	}
	if after, _ := ladderTestLabelY(t, driver, "乙00"); after != other {
		t.Fatalf("the card beside it scrolled too: %.1f to %.1f", other, after)
	}
	if after, ok := ladderTestLabelY(t, driver, "甲05"); !ok || after >= moved {
		t.Fatalf("the stock stayed at %.1f; the card did not scroll", moved)
	}
	if after, visible := ladderTestLabelY(t, driver, "甲00"); visible && after >= first {
		t.Fatalf("the first stock stayed at %.1f; the card did not scroll", first)
	}
}

// TestLadderRungsShareTheHeight: one column of two rungs puts the second card
// halfway down, not below the first card's whole stock list.
func TestLadderRungsShareTheHeight(t *testing.T) {
	driver := ladderTestDriver(t, []domain.StockLimitRung{
		{Level: 1, Entries: ladderTestEntries("甲", 30)},
		{Level: 2, Entries: ladderTestEntries("乙", 30)},
	}, 500, 800)

	one, ok := ladderTestLabelY(t, driver, "1")
	if !ok {
		t.Fatal("the first rung's header did not paint")
	}
	two, ok := ladderTestLabelY(t, driver, "2")
	if !ok {
		t.Fatal("the second rung's header did not paint")
	}
	if one > 100 {
		t.Fatalf("the first rung starts at %.1f, not at the top", one)
	}
	if two < 300 || two > 600 {
		t.Fatalf("the second rung starts at %.1f, not on the second half of the height", two)
	}
}
