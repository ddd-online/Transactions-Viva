// Package imageviewer is the one way a picture opens enlarged: a dialog shaped
// like the picture and no larger than nine tenths of the window, a wheel zoom, a
// drag, and whatever footer the page puts under it — the events page's
// 上一张 / 下一张, or nothing, as the notes page has it. 笔记 and 事件 both enlarge
// pictures, and they enlarge them the same way; the size and the gestures live
// here so the two cannot drift apart.
//
// The dialog opens without motion on purpose. Viva's overlay entrance fades the
// scrim and the panel but skips bitmaps (see the presence pass in gfx), so a
// fading picture dialog arrives fully bright over a window that is still
// undimmed, and the background darkens behind a picture that is already there. A
// viewer opens at once instead: scrim, panel and picture land in one frame.
package imageviewer

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
)

const (
	// share is how much of the window a picture may take. A screenshot opens at
	// nine tenths of it rather than in a fixed box, so a wide picture fills the
	// screen and a portrait one is not lost in a letterbox.
	share float32 = 0.9
	// zoomLeast and zoomMost bound the wheel: fit size to six times it.
	zoomLeast, zoomMost float32 = 1, 6
	// zoomPerPixel is how fast the wheel zooms. A wheel notch reports 120, so one
	// notch is about 1.12×; a trackpad's small deltas stay smooth.
	zoomPerPixel float32 = 0.001
	// defaultWidth and defaultHeight are the dialog while the picture's shape is
	// unknown: a picture block that lost its size and has no bitmap yet.
	defaultWidth, defaultHeight float32 = 900, 540
)

// Picture is the picture the viewer shows. The page owns where it comes from and
// which one is open; the viewer owns how it looks.
type Picture struct {
	// Path names the picture. Showing a different path restarts the view at fit
	// size; showing the same one keeps the user's zoom and pan.
	Path string
	// Width and Height are the picture's own pixel size. The dialog takes its
	// shape from them, so a picture whose bitmap is still arriving opens at its
	// final size and never resizes. Zero falls back to the bitmap's size, then to
	// the stand-in's.
	Width, Height int
	// Bitmap is the decoded picture, empty until it arrives.
	Bitmap gfx.Bitmap
	// Waiting is the stand-in painted while Bitmap is empty, such as the
	// thumbnail the page already has.
	Waiting gfx.Bitmap
	// Backdrop is an optional blurred cover painted behind the picture and dimmed
	// under it, for a pane that would otherwise be a flat letterbox.
	Backdrop gfx.Bitmap
}

// Spec is what the page adds to one paint of the viewer.
type Spec struct {
	// Title names the dialog for assistive technology.
	Title string
	// Label is the picture control's accessible name.
	Label string
	// Footer, when not nil, is declared under the picture. It closes over the
	// page's C.
	Footer func()
	// Keys is the page's own key handling, such as paging through a gallery's
	// pictures; the viewer's own keys come after it.
	Keys func(gfx.KeyEvent) gfx.EventResult
}

// Viewer is one page's viewer: which picture is open, the user's zoom and pan,
// and the dialog's own bookkeeping.
type Viewer struct {
	open    bool
	picture Picture
	spec    Spec

	zoom     float32
	pan      gfx.Point
	dragFrom gfx.Point
	dragging bool

	handlers ui.Handlers
	// The dialog paints with motion off, because a bitmap cannot fade; these are
	// the canvas it borrowed and the setting to give back when it closes.
	canvas       *gfx.Canvas
	reduceBefore bool
	reduceSaved  bool
}

// New returns a viewer with nothing open.
func New() *Viewer {
	viewer := &Viewer{zoom: zoomLeast}
	viewer.handlers = ui.Handlers{
		Pointer: viewer.pointer,
		Scroll:  viewer.scroll,
		Key:     viewer.key,
	}
	return viewer
}

// Show opens the viewer on picture, or updates the picture it already shows.
// Changing to another picture restarts the zoom; the same picture with a bitmap
// that has just arrived keeps it.
func (v *Viewer) Show(picture Picture) {
	if !v.open || picture.Path != v.picture.Path {
		v.zoom, v.pan, v.dragging = zoomLeast, gfx.Point{}, false
	}
	v.picture, v.open = picture, true
}

// Hide closes the viewer.
func (v *Viewer) Hide() {
	v.open, v.dragging, v.pan = false, false, gfx.Point{}
}

