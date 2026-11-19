// Package components holds the UI pieces shared by more than one page: the
// pending/empty state and, as the migration goes on, the settings row, page
// header, table cells and dialog shells.
//
// A component here is an ordinary page function — func Name(c ui.C, ...) ui.Box
// — and never owns a ui.C, Box or goroutine past the paint.
package components

import (
	"github.com/vivaui/viva/ui"
)

// Pending is the honest state of a screen whose structure exists but whose
// content has not been migrated yet. It names what will live there, so the
// scaffold never pretends to hold data.
func Pending(c ui.C, title, note string) {
	metrics := c.Theme().Metrics
	ui.Column(c).Grow(1).FillWidth().Pad(metrics.PadSurface * 2).Gap(metrics.GapItem).Center().Children(func() {
		ui.Text(c, title).FontSize(metrics.FontSizeL).Bold().TextCenter()
		ui.Paragraph(c, note, 4).Muted().TextCenter()
	})
}
