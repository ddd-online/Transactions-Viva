package assets

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gen2brain/h265/heic"
)

// picture returns a small RGBA picture with one pixel per colour, top row
// first, so orientation tests can name a pixel.
func picture(width, height int, colors ...color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for index, c := range colors {
		x, y := index%width, index/width
		if y >= height {
			break
		}
		img.SetRGBA(x, y, c)
	}
	return img
}

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buffer.Bytes()
}

func jpegBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buffer.Bytes()
}

func gifBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := gif.Encode(&buffer, img, nil); err != nil {
		t.Fatalf("gif.Encode: %v", err)
	}
	return buffer.Bytes()
}

func TestSniffReadsFormatsFromBytes(t *testing.T) {
	pngData := pngBytes(t, picture(2, 2, color.RGBA{R: 255, A: 255}))
	jpegData := jpegBytes(t, picture(2, 2, color.RGBA{R: 255, A: 255}))
	gifData := gifBytes(t, picture(2, 2, color.RGBA{R: 255, A: 255}))
	webp := append([]byte("RIFF\x10\x00\x00\x00WEBP"), make([]byte, 4)...)
	heic := []byte("\x00\x00\x00\x14ftypheic\x00\x00\x00\x00mif1heic")
	avif := []byte("\x00\x00\x00\x14ftypavif\x00\x00\x00\x00mif1avif")

	cases := map[string]struct {
		data []byte
		want Format
	}{
		"png":  {pngData, FormatPNG},
		"jpeg": {jpegData, FormatJPEG},
		"gif":  {gifData, FormatGIF},
		"webp": {webp, FormatWebP},
		"heic": {heic, FormatHEIC},
		"avif": {avif, ""},
		"text": {[]byte("not an image at all"), ""},
	}
	for name, test := range cases {
		if got := Sniff(test.data); got != test.want {
			t.Fatalf("%s: Sniff = %q, want %q", name, got, test.want)
		}
	}
}

func TestFormatExtensionsAndDisplayability(t *testing.T) {
	if FormatJPEG.Extension() != "jpg" || FormatPNG.Extension() != "png" ||
		FormatGIF.Extension() != "gif" || FormatWebP.Extension() != "webp" {
		t.Fatal("extensions drifted")
	}
	if !FormatJPEG.Displayable() || !FormatWebP.Displayable() || !FormatHEIC.Displayable() {
		t.Fatal("a drawable format is not displayable")
	}
	if Format("").Displayable() {
		t.Fatal("an unknown format claims to be displayable")
	}
}

func TestDecodeGIFDrawsItsFirstFrame(t *testing.T) {
	img := picture(2, 2, color.RGBA{R: 255, A: 255})
	data := gifBytes(t, img)
	picture, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if picture.Bounds().Dx() != 2 || picture.Bounds().Dy() != 2 {
		t.Fatalf("gif size = %v", picture.Bounds())
	}
}

// heicBytes encodes a small picture as a real HEIC file, through the same pure
// Go codec the application decodes with.
func heicBytes(t *testing.T, picture image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := heic.Encode(&buffer, picture); err != nil {
		t.Fatalf("heic.Encode: %v", err)
	}
	return buffer.Bytes()
}

