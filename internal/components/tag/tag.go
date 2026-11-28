// Package tag draws a record's tags with Viva's tag component (ui/tag), the way
// this application's tables show them: one compact tag per tag, on one line, with
// the tags that do not fit folded into a "+N" tag of the same shape.
//
// The component keeps the column's fit, not the tag's look: the capsule, its
// colours and its type are ui/tag's, so a tag here wears the same shape as a tag
// anywhere else.
package tag

import (
	"fmt"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	vivatag "github.com/vivaui/viva/ui/tag"

	"transactions/internal/components/typography"
)

const (
	// chipGap separates two tags, and the counter from the last of them.
	chipGap float32 = 4
	// vivaTagPadX and vivaTagRim are what ui/tag's compact size keeps around its
	// label — its side padding, and its 1px rim on each side — so a column can
	// measure a tag before painting it.
	vivaTagPadX float32 = 6
	vivaTagRim  float32 = 2
	// vivaTagSize is the compact size: a table cell wants the tag one step under
	// the page's small type.
	vivaTagSize = vivatag.Small
)

// Chip is one tag: Viva's compact tag, taking its own width from its label, so
// callers put it in a Flow or a Row.
func Chip(c ui.C, label string) ui.Box {
	return vivatag.View(c, label, vivatag.Spec{Size: vivaTagSize}).Box
}

// List draws as many tags as fit in width on one line. Tags that do not fit
// become a single "+N" tag, so a narrow column still says how many the record
// carries; the editor shows their names. With no tags it draws the table's empty
// dash, the same em dash every table uses for a missing value.
func List(c ui.C, labels []string, width float32) {
	theme := c.Theme()
	if len(labels) == 0 {
		// The dash keeps the column's width. A cell that shrinks to its glyph
		// would pull the next column left, and one row without tags would break
		// the table's grid.
		ui.Text(c, "—").Width(width).FontSize(typography.SizeBody).Color(theme.MutedForeground)
		return
	}
	canvas := c.Canvas()
	metrics := theme.Metrics
	widths := make([]float32, len(labels))
	for index, label := range labels {
		// One pixel of slack keeps a rounding difference from clipping the last
		// tag.
		widths[index] = chipWidth(canvas, metrics, label) + 1
	}
	shown := fit(widths, width, func(hidden int) float32 {
		return chipWidth(canvas, metrics, fmt.Sprintf("+%d", hidden))
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

// chipWidth is the width Viva's compact tag gives label: its text at the tag's
// size and weight, its side padding and its rim.
func chipWidth(canvas *gfx.Canvas, metrics gfx.Metrics, label string) float32 {
	size := metrics.FontSizeS - 1
	text := canvas.TextWidthStyle(label, gfx.TextStyle{Size: size, Weight: gfx.WeightMedium})
	return text + 2*vivaTagPadX + vivaTagRim
}

// fit returns how many leading tags fit in width once the counter that stands
// for the rest has taken its room. It never returns more than the list holds,
// and it returns a prefix length so the visible tags always follow the tag order.
func fit(widths []float32, width float32, counter func(hidden int) float32) int {
	if width <= 0 || len(widths) == 0 {
		return 0
	}
	// prefix[i] is the width of the first i tags with the gaps between them.
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
