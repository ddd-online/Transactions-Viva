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

func BenchmarkEventDetailPaint(b *testing.B) {
	for _, count := range []int{14, 120, 400} {
		b.Run(fmt.Sprintf("%d-images", count), func(b *testing.B) {
			fixture := storetest.Open(b)
			model := New()
			if err := model.adoptWorkspace(fixture.Workspace); err != nil {
				b.Fatal(err)
			}
			date := time.Now().Format("2006-01-02")
			title := "Benchmark event"
			if _, err := model.workspace.SaveKeyEvent(context.Background(), date, title, "抽奖与聚餐。\n\n今年的节目比往年都热闹。", ""); err != nil {
				b.Fatal(err)
			}
			for index := range count {
				name := fmt.Sprintf("bench-image-%04d", index)
				if _, err := model.workspace.CreateKeyEventImage(context.Background(), domain.KeyEventImage{
					ID:        name,
					EventDate: date,
					FilePath:  "bench/" + name + ".jpg",
					ThumbPath: "bench/" + name + "-thumb.jpg",
				}); err != nil {
					b.Fatal(err)
				}
			}
			bitmap := gfx.Bitmap{ID: 1, Width: 160, Height: 90, Stride: 160 * 4, Pix: make([]byte, 160*90*4)}
			model.events.SetCommands(events.Commands{
				LoadImage: func(string) (gfx.Bitmap, error) { return bitmap, nil },
			})
			page := ui.NewPage(ui.PageConfig{Tree: &model.tree, View: model.page, Theme: &model.theme})
			driver, err := app.NewDriver(ui.Configure(page, newConfig(model)), nil)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(driver.Close)
			clickBench(b, driver, "事件")
			clickBench(b, driver, title)
			if err := driver.Paint(); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for range b.N {
				if err := driver.Paint(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func clickBench(b *testing.B, driver *app.Driver, name string) {
	b.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name {
			if err := driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				b.Fatal(err)
			}
			return
		}
	}
	b.Fatalf("no control %q", name)
}
