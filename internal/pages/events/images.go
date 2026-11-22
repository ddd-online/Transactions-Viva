package events

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/domain"
)

// The page paints pictures itself: it decodes through the shell's LoadImage
// command and keeps the bitmaps for the selected event. A canvas draw is pure,
// so every decode happens where the selection changes — never in View.

// bitmap returns a decoded image, loading it once. A path that failed is
// remembered so a paint does not ask for it again.
func (m *Model) bitmap(relative string) (gfx.Bitmap, bool) {
	if relative == "" {
		return gfx.Bitmap{}, false
	}
	if m.bits == nil {
		m.bits = map[string]gfx.Bitmap{}
	}
	if m.bitErrs == nil {
		m.bitErrs = map[string]string{}
	}
	if bitmap, ok := m.bits[relative]; ok {
		return bitmap, true
	}
	if _, failed := m.bitErrs[relative]; failed {
		return gfx.Bitmap{}, false
	}
	if m.cmds.LoadImage == nil {
		m.bitErrs[relative] = "图片无法显示"
		return gfx.Bitmap{}, false
	}
	bitmap, err := m.cmds.LoadImage(relative)
	if err != nil {
		m.bitErrs[relative] = err.Error()
		return gfx.Bitmap{}, false
	}
	m.bits[relative] = bitmap
	return bitmap, true
}

// blur returns a heavily blurred copy of an image's thumbnail, the backdrop the
// stage puts behind a photo that does not fill the pane.
func (m *Model) blur(image domain.KeyEventImage) (gfx.Bitmap, bool) {
	key := "blur:" + image.ID
	if bitmap, ok := m.bits[key]; ok {
		return bitmap, true
	}
	base, ok := m.bitmap(image.ThumbPath)
	if !ok {
		base, ok = m.bitmap(image.FilePath)
	}
	if !ok {
		return gfx.Bitmap{}, false
	}
	blurred := gfx.GaussianBlurBitmap(base, 12)
	m.bits[key] = blurred
	return blurred, true
}

// ensureBitmaps decodes what the current selection paints: every thumbnail, the
// selected photo and its blurred backdrop. It is called when the selection or
// the list changes, not by View.
func (m *Model) ensureBitmaps() {
	for index := range m.images {
		image := m.images[index]
		if _, ok := m.bitmap(image.ThumbPath); !ok {
			// A missing thumbnail falls back to the display file, loaded here so
			// a draw never decodes anything.
			m.bitmap(image.FilePath)
		}
	}
	image, ok := m.mainImage()
	if !ok {
		return
	}
	m.bitmap(image.FilePath)
	m.blur(*image)
}

// drawStage paints the stage: the blurred thumbnail filling the pane, a scrim
// over it, then the photo whole (contain).
func (m *Model) drawStage(d *ui.DrawContext, image domain.KeyEventImage) {
	theme := *d.Theme
	d.Canvas.Fill(d.Rect, theme.Muted)
	if backdrop, ok := m.blur(image); ok {
		d.Canvas.DrawBitmapRegion(backdrop, coverSource(backdrop, d.Rect), d.Rect)
		d.Canvas.Fill(d.Rect, theme.Foreground.WithAlpha(0.18))
	}
	display, ok := m.bitmap(image.FilePath)
	if !ok {
		m.paintMissing(d)
		return
	}
	drawContained(d.Canvas, display, d.Rect)
}

// drawThumb paints one 160×90 slot with its picture cropped to the slot.
func (m *Model) drawThumb(d *ui.DrawContext, image domain.KeyEventImage) {
	theme := *d.Theme
	d.Canvas.Fill(d.Rect, theme.Muted)
	bitmap, ok := m.bitmap(image.ThumbPath)
	if !ok {
		bitmap, ok = m.bitmap(image.FilePath)
	}
	if !ok {
		return
	}
	d.Canvas.DrawBitmapRegion(bitmap, coverSource(bitmap, d.Rect), d.Rect)
}

