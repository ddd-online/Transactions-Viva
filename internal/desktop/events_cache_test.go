package desktop

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
	"transactions/internal/pages/events"
	"transactions/internal/store/storetest"
)

// TestEventImageCacheHoldsOnlyTheSelectedDay guards the decoded-image budget:
// the page keeps the selected day's thumbnails, one full photo and one backdrop,
// and releases them when another day or photo is chosen. A view that kept every
// visited day and every viewed full photo grew without bound.
func TestEventImageCacheHoldsOnlyTheSelectedDay(t *testing.T) {
	fixture := storetest.Open(t)
	model := New()
	if err := model.adoptWorkspace(fixture.Workspace); err != nil {
		t.Fatal(err)
	}
	bitmap := gfx.Bitmap{ID: 1, Width: 160, Height: 90, Stride: 160 * 4, Pix: make([]byte, 160*90*4)}
	model.events.SetCommands(events.Commands{LoadImage: func(string) (gfx.Bitmap, error) { return bitmap, nil }})
	year := time.Now().Year()
	days := []string{fmt.Sprintf("%d-11-22", year), fmt.Sprintf("%d-11-21", year)}
	titles := []string{"缓存日 A", "缓存日 B"}
	for position, day := range days {
		if _, err := model.workspace.SaveKeyEvent(context.Background(), day, titles[position], "正文", ""); err != nil {
			t.Fatal(err)
		}
		for index := range 4 {
			name := fmt.Sprintf("%s-%d", day, index)
			if _, err := model.workspace.CreateKeyEventImage(context.Background(), domain.KeyEventImage{
				ID: name, EventDate: day, FilePath: "bench/" + name + ".jpg", ThumbPath: "bench/" + name + "-thumb.jpg",
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	page := ui.NewPage(ui.PageConfig{Tree: &model.tree, View: model.page, Theme: &model.theme})
	driver, err := app.NewDriver(ui.Configure(page, newConfig(model)), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	// Four thumbnails, one selected display and one backdrop.
	want := 6 * len(bitmap.Pix)
	clickCache(t, driver, "事件")
	clickCache(t, driver, titles[0])
	assertCacheBytes(t, model, want, "after the first day")
	clickCache(t, driver, titles[1])
	assertCacheBytes(t, model, want, "after switching days")
	for index := 1; index < 4; index++ {
		clickCache(t, driver, fmt.Sprintf("查看第 %d 张图片，共 4 张", index+1))
		assertCacheBytes(t, model, want, "after switching photos")
	}
	clickCache(t, driver, titles[0])
	assertCacheBytes(t, model, want, "after returning to the first day")
}

func assertCacheBytes(t *testing.T, model *Model, want int, when string) {
	t.Helper()
	if got := model.events.CachedImageBytes(); got != want {
		t.Fatalf("%s: decoded images hold %d bytes, want %d", when, got, want)
	}
}

func clickCache(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name {
			if err := driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control %q", name)
}
