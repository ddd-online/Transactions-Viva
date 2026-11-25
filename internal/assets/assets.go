// Package assets owns the image files in a workspace: <workspace>/data/assets
// and the key_events layout inside it. Event images live under
// key_events/<YYYY-MM-DD>/ as
//
//	<id>.<ext>          the display file, in the format it was imported in
//	thumb_<id>.jpg      the list thumbnail, 300 px wide at most, JPEG q75
//	<id>.heic           the HEIC/HEIF original, when the display file is a JPEG
//	                    made from it at import time
//	<id>.<ext>          the live photo's motion part, when the photo is one
//
// Decoding is the application's job, not the framework's: JPEG, PNG, GIF and
// WebP come from the standard library and golang.org/x/image, HEIC from the
// pure Go HEVC decoder github.com/gen2brain/h265/heic. HEIC is expensive to
// decode, so the importer runs it once and stores a JPEG display copy beside the
// original; a paint only ever reads the cheap copy.
//
// The package is deliberately outside the pages: a page may paint a gfx.Bitmap
// but never opens a file. The shell and the import tool call these functions.
package assets

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"transactions/internal/ids"
)

// Limits protect a local application from a picture it cannot afford to decode.
// The project had no caps because a browser did the decoding; here the
// process does it, so a 200 megapixel panorama is refused with a sentence
// instead of exhausting memory.
const (
	// MaxImageBytes is the largest still a caller may hand to Decode or Save.
	MaxImageBytes = 64 << 20
	// MaxImagePixels is the largest pixel count Decode accepts.
	MaxImagePixels = 64_000_000
	// MaxDisplayEdge is the longest edge a bitmap loaded for painting keeps.
	MaxDisplayEdge = 2048
	// ThumbnailWidth is the thumbnail's width in bitmap pixels.
	ThumbnailWidth = 300
	// ThumbnailQuality is the JPEG quality of a thumbnail.
	ThumbnailQuality = 75
	// ViewMaxEdge is the longest edge of the stage's view copy: large enough
	// for the detail pane, small enough that switching pictures does not decode
	// a 12 megapixel camera original every time. A caller with a larger pane asks
	// for a larger copy — up to MaxDisplayEdge — and gets one cached beside the
	// picture like this one.
	ViewMaxEdge = 1600
	// ViewJPEGQuality is the JPEG quality of that view copy.
	ViewJPEGQuality = 85
	// DisplayJPEGQuality is the JPEG quality of a HEIC still's display copy.
	// The application transcoded at 92: close to the original's look while
	// still a format every platform draws without a codec.
	DisplayJPEGQuality = 92
)

// Root is the assets directory of a workspace.
func Root(workspaceDir string) string { return filepath.Join(workspaceDir, "data", "assets") }

// keyEventDir is the relative directory of one day's event images.
func keyEventDir(date string) string { return path.Join("key_events", date) }

// ViewPath is the stage copy stored beside an image. It is derived, not stored
// in the database: an older workspace gets one the first time the picture is
// shown, and deleting the image removes it with the other files.
func ViewPath(relative string) string {
	if relative == "" {
		return ""
	}
	return relative + ".view.jpg"
}

// Resolve turns a stored relative path into a file path inside the workspace's
// assets directory. A path that escapes it (an absolute path, "..", a drive) is
// refused: the database is user data and must not be able to make the
// application read or delete anything outside the workspace.
func Resolve(workspaceDir, relative string) (string, error) {
	relative = strings.TrimSpace(relative)
	if relative == "" {
		return "", errors.New("资产路径为空")
	}
	if filepath.IsAbs(relative) || path.IsAbs(relative) || strings.ContainsRune(relative, '\x00') {
		return "", fmt.Errorf("非法资产路径：%s", relative)
	}
	root, err := filepath.Abs(Root(workspaceDir))
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法资产路径：%s", relative)
	}
	return candidate, nil
}

// Files is one image prepared for storage.
type Files struct {
	// Display is the still's bytes, in DisplayFormat.
	Display []byte
	// DisplayFormat is the sniffed format; its extension names the file.
	DisplayFormat Format
	// Thumb is the JPEG thumbnail made from the still.
	Thumb []byte
	// View is a smaller JPEG the stage paints; empty leaves it to the loader to
	// make one on first use.
	View []byte
	// Motion is the live photo's video; MotionExt says what it is.
	Motion    []byte
	MotionExt string
	// Original is the HEIC/HEIF source when Display is a JPEG made from it.
	// The application kept the original beside its display copy so the
	// best-quality bytes are never lost.
	Original       []byte
	OriginalFormat Format
}

// Paths are the stored paths, relative to the assets root.
type Paths struct {
	FilePath     string
	ThumbPath    string
	OriginalPath string
	MotionPath   string
	ViewPath     string
}

// NewName is the image's id and the stem its files are named after.
func NewName() string { return ids.New() }

