// Package emptystate is the one way a column, a rail or a whole page says it
// has nothing to show yet: a quiet line inside a section (Notice), or the
// page's own empty state with an icon and the actions that leave it (Page).
package emptystate

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
)

// pageWidth is the widest a page empty state runs: about 27 Chinese characters
// a line, the same max-w-sm the framework's own Empty uses.
const pageWidth = 384

// Notice centers one line of muted text in the space left in a column, so the
// 标签 columns and the analysis rail all say it the same way: a dense line, one
// step below the rows it replaces.
func Notice(c ui.C, text string) {
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		typography.Body(c, text).Muted().TextCenter()
	})
}

// Page is the empty state of a whole page or pane: an icon, the page's title,
// one sentence and, when it has them, the actions that leave the state. The
// framework's Empty paints its own title at 14 + 5, Medium — a size this
// application does not have, and for Chinese a weight it does not have either
// — so the shape is composed here from Title and Body, and the actions are real
// buttons rather than canvas paint.
func Page(c ui.C, icon *gfx.Icon, title, sentence string, actions func()) {
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(pageWidth).Gap(16).Items(ui.Center).Children(func() {
			if icon != nil {
				ui.Icon(c, *icon).Size(28, 28).Color(c.Theme().MutedForeground)
			}
			ui.Column(c).FillWidth().Gap(8).Items(ui.Center).Children(func() {
				typography.Title(c, title).TextCenter()
				if sentence != "" {
					typography.BodyParagraph(c, sentence, 2).
						Muted().TextCenter()
				}
			})
			if actions != nil {
				ui.Row(c).Gap(8).Items(ui.Center).Children(actions)
			}
		})
	})
}
