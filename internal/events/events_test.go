package events

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gen2brain/h265/heic"

	"transactions/internal/assets"
	"transactions/internal/domain"
	"transactions/internal/repository"
	"transactions/internal/store"
)

// workspace opens a temporary workspace and returns it with its directory.
func workspace(t *testing.T) (*store.Workspace, string) {
	t.Helper()
	directory := t.TempDir()
	space, err := store.Open(directory)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = space.Close() })
	return space, directory
}

// writePNG writes a small solid PNG and returns its path.
func writePNG(t *testing.T, directory, name string, width, height int, c color.RGBA) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, c)
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, directory, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddImagesPairsLivePhotosAndStoresFiles(t *testing.T) {
	space, directory := workspace(t)
	ctx := context.Background()
	if _, err := space.SaveKeyEvent(ctx, "2026-01-15", "年会", "正文", "#D9705A"); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	picked := t.TempDir()
	still := writePNG(t, picked, "IMG_0001.png", 40, 20, color.RGBA{R: 200, A: 255})
	motion := writeFile(t, picked, "IMG_0001.MOV", []byte("motion-bytes"))
	second := writePNG(t, picked, "IMG_0002.png", 10, 10, color.RGBA{B: 200, A: 255})

	result, err := AddImages(ctx, space, directory, "2026-01-15", []string{still, motion, second})
	if err != nil {
		t.Fatalf("AddImages: %v", err)
	}
	if result.Added != 2 || len(result.Problems) != 0 {
		t.Fatalf("result = %+v", result)
	}
	images, err := space.KeyEventImages(ctx, "2026-01-15")
	if err != nil || len(images) != 2 {
		t.Fatalf("images = %+v, err=%v", images, err)
	}
	first := images[0]
	if first.FilePath != "key_events/2026-01-15/"+first.ID+".png" ||
		first.ThumbPath != "key_events/2026-01-15/thumb_"+first.ID+".jpg" ||
		first.MotionPath != "key_events/2026-01-15/"+first.ID+".mov" {
		t.Fatalf("first image = %+v", first)
	}
	for _, relative := range []string{first.FilePath, first.ThumbPath, first.MotionPath, images[1].FilePath} {
		full, err := assets.Resolve(directory, relative)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", relative, err)
		}
		if _, err := os.Stat(full); err != nil {
			t.Fatalf("stored file %q: %v", relative, err)
		}
	}
	if images[1].MotionPath != "" {
		t.Fatalf("a still without a video got a motion path: %+v", images[1])
	}
}

func TestAddImagesReportsPickedProblems(t *testing.T) {
	space, directory := workspace(t)
	ctx := context.Background()
	if _, err := space.SaveKeyEvent(ctx, "2026-01-15", "年会", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	picked := t.TempDir()
	orphan := writeFile(t, picked, "IMG_0009.MOV", []byte("video"))
	unknown := writeFile(t, picked, "notes.txt", []byte("not an image"))

	result, err := AddImages(ctx, space, directory, "2026-01-15", []string{orphan, unknown})
	if err != nil {
		t.Fatalf("AddImages: %v", err)
	}
	if result.Added != 0 || len(result.Problems) != 2 {
		t.Fatalf("result = %+v", result)
	}
	joined := strings.Join(result.Problems, "；")
	for _, want := range []string{"实况视频要和同名的照片一起选：IMG_0009.MOV", "不支持的格式：notes.txt"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("problems %q do not contain %q", joined, want)
		}
	}
	if images, err := space.KeyEventImages(ctx, "2026-01-15"); err != nil || len(images) != 0 {
		t.Fatalf("images after problems = %+v, err=%v", images, err)
	}
	root := filepath.Join(directory, "data", "assets", "key_events", "2026-01-15")
	entries, readErr := os.ReadDir(root)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("refused imports left %d files", len(entries))
	}
}

func TestAddImagesNeedsTheEvent(t *testing.T) {
	space, directory := workspace(t)
	still := writePNG(t, t.TempDir(), "a.png", 10, 10, color.RGBA{A: 255})
	if _, err := AddImages(context.Background(), space, directory, "2026-01-15", []string{still}); err == nil {
		t.Fatal("AddImages accepted a day without an event")
	}
}

// failingImages wraps a repository and refuses every image row.
type failingImages struct {
	repository.Repository
}

func (failingImages) CreateKeyEventImage(context.Context, domain.KeyEventImage) (domain.KeyEventImage, error) {
	return domain.KeyEventImage{}, errors.New("数据库拒绝写入")
}

func TestAddImagesRemovesFilesWhenTheRowFails(t *testing.T) {
	space, directory := workspace(t)
	ctx := context.Background()
	if _, err := space.SaveKeyEvent(ctx, "2026-01-15", "年会", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	still := writePNG(t, t.TempDir(), "a.png", 10, 10, color.RGBA{A: 255})
	result, err := AddImages(ctx, failingImages{space}, directory, "2026-01-15", []string{still})
	if err != nil {
		t.Fatalf("AddImages: %v", err)
	}
	if result.Added != 0 || len(result.Problems) != 1 || !strings.Contains(result.Problems[0], "数据库拒绝写入") {
		t.Fatalf("result = %+v", result)
	}
	entries, readErr := os.ReadDir(filepath.Join(directory, "data", "assets", "key_events", "2026-01-15"))
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("a failed row left %d files", len(entries))
	}
}

