package breadcrumb

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// painted drives a page of the given width that holds nothing but the path, and
// returns the text it painted, in reading order.
func painted(t *testing.T, width float32, names []string) ([]string, []app.Label) {
	t.Helper()
	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: func(c ui.C) {
		ui.Column(c).Width(width).Children(func() {
			Path(c, names)
		})
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 600, Height: 120}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	labels := driver.Snapshot().Labels
	values := make([]string, 0, len(names))
	for _, label := range labels {
		values = append(values, label.Value)
	}
	return values, labels
}

// TestPathNamesThePlacesInOrder: every name is painted, left to right, with room
// for a chevron between two of them, and only the last one wears the foreground
// ink — the rest are the context it stands in.
func TestPathNamesThePlacesInOrder(t *testing.T) {
	names := []string{"我的笔记", "12121", "会议纪要"}
	values, labels := painted(t, 400, names)
	if len(values) != len(names) {
		t.Fatalf("painted %d names, want %d: %v", len(values), len(names), values)
	}
	for index, want := range names {
		if values[index] != want {
			t.Fatalf("name %d = %q, want %q (%v)", index, values[index], want, values)
		}
	}
	theme := ui.LightTheme()
	for index, label := range labels {
		want := theme.MutedForeground
		if index == len(labels)-1 {
			want = theme.Foreground
		}
		if label.Color != want {
			t.Fatalf("name %q is painted %v, want %v", label.Value, label.Color, want)
		}
	}
	for index := 1; index < len(labels); index++ {
		before, after := labels[index-1].Bounds, labels[index].Bounds
		if after.Y != before.Y {
			t.Fatalf("names %q and %q are not on one line", labels[index-1].Value, labels[index].Value)
		}
		if after.X < before.X+before.Width+markSize {
			t.Fatalf("name %q at %.1f leaves no room for a chevron after %q",
				labels[index].Value, after.X, labels[index-1].Value)
		}
	}
}

// TestPathPaintsNothingWithoutPlaces: a page with no path keeps its band empty.
func TestPathPaintsNothingWithoutPlaces(t *testing.T) {
	values, _ := painted(t, 400, nil)
	if len(values) != 0 {
		t.Fatalf("an empty path painted %v", values)
	}
}

// TestPathGivesUpTheAncestorsFirst: a narrow column takes the ends of the folders
// that hold the note before it takes the note's own name, and the path stays on one
// line whatever it has to drop.
func TestPathGivesUpTheAncestorsFirst(t *testing.T) {
	// Two names of the same length, in a column narrower than both of them: what
	// each one keeps says which the line protects.
	names := []string{"文件夹甲", "会议纪要"}
	values, labels := painted(t, 90, names)
	if len(values) != 2 {
		t.Fatalf("the path painted %v, want both names", values)
	}
	folder, place := labels[0], labels[1]
	if place.Bounds.Y != folder.Bounds.Y {
		t.Fatalf("the path wrapped: %q sits on another line", place.Value)
	}
	if place.Bounds.Width <= folder.Bounds.Width {
		t.Fatalf("the place %q kept %.1f points, no more than its folder %q's %.1f",
			place.Value, place.Bounds.Width, folder.Value, folder.Bounds.Width)
	}
}
