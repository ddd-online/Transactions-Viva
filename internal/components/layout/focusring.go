package layout

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// FocusRing paints the 2px keyboard ring of a full-width row, inside its own
// bounds. A ring drawn around a row that runs from the table's left edge to its
// right is cut away by the table's clip — only a sliver of the outer edge would
// show — so a row wears the ring inset, where nothing can cut it. The wash stays
// what it always meant: the row that is chosen.
func FocusRing(c ui.C, row ui.Box, focused bool) ui.Box {
	if !focused {
		return row
	}
	theme := c.Theme()
	return row.DrawOver(func(d *ui.DrawContext) {
		// A row is square: four 2px bars inside its own bounds, which no table
		// edge can cut.
		r := d.Rect
		d.Canvas.Fill(gfx.Rect{X: r.X, Y: r.Y, Width: r.Width, Height: 2}, theme.Ring)
		d.Canvas.Fill(gfx.Rect{X: r.X, Y: r.Y + r.Height - 2, Width: r.Width, Height: 2}, theme.Ring)
		d.Canvas.Fill(gfx.Rect{X: r.X, Y: r.Y + 2, Width: 2, Height: r.Height - 4}, theme.Ring)
		d.Canvas.Fill(gfx.Rect{X: r.X + r.Width - 2, Y: r.Y + 2, Width: 2, Height: r.Height - 4}, theme.Ring)
	})
}
