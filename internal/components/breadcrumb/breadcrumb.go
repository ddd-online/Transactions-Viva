// Package breadcrumb paints where something lives: the names of the places that
// hold it, and then the name of the thing itself.
//
// The path is one quiet line. Its ancestors stay muted — they are context — and
// the last name, the place the reader is at, wears the foreground ink, so the line
// says "you are here" without a link, a pill or a second accent. A breadcrumb is a
// place, so it never wraps: a path with nowhere to go in its room ends its name in
// an ellipsis instead of taking a second line.
package breadcrumb

import (
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/icons"
)

const (
	// gap is the air on each side of a separator: two names never touch, and the
	// chevron between them reads as punctuation rather than as a word.
	gap float32 = 4
	// markSize is the chevron between two names. It is smaller than the icons a
	// rail draws beside a name, because it separates the line rather than names a
	// thing in it.
	markSize float32 = 12
)

// Path paints names from the outermost place to the one the reader is at. An
// empty path paints nothing, so a page with nowhere to point keeps its band empty.
//
// The caller owns where the line sits and how much room it gets; every name is a
// single ellipsized line, so the path shrinks inside a narrow column instead of
// pushing the controls beside it. The ancestors give up their room first: the place
// the reader is at is the name the line exists to say.
func Path(c ui.C, names []string) ui.Box {
	theme := c.Theme()
	path := ui.Row(c).Gap(gap).Items(ui.Center).MinWidth(0)
	path.Children(func() {
		for index, name := range names {
			if index > 0 {
				ui.Icon(c, icons.ChevronRight).Size(markSize, markSize).
					Color(theme.MutedForeground).Shrink(0)
			}
			place := typography.Caption(c, name).MaxLines(1).Ellipsis().MinWidth(0)
			if index == len(names)-1 {
				place.Color(theme.Foreground).Shrink(1)
			} else {
				// Twice the leaf's weight, so a narrow band takes the ancestors' ends
				// before it takes the name of the place itself.
				place.Shrink(2)
			}
		}
	})
	return path
}
