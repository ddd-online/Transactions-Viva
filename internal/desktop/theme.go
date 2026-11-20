package desktop

import "github.com/vivaui/viva/gfx"

// The application's two themes: Cool Graphite (light) and Cool Ink (dark).
//
// Viva ships a neutral preset per appearance; a product themes it by replacing
// the value it paints with. That is the whole mechanism: gfx.Theme is a value,
// every page reads it through c.Theme(), and swapping it is swapping the value
// the window paints with. So this file is the single place the application's
// color lives: pages name tokens, never hex, and a change here repaints every
// screen in both appearances.
//
// The world is a graphite instrument: white paper, cool neutral surfaces, and
// one blue that means "act". The dark appearance is not the light one
// inverted — the rail always sits between the shell and the paper, and the two
// appearances simply point that ladder the other way. Every pair was picked on
// its own surface and measured:
//
//   - body text and the four data colors (income / expense / transfer /
//     outlier) clear 4.5:1 on the paper (light) or the near-black (dark), and
//     still clear 4.5:1 when they sit on their own 12% tint (chips, marks);
//   - the secondary grey clears 4.5:1 on both the paper and the muted fill;
//   - the primary button's label clears 4.5:1 on the primary fill;
//   - the focus ring clears 3:1 on the surface it is drawn on.
//
// The data hues are darker than the old project's: its income green (#16a34a)
// and expense red (#dc2626) sit at 3.3:1 and 4.0:1 on white, which is fine for
// a large figure but not for a 12px amount in a table. The hues are the same;
// only their lightness changed. Transfer is a sky blue deliberately apart from
// the action blue, so a transaction type never reads as a button.

// newTheme returns the application theme for the light or dark appearance.
func newTheme(dark bool) gfx.Theme {
	if dark {
		return coolInk()
	}
	return coolGraphite()
}

// coolGraphite is the light appearance: white paper on a cool grey shell, with
// graphite text and a single precise blue.
func coolGraphite() gfx.Theme {
	return gfx.Theme{
		Background:          gfx.RGB24(0xffffff), // the card: white paper
		Foreground:          gfx.RGB24(0x18181b), // graphite
		Card:                gfx.RGB24(0xffffff),
		CardForeground:      gfx.RGB24(0x18181b),
		CardElevated:        gfx.RGB24(0xfafafa), // the navigation rail
		Popover:             gfx.RGB24(0xffffff),
		PopoverForeground:   gfx.RGB24(0x18181b),
		Muted:               gfx.RGB24(0xf4f4f5), // the window shell, chips, tracks
		MutedForeground:     gfx.RGB24(0x646b76), // secondary text, table headers
		Border:              gfx.RGB24(0xe4e4e7), // every hairline
		Hover:               gfx.RGB24(0xf4f4f5), // row hover
		Input:               gfx.RGB24(0xd4d4d8), // field and control strokes
		Primary:             gfx.RGB24(0x1d4ed8), // Signal Blue
		PrimaryForeground:   gfx.RGB24(0xffffff),
		Selection:           gfx.RGB24(0xc7d7fe),
		SelectionForeground: gfx.RGB24(0x111827),
		Secondary:           gfx.RGB24(0xf4f4f5),
		SecondaryForeground: gfx.RGB24(0x18181b),
		Accent:              gfx.RGB24(0xe4e4e7), // navigation hover and active row
		AccentForeground:    gfx.RGB24(0x18181b),
		Ring:                gfx.RGB24(0x52525b), // focus, 7.7:1 on the paper
		Success:             gfx.RGB24(0x0f6b34), // income
		Danger:              gfx.RGB24(0xb91c1c), // expense, deletion, errors
		DangerForeground:    gfx.RGB24(0xffffff),
		Chart: [5]gfx.Color{
			gfx.RGB24(0xb91c1c), // expense red
			gfx.RGB24(0x0f6b34), // income green
			gfx.RGB24(0x0369a1), // transfer sky
			gfx.RGB24(0x8a5a06), // outlier amber
			gfx.RGB24(0x7c3aed), // violet, the first hue past the data family
		},
		Metrics:    gfx.DefaultMetrics(),
		Transition: gfx.DefaultMotion(),
	}
}

// coolInk is the dark appearance: the paper sits between a lighter shell and a
// slightly darker rail, the accent is lightened so it does not sink, and the
// danger fill carries ink text because its red is too bright for white.
func coolInk() gfx.Theme {
	return gfx.Theme{
		Background:          gfx.RGB24(0x18181b),
		Foreground:          gfx.RGB24(0xfafafa),
		Card:                gfx.RGB24(0x1c1c1f),
		CardForeground:      gfx.RGB24(0xfafafa),
		CardElevated:        gfx.RGB24(0x1f1f23),
		Popover:             gfx.RGB24(0x1c1c1f),
		PopoverForeground:   gfx.RGB24(0xfafafa),
		Muted:               gfx.RGB24(0x27272a),
		MutedForeground:     gfx.RGB24(0xa1a1aa),
		Border:              gfx.RGBA(250, 250, 250, 0.10),
		Hover:               gfx.RGB24(0x232326),
		Input:               gfx.RGBA(250, 250, 250, 0.14),
		Primary:             gfx.RGB24(0x2f5ce0),
		PrimaryForeground:   gfx.RGB24(0xfafafa),
		Selection:           gfx.RGB24(0x2b4a7d),
		SelectionForeground: gfx.RGB24(0xeef2ff),
		Secondary:           gfx.RGB24(0x27272a),
		SecondaryForeground: gfx.RGB24(0xfafafa),
		Accent:              gfx.RGB24(0x2a2a2d),
		AccentForeground:    gfx.RGB24(0xfafafa),
		Ring:                gfx.RGB24(0xa1a1aa),
		Success:             gfx.RGB24(0x4ade80),
		Danger:              gfx.RGB24(0xf87171),
		DangerForeground:    gfx.RGB24(0x18181b),
		Chart: [5]gfx.Color{
			gfx.RGB24(0xf87171),
			gfx.RGB24(0x4ade80),
			gfx.RGB24(0x38bdf8),
			gfx.RGB24(0xd9a04a),
			gfx.RGB24(0xb08bd8),
		},
		Metrics:    gfx.DefaultMetrics(),
		Transition: gfx.DefaultMotion(),
	}
}
