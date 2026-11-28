package notes

import (
	"context"
	"errors"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/imageviewer"
	"transactions/internal/domain"
)

const (
	// imageEdge is the longest edge a note picture is decoded at. The lightbox
	// shows nine tenths of the window, and the body draws the same bitmap scaled
	// down to its column, so one decode serves both: opening the note decodes a
	// picture once, and opening it again in the lightbox decodes nothing.
	imageEdge = 2048
)

// errNoImage is the lightbox's report while the shell cannot decode a picture.
var errNoImage = errors.New("没有可显示的图片")

// imageMenuState is a picture block's right-click menu: the block it commands and
// the point of the body it hangs from, so it opens under the pointer.
type imageMenuState struct {
	open  bool
	block int
	at    gfx.Point
}

// imageContextMenu declares the picture's menu as a Viva context menu at the point
// the secondary press landed, the way the "/" menu hangs from the caret. It is
// declared after the body, so the press that opens it never reaches the text.
func (m *Model) imageContextMenu(c ui.C, body ui.Box) {
	if !m.imageMenu.open {
		return
	}
	block := m.imageMenu.block
	ui.ContextMenuAt(c, body, &m.imageMenu.open, m.imageMenu.at, func(items *ui.MenuItems) {
		if items.Item("复制图片到剪贴板").Clicked() {
			m.copyImage(block)
			return
		}
		if items.Item("删除图片块").Destructive().Clicked() {
			m.deleteImage(block)
		}
	})
}

// openImageMenu puts the menu under the pointer over one picture block, and marks
// that block selected so the picture the menu commands is the one in the frame.
func (m *Model) openImageMenu(block int, at gfx.Point) {
	m.imageMenu = imageMenuState{open: true, block: block, at: at}
	m.selectedImage = block
}

// copyImage puts the picture on the clipboard as the picture itself and says so:
// any application's Paste inserts it, the way copying one from a photo viewer does.
func (m *Model) copyImage(block int) {
	m.imageMenu = imageMenuState{block: -1}
	if block < 0 || block >= len(m.blocks) || m.blocks[block].Kind != domain.NoteBlockImage ||
		m.cmds.CopyImage == nil {
		return
	}
	if err := m.cmds.CopyImage(m.blocks[block].ImagePath); err != nil {
		m.report(err)
		return
	}
	m.focusBody = true
	m.toast("图片已复制到剪贴板")
}

// deleteImage takes a picture block out of the note: the lines around it close up,
// and one undo brings the picture back.
func (m *Model) deleteImage(block int) {
	m.imageMenu = imageMenuState{block: -1}
	m.deleteBlock(block)
}

// lightbox is the picture the viewer shows. The viewer owns the zoom, the pan and
// the dialog; the page owns which picture it is and the decode behind it.
type lightbox struct {
	path string
	// width and height are the picture's own pixel size, taken from the block
	// that holds it: the dialog takes its shape from them, so it opens at its
	// final size before anything is decoded.
	width, height int
	bitmap        gfx.Bitmap
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
// opens on those pixels; the effect in View only stands in when that decode has
// not landed yet.
func (m *Model) openLightbox(path string, width, height int) {
	if path == "" {
		return
	}
	m.lightbox = lightbox{path: path, width: width, height: height}
	if bitmap, ok := m.noteImages[path]; ok && bitmap.Width > 0 {
		m.lightbox.bitmap = bitmap
	}
	m.viewer.Show(m.lightboxPicture())
}

// closeLightbox closes the picture.
func (m *Model) closeLightbox() {
	m.lightbox = lightbox{}
	m.viewer.Hide()
}

// lightboxPicture is what the viewer paints this paint: the copy the note decoded
// for its body, or the one the lightbox's own effect brought.
func (m *Model) lightboxPicture() imageviewer.Picture {
	return imageviewer.Picture{
		Path:   m.lightbox.path,
		Width:  m.lightbox.width,
		Height: m.lightbox.height,
		Bitmap: m.lightbox.bitmap,
	}
}

// lightboxView opens the viewer on the picture the page holds. The dialog is
// sized to the picture, nine tenths of the window at most, and the decoding runs
// off the paint.
func (m *Model) lightboxView(c ui.C) {
	if m.lightbox.path != "" {
		if bitmap, ok := m.noteImages[m.lightbox.path]; ok && bitmap.Width > 0 {
			m.lightbox.bitmap = bitmap
		} else {
			ui.Effect(c, m.lightboxEffect, m.lightboxKey())
			if bitmap, err, ok := m.lightboxEffect.Take(); ok && err == nil && bitmap.Width > 0 {
				m.lightbox.bitmap = bitmap
			}
		}
		m.viewer.Show(m.lightboxPicture())
	}
	if !m.viewer.View(c, imageviewer.Spec{Title: "笔记图片", Label: "笔记图片预览"}) {
		// The dialog closed itself: Escape, a press on the dim, or the close.
		m.lightbox = lightbox{}
	}
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