// Save writes one image's files under key_events/<date>/ and returns their
// relative paths. A file written before a later write fails is removed, so a
// failed save leaves nothing behind.
func Save(workspaceDir, date, name string, files Files) (Paths, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return Paths{}, fmt.Errorf("非法事件日期：%s", date)
	}
	if err := checkName(name); err != nil {
		return Paths{}, err
	}
	if !files.DisplayFormat.Displayable() {
		return Paths{}, fmt.Errorf("不支持的图片格式：%s", files.DisplayFormat)
	}
	// The display copy is what every paint decodes, so it is never HEIC: the
	// importer transcodes HEIC to JPEG and keeps the original separately.
	if files.DisplayFormat == FormatHEIC {
		return Paths{}, errors.New("HEIC 展示图需要先转码成 JPEG")
	}
	if len(files.Display) == 0 {
		return Paths{}, errors.New("图片内容为空")
	}
	if len(files.Thumb) == 0 || Sniff(files.Thumb) != FormatJPEG {
		return Paths{}, errors.New("缩略图必须是 JPEG")
	}
	if len(files.View) > 0 && Sniff(files.View) != FormatJPEG {
		return Paths{}, errors.New("展示图必须是 JPEG")
	}
	motionExt := strings.ToLower(strings.TrimSpace(files.MotionExt))
	if len(files.Motion) > 0 && !motionExtension(motionExt) {
		return Paths{}, fmt.Errorf("实况视频格式不支持：%s", files.MotionExt)
	}
	if len(files.Original) > 0 {
		if files.OriginalFormat != FormatHEIC || Sniff(files.Original) != FormatHEIC {
			return Paths{}, errors.New("只能保留 HEIC / HEIF 原图")
		}
	}

	directory := filepath.Join(Root(workspaceDir), filepath.FromSlash(keyEventDir(date)))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return Paths{}, fmt.Errorf("创建图片目录失败：%w", err)
	}
	displayName := name + "." + files.DisplayFormat.Extension()
	thumbName := "thumb_" + name + ".jpg"
	viewName := displayName + ".view.jpg"
	originalName := ""
	if len(files.Original) > 0 {
		originalName = name + "." + files.OriginalFormat.Extension()
	}
	motionName := ""
	if len(files.Motion) > 0 {
		motionName = name + "." + motionExt
	}

	paths := Paths{
		FilePath:  path.Join(keyEventDir(date), displayName),
		ThumbPath: path.Join(keyEventDir(date), thumbName),
	}
	written := make([]string, 0, 5)
	write := func(fileName string, data []byte) error {
		full := filepath.Join(directory, fileName)
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
		written = append(written, full)
		return nil
	}
	if err := write(displayName, files.Display); err != nil {
		return Paths{}, fmt.Errorf("保存图片失败：%w", err)
	}
	if err := write(thumbName, files.Thumb); err != nil {
		removeAll(written)
		return Paths{}, fmt.Errorf("保存缩略图失败：%w", err)
	}
	if len(files.View) > 0 {
		if err := write(viewName, files.View); err != nil {
			removeAll(written)
			return Paths{}, fmt.Errorf("保存展示图失败：%w", err)
		}
		paths.ViewPath = path.Join(keyEventDir(date), viewName)
	}
	if originalName != "" {
		if err := write(originalName, files.Original); err != nil {
			removeAll(written)
			return Paths{}, fmt.Errorf("保存原图失败：%w", err)
		}
		paths.OriginalPath = path.Join(keyEventDir(date), originalName)
	}
	if motionName != "" {
		if err := write(motionName, files.Motion); err != nil {
			removeAll(written)
			return Paths{}, fmt.Errorf("保存实况视频失败：%w", err)
		}
		paths.MotionPath = path.Join(keyEventDir(date), motionName)
	}
	return paths, nil
}

// Remove deletes the files at the given relative paths. A missing file is not
// an error: cleanup after a database delete must be idempotent.
func Remove(workspaceDir string, relativePaths ...string) {
	for _, relative := range relativePaths {
		if strings.TrimSpace(relative) == "" {
			continue
		}
		full, err := Resolve(workspaceDir, relative)
		if err != nil {
			continue
		}
		_ = os.Remove(full)
	}
}

// Copy writes one stored asset to destination, for 保存图片. The bytes are
// copied unchanged.
func Copy(workspaceDir, relative, destination string) error {
	source, err := Resolve(workspaceDir, relative)
	if err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("读取图片失败：%w", err)
	}
	defer in.Close()
	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("写入图片失败：%w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(destination)
		return fmt.Errorf("写入图片失败：%w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("写入图片失败：%w", err)
	}
	return nil
}

// checkName refuses a file stem that could name another directory.
func checkName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\:`) || strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("非法图片名：%s", name)
	}
	return nil
}

// motionExtension reports whether ext is one of the live photo video formats
// the application kept.
func motionExtension(ext string) bool {
	switch ext {
	case "mov", "mp4", "m4v", "webm":
		return true
	}
	return false
}

// removeAll removes files written by a failed Save; each is an absolute path
// produced here, never a stored path.
func removeAll(fullPaths []string) {
	for _, full := range fullPaths {
		_ = os.Remove(full)
	}
}
