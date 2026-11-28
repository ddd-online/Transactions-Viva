// Package components holds the UI pieces shared by more than one page: the
// pending/empty state, and later the page header and table cells. A piece only
// one page uses belongs in that page's controls subpackage instead
// (internal/pages/<page>/controls).
//
// A component here is an ordinary page function — func Name(c ui.C, ...) ui.Box
// — and never owns a ui.C, Box or goroutine past the paint.
package components

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/state"
	"transactions/internal/components/typography"
)

// IconKey is one bare icon command: the 28px key a rail header or a toolbar puts
// beside its title — the theme's control corner, the pointer wash, the keyboard ring,
// a 15px mark and the command's name as its tooltip. The caller reads Clicked itself
// through the returned box, so a command that can be on adds its own background.
//
// The diary rail and the notes page both keep their commands this way, so the two
// headers cannot drift apart.
func IconKey(c ui.C, label string, icon gfx.Icon, action func()) ui.Box {
	theme := c.Theme()
	button := ui.ButtonBase(c, label).Size(28, 28).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(state.Hover(*theme))).
		Pressed(ui.Look{}.Background(state.Pressed(*theme))).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Tooltip(label)
	button.Children(func() {
		ui.Icon(c, icon).Size(15, 15)
	})
	if button.Clicked() {
		action()
	}
	return button
}

// Pending is the honest state of a screen whose structure exists but whose
// content has not been implemented yet. It names what will live there, so the
// scaffnever pretends to hexisting data.
func Pending(c ui.C, title, note string) {
	metrics := c.Theme().Metrics
	ui.Column(c).Grow(1).FillWidth().Pad(metrics.PadSurface * 2).Gap(metrics.GapItem).Center().Children(func() {
		typography.Title(c, title).TextCenter()
		typography.BodyParagraph(c, note, 4).Muted().TextCenter()
	})
}
