// Package semantic holds the colors that carry meaning in data: the three
// transaction types and the outlier mark. They are not chrome, so they are read
// from the theme's data tokens rather than written as literals in a page —
// Success and Danger for income and expense, and the chart palette's third and
// fourth slots for transfer and outlier. Those two slots are also the first
// series hues of the analysis page, so an amount, a type chip, a mark and a
// chart line can never drift apart.
//
// One edit in the theme (internal/desktop/theme.go) moves all of them, in both
// appearances. Every pair there is measured at 4.5:1 on its own surface and on
// its own 12% tint, which is the background chips put behind the text.
package semantic

import "github.com/vivaui/viva/gfx"

// The transaction types, spelled as the old project stores them.
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