// Shown reports whether the viewer is open.
func (v *Viewer) Shown() bool { return v.open }

// View declares the dialog while the viewer is open, and reports whether it was:
// a page ends the picture it keeps when the dialog closed itself (Escape, a
// press on the dim, or the close button).
func (v *Viewer) View(c ui.C, spec Spec) bool {
	if !v.open {
		v.restoreMotion()
		return false
	}
	v.spec = spec
	theme := c.Theme()
	// The dialog insets its body by the surface padding, and the picture keeps
	// the same room around itself; asking for the body's width plus that padding
	// makes the body exactly the picture's width.
	pad := theme.Metrics.PadSurface + theme.Metrics.GapItem
	// A footer is part of the dialog, not of the picture: the picture gives it
	// room, so the buttons land under it instead of over its bottom edge.
	reserve := float32(0)
	if spec.Footer != nil {
		reserve = theme.Metrics.ControlHeight + theme.Metrics.GapItem
	}
	size, shaped := v.fit(c.Bounds(), reserve)
	modal := ui.ModalSpec{Width: defaultWidth}
	height := defaultHeight
	if shaped {
		modal.Width = size.Width + 2*pad
		height = size.Height
	}
	// A picture cannot fade, so the lightbox paints with motion off: scrim,
	// panel and picture land together. The setting is given back as soon as the
	// dialog closes, and a reader who asked for reduced motion keeps it.
	v.canvas = c.Canvas()
	if !v.reduceSaved {
		v.reduceBefore, v.reduceSaved = v.canvas.ReduceMotion(), true
	}
	v.canvas.SetReduceMotion(true)
	ui.ModalWith(c, &v.open, spec.Title, modal, func() {
		ui.Control(c, spec.Label, &v.handlers).
			FillWidth().Height(height).
			Radius(theme.Metrics.RadiusSurface).Clip().
			Draw(v.draw)
	}, spec.Footer)
	return v.open
}

// restoreMotion gives the canvas back the reduced-motion setting the viewer
// borrowed, so the rest of the application keeps its motion.
func (v *Viewer) restoreMotion() {
	if !v.reduceSaved || v.canvas == nil {
		return
	}
	v.canvas.SetReduceMotion(v.reduceBefore)
	v.reduceSaved = false
}

// fit is the size the picture takes: its own aspect inside share of the window,
// less whatever the dialog keeps for itself under it.
func (v *Viewer) fit(bounds gfx.Rect, reserve float32) (gfx.Size, bool) {
	width, height := float32(v.picture.Width), float32(v.picture.Height)
	if width <= 0 || height <= 0 {
		width, height = float32(v.picture.Bitmap.Width), float32(v.picture.Bitmap.Height)
	}
	if width <= 0 || height <= 0 {
		width, height = float32(v.picture.Waiting.Width), float32(v.picture.Waiting.Height)
	}
	if width <= 0 || height <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return gfx.Size{}, false
	}
	maxHeight := bounds.Height*share - reserve
	if maxHeight <= 0 {
		return gfx.Size{}, false
	}
	scale := min(bounds.Width*share/width, maxHeight/height)
	return gfx.Size{Width: width * scale, Height: height * scale}, true
}

// draw paints the pane: the muted surface, the backdrop when the page has one,
// then the picture — or its stand-in while it is still arriving.
func (v *Viewer) draw(d *ui.DrawContext) {
	theme := *d.Theme
	canvas := d.Canvas
	canvas.Fill(d.Rect, theme.Muted)
	if v.picture.Backdrop.Width > 0 {
		canvas.DrawBitmapRegion(v.picture.Backdrop, coverSource(v.picture.Backdrop, d.Rect), d.Rect)
		canvas.Fill(d.Rect, theme.Foreground.WithAlpha(0.45))
	}
	if v.picture.Bitmap.Width > 0 {
		drawZoomed(canvas, v.picture.Bitmap, d.Rect, v.zoom, v.pan)
		return
	}
	if v.picture.Waiting.Width > 0 {
		drawContained(canvas, v.picture.Waiting, d.Rect)
		return
	}
	canvas.Label("正在载入图片……", gfx.Rect{
		X: d.Rect.X + theme.Metrics.PadSurface, Y: d.Rect.Y + theme.Metrics.PadSurface,
		Width: max(d.Rect.Width-2*theme.Metrics.PadSurface, 0), Height: 20,
	}, gfx.TextStyle{Size: typography.SizeBody, Color: theme.MutedForeground})
}

