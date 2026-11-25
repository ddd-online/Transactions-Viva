package imageviewer

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// window is the page the tests size against.
var window = gfx.Rect{Width: 1500, Height: 1000}

// TestFitIsNineTenthsOfTheWindow pins the size both pages open at: a landscape
// picture fills nine tenths of the window's height and keeps its aspect, and the
// dialog leaves room under it when the page asks for a footer.
func TestFitIsNineTenthsOfTheWindow(t *testing.T) {
	tests := []struct {
		name          string
		picture       Picture
		reserve       float32
		wantWidth     float32
		wantHeight    float32
		wantUnshaped  bool
		toleranceFine float32
	}{
		{
			name:    "landscape fills the height",
			picture: Picture{Width: 4000, Height: 3000},
			// 1500×0.9 = 1350 wide, 1000×0.9 = 900 high; 4000×3000 fits the height.
			wantWidth:  1200,
			wantHeight: 900,
		},
		{
			name:    "landscape with a footer",
			picture: Picture{Width: 4000, Height: 3000},
			reserve: 40,
			// (1000×0.9 − 40) = 860 high.
			wantWidth:  1146.667,
			wantHeight: 860,
		},
		{
			name:       "square",
			picture:    Picture{Width: 3000, Height: 3000},
			reserve:    40,
			wantWidth:  860,
			wantHeight: 860,
		},
		{
			name:         "the bitmap stands in for a size the block lost",
			picture:      Picture{Bitmap: gfx.Bitmap{Width: 2000, Height: 1000}},
			wantWidth:    1350,
			wantHeight:   675,
			reserve:      0,
			wantUnshaped: false,
		},
		{
			name:         "the stand-in stands in for a lost bitmap",
			picture:      Picture{Waiting: gfx.Bitmap{Width: 1000, Height: 1000}},
			wantWidth:    900,
			wantHeight:   900,
			wantUnshaped: false,
		},
		{
			name:         "nothing to shape the dialog from",
			picture:      Picture{},
			wantUnshaped: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viewer := New()
			viewer.Show(test.picture)
			size, shaped := viewer.fit(window, test.reserve)
			if shaped == test.wantUnshaped {
				t.Fatalf("shaped = %v, want %v", shaped, !test.wantUnshaped)
			}
			if !shaped {
				return
			}
			if size.Width < test.wantWidth-0.01 || size.Width > test.wantWidth+0.01 {
				t.Fatalf("width = %v, want %v", size.Width, test.wantWidth)
			}
			if size.Height < test.wantHeight-0.01 || size.Height > test.wantHeight+0.01 {
				t.Fatalf("height = %v, want %v", size.Height, test.wantHeight)
			}
			if size.Width > window.Width*share+0.01 || size.Height > window.Height*share+0.01 {
				t.Fatalf("the picture takes %vx%v, more than nine tenths of the window", size.Width, size.Height)
			}
		})
	}
}

// TestShowKeepsTheZoomOfTheSamePicture pins the rule the pages rely on: a bitmap
// that arrives for the picture already open must not throw the user's zoom away,
// and stepping to another picture must.
func TestShowKeepsTheZoomOfTheSamePicture(t *testing.T) {
	viewer := New()
	viewer.Show(Picture{Path: "a.jpg", Width: 4000, Height: 3000})
	viewer.zoom, viewer.pan = 2, gfx.Point{X: 30, Y: 20}

	viewer.Show(Picture{Path: "a.jpg", Width: 4000, Height: 3000, Bitmap: gfx.Bitmap{Width: 2048, Height: 1536}})
	if viewer.zoom != 2 || viewer.pan != (gfx.Point{X: 30, Y: 20}) {
		t.Fatalf("zoom %v pan %v after the bitmap arrived; want the user's zoom and pan", viewer.zoom, viewer.pan)
	}

	viewer.Show(Picture{Path: "b.jpg", Width: 2000, Height: 2000})
	if viewer.zoom != zoomLeast || viewer.pan != (gfx.Point{}) {
		t.Fatalf("zoom %v pan %v after another picture; want fit and centred", viewer.zoom, viewer.pan)
	}
}

// TestScrollZoomsBetweenFitAndSixTimes bounds the wheel: the pages never zoom
// past six times, and coming back to fit recentres the picture.
func TestScrollZoomsBetweenFitAndSixTimes(t *testing.T) {
	viewer := New()
	viewer.Show(Picture{Path: "a.jpg", Width: 4000, Height: 3000})
	for range 50 {
		viewer.scroll(gfx.ScrollEvent{DeltaY: -120})
	}
	if viewer.zoom != zoomMost {
		t.Fatalf("zoom = %v after scrolling in, want %v", viewer.zoom, zoomMost)
	}
	viewer.pan = gfx.Point{X: 40, Y: 40}
	for range 100 {
		viewer.scroll(gfx.ScrollEvent{DeltaY: 120})
	}
	if viewer.zoom != zoomLeast {
		t.Fatalf("zoom = %v after scrolling out, want %v", viewer.zoom, zoomLeast)
	}
	if viewer.pan != (gfx.Point{}) {
		t.Fatalf("pan = %v at fit size, want the picture centred", viewer.pan)
	}
}

// TestPointerOnlyDragsAZoomedPicture keeps a press at fit size from moving the
// picture: the press that opened the dialog is not a drag.
func TestPointerOnlyDragsAZoomedPicture(t *testing.T) {
	viewer := New()
	viewer.Show(Picture{Path: "a.jpg", Width: 4000, Height: 3000})
	if result := viewer.pointer(gfx.PointerEvent{Kind: gfx.PointerDown, X: 10, Y: 10}); result.Handled {
		t.Fatal("a press at fit size was taken as a drag")
	}
	viewer.zoom = 2
	viewer.pointer(gfx.PointerEvent{Kind: gfx.PointerDown, X: 10, Y: 10})
	viewer.pointer(gfx.PointerEvent{Kind: gfx.PointerMove, X: 30, Y: 25})
	if viewer.pan != (gfx.Point{X: 20, Y: 15}) {
		t.Fatalf("pan = %v after a drag, want the pointer's movement", viewer.pan)
	}
}
