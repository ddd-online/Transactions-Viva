package tag

import "testing"

func TestFitFoldsTheTailIntoTheCounter(t *testing.T) {
	widths := []float32{40, 60, 40, 60}
	counter := func(hidden int) float32 { return 30 }
	cases := []struct {
		name  string
		width float32
		want  int
	}{
		// Everything fits: 40+4+60+4+40+4+60 = 212.
		{"all four", 260, 4},
		// Two capsules plus the counter: 40+4+60 = 104, then 4+30 = 138.
		{"two and a counter", 150, 2},
		// Not even one capsule beside the counter: only the counter shows.
		{"counter only", 60, 0},
		{"no room", 0, 0},
	}
	for _, test := range cases {
		if got := fit(widths, test.width, counter); got != test.want {
			t.Errorf("%s: fit = %d, want %d", test.name, got, test.want)
		}
	}
}

func TestFitKeepsTheOrderItShows(t *testing.T) {
	widths := []float32{20, 20, 20}
	none := func(int) float32 { return 0 }
	if got := fit(widths, 200, none); got != len(widths) {
		t.Fatalf("fit = %d, want all %d", got, len(widths))
	}
	// Two capsules and the gap before the counter: 20+4+20+4 = 48.
	if got := fit(widths, 48, none); got != 2 {
		t.Fatalf("fit = %d, want the first two capsules", got)
	}
	// One pixel short, the second capsule folds into the counter.
	if got := fit(widths, 47, none); got != 1 {
		t.Fatalf("fit = %d, want one capsule", got)
	}
}
