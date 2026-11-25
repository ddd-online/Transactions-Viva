package notes

import (
	"context"
	"errors"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
)

const (
	// imageEdge is the longest edge a note picture is decoded at. The lightbox
	// shows nine tenths of the window, and the body draws the same bitmap scaled
	// down to its column, so one decode serves both: opening the note decodes a
	// picture once, and opening it again in the lightbox decodes nothing.
	imageEdge = 2048
	// lightboxShare is how much of the window a picture may take in the
	// lightbox: a screenshot opens at nine tenths of it, not in a fixed box.
	lightboxShare float32 = 0.9
)

// errNoImage is the lightbox's report while the shell cannot decode a picture.
var errNoImage = errors.New("没有可显示的图片")

// lightboxState is the one picture the user opened: its zoom and pan, and the
// gestures over it.
type lightboxState struct {
	open bool
	path string
	// width and height are the picture's own pixel size, taken from the block
	// that holds it. The dialog takes its shape from them, not from the bitmap:
	// a picture that is still decoding opens the dialog at its final size, so the
	// frame never chases the picture a frame behind.
	width, height int
	loaded        bool
	bitmap        gfx.Bitmap
	zoom          float32
	pan           gfx.Point
	dragFrom      gfx.Point
	dragging      bool
	handlers      ui.Handlers
}

// loadNoteImages decodes the pictures of one note for the editor. It runs off
// the paint. The view copies are read, not the originals: a note's pictures are
// scaled once when they are imported or first shown, and coming back to the note
// decodes a few megapixels instead of every camera original again.
func (m *Model) loadNoteImages(ctx context.Context, key noteImageKey) (map[string]gfx.Bitmap, error) {
	images := map[string]gfx.Bitmap{}
	if m.repo == nil || m.cmds.LoadImage == nil {
		return images, nil
	}
	note, err := m.repo.Note(ctx, key.note)
	if err != nil {
		return nil, err
	}
	for _, block := range note.Blocks {
		if block.Kind != domain.NoteBlockImage || block.ImagePath == "" {
			continue
		}
		if _, ok := images[block.ImagePath]; ok {
			continue
		}
		if bitmap, err := m.loadPicture(block.ImagePath, imageEdge); err == nil {
			images[block.ImagePath] = bitmap
		}
	}
	return images, nil
}

// loadPicture decodes one stored picture at maxEdge, through its view copy when
// the platform makes one.
func (m *Model) loadPicture(relative string, maxEdge int) (gfx.Bitmap, error) {
	if m.cmds.LoadView != nil {
		return m.cmds.LoadView(relative, maxEdge)
	}
	return m.cmds.LoadImage(relative, maxEdge)
}

// loadLightboxImage decodes the one picture the lightbox shows.
func (m *Model) loadLightboxImage(_ context.Context, key string) (gfx.Bitmap, error) {
	if m.cmds.LoadImage == nil {
		return gfx.Bitmap{}, errNoImage
	}
	return m.loadPicture(key, imageEdge)
}

// lightboxKey names the lightbox's work: the path alone, so the same picture is
// decoded once and reopening it is a lookup.
func (m *Model) lightboxKey() string {
	return m.lightbox.path
}

// openLightbox shows one picture, whose pixel size the block already knows. The
// note decoded it for its body too — same picture, same edge — so the dialog
// opens on those pixels; the effect below only stands in when that decode has not
// landed yet.
func (m *Model) openLightbox(path string, width, height int) {
	if path == "" {
		return
	}
	state := lightboxState{
		open: true, path: path, width: width, height: height,
		zoom: 1, handlers: m.lightbox.handlers,
	}
	if bitmap, ok := m.noteImages[path]; ok && bitmap.Width > 0 {
		state.bitmap, state.loaded = bitmap, true
	}
	m.lightbox = state
}

// closeLightbox closes the picture.
func (m *Model) closeLightbox() {
	m.lightbox = lightboxState{handlers: m.lightbox.handlers}
}

// lightboxView declares the lightbox modal. The dialog is sized to the picture,
// which itself is laid out inside nine tenths of the window: a wide screenshot
// fills it instead of sitting in a box of a fixed size, and a small picture gets
// a small dialog.
func (m *Model) lightboxView(c ui.C) {
	if !m.lightbox.open {
		return
	}
	theme := c.Theme()
	// The dialog insets its body by the surface padding, and the picture keeps
	// the same room around itself; asking for the body's width plus that padding
	// makes the body exactly the picture's width.
	pad := theme.Metrics.PadSurface + theme.Metrics.GapItem
	spec := ui.ModalSpec{Width: 900}
	size, sized := m.lightboxFit(c.Bounds())
	if sized {
		spec.Width = size.Width + 2*pad
	}
	ui.ModalWith(c, &m.lightbox.open, "笔记图片", spec, func() {
		view := ui.Control(c, "笔记图片预览", &m.lightbox.handlers).
			FillWidth().Height(540).
			Radius(theme.Metrics.RadiusSurface).Clip().
			Draw(m.drawLightbox)
		if sized {
			view.Height(size.Height)
		}
	}, nil)
	if !m.lightbox.open {
		m.closeLightbox()
	}
}

