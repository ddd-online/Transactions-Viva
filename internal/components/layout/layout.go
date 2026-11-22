// Package layout is the application's spatial scale: the measurements the
// screens share, so a toolbar, a table header and a list row are the same
// height on 记录, 分析, 标签 and 模板, and a table cell starts on the same
// vertical line as the toolbar's first control.
//
// The scale is 4-based (4 / 8 / 12 / 16 / 24 / 32) and page code names a token
// instead of passing a number. One change here moves every screen: the same
// rule the theme follows for color.
//
// There are two scales, and they agree: gfx.Metrics is the control scale (the
// height of a button, the radius of a field, the padding inside them), and this
// package is the page scale (the band above the content, the row of a table,
// the width of a rail). A page never invents a third number.
package layout

import "github.com/vivaui/viva/ui"

const (
	// Inset is the page's horizontal inset: the padding of a toolbar, a table
	// header, a row, a panel and an empty state. A full-bleed table keeps it, so
	// the first column lines up with the toolbar above it.
	Inset float32 = 16

	// RailWidth is the width of a page's own section rail: the analysis chart
	// list, the tags category column and the settings navigation. The shell's
	// navigation is a different, narrower column and does not use it.
	RailWidth float32 = 240

	// ToolbarHeight is the band above a page's content: ledger, filters and the
	// page's primary action.
	ToolbarHeight float32 = 56

	// TableHeaderHeight is the column-header row of a data table.
	TableHeaderHeight float32 = 36

	// SectionHeaderHeight is a column's own header inside a page: its title, its
	// count and the action that adds to it.
	SectionHeaderHeight float32 = 44

	// RowHeight is one row of a data table or of a rail list.
	RowHeight float32 = 44

	// FooterHeight is the band under a table that carries its totals.
	FooterHeight float32 = 40

	// Gap separates controls inside a toolbar or a row of buttons.
	Gap float32 = 8

	// ColumnGap separates the cells of a table.
	ColumnGap float32 = 12

	// BlockGap separates stacked blocks inside a panel or a card.
	BlockGap float32 = 12

	// GroupGap separates one group of content from the next.
	GroupGap float32 = 24
)

// Toolbar is the standard band above a page's content: one 56px row, the
// page's horizontal inset, one control gap, and vertical centering for every
// control in it.
func Toolbar(c ui.C, contents func()) {
	ui.Row(c).Height(ToolbarHeight).FillWidth().PadX(Inset).Gap(Gap).
		Items(ui.Center).Children(contents)
}

// FlowToolbar is the wrapping variant for a toolbar whose controls do not fit
// one line. It keeps the same height, inset and control gap; the second line
// starts at the left edge because a Flow has no per-line cross alignment.
func FlowToolbar(c ui.C, contents func()) {
	ui.Flow(c).FillWidth().MinHeight(ToolbarHeight).PadX(Inset).PadY(6).
		Gap(Gap).LineGap(Gap).Items(ui.Center).Children(contents)
}

// Page is the common page shell: the toolbar at the top, a hairline, then the
// main content taking the rest of the height. A page supplies its own toolbar
// and body; dialogs stay after the shell and are declared by the page.
func Page(c ui.C, toolbar, body func(ui.C)) {
	ui.Column(c).Fill().Children(func() {
		if toolbar != nil {
			toolbar(c)
			ui.Divider(c)
		}
		body(c)
	})
}