func TestDecodeReadsHEIC(t *testing.T) {
	data := heicBytes(t, picture(24, 16, color.RGBA{R: 255, A: 255}))
	if Sniff(data) != FormatHEIC {
		t.Fatalf("HEIC sniff = %q", Sniff(data))
	}
	config, err := ImageConfig(data)
	if err != nil || config.Width != 24 || config.Height != 16 {
		t.Fatalf("config = %dx%d, err=%v", config.Width, config.Height, err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Bounds().Dx() != 24 || decoded.Bounds().Dy() != 16 {
		t.Fatalf("decoded size = %v", decoded.Bounds())
	}
	thumb, err := Thumbnail(decoded, ThumbnailWidth)
	if err != nil || Sniff(thumb) != FormatJPEG {
		t.Fatalf("thumbnail = %d bytes, err=%v", len(thumb), err)
	}
}

func TestDecodeRefusesUnknown(t *testing.T) {
	if _, err := Decode([]byte("nope")); err == nil || !strings.Contains(err.Error(), "不是可解码") {
		t.Fatalf("unknown error = %v", err)
	}
}

func TestSaveKeepsTheHEICOriginal(t *testing.T) {
	workspace := t.TempDir()
	original := heicBytes(t, picture(24, 16, color.RGBA{B: 255, A: 255}))
	decoded, err := Decode(original)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	display, err := EncodeJPEG(decoded, DisplayJPEGQuality)
	if err != nil {
		t.Fatalf("EncodeJPEG: %v", err)
	}
	thumb, err := Thumbnail(decoded, ThumbnailWidth)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	paths, err := Save(workspace, "2026-01-15", "iphone", Files{
		Display: display, DisplayFormat: FormatJPEG, Thumb: thumb,
		Original: original, OriginalFormat: FormatHEIC,
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if Sniff(display) != FormatJPEG || paths.FilePath != "key_events/2026-01-15/iphone.jpg" ||
		paths.OriginalPath != "key_events/2026-01-15/iphone.heic" {
		t.Fatalf("paths = %+v", paths)
	}
	full, err := Resolve(workspace, paths.OriginalPath)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(full); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("original copy differs, err=%v", err)
	}
}

func TestSaveRefusesHEICAsTheDisplayCopy(t *testing.T) {
	workspace := t.TempDir()
	original := heicBytes(t, picture(8, 8, color.RGBA{A: 255}))
	thumb, err := Thumbnail(picture(8, 8, color.RGBA{A: 255}), ThumbnailWidth)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Save(workspace, "2026-01-15", "raw", Files{
		Display: original, DisplayFormat: FormatHEIC, Thumb: thumb,
	}); err == nil {
		t.Fatal("Save accepted HEIC as the display copy")
	}
}

func TestDecodeAppliesJPEGOrientation(t *testing.T) {
	// A 40×20 picture with a red left half and a blue right half, stored with
	// orientation 6 (rotate 90° clockwise): the decoded picture must be 20×40
	// with the red half on top. Solid halves survive JPEG's chroma subsampling.
	source := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := range 20 {
		for x := range 40 {
			if x < 20 {
				source.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			} else {
				source.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}
	data := withExifOrientation(t, jpegBytes(t, source), 6)

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Bounds().Dx() != 20 || decoded.Bounds().Dy() != 40 {
		t.Fatalf("rotated size = %v", decoded.Bounds())
	}
	top := color.RGBAModel.Convert(decoded.At(10, 5)).(color.RGBA)
	bottom := color.RGBAModel.Convert(decoded.At(10, 35)).(color.RGBA)
	if top.R < 200 || top.B > 80 {
		t.Fatalf("top pixel after rotation = %+v, want red", top)
	}
	if bottom.B < 200 || bottom.R > 80 {
		t.Fatalf("bottom pixel after rotation = %+v, want blue", bottom)
	}
}

// withExifOrientation inserts an Exif APP1 segment carrying tag 0x0112 into a
// JPEG, the way a camera writes it.
func withExifOrientation(t *testing.T, jpegData []byte, orientation int) []byte {
	t.Helper()
	tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0,
		1, 0, // one entry
		0x12, 0x01, 3, 0, 1, 0, 0, 0, byte(orientation), 0, 0, 0,
		0, 0, 0, 0} // next IFD
	payload := append([]byte("Exif\x00\x00"), tiff...)
	length := len(payload) + 2
	segment := []byte{0xff, 0xe1, byte(length >> 8), byte(length)}
	segment = append(segment, payload...)
	result := append([]byte{}, jpegData[:2]...)
	result = append(result, segment...)
	result = append(result, jpegData[2:]...)
	return result
}

func TestThumbnailKeepsTheAspectAndTheJPEGShape(t *testing.T) {
	data, err := Thumbnail(picture(600, 300), ThumbnailWidth)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	if Sniff(data) != FormatJPEG {
		t.Fatalf("thumbnail sniff = %q", Sniff(data))
	}
	config, err := ImageConfig(data)
	if err != nil {
		t.Fatalf("ImageConfig: %v", err)
	}
	if config.Width != 300 || config.Height != 150 {
		t.Fatalf("thumbnail = %dx%d", config.Width, config.Height)
	}

	// A small picture is never upscaled.
	small, err := Thumbnail(picture(40, 20), ThumbnailWidth)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	if config, err := ImageConfig(small); err != nil || config.Width != 40 {
		t.Fatalf("small thumbnail = %dx%d, err=%v", config.Width, config.Height, err)
	}
}

func TestThumbnailCompositesTransparencyOnWhite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2)) // fully transparent
	data, err := Thumbnail(img, ThumbnailWidth)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("jpeg.Decode: %v", err)
	}
	got := color.RGBAModel.Convert(decoded.At(0, 0)).(color.RGBA)
	if got.R < 240 || got.G < 240 || got.B < 240 {
		t.Fatalf("transparent thumbnail pixel = %+v, want white", got)
	}
}