// paintMissing says why a picture is not on screen instead of leaving the stage
// as a bare coloured rectangle.
func (m *Model) paintMissing(d *ui.DrawContext) {
	theme := *d.Theme
	message := "图片文件缺失或无法读取"
	image, ok := m.mainImage()
	if ok {
		if detail := m.bitErrs[image.FilePath]; detail != "" {
			message = "图片无法显示：" + detail
		}
	}
	d.Canvas.LabelCenter(message, gfx.Rect{
		X: d.Rect.X, Y: d.Rect.Y + d.Rect.Height/2 - 10,
		Width: d.Rect.Width, Height: 20,
	}, gfx.TextStyle{Size: typography.SizeBody, Color: theme.MutedForeground})
}

// openPreview shows the selected picture in the lightbox at fit size.
func (m *Model) openPreview() {
	image, ok := m.mainImage()
	if !ok {
		return
	}
	m.preview.open = true
	m.preview.index = m.imageIndex(image.ID)
	m.preview.zoom = 1
	m.preview.pan = gfx.Point{}
	m.preview.dragging = false
	m.bitmap(image.FilePath)
}

// stepPreview moves the lightbox to the next or previous picture, wrapping.
func (m *Model) stepPreview(delta int) {
	if len(m.images) == 0 {
		return
	}
	index := (m.preview.index + delta) % len(m.images)
	if index < 0 {
		index += len(m.images)
	}
	m.preview.index = index
	m.selectedImageID = m.images[index].ID
	m.preview.zoom = 1
	m.preview.pan = gfx.Point{}
	m.bitmap(m.images[index].FilePath)
	m.ensureBitmaps()
}

// previewPointer drags the zoomed lightbox picture.
func (m *Model) previewPointer(event gfx.PointerEvent) gfx.EventResult {
	switch event.Kind {
	case gfx.PointerDown:
		m.preview.dragging = true
		m.preview.last = gfx.Point{X: event.X, Y: event.Y}
		return gfx.EventResult{Handled: true}
	case gfx.PointerMove:
		if !m.preview.dragging {
			return gfx.EventResult{}
		}
		m.preview.pan.X += event.X - m.preview.last.X
		m.preview.pan.Y += event.Y - m.preview.last.Y
		m.preview.last = gfx.Point{X: event.X, Y: event.Y}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.PointerUp:
		if !m.preview.dragging {
			return gfx.EventResult{}
		}
		m.preview.dragging = false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// previewScroll zooms the lightbox between fit (1×) and 6×; back at fit it also
// recentres the picture.
func (m *Model) previewScroll(event gfx.ScrollEvent) gfx.EventResult {
	if !m.preview.open {
		return gfx.EventResult{}
	}
	const step = 1.12
	if event.DeltaY < 0 {
		m.preview.zoom *= step
	} else {
		m.preview.zoom /= step
	}
	m.preview.zoom = clamp(m.preview.zoom, 1, 6)
	if m.preview.zoom == 1 {
		m.preview.pan = gfx.Point{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// previewKey pages the lightbox with the arrow keys.
func (m *Model) previewKey(event gfx.KeyEvent) gfx.EventResult {
	if !m.preview.open {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyLeft:
		m.stepPreview(-1)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyRight:
		m.stepPreview(1)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// drawPreview paints the lightbox: a blurred backdrop, then the picture at the
// user's zoom and pan.
func (m *Model) drawPreview(d *ui.DrawContext) {
	theme := *d.Theme
	d.Canvas.Fill(d.Rect, theme.Muted)
	image, ok := m.mainImage()
	if !ok {
		return
	}
	if backdrop, ok := m.blur(*image); ok {
		d.Canvas.DrawBitmapRegion(backdrop, coverSource(backdrop, d.Rect), d.Rect)
		d.Canvas.Fill(d.Rect, theme.Foreground.WithAlpha(0.45))
	}
	display, ok := m.bitmap(image.FilePath)
	if !ok {
		m.paintMissing(d)
		return
	}
	drawZoomed(d.Canvas, display, d.Rect, m.preview.zoom, m.preview.pan)
}

// coverSource is the centred source rectangle that fills bounds without
// distorting the picture: the crop a thumbnail slot uses.
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

// drawContained draws the whole picture inside bounds, centred, without
// cropping it. The letterbox is the caller's backdrop.
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
