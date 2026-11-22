package desktop

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestStockSettingsScrollsAtSmallWindow(t *testing.T) {
	driver, _ := open(t)
	if err := driver.Resize(960, 620); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "设置")
	click(t, driver, "股票设置")

	before := textTop(driver, "重置股票数据")
	if before <= 0 {
		t.Fatal("reset row is not on screen")
	}
	titleBefore := textTopAfter(driver, "股票设置", 300)
	if titleBefore <= 0 {
		t.Fatal("section title is not on screen")
	}
	if err := driver.Scroll(gfx.ScrollEvent{
		X: 700, Y: 400, DeltaY: 500, Viewport: gfx.Size{Width: 960, Height: 620},
	}); err != nil {
		t.Fatal(err)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	after := textTop(driver, "重置股票数据")
	if after >= before {
		t.Fatalf("settings did not scroll: reset row y %v -> %v", before, after)
	}
	if titleAfter := textTopAfter(driver, "股票设置", 300); titleAfter != titleBefore {
		t.Fatalf("section title moved: y %v -> %v", titleBefore, titleAfter)
	}
}

func textTop(driver *app.Driver, want string) float32 {
	return textTopAfter(driver, want, 0)
}

func textTopAfter(driver *app.Driver, want string, minX float32) float32 {
	for _, text := range driver.Snapshot().Texts {
		if strings.Contains(text.Value, want) && text.Bounds.X >= minX {
			return text.Bounds.Y
		}
	}
	return 0
}
