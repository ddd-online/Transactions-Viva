package desktop

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// textY finds the first painted line whose value contains want.
func textY(driver *app.Driver, want string) (float32, bool) {
	for _, text := range driver.Snapshot().Texts {
		if strings.Contains(text.Value, want) {
			return text.Bounds.Y, true
		}
	}
	return 0, false
}

// TestSettingsPageScrollsFromTheKeyboard: a window shorter than the section still
// reaches the whole page, because PageDown moves the page's scroller. Without it
// the content below the fold is not only out of sight — the frame clips its hits,
// so it cannot take the focus either.
func TestSettingsPageScrollsFromTheKeyboard(t *testing.T) {
	driver, _ := open(t)
	if err := driver.Resize(960, 620); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "设置")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	before, ok := textY(driver, "功能开关")
	if !ok {
		t.Fatal("the settings page shows no 功能开关")
	}
	if err := driver.Key(gfx.KeyPageDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	after, ok := textY(driver, "功能开关")
	if !ok {
		t.Fatal("功能开关 scrolled off the page")
	}
	if after >= before {
		t.Fatalf("PageDown left 功能开关 at y=%g, want it above %g", after, before)
	}
}
