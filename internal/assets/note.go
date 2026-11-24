package assets

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// NoteDir is the relative directory of one note's images.
func NoteDir(noteID string) string { return path.Join("notes", noteID) }

// SaveNoteImage writes one note image under notes/<noteID>/ and returns its path
// relative to the assets root. A note image is a single display file: HEIC is
// transcoded to JPEG by the caller before it gets here, and no thumbnail or view
// copy is made, because the note editor decodes what it shows at the width it
// needs.
func SaveNoteImage(workspaceDir, noteID, name string, display []byte, format Format) (string, error) {
	noteID = strings.TrimSpace(noteID)
	if err := checkName(noteID); err != nil {
		return "", fmt.Errorf("非法笔记：%w", err)
	}
	if err := checkName(name); err != nil {
		return "", err
	}
	if !format.Displayable() {
		return "", fmt.Errorf("不支持的图片格式：%s", format)
	}
	if format == FormatHEIC {
		return "", errors.New("HEIC 展示图需要先转码成 JPEG")
	}
	if len(display) == 0 {
		return "", errors.New("图片内容为空")
	}
	if len(display) > MaxImageBytes {
		return "", fmt.Errorf("图片超过 %d MB", MaxImageBytes>>20)
	}
	if sniffed := Sniff(display); sniffed != format {
		return "", fmt.Errorf("图片格式与内容不符：%s", sniffed)
	}
	directory := filepath.Join(Root(workspaceDir), filepath.FromSlash(NoteDir(noteID)))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("创建笔记图片目录失败：%w", err)
	}
	fileName := name + "." + format.Extension()
	full := filepath.Join(directory, fileName)
	if err := os.WriteFile(full, display, 0o644); err != nil {
		return "", fmt.Errorf("保存笔记图片失败：%w", err)
	}
	return path.Join(NoteDir(noteID), fileName), nil
}