func TestDeleteImageAndEventRemoveTheirFiles(t *testing.T) {
	space, directory := workspace(t)
	ctx := context.Background()
	if _, err := space.SaveKeyEvent(ctx, "2026-01-15", "年会", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	picked := t.TempDir()
	first := writePNG(t, picked, "a.png", 10, 10, color.RGBA{A: 255})
	second := writePNG(t, picked, "b.png", 10, 10, color.RGBA{A: 255})
	result, err := AddImages(ctx, space, directory, "2026-01-15", []string{first, second})
	if err != nil || result.Added != 2 {
		t.Fatalf("AddImages = %+v, err=%v", result, err)
	}
	images, _ := space.KeyEventImages(ctx, "2026-01-15")

	if err := DeleteImage(ctx, space, directory, images[0].ID); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	for _, relative := range []string{images[0].FilePath, images[0].ThumbPath} {
		full, _ := assets.Resolve(directory, relative)
		if _, err := os.Stat(full); !os.IsNotExist(err) {
			t.Fatalf("image file %q survived its row", relative)
		}
	}

	if err := DeleteEvent(ctx, space, directory, "2026-01-15"); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if _, err := space.KeyEvent(ctx, "2026-01-15"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("event after delete = %v", err)
	}
	for _, relative := range []string{images[1].FilePath, images[1].ThumbPath} {
		full, _ := assets.Resolve(directory, relative)
		if _, err := os.Stat(full); !os.IsNotExist(err) {
			t.Fatalf("event file %q survived the event", relative)
		}
	}
}

func TestSaveImageCopiesTheStoredBytes(t *testing.T) {
	space, directory := workspace(t)
	ctx := context.Background()
	if _, err := space.SaveKeyEvent(ctx, "2026-01-15", "年会", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	still := writePNG(t, t.TempDir(), "a.png", 10, 10, color.RGBA{A: 255})
	if result, err := AddImages(ctx, space, directory, "2026-01-15", []string{still}); err != nil || result.Added != 1 {
		t.Fatalf("AddImages = %+v, err=%v", result, err)
	}
	images, _ := space.KeyEventImages(ctx, "2026-01-15")
	destination := filepath.Join(t.TempDir(), "saved.png")
	if err := SaveImage(directory, images[0].FilePath, destination); err != nil {
		t.Fatalf("SaveImage: %v", err)
	}
	source, _ := assets.Resolve(directory, images[0].FilePath)
	want, _ := os.ReadFile(source)
	got, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("saved copy differs, err=%v", err)
	}
}

// heicFile writes a real HEIC picture and returns its path and bytes.
func heicFile(t *testing.T, directory, name string, width, height int) (string, []byte) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, color.RGBA{R: 120, G: 160, B: 90, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := heic.Encode(&buffer, img); err != nil {
		t.Fatalf("heic.Encode: %v", err)
	}
	data := buffer.Bytes()
	return writeFile(t, directory, name, data), data
}

func TestAddImagesTranscodesHEICAndKeepsTheOriginal(t *testing.T) {
	space, directory := workspace(t)
	ctx := context.Background()
	if _, err := space.SaveKeyEvent(ctx, "2026-01-15", "苹果照片", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	path, original := heicFile(t, t.TempDir(), "IMG_0100.HEIC", 32, 20)
	result, err := AddImages(ctx, space, directory, "2026-01-15", []string{path})
	if err != nil {
		t.Fatalf("AddImages: %v", err)
	}
	if result.Added != 1 || len(result.Problems) != 0 {
		t.Fatalf("result = %+v", result)
	}
	images, err := space.KeyEventImages(ctx, "2026-01-15")
	if err != nil || len(images) != 1 {
		t.Fatalf("images = %+v, err=%v", images, err)
	}
	image := images[0]
	if !strings.HasSuffix(image.FilePath, ".jpg") || !strings.HasSuffix(image.ThumbPath, ".jpg") ||
		!strings.HasSuffix(image.OriginalPath, ".heic") {
		t.Fatalf("image paths = %+v", image)
	}
	display, err := assets.Resolve(directory, image.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(display); err != nil || assets.Sniff(data) != assets.FormatJPEG {
		t.Fatalf("display file = %d bytes, err=%v", len(data), err)
	}
	originalPath, err := assets.Resolve(directory, image.OriginalPath)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(originalPath); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("original file differs, err=%v", err)
	}
	bitmap, err := assets.Load(directory, image.FilePath, 512)
	if err != nil || bitmap.Width != 32 || bitmap.Height != 20 {
		t.Fatalf("loaded display = %dx%d, err=%v", bitmap.Width, bitmap.Height, err)
	}
}
