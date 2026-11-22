package controls

import (
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
)

// Section is one settings group: its name over its content, spaced the way
// 关于软件's groups are — the name, then the block at the row gap. A page that
// stacks groups separates them with layout.GroupGap, so every settings page
// reads as the same list of named groups.
//
// The body is the group's content, usually a Card or the odd header; an empty
// name is a group with content only (a card of rows under the page title).
func Section(c ui.C, title string, body func()) {
	ui.Column(c).FillWidth().Shrink(0).Gap(layout.Gap).Children(func() {
		if title != "" {
			typography.Section(c, title)
		}
		body()
	})
}

// Group is a settings card: a titled surface holding rows separated by
// hairlines. It follows the reference application's settings page, where each
// topic is a rounded, bordered card of rows rather than one long form.
//
// The caller declares the rows and the dividers between them, so a row can hold
// any control without this component guessing.
func Group(c ui.C, title string, rows func()) {
	Section(c, title, func() { Card(c, rows) })
}

// Card is the bordered surface of a settings card without its title: the rows
// a page shows under its own heading (账本管理) use it directly, so every card
// in the panel has the same chrome as the titled ones.
func Card(c ui.C, rows func()) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Background(theme.Card).
		Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
		Children(rows)
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
			// No half weight: on a Chinese face 500 draws as 400, so a row's
			// name would look like its explanation. The name is the row's
			// only unbounded text (a ledger can be named anything), so it
			// keeps one line and ends with … instead of running into the
			// control on the right.
			typography.Body(c, title).FillWidth().Ellipsis()
			if description != "" {
				typography.CaptionParagraph(c, description, 2)
			}
		})
		if control != nil {
			// The control area can hmore than one control (a ledger row's
			// 当前 mark beside its edit and delete buttons), so its parts keep
			// the row's item gap instead of touching.
			ui.Row(c).Shrink(0).Items(ui.Center).Justify(ui.End).
				Gap(metrics.GapItem).Children(control)
		}
	})
}
