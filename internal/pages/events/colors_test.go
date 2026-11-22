package events

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The surfaces an event's bar and dot are painted on: the page paper and the
// card, which are both white in the light appearance. The muted chrome tone is
// not one of them — the rails and the detail pane sit on the card — and the old
// project measured its palette against paper white and its canvas, not a tinted
// hover. Dark uses the page background and the card.
var (
	lightSurfaces = []gfx.Color{gfx.RGB24(0xffffff), gfx.RGB24(0xffffff)}
	darkSurfaces  = []gfx.Color{gfx.RGB24(0x18181b), gfx.RGB24(0x1c1c1f)}
)

// contrast is the WCAG 1.4.11 ratio between two colours. The channels in
// gfx.Color are sRGB-encoded, so they are linearized first.
func contrast(a, b gfx.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c gfx.Color) float64 {
	linear := func(channel float32) float64 {
		v := float64(channel)
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.Red) + 0.7152*linear(c.Green) + 0.0722*linear(c.Blue)
}

// TestEveryToneClearsThreeToOne is the old project's unit test for the palette:
// a 6px dot and a 3px bar are meaningful graphics (WCAG 1.4.11), so every tone
// must clear 3:1 on both light surfaces and, after the dark lift, on both dark
// ones. 4.5:1 would be text, which these are not.
func TestEveryToneClearsThreeToOne(t *testing.T) {
	light := gfx.Theme{Background: gfx.RGB24(0xffffff)}
	dark := gfx.Theme{Background: gfx.RGB24(0x18181b)}
	for _, tone := range eventTones {
		lightColor := toneColor(tone.value, light)
		for _, surface := range lightSurfaces {
			if ratio := contrast(lightColor, surface); ratio < 3 {
				t.Errorf("%s (%s) is %.2f:1 on a light surface", tone.name, tone.value, ratio)
			}
		}
		darkColor := toneColor(tone.value, dark)
		for _, surface := range darkSurfaces {
			if ratio := contrast(darkColor, surface); ratio < 3 {
				t.Errorf("%s (%s) is %.2f:1 on a dark surface", tone.name, tone.value, ratio)
			}
		}
	}
}

func TestToneColorFallsBackToThePrimary(t *testing.T) {
	theme := gfx.Theme{Background: gfx.RGB24(0xffffff), Primary: gfx.RGB24(0x1d4ed8)}
	if got := toneColor("", theme); got != theme.Primary {
		t.Fatalf("empty tone = %+v, want the primary", got)
	}
	if got := toneColor("#zzzzzz", theme); got != theme.Primary {
		t.Fatalf("malformed tone = %+v, want the primary", got)
	}
	if got := toneColor("#D9705A", theme); got != gfx.RGB24(0xd9705a) {
		t.Fatalf("stored tone = %+v", got)
	}
}

func TestToneColorLiftsInTheDarkAppearance(t *testing.T) {
	dark := gfx.Theme{Background: gfx.RGB24(0x18181b)}
	got := toneColor("#D9705A", dark)
	want := gfx.RGB(lift(0xd9), lift(0x70), lift(0x5a))
	if got != want {
		t.Fatalf("dark tone = %+v, want %+v", got, want)
	}
	if got.Red <= float32(0xd9)/255 {
		t.Fatalf("dark tone was not lifted: %+v", got)
	}
}
