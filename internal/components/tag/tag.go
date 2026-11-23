// Package tag draws a record's tags as capsules, the way the tables in this
// application show them: one muted capsule per tag, on one line, with the tags
// that do not fit folded into a counter.
//
// Viva has no tag component of its own — the chips inside components.MultiSelect
// belong to that control — so this is the application's: the same capsule in the
// records table and the templates table, so a tag never looks like two things.
package tag

import (
	"fmt"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
)

const (
	// chipPadX is the capsule's horizontal padding; its height comes from the
	// small font size and chipPadY.
	chipPadX float32 = 6
	chipPadY float32 = 1
	// chipGap separates two capsules, and the counter from the last of them.
	chipGap float32 = 4
)

// Chip is one tag: a muted capsule with the tag's name. It takes its own width
// from its label, so callers put it in a Flow or a Row.
func Chip(c ui.C, label string) ui.Box {
	theme := c.Theme()
	return ui.Text(c, label).FontSize(typography.SizeCaption).
		Color(theme.MutedForeground).Background(theme.Muted).
		PadX(chipPadX).PadY(chipPadY).Radius(theme.Metrics.RadiusItem())
}

// List draws as many capsules as fit in width on one line. Tags that do not fit
// become a single "+N" capsule, so a narrow column still says how many the
// record carries; the editor shows their names. With no tags it draws the
// table's empty dash, the same em dash every table uses for a missing value.
func List(c ui.C, labels []string, width float32) {
	theme := c.Theme()
	if len(labels) == 0 {
		// The dash keeps the column's width. A cell that shrinks to its glyph
		// would pull the next column left, and one row without tags would break
		// the table's grid.
		ui.Text(c, "—").Width(width).FontSize(typography.SizeBody).Color(theme.MutedForeground)
		return
	}
	style := gfx.TextStyle{Size: typography.SizeCaption}
	canvas := c.Canvas()
	widths := make([]float32, len(labels))
	for index, label := range labels {
		// The measured text plus the capsule's padding; one pixel of slack keeps
		// a rounding difference from clipping the last capsule.
		widths[index] = canvas.TextWidthStyle(label, style) + 2*chipPadX + 1
	}
	shown := fit(widths, width, func(hidden int) float32 {
		return canvas.TextWidthStyle(fmt.Sprintf("+%d", hidden), style) + 2*chipPadX
	})
	ui.Row(c).Width(width).Gap(chipGap).Items(ui.Center).Clip().Children(func() {
		for _, label := range labels[:shown] {
			Chip(c, label)
		}
		if hidden := len(labels) - shown; hidden > 0 {
			Chip(c, fmt.Sprintf("+%d", hidden))
		}
	})
}

// fit returns how many leading capsules fit in width once the counter that
// stands for the rest has taken its room. It never returns more than the list
// holds, and it returns a prefix length so the visible capsules always follow
// the tag order.
func fit(widths []float32, width float32, counter func(hidden int) float32) int {
	if width <= 0 || len(widths) == 0 {
		return 0
	}
	// prefix[i] is the width of the first i capsules with the gaps between them.
	prefix := make([]float32, len(widths)+1)
	for index, capsule := range widths {
		prefix[index+1] = prefix[index]
		if index > 0 {
			prefix[index+1] += chipGap
		}
		prefix[index+1] += capsule
	}
	if prefix[len(widths)] <= width {
		return len(widths)
	}
	for shown := len(widths) - 1; shown >= 0; shown-- {
		folded := prefix[shown]
		if shown > 0 {
			folded += chipGap
		}
		if folded+counter(len(widths)-shown) <= width {
			return shown
		}
	}
	return 0
}
