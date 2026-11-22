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
	bitmap := gfx.Bitmap{ID: 1, Width: 64, Height: 36, Stride: 64 * 4, Pix: make([]byte, 64*36*4)}
	viewLoads := 0
	model.events.SetCommands(events.Commands{
		LoadImage: func(string) (gfx.Bitmap, error) { return bitmap, nil },
		LoadView: func(string) (gfx.Bitmap, error) {
			viewLoads++
			return bitmap, nil
		},
	})
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
	waitCache(t, driver, model, want, "after the first day")
	clickCache(t, driver, titles[1])
	waitCache(t, driver, model, want, "after switching days")
	for index := 1; index < 4; index++ {
		clickCache(t, driver, fmt.Sprintf("查看第 %d 张图片，共 4 张", index+1))
		waitCache(t, driver, model, want, "after switching photos")
	}
	clickCache(t, driver, titles[0])
	waitCache(t, driver, model, want, "after returning to the first day")
	if viewLoads == 0 {
		t.Fatal("the stage never asked for a view copy")
	}
}

func waitCache(t *testing.T, driver *app.Driver, model *Model, want int, when string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if got := model.events.CachedImageBytes(); got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: decoded images hold %d bytes, want %d",
				when, model.events.CachedImageBytes(), want)
		}
		if err := driver.Paint(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
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
