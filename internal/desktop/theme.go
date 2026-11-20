package desktop

import "github.com/vivaui/viva/gfx"

// The application's two themes.
//
// Viva ships a neutral preset per appearance; a product themes it by
// overriding the tokens it paints with, and that is the whole mechanism: a
// theme is a value of type gfx.Theme, every page reads it through c.Theme(),
// and swapping it is swapping the value the window paints with. So this file
// is the single place the application's color lives: pages name tokens, never
// hex, and a change here repaints every screen in both appearances.
//
// The world is "warm paper": the content card is a sheet, the navigation rail
// and the window shell are the desk it lies on, and the single accent stays a
// signal blue. The dark appearance is not the light one inverted — each pair
// was picked on its own surface, then measured:
//
//   - body text and the four data colors (income / expense / transfer /
//     outlier) clear 4.5:1 on the paper (light) or the near-black (dark), and
//     still clear 4.5:1 when they sit on their own 12% tint (chips, marks);
//   - the secondary grey clears 4.5:1 on both the paper and the muted fill;
//   - the primary button's label clears 4.5:1 on the primary fill;
//   - the focus ring clears 3:1 on the surface it is drawn on.
//
// Contrast is why a few values look darker than the old project's tokens: its
// income green (#16a34a) and expense red (#dc2626) sit at 3.3:1 and 4.0:1 on
// white, which is fine for a large figure but not for a 12px amount in a
// table. Warm paper costs a little lightness; the hues are the same.

// newTheme returns the application theme for the light or dark appearance.
func newTheme(dark bool) gfx.Theme {
	if dark {
		return warmInk()
	}
	return warmPaper()
}

// warmPaper is the light appearance: warm paper for content, a slightly deeper
// warm grey for the shell and the navigation rail, warm ink for text.
func warmPaper() gfx.Theme {
	return gfx.Theme{
		Background:          gfx.RGB24(0xfffdf8), // the card: warm paper
		Foreground:          gfx.RGB24(0x1a1611), // warm ink
		Card:                gfx.RGB24(0xfffdf8),
		CardForeground:      gfx.RGB24(0x1a1611),
		CardElevated:        gfx.RGB24(0xf7f2e8), // the navigation rail
		Popover:             gfx.RGB24(0xfffefa),
		PopoverForeground:   gfx.RGB24(0x1a1611),
		Muted:               gfx.RGB24(0xf2ece0), // the window shell, chips, tracks
		MutedForeground:     gfx.RGB24(0x6f6656), // secondary text, table headers
		Border:              gfx.RGB24(0xe7dfd0), // every hairline
		Hover:               gfx.RGB24(0xf8f3ea), // row hover, just above the paper
		Input:               gfx.RGB24(0xddd3c1), // field and control strokes
		Primary:             gfx.RGB24(0x1447e6), // Signal Blue
		PrimaryForeground:   gfx.RGB24(0xf7f4ec),
		Selection:           gfx.RGB24(0xd6e0fb),
		SelectionForeground: gfx.RGB24(0x14110d),
		Secondary:           gfx.RGB24(0xf0e9dc),
		SecondaryForeground: gfx.RGB24(0x241f18),
		Accent:              gfx.RGB24(0xeae3d4), // navigation hover and active row
		AccentForeground:    gfx.RGB24(0x1a1611),
		Ring:                gfx.RGB24(0x6f6656), // focus, 5.6:1 on the paper
		Success:             gfx.RGB24(0x12703a), // income
		Danger:              gfx.RGB24(0xc0271f), // expense, deletion, errors
		DangerForeground:    gfx.RGB24(0xfffdf8),
		Chart: [5]gfx.Color{
			gfx.RGB24(0xc0271f), // expense red
			gfx.RGB24(0x12703a), // income green
			gfx.RGB24(0x2b64c4), // transfer blue
			gfx.RGB24(0x94570a), // outlier amber
			gfx.RGB24(0x7b4fa5), // violet, the first hue past the data family
		},
		Metrics:    gfx.DefaultMetrics(),
		Transition: gfx.DefaultMotion(),
	}
}

// warmInk is the dark appearance: a near-black warm brown for content, a
// lifted warm grey for the shell, paper-warm white for text. The accent is a
// lighter blue because a deep one disappears into the shell, and the danger
// fill carries ink text for the same reason.
func warmInk() gfx.Theme {
	return gfx.Theme{
		Background:          gfx.RGB24(0x17130f),
		Foreground:          gfx.RGB24(0xf3ede2),
		Card:                gfx.RGB24(0x1d1813),
		CardForeground:      gfx.RGB24(0xf3ede2),
		CardElevated:        gfx.RGB24(0x211b15),
		Popover:             gfx.RGB24(0x221c16),
		PopoverForeground:   gfx.RGB24(0xf3ede2),
		Muted:               gfx.RGB24(0x2a231b),
		MutedForeground:     gfx.RGB24(0xa89d8b),
		Border:              gfx.RGBA(243, 237, 226, 0.12),
		Hover:               gfx.RGB24(0x241d16),
		Input:               gfx.RGBA(243, 237, 226, 0.16),
		Primary:             gfx.RGB24(0x2f5ce0),
		PrimaryForeground:   gfx.RGB24(0xf6f3ec),
		Selection:           gfx.RGB24(0x2b4a7d),
		SelectionForeground: gfx.RGB24(0xeef2ff),
		Secondary:           gfx.RGB24(0x2b241c),
		SecondaryForeground: gfx.RGB24(0xf3ede2),
		Accent:              gfx.RGB24(0x2e261d),
		AccentForeground:    gfx.RGB24(0xf6f0e6),
		Ring:                gfx.RGB24(0xa89d8b),
		Success:             gfx.RGB24(0x58b06b),
		Danger:              gfx.RGB24(0xe2645c),
		DangerForeground:    gfx.RGB24(0x17130f),
		Chart: [5]gfx.Color{
			gfx.RGB24(0xe2645c),
			gfx.RGB24(0x58b06b),
			gfx.RGB24(0x6f95e8),
			gfx.RGB24(0xd9a04a),
			gfx.RGB24(0xb08bd8),
		},
		Metrics:    gfx.DefaultMetrics(),
		Transition: gfx.DefaultMotion(),
	}
}
