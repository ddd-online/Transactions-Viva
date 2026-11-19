package controls

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Group is a settings card: a titled surface holding rows separated by
// hairlines. It follows the reference application's settings page, where each
// topic is a rounded, bordered card of rows rather than one long form.
//
// The caller declares the rows and the dividers between them, so a row can hold
// any control without this component guessing.
func Group(c ui.C, title string, rows func()) {
	theme := c.Theme()
	metrics := theme.Metrics
	ui.Column(c).FillWidth().Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(metrics.FontSizeM).Bold().
				MarginBottom(metrics.GapItem)
		}
		ui.Column(c).FillWidth().Background(theme.Card).
			Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
			Children(rows)
	})
}

// Row is one row of a settings card: the name and its explanation on
// the left, the control on the right. A nil control leaves the row as a name
// and explanation only.
//
// The row is two control heights tall and grows when an explanation needs more
// than the two lines it is limited to.
func Row(c ui.C, title, description string, control func()) {
	theme := c.Theme()
	metrics := theme.Metrics
	ui.Row(c).MinHeight(metrics.ControlHeight * 2).Shrink(0).PadX(metrics.PadSurface).
		Gap(metrics.GapItem * 2).Items(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, title).FontSize(metrics.FontSizeM).Weight(gfx.WeightMedium)
			if description != "" {
				ui.Paragraph(c, description, 2).FontSize(metrics.FontSizeS).Muted()
			}
		})
		if control != nil {
			ui.Row(c).Shrink(0).Items(ui.Center).Justify(ui.End).Children(control)
		}
	})
}
