package components

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// SettingsGroup is a settings card: a titled surface holding rows separated by
// hairlines. It follows the reference application's settings page, where each
// topic is a rounded, bordered card of rows rather than one long form.
//
// The caller declares the rows and the dividers between them, so a row can hold
// any control without this component guessing.
func SettingsGroup(c ui.C, title string, rows func()) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(theme.Metrics.FontSizeM).Bold().
				LineHeight(22).MarginSides(0, 4, 6, 4)
		}
		ui.Column(c).FillWidth().Background(theme.Card).
			Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
			Children(rows)
	})
}

// SettingsRow is one row of a settings card: the name and its explanation on
// the left, the control on the right. A nil control leaves the row as a name
// and explanation only.
//
// The row is a normal settings row's 64 high and grows when an explanation
// needs more than the two lines it is limited to.
func SettingsRow(c ui.C, title, description string, control func()) {
	theme := c.Theme()
	ui.Row(c).MinHeight(64).Shrink(0).PadX(18).Gap(24).Items(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, title).FontSize(theme.Metrics.FontSizeM).
				Weight(gfx.WeightMedium).LineHeight(21)
			if description != "" {
				ui.Paragraph(c, description, 2).FontSize(theme.Metrics.FontSizeS).
					Muted().LineHeight(15).MarginTop(1)
			}
		})
		if control != nil {
			ui.Row(c).Height(34).Shrink(0).Items(ui.Center).Justify(ui.End).
				Children(control)
		}
	})
}
