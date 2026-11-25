// Package semantic holds the colors that carry meaning: the three transaction
// types, the outlier mark, and what a row of a rail is. The data hues are read
// from the theme's data tokens rather than written as literals in a page —
// Success and Danger for income and expense, and the chart palette's third and
// fourth slots for transfer and outlier. Those two slots are also the first
// series hues of the analysis page, so an amount, a type chip, a mark and a
// chart line can never drift apart.
//
// One edit in the theme (internal/desktop/theme.go) moves all of them, in both
// appearances. Every pair there is measured at 4.5:1 on its own surface and on
// its own 12% tint, which is the background chips put behind the text.
//
// The rail's two inks are the exception to "the theme defines every colour": the
// framework's gfx.Theme has no slot for a chrome accent, and the chart palette
// belongs to data. They live here, keyed by appearance, so the pair is still
// written once and the pages still never name a colour.
package semantic

import "github.com/vivaui/viva/gfx"

// The transaction types, spelled as the project stores them.
const (
	KindExpense  = "expense"
	KindIncome   = "income"
	KindTransfer = "transfer"
)

// Income is the money-in hue: a green that clears body-text contrast on paper
// and on the near-black.
func Income(theme gfx.Theme) gfx.Color { return theme.Success }

// Expense is the money-out hue; it doubles as the danger color, so a deletion
// confirmation and a negative amount speak the same red.
func Expense(theme gfx.Theme) gfx.Color { return theme.Danger }

// Transfer is the third chart hue: a blue that is deliberately not the accent
// blue, because the accent belongs to actions, not to a transaction type.
func Transfer(theme gfx.Theme) gfx.Color { return theme.Chart[2] }

// Outlier is the warning amber of the 离群值 mark and the linked-event star.
func Outlier(theme gfx.Theme) gfx.Color { return theme.Chart[3] }

// Kind maps a transaction type to its hue. An unknown type takes the muted
// foreground, the same quiet fallback the tables use for an empty cell.
func Kind(theme gfx.Theme, kind string) gfx.Color {
	switch kind {
	case KindIncome:
		return Income(theme)
	case KindTransfer:
		return Transfer(theme)
	case KindExpense:
		return Expense(theme)
	}
	return theme.MutedForeground
}

// FolderInk is the ink of a container row — the notes tree's folder. It is the
// colour a paper folder is, a kraft sand, and it is the product's only warm ink
// besides the attention amber: no amount and no transaction type wears it, and it
// is not the accent, so a tree with three folders does not read as three actions.
//
// The pair below is measured on the rail (#f4f4f5) and on the chosen row's wash
// (#dededf): 4.58:1 and 3.74:1 in the light appearance, 7.34:1 and 5.42:1 in the
// dark one — a 15px glyph that stays legible while the row is selected.
func FolderInk(theme gfx.Theme) gfx.Color {
	if isDark(theme) {
		return gfx.RGB24(0xdfae5c)
	}
	return gfx.RGB24(0x94661a)
}

// LeafInk is the ink of a leaf row — the notes tree's note. A page is content, not
// structure, so it takes the product's secondary ink: the tree spends its one hue
// on the rows that hold other rows, and the leaves read as paper.
func LeafInk(theme gfx.Theme) gfx.Color { return theme.MutedForeground }

// isDark reports whether the theme is the dark appearance. The paper colour is the
// figure the rest of the application compares to.
func isDark(theme gfx.Theme) bool {
	return 0.2126*theme.Background.Red+0.7152*theme.Background.Green+0.0722*theme.Background.Blue < 0.5
}

// The opacities a data hue is used at. A chip puts its hue behind its own label
// at TintAlpha (the rule DESIGN.md records: every pair still clears 4.5:1 on the
// tint); WashAlpha is the wider background a whole column or group takes, where
// the hue marks the region rather than a word.
const (
	TintAlpha = 0.12
	WashAlpha = 0.06
)

// Tint is color as a chip background behind its own label.
func Tint(color gfx.Color) gfx.Color { return color.WithAlpha(TintAlpha) }

// Wash is color as a wide background that marks a region.
func Wash(color gfx.Color) gfx.Color { return color.WithAlpha(WashAlpha) }
