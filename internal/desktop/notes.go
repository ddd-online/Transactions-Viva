package desktop

import (
	"errors"
	"fmt"
	"os"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"

	"transactions/internal/assets"
	notespage "transactions/internal/pages/notes"
)

// errNoNoteWorkspace is what the notes commands report while no workspace is
// open. The section is not reachable then, so this only guards a stale command.
var errNoNoteWorkspace = errors.New("还没有打开工作空间")

// notesCommands is the platform work the 笔记 page asks for: the picture dialog,
// the workspace's picture files and the clipboard.
func (m *Model) notesCommands() notespage.Commands {
	return notespage.Commands{
		PickImage:   m.pickNoteImage,
		ImportImage: m.importNoteImage,
		PasteImage:  m.pasteNoteImage,
		LoadImage:   m.loadNoteImage,
		LoadView:    m.loadNoteView,
	}
}

// pickNoteImage opens the picture dialog and returns the chosen file. The
// picker is the shell's own, so a test can answer it without a native dialog.
func (m *Model) pickNoteImage() (string, error) {
	if m.pickNoteFile == nil {
		return "", nil
	}
	path, err := m.pickNoteFile()
	if errors.Is(err, app.ErrCanceled) {
		return "", nil
	}
	return path, err
}

// importNoteImage stores one file under notes/<noteID>/ and reports its size.
func (m *Model) importNoteImage(noteID, source string) (string, int, int, error) {
	if m.workspace == nil {
		return "", 0, 0, errNoNoteWorkspace
	}
	display, format, width, height, err := readNotePicture(source)
	if err != nil {
		return "", 0, 0, err
	}
	path, err := assets.SaveNoteImage(m.workspace.Directory(), noteID, assets.NewName(), display, format)
	if err != nil {
		return "", 0, 0, err
	}
	return path, width, height, nil
}

// pasteNoteImage stores the clipboard's picture under notes/<noteID>/.
func (m *Model) pasteNoteImage(noteID string) (string, int, int, error) {
	if m.workspace == nil {
		return "", 0, 0, errNoNoteWorkspace
	}
	data, err := app.ClipboardImagePNG()
	if err != nil {
		return "", 0, 0, err
	}
	picture, err := assets.Decode(data)
	if err != nil {
		return "", 0, 0, err
	}
	path, err := assets.SaveNoteImage(m.workspace.Directory(), noteID, assets.NewName(), data, assets.FormatPNG)
	if err != nil {
		return "", 0, 0, err
	}
	bounds := picture.Bounds()
	return path, bounds.Dx(), bounds.Dy(), nil
}

// loadNoteImage decodes one stored picture for painting.
func (m *Model) loadNoteImage(relative string, maxEdge int) (gfx.Bitmap, error) {
	if m.workspace == nil {
		return gfx.Bitmap{}, errNoNoteWorkspace
	}
	return assets.Load(m.workspace.Directory(), relative, maxEdge)
}

// loadNoteView decodes a picture's view copy: the picture scaled and encoded once
// beside the original, so opening a note or its lightbox decodes a few
// megapixels instead of the camera original. A picture imported before the copy
// existed gets one on its first open, and never pays for it again.
func (m *Model) loadNoteView(relative string, maxEdge int) (gfx.Bitmap, error) {
	if m.workspace == nil {
		return gfx.Bitmap{}, errNoNoteWorkspace
	}
	return assets.LoadView(m.workspace.Directory(), relative, maxEdge)
}

// readNotePicture reads one file and returns the bytes to store, their format
// and the picture's size. HEIC is transcoded to JPEG first: the workspace keeps
// one display copy, and the editor decodes that copy.
func readNotePicture(source string) ([]byte, assets.Format, int, int, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, "", 0, 0, fmt.Errorf("读取图片失败：%w", err)
	}
	if len(data) > assets.MaxImageBytes {
		return nil, "", 0, 0, fmt.Errorf("图片超过 %d MB", assets.MaxImageBytes>>20)
	}
	picture, err := assets.Decode(data)
	if err != nil {
		return nil, "", 0, 0, err
	}
	format := assets.Sniff(data)
	if format == assets.FormatHEIC {
		data, err = assets.EncodeJPEG(picture, assets.DisplayJPEGQuality)
		if err != nil {
			return nil, "", 0, 0, err
		}
		format = assets.FormatJPEG
	}
	if !format.Displayable() {
		return nil, "", 0, 0, fmt.Errorf("不支持的图片格式：%s", format)
	}
	bounds := picture.Bounds()
	return data, format, bounds.Dx(), bounds.Dy(), nil
}
