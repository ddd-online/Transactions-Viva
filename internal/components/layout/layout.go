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

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
)

const (
	// Inset is the page's horizontal inset: the padding of a toolbar, a table
	// header, a row, a panel and an empty state. A full-bleed table keeps it, so
	// the first column lines up with the toolbar above it.
	Inset float32 = 16

	// RailWidth is the width of a page's own section rail: the analysis chart
	// list, the tags category column, the settings navigation, the stock
	// positions / records rail, the diary date tree and the event list. The
	// shell's navigation is a different, narrower column and does not use it.
	RailWidth float32 = 280

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

	// TableHeaderInsetX and TableHeaderInsetY set the shared table header band
	// inside the table edge, so the band never merges with the page's side rail.
	TableHeaderInsetX float32 = 6
	TableHeaderInsetY float32 = 4
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

// StackedToolbar is the two-row toolbar for a page whose filters do not fit
// one line at the window's minimum width: the filters wrap on the first row
// and the page's primary action keeps the trailing edge on the second. Use it
// only below the width where Toolbar stops fitting; the extra row costs one
// control height of content.
func StackedToolbar(c ui.C, filters, actions func()) {
	ui.Column(c).FillWidth().PadX(Inset).PadY(6).Gap(Gap).Children(func() {
		ui.Flow(c).FillWidth().Gap(Gap).LineGap(Gap).Items(ui.Center).Children(filters)
		ui.Row(c).FillWidth().Justify(ui.End).Items(ui.Center).Children(actions)
	})
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

// Rail is a page's own section column: one fixed width, the page's full
// height, never squeezed by the content beside it. The analysis chart list,
// the tags categories, the stock positions and history, the event list, the
// diary tree and the settings sections all use it; the shell's navigation is
// a different, narrower column and does not.
func Rail(c ui.C, width float32, body func(ui.C)) {
	ui.Column(c).Width(width).MinWidth(width).Shrink(0).FillHeight().
		Children(func() { body(c) })
}

// SectionHeader is the 44px band that names a column of content: its title,
// the count beside it and the action that changes it, inset by padX and
// spaced by Gap, every part on the band's center line. The rails, the detail
// headers and a table's own sub-head all draw this band, so their hairlines
// meet on one line when the columns sit side by side.
//
// The caller declares the title and the actions inside it and appends the
// clipping or shrinking its own column needs; a band nested inside content
// that is already inset passes zero.
func SectionHeader(c ui.C, padX float32) ui.Box {
	return ui.Row(c).Height(SectionHeaderHeight).FillWidth().
		PadX(padX).Gap(Gap).Items(ui.Center)
}

// SectionCount is the number that follows a section title, or the "3 只" that
// says how much of it there is: a Caption on the mono grid, so the counts line
// up down every rail. It carries no pill: the band's title already names the
// group, and a counted pill reads as a tag.
func SectionCount(c ui.C, text string) ui.Box {
	return typography.Caption(c, text).Mono()
}

// ErrorBanner is a page's read error: one danger caption under the toolbar, at
// the page's inset, wrapping rather than clipping. Every page that reads shows
// the same band, so a failed read never moves content by a page-specific
// amount.
func ErrorBanner(c ui.C, message string) {
	ui.Column(c).FillWidth().PadX(Inset).PadY(6).Children(func() {
		typography.CaptionParagraph(c, message, 2).Color(c.Theme().Danger)
	})
}

// TableHeaderSpec is one table's header geometry. A zero Height uses the
// standard table-header height; a zero Width fills the available width.
type TableHeaderSpec struct {
	Height float32
	Width  float32
	PadX   float32
	Gap    float32
}

// TableHeader draws the shared inset header band: a rounded Muted surface set
// in from the table by TableHeaderInsetX and TableHeaderInsetY, with the column
// labels laid out on the full table width so header cells stay over their rows.
func TableHeader(c ui.C, spec TableHeaderSpec, body func(ui.C)) {
	height := spec.Height
	if height <= 0 {
		height = TableHeaderHeight
	}
	row := ui.Row(c).Height(height)
	if spec.Width > 0 {
		row = row.Width(spec.Width)
	} else {
		row = row.FillWidth()
	}
	row.PadX(spec.PadX).Gap(spec.Gap).Items(ui.Center).
		Draw(func(d *ui.DrawContext) {
			band := d.Rect
			band.X += TableHeaderInsetX
			band.Width -= 2 * TableHeaderInsetX
			band.Y += TableHeaderInsetY
			band.Height -= 2 * TableHeaderInsetY
			if band.Width <= 0 || band.Height <= 0 {
				return
			}
			d.Canvas.Rounded(band, d.Theme.Metrics.RadiusItem(), d.Theme.Muted)
		}).Children(func() { body(c) })
}

// Footer is the 40px band under a table that carries its totals: the page's
// inset, GroupGap between the readings, one line, no surface of its own. A
// band whose readings summarize the whole page rather than trail one table
// spreads them across the width (spread); the rest run together at the
// leading edge like a sentence.
func Footer(c ui.C, spread bool, body func()) {
	row := ui.Row(c).Height(FooterHeight).FillWidth().
		PadX(Inset).Gap(GroupGap).Items(ui.Center)
	if spread {
		row = row.Justify(ui.Between)
	}
	row.Children(body)
}

// Reading is one label and its figure in a footer: the name as a Caption, the
// number on the mono grid in the same step, and an optional note after it. A
// footer is chrome, so it keeps the caption size every footer uses; the number
// that matters is already large in the table above.
func Reading(c ui.C, label, value, note string, color gfx.Color) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		typography.Caption(c, label)
		ui.Text(c, value).FontSize(typography.SizeCaption).Mono().
			Weight(gfx.WeightMedium).Color(color)
		if note != "" {
			typography.Caption(c, note).Ellipsis()
		}
	})
}

// MetricGrid lays readings in rows of perRow, every cell an equal share of the
// width and the steps between cells the shared ColumnGap and BlockGap. A short
// last row keeps the cells it has at the same width, so a grid of six readings
// leaves the same column lines as a full row.
func MetricGrid(c ui.C, total, perRow int, cell func(ui.C, int)) {
	if perRow <= 0 {
		perRow = 4
	}
	ui.Column(c).FillWidth().Gap(BlockGap).Children(func() {
		for start := 0; start < total; start += perRow {
			ui.Row(c).FillWidth().Gap(ColumnGap).Items(ui.Start).Children(func() {
				for index := start; index < start+perRow; index++ {
					if index >= total {
						ui.Spacer(c).Grow(1).Basis(0)
						continue
					}
					cell(c, index)
				}
			})
		}
	})
}
