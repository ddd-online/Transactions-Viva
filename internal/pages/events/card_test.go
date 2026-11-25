package events

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
)

// TestCardDeleteKeyWaitsForThePointer is the card's delete command: a card nobody
// points at is a card whose list stays quiet, and the key appears when the pointer
// is on the card — the rule every rail in the application follows.
func TestCardDeleteKeyWaitsForThePointer(t *testing.T) {
	m := New()
	m.events = []domain.KeyEvent{
		{Date: "2026-11-20", Title: "测试事件", Content: "测试事件"},
		{Date: "2026-11-21", Title: "第二件事", Content: "第二件事"},
	}
	page := ui.NewPage(ui.PageConfig{View: m.View})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	key := "删除事件「测试事件」"
	if hasHit(driver, key) {
		t.Fatal("a card nobody points at shows its delete key")
	}
	card := hitBounds(t, driver, "测试事件")
	if err := driver.Move(card.X+card.Width/2, card.Y+card.Height/2); err != nil {
		t.Fatal(err)
	}
	if !hasHit(driver, key) {
		t.Fatal("the delete key did not appear under the pointer")
	}
	if hasHit(driver, "删除事件「第二件事」") {
		t.Fatal("the other card's key appeared with it")
	}
}

// hasHit reports whether a control with exactly this name is painted.
func hasHit(driver *app.Driver, name string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return true
		}
	}
	return false
}

// hitBounds finds one control by its name.
func hitBounds(t *testing.T, driver *app.Driver, name string) gfx.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return hit.Bounds
		}
	}
	t.Fatalf("no control named %q", name)
	return gfx.Rect{}
}
