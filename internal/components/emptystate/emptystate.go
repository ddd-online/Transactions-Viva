// Package emptystate is the one way a column or a rail says it has nothing to
// show yet. Page-level empty states use Viva's components.Empty with an icon and
// their own actions; this is the quiet line inside a section.
package emptystate

import (
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
)

// Notice centers one line of muted text in the space left in a column, so the
// 标签 columns and the analysis rail all say it the same way: a dense line, one
// step below the rows it replaces.
func Notice(c ui.C, text string) {
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		typography.Body(c, text).Muted().TextCenter()
	})
}