func TestSaveLoadCopyAndRemove(t *testing.T) {
	workspace := t.TempDir()
	img := picture(40, 20, color.RGBA{B: 255, A: 255})
	display := pngBytes(t, img)
	thumb, err := Thumbnail(img, ThumbnailWidth)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	paths, err := Save(workspace, "2026-01-15", "abc", Files{
		Display:       display,
		DisplayFormat: FormatPNG,
		Thumb:         thumb,
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if paths.FilePath != "key_events/2026-01-15/abc.png" ||
		paths.ThumbPath != "key_events/2026-01-15/thumb_abc.jpg" || paths.MotionPath != "" {
		t.Fatalf("paths = %+v", paths)
	}
	full := filepath.Join(workspace, "data", "assets", filepath.FromSlash(paths.FilePath))
	if data, err := os.ReadFile(full); err != nil || !bytes.Equal(data, display) {
		t.Fatalf("display file = %d bytes, err=%v", len(data), err)
	}

	bitmap, err := Load(workspace, paths.FilePath, 1024)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bitmap.Width != 40 || bitmap.Height != 20 || len(bitmap.Pix) != 40*20*4 {
		t.Fatalf("bitmap = %dx%d, %d bytes", bitmap.Width, bitmap.Height, len(bitmap.Pix))
	}
	if bitmap.Pix[0] == 0 && bitmap.Pix[2] == 0 {
		t.Fatal("bitmap pixels are empty")
	}

	destination := filepath.Join(t.TempDir(), "saved.png")
	if err := Copy(workspace, paths.FilePath, destination); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if data, err := os.ReadFile(destination); err != nil || !bytes.Equal(data, display) {
		t.Fatalf("copied file differs, err=%v", err)
	}

	Remove(workspace, paths.FilePath, paths.ThumbPath, paths.MotionPath)
	if _, err := os.Stat(full); !os.IsNotExist(err) {
		t.Fatalf("display file still present: %v", err)
	}
	// Removing again is not an error.
	Remove(workspace, paths.FilePath)
}

func TestSaveWritesTheMotionPart(t *testing.T) {
	workspace := t.TempDir()
	img := picture(10, 10, color.RGBA{R: 255, A: 255})
	thumb, _ := Thumbnail(img, ThumbnailWidth)
	paths, err := Save(workspace, "2026-01-15", "live", Files{
		Display:       jpegBytes(t, img),
		DisplayFormat: FormatJPEG,
		Thumb:         thumb,
		Motion:        []byte("motion-bytes"),
		MotionExt:     "MOV",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if paths.MotionPath != "key_events/2026-01-15/live.mov" {
		t.Fatalf("motion path = %q", paths.MotionPath)
	}
	full, err := Resolve(workspace, paths.MotionPath)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if data, err := os.ReadFile(full); err != nil || string(data) != "motion-bytes" {
		t.Fatalf("motion file = %q, err=%v", data, err)
	}
}

func TestSaveCleansUpAfterAFailure(t *testing.T) {
	workspace := t.TempDir()
	img := picture(4, 4, color.RGBA{R: 255, A: 255})
	thumb, _ := Thumbnail(img, ThumbnailWidth)
	_, err := Save(workspace, "2026-01-15", "bad", Files{
		Display:       jpegBytes(t, img),
		DisplayFormat: FormatJPEG,
		Thumb:         thumb,
		Motion:        []byte("video"),
		MotionExt:     "avi",
	})
	if err == nil {
		t.Fatal("Save accepted an unsupported motion extension")
	}
	entries, readErr := os.ReadDir(filepath.Join(workspace, "data", "assets", "key_events", "2026-01-15"))
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("failed Save left %d files", len(entries))
	}
}

func TestResolveRefusesEscapes(t *testing.T) {
	workspace := t.TempDir()
	for _, relative := range []string{"", "..", "../secret", "key_events/../../x", "/etc/passwd", `C:\Windows\win.ini`} {
		if _, err := Resolve(workspace, relative); err == nil {
			t.Fatalf("Resolve accepted %q", relative)
		}
	}
	if _, err := Resolve(workspace, "key_events/2026-01-15/a.png"); err != nil {
		t.Fatalf("Resolve refused a normal path: %v", err)
	}
}

func TestSaveRefusesBadNamesAndDates(t *testing.T) {
	workspace := t.TempDir()
	img := picture(4, 4, color.RGBA{R: 255, A: 255})
	thumb, _ := Thumbnail(img, ThumbnailWidth)
	for _, test := range []struct {
		date string
		name string
	}{
		{"2026-1-5", "abc"},
		{"2026-01-15", "../abc"},
		{"2026-01-15", "a/b"},
		{"2026-01-15", ".."},
	} {
		if _, err := Save(workspace, test.date, test.name, Files{
			Display:       jpegBytes(t, img),
			DisplayFormat: FormatJPEG,
			Thumb:         thumb,
		}); err == nil {
			t.Fatalf("Save accepted date %q name %q", test.date, test.name)
		}
	}
}

func TestLoadScalesTheLongestEdge(t *testing.T) {
	workspace := t.TempDir()
	img := picture(2000, 1000, color.RGBA{G: 255, A: 255})
	paths, err := Save(workspace, "2026-01-15", "wide", Files{
		Display:       pngBytes(t, img),
		DisplayFormat: FormatPNG,
		Thumb: func() []byte {
			thumb, _ := Thumbnail(img, ThumbnailWidth)
			return thumb
		}(),
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	bitmap, err := Load(workspace, paths.FilePath, 1000)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bitmap.Width != 1000 || bitmap.Height != 500 {
		t.Fatalf("scaled bitmap = %dx%d", bitmap.Width, bitmap.Height)
	}
}

func TestLoadReportsAMissingFile(t *testing.T) {
	workspace := t.TempDir()
	if _, err := Load(workspace, "key_events/2026-01-15/missing.png", 1024); err == nil {
		t.Fatal("Load accepted a missing file")
	}
}

// TestDecodeRealHEICFixtures decodes the two HEIC files the application
// shipped as its own test fixtures: a flat 64 px tile and a 512 px gradient.
// They were produced by libheif, so they prove the decoder against another
// encoder's output, not only against the one this module also ships.
func TestDecodeRealHEICFixtures(t *testing.T) {
	cases := []struct {
		name          string
		width, height int
	}{
		{"flat-64.heic", 64, 64},
		{"gradient-512.heic", 512, 512},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", test.name))
			if err != nil {
				t.Fatal(err)
			}
			if Sniff(data) != FormatHEIC {
				t.Fatalf("sniff = %q", Sniff(data))
			}
			config, err := ImageConfig(data)
			if err != nil || config.Width != test.width || config.Height != test.height {
				t.Fatalf("config = %dx%d, err=%v", config.Width, config.Height, err)
			}
			decoded, err := Decode(data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if decoded.Bounds().Dx() != test.width || decoded.Bounds().Dy() != test.height {
				t.Fatalf("decoded size = %v", decoded.Bounds())
			}
			thumb, err := Thumbnail(decoded, ThumbnailWidth)
			if err != nil || len(thumb) == 0 || Sniff(thumb) != FormatJPEG {
				t.Fatalf("thumbnail = %d bytes, err=%v", len(thumb), err)
			}
		})
	}
}

func TestLoadViewCachesAStageSizedCopy(t *testing.T) {
	workspace := t.TempDir()
	root := Root(workspace)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	photo := picture(2000, 1200, color.RGBA{R: 200, G: 120, B: 60, A: 255})
	if err := os.WriteFile(filepath.Join(root, "photo.png"), pngBytes(t, photo), 0o644); err != nil {
		t.Fatal(err)
	}

	bitmap, err := LoadView(workspace, "photo.png", ViewMaxEdge)
	if err != nil {
		t.Fatalf("LoadView: %v", err)
	}
	if bitmap.Width > ViewMaxEdge || bitmap.Height > ViewMaxEdge {
		t.Fatalf("the view is %dx%d, larger than %d", bitmap.Width, bitmap.Height, ViewMaxEdge)
	}
	view, err := Resolve(workspace, ViewPath("photo.png"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(view)
	if err != nil {
		t.Fatalf("the view copy was not written: %v", err)
	}
	if Sniff(data) != FormatJPEG {
		t.Fatalf("the view copy is %s, want JPEG", Sniff(data))
	}

	again, err := LoadView(workspace, "photo.png", ViewMaxEdge)
	if err != nil {
		t.Fatalf("second LoadView: %v", err)
	}
	if again.Width != bitmap.Width || again.Height != bitmap.Height {
		t.Fatalf("the cached view is %dx%d, first was %dx%d",
			again.Width, again.Height, bitmap.Width, bitmap.Height)
	}
}
