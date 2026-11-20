// Package components holds the UI pieces shared by more than one page: the
// pending/empty state, and later the page header and table cells. A piece only
// one page uses belongs in that page's controls subpackage instead
// (internal/pages/<page>/controls).
//
// A component here is an ordinary page function — func Name(c ui.C, ...) ui.Box
// — and never owns a ui.C, Box or goroutine past the paint.
package components

import (
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
)

// Pending is the honest state of a screen whose structure exists but whose
// content has not been migrated yet. It names what will live there, so the
// scaffold never pretends to hold data.
func Pending(c ui.C, title, note string) {
	metrics := c.Theme().Metrics
	ui.Column(c).Grow(1).FillWidth().Pad(metrics.PadSurface * 2).Gap(metrics.GapItem).Center().Children(func() {
		typography.Title(c, title).TextCenter()
		typography.BodyParagraph(c, note, 4).Muted().TextCenter()
	})
}