// lightboxFit is the size the picture takes in the lightbox: its own aspect
// ratio inside lightboxShare of the window. The shape comes from the block's
// pixel size, so it is known before the picture is decoded and does not change
// when it arrives; a block that lost its size falls back to the bitmap.
func (m *Model) lightboxFit(bounds gfx.Rect) (gfx.Size, bool) {
	width, height := float32(m.lightbox.width), float32(m.lightbox.height)
	if width <= 0 || height <= 0 {
		width, height = float32(m.lightbox.bitmap.Width), float32(m.lightbox.bitmap.Height)
	}
	if width <= 0 || height <= 0 {
		return gfx.Size{}, false
	}
	maxWidth, maxHeight := bounds.Width*lightboxShare, bounds.Height*lightboxShare
	if maxWidth <= 0 || maxHeight <= 0 {
		return gfx.Size{}, false
	}
	scale := min(maxWidth/width, maxHeight/height)
	return gfx.Size{Width: width * scale, Height: height * scale}, true
}

// drawLightbox paints the picture at its zoom and pan.
func (m *Model) drawLightbox(d *ui.DrawContext) {
	theme := *d.Theme
	d.Canvas.Fill(d.Rect, theme.Muted)
	if !m.lightbox.loaded || m.lightbox.bitmap.Width == 0 {
		d.Canvas.Label("正在载入图片……", gfx.Rect{X: d.Rect.X + 16, Y: d.Rect.Y + 16, Width: d.Rect.Width - 32, Height: 20},
			gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
		return
	}
	drawZoomed(d.Canvas, m.lightbox.bitmap, d.Rect, m.lightbox.zoom, m.lightbox.pan)
}

// lightboxScroll zooms the picture with the wheel.
func (m *Model) lightboxScroll(event gfx.ScrollEvent) gfx.EventResult {
	zoom := m.lightbox.zoom * (1 - event.DeltaY*0.0015)
	m.lightbox.zoom = clamp(zoom, 1, 6)
	if m.lightbox.zoom <= 1 {
		m.lightbox.pan = gfx.Point{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// lightboxPointer drags the zoomed picture.
func (m *Model) lightboxPointer(event gfx.PointerEvent) gfx.EventResult {
	switch event.Kind {
	case gfx.PointerDown:
		m.lightbox.dragging = m.lightbox.zoom > 1
		m.lightbox.dragFrom = gfx.Point{X: event.X, Y: event.Y}
		if m.lightbox.dragging {
			return gfx.EventResult{Handled: true}
		}
	case gfx.PointerMove:
		if !m.lightbox.dragging {
			return gfx.EventResult{}
		}
		m.lightbox.pan.X += event.X - m.lightbox.dragFrom.X
		m.lightbox.pan.Y += event.Y - m.lightbox.dragFrom.Y
		m.lightbox.dragFrom = gfx.Point{X: event.X, Y: event.Y}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.PointerUp:
		if m.lightbox.dragging {
			m.lightbox.dragging = false
			return gfx.EventResult{Handled: true}
		}
	}
	return gfx.EventResult{}
}

// pickPicture asks the shell for a picture and inserts it after the caret.
// pickPicture asks the shell for a picture from an input pass that has no C: the
// deferred call runs after the pointer pass, never inside the paint.
func (m *Model) pickPicture() {
	if m.cmds.PickImage == nil || m.selected == "" || m.canvas == nil {
		return
	}
	m.canvas.AfterPaint(func() { m.pickAndInsert() })
}

// pickAndInsert runs the picture dialog. The shell calls it from c.After, so the
// native dialog never runs inside a paint.
func (m *Model) pickAndInsert() {
	if m.selected == "" || m.cmds.PickImage == nil || m.cmds.ImportImage == nil {
		return
	}
	source, err := m.cmds.PickImage()
	if err != nil || source == "" {
		return
	}
	path, width, height, err := m.cmds.ImportImage(m.selected, source)
	if err != nil {
		m.report(err)
		return
	}
	m.insertPicture(domain.NoteBlock{
		Kind: domain.NoteBlockImage, ImagePath: path, ImageWidth: width,
		ImageHeight: height, ImageScale: domain.NoteImageDefaultScale,
	})
}

// dropImage forgets a picture the workspace deleted.
func (m *Model) dropImage(path string) {
	delete(m.noteImages, path)
	if m.lightbox.path == path {
		m.closeLightbox()
	}
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
	canvas.DrawBitmap(bitmap, gfx.Rect{X: x, Y: y, Width: targetWidth, Height: targetHeight})
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
