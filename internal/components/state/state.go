// Package state names the fills a control paints when the pointer is over it,
// when it is chosen, and while it is held down.
//
// They are layers over the surface a control sits on, not colors of their own:
// the theme turns its ink into a faint hover wash and a stronger selected wash,
// so a row reads the same on the white card and on the shell. The old opaque tokens
// disappeared into the shell in the light appearance and reversed direction on
// it in the dark one. Pressed is the next step on the same ladder — Selected
// over Hover — so a held row never falls back to a surface color.
//
// The accent hue's chosen/held steps live here too, for the controls that are
// tinted rather than washed: the active sort key. Lists and rails use the
// wash, so a chosen row reads the same on every page. DESIGN.md records the
// three opacities; a page never writes one.
package state

import "github.com/vivaui/viva/gfx"

// Hover is the fill under the pointer.
func Hover(theme gfx.Theme) gfx.Color { return theme.Hover }

// Selected is the fill of the chosen row, tab or card.
func Selected(theme gfx.Theme) gfx.Color { return theme.Accent }

// Pressed is the fill while the pointer is held down: one step past Selected.
func Pressed(theme gfx.Theme) gfx.Color { return theme.Accent.Over(theme.Hover) }

// PrimaryTint is the fill of a chosen control that is tinted with the accent
// hue rather than washed with the ink.
func PrimaryTint(theme gfx.Theme) gfx.Color { return theme.Primary.WithAlpha(0.14) }

// PrimaryHover is PrimaryTint under the pointer.
func PrimaryHover(theme gfx.Theme) gfx.Color { return theme.Primary.WithAlpha(0.22) }

// PrimaryPressed is PrimaryTint while the pointer is held down.
func PrimaryPressed(theme gfx.Theme) gfx.Color { return theme.Primary.WithAlpha(0.30) }
