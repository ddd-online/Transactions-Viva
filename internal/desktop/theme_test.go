package desktop

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"

	"transactions/internal/components/state"
)

// relativeLuminance is WCAG 2.x relative luminance for an opaque color.
func relativeLuminance(c gfx.Color) float64 {
	channel := func(v float32) float64 {
		x := float64(max(v, 0))
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.Red) + 0.7152*channel(c.Green) + 0.0722*channel(c.Blue)
}

// over composites c on top of an opaque background.
func over(c, background gfx.Color) gfx.Color {
	alpha := float64(c.Alpha)
	blend := func(front, back float32) float32 {
		return float32(float64(front)*alpha + float64(back)*(1-alpha))
	}
	return gfx.Color{
		Red:   blend(c.Red, background.Red),
		Green: blend(c.Green, background.Green),
		Blue:  blend(c.Blue, background.Blue),
		Alpha: 1,
	}
}

// tint is color at opacity over the background: the fill a chip puts behind
// its own label.
func tint(color, background gfx.Color, opacity float32) gfx.Color {
	return over(color.WithAlpha(opacity), background)
}

func contrast(a, b gfx.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	hi, lo := math.Max(la, lb), math.Min(la, lb)
	return (hi + 0.05) / (lo + 0.05)
}

// TestThemesMeetTheContrastFloor is the theme's quality gate: the tokens a
// page paints text with, and the ones it paints data with, must clear AA on
// the surface they actually sit on — including the 12% tint behind a chip.
func TestThemesMeetTheContrastFloor(t *testing.T) {
	for _, appearance := range []struct {
		name  string
		theme gfx.Theme
	}{
		{"light", newTheme(false)},
		{"dark", newTheme(true)},
	} {
		theme := appearance.theme
		paper := theme.Background
		text := []struct {
			name  string
			color gfx.Color
			bg    gfx.Color
		}{
			{"foreground on paper", theme.Foreground, paper},
			{"foreground on card", theme.CardForeground, theme.Card},
			{"muted foreground on paper", theme.MutedForeground, paper},
			{"muted foreground on muted fill", theme.MutedForeground, theme.Muted},
			{"primary label on primary", theme.PrimaryForeground, theme.Primary},
			{"danger label on danger", theme.DangerForeground, theme.Danger},
			{"income on paper", theme.Success, paper},
			{"expense on paper", theme.Danger, paper},
			{"transfer on paper", theme.Chart[2], paper},
			{"outlier on paper", theme.Chart[3], paper},
		}
		for _, check := range text {
			if got := contrast(over(check.color, check.bg), check.bg); got < 4.5 {
				t.Errorf("%s: %s is %.2f:1, want at least 4.5:1", appearance.name, check.name, got)
			}
		}
		// The same data colors, on the tint a chip paints behind them.
		for _, data := range []struct {
			name  string
			color gfx.Color
		}{
			{"income", theme.Success},
			{"expense", theme.Danger},
			{"transfer", theme.Chart[2]},
			{"outlier", theme.Chart[3]},
		} {
			chip := tint(data.color, paper, 0.12)
			if got := contrast(over(data.color, chip), chip); got < 4.5 {
				t.Errorf("%s: %s on its 12%% tint is %.2f:1, want at least 4.5:1", appearance.name, data.name, got)
			}
		}
		// A focus ring only has to be seen, not read.
		if got := contrast(over(theme.Ring, paper), paper); got < 3 {
			t.Errorf("%s: focus ring is %.2f:1 on paper, want at least 3:1", appearance.name, got)
		}
		// Chart strokes are graphics, not body text.
		for i, line := range theme.Chart {
			if got := contrast(over(line, paper), paper); got < 3 {
				t.Errorf("%s: chart colour %d is %.2f:1 on paper, want at least 3:1", appearance.name, i, got)
			}
		}
	}
}

// TestAppearancesAreDistinct guards against a copy-paste that leaves one
// appearance painted with the other's surface.
func TestAppearancesAreDistinct(t *testing.T) {
	light, dark := newTheme(false), newTheme(true)
	if light.Background == dark.Background || light.Foreground == dark.Foreground {
		t.Fatal("the two appearances share a surface or a text color")
	}
	if relativeLuminance(light.Background) <= relativeLuminance(dark.Background) {
		t.Fatal("the light paper is not lighter than the dark ink")
	}
}

// TestInteractionStatesReadOnBothSurfaces guards the state washes: a row can
// sit on the paper card or on the shell, and one opaque grey cannot step away
// from both — the old tokens vanished into the light chrome and reversed
// direction on the dark one. Hover, selected and pressed are washes of the ink,
// so each one must move away from the surface it lands on, in that order.
func TestInteractionStatesReadOnBothSurfaces(t *testing.T) {
	distance := func(a, b gfx.Color) float32 {
		channel := func(x, y float32) float32 {
			if x < y {
				x, y = y, x
			}
			return x - y
		}
		return max(channel(a.Red, b.Red), max(channel(a.Green, b.Green), channel(a.Blue, b.Blue)))
	}
	for _, appearance := range []struct {
		name  string
		theme gfx.Theme
	}{
		{"light", newTheme(false)},
		{"dark", newTheme(true)},
	} {
		for _, surface := range []struct {
			name string
			bg   gfx.Color
		}{
			{"paper", appearance.theme.Card},
			{"shell", appearance.theme.Muted},
		} {
			hover := over(state.Hover(appearance.theme), surface.bg)
			selected := over(state.Selected(appearance.theme), surface.bg)
			pressed := over(state.Pressed(appearance.theme), surface.bg)
			if step := distance(hover, surface.bg); step < 0.02 {
				t.Errorf("%s on %s: hover is %.3f from the surface", appearance.name, surface.name, step)
			}
			if step := distance(selected, hover); step < 0.02 {
				t.Errorf("%s on %s: selected is %.3f from hover", appearance.name, surface.name, step)
			}
			if step := distance(pressed, selected); step < 0.02 {
				t.Errorf("%s on %s: pressed is %.3f from selected", appearance.name, surface.name, step)
			}
		}
	}
}