// pointer drags a zoomed picture. At fit size there is nothing to move, so a
// press does nothing and the press that opened the dialog stays a press.
func (v *Viewer) pointer(event gfx.PointerEvent) gfx.EventResult {
	switch event.Kind {
	case gfx.PointerDown:
		v.dragging = v.zoom > zoomLeast
		v.dragFrom = gfx.Point{X: event.X, Y: event.Y}
		if v.dragging {
			return gfx.EventResult{Handled: true}
		}
	case gfx.PointerMove:
		if !v.dragging {
			return gfx.EventResult{}
		}
		v.pan.X += event.X - v.dragFrom.X
		v.pan.Y += event.Y - v.dragFrom.Y
		v.dragFrom = gfx.Point{X: event.X, Y: event.Y}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.PointerUp:
		if v.dragging {
			v.dragging = false
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	return gfx.EventResult{}
}

// scroll zooms the picture with the wheel; back at fit size it also recentres it.
func (v *Viewer) scroll(event gfx.ScrollEvent) gfx.EventResult {
	zoom := v.zoom * (1 - event.DeltaY*zoomPerPixel)
	v.zoom = clamp(zoom, zoomLeast, zoomMost)
	if v.zoom <= zoomLeast {
		v.pan = gfx.Point{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// key lets the page page through its pictures first, then handles nothing of its
// own: the dialog owns Escape and the keyboard trap.
func (v *Viewer) key(event gfx.KeyEvent) gfx.EventResult {
	if v.spec.Keys == nil {
		return gfx.EventResult{}
	}
	return v.spec.Keys(event)
}

// drawZoomed draws the picture at fit × zoom, moved by pan, clamped so it cannot
// be thrown clear of the pane.
func drawZoomed(canvas *gfx.Canvas, bitmap gfx.Bitmap, bounds gfx.Rect, zoom float32, pan gfx.Point) {
	width, height := float32(bitmap.Width), float32(bitmap.Height)
	if width <= 0 || height <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	scale := min(bounds.Width/width, bounds.Height/height) * max(zoom, 1)
	targetWidth, targetHeight := width*scale, height*scale
	maxX := max((targetWidth-bounds.Width)/2, 0)
	maxY := max((targetHeight-bounds.Height)/2, 0)
	x := bounds.X + (bounds.Width-targetWidth)/2 + clamp(pan.X, -maxX, maxX)
	y := bounds.Y + (bounds.Height-targetHeight)/2 + clamp(pan.Y, -maxY, maxY)
	canvas.Clip(bounds)
	canvas.DrawBitmap(bitmap, gfx.Rect{X: x, Y: y, Width: targetWidth, Height: targetHeight})
	canvas.Unclip()
}

// drawContained draws the whole picture inside bounds, centred, without cropping
// it: the stand-in is shown as the user will see it, not stretched.
func drawContained(canvas *gfx.Canvas, bitmap gfx.Bitmap, bounds gfx.Rect) {
	width, height := float32(bitmap.Width), float32(bitmap.Height)
	if width <= 0 || height <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	scale := min(bounds.Width/width, bounds.Height/height)
	target := gfx.Rect{
		X: bounds.X + (bounds.Width-width*scale)/2, Y: bounds.Y + (bounds.Height-height*scale)/2,
		Width: width * scale, Height: height * scale,
	}
	canvas.Clip(bounds)
	canvas.DrawBitmap(bitmap, target)
	canvas.Unclip()
}

// coverSource is the centred source rectangle that fills bounds without
// distorting the picture: the crop a backdrop uses.
func coverSource(bitmap gfx.Bitmap, bounds gfx.Rect) gfx.Rect {
	width, height := float32(bitmap.Width), float32(bitmap.Height)
	if width <= 0 || height <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return gfx.Rect{Width: width, Height: height}
	}
	target := bounds.Width / bounds.Height
	sourceWidth, sourceHeight := width, height
	if width/height > target {
		sourceWidth = height * target
	} else {
		sourceHeight = width / target
	}
	return gfx.Rect{
		X: (width - sourceWidth) / 2, Y: (height - sourceHeight) / 2,
		Width: sourceWidth, Height: sourceHeight,
	}
}

func clamp(value, least, greatest float32) float32 {
	if value < least {
		return least
	}
	if value > greatest {
		return greatest
	}
	return value
}
