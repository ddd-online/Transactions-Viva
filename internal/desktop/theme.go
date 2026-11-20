package desktop

import "github.com/vivaui/viva/gfx"

// newTheme returns the application theme for the light or dark appearance.
//
// The colors are Viva's own (gfx.LightTheme / gfx.DarkTheme); the old project's
// palette from DESIGN.md is not ported yet. Pages and components read only
// c.Theme(), so when that palette is migrated this is the one place to override
// tokens — do not hardcode colors in pages.
func newTheme(dark bool) gfx.Theme {
	theme := gfx.LightTheme()
	if dark {
		theme = gfx.DarkTheme()
	}
	// Viva paints the hover of menu and popup rows with Accent. The stock token
	// sits almost on the Popover color (#f5f5f5 on #ffffff, #262626 on #171717),
	// so the highlight is technically there but reads as no hover at all. Keep
	// the neutral hue and deepen it until a submenu row is unmistakable.
	if dark {
		theme.Accent = gfx.RGB(52, 52, 52)
	} else {
		theme.Accent = gfx.RGB(230, 230, 230)
	}
	return theme
}
