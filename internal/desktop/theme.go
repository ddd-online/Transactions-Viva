package desktop

import "github.com/vivaui/viva/gfx"

// newTheme returns the application theme for the light or dark appearance.
//
// The colors are Viva's own (gfx.LightTheme / gfx.DarkTheme); the old project's
// palette from DESIGN.md is not ported yet. Pages and components read only
// c.Theme(), so when that palette is migrated this is the one place to override
// tokens — do not hardcode colors in pages.
func newTheme(dark bool) gfx.Theme {
	if dark {
		return gfx.DarkTheme()
	}
	return gfx.LightTheme()
}
