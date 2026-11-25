package assets

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// benchPicture writes one 12 megapixel JPEG into a workspace and returns its
// path relative to the assets root. The pixels carry a gradient rather than one
// flat colour: a flat picture compresses to nothing and decodes far faster than a
// photograph, which would make the benchmark lie.
func benchPicture(b *testing.B, workspace string, width, height int) string {
	b.Helper()
	directory := Root(workspace)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		b.Fatalf("mkdir: %v", err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			picture.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / width),
				G: uint8(y * 255 / height),
				B: uint8((x + y) * 127 / (width + height)),
				A: 255,
			})
		}
	}
	full := filepath.Join(directory, "bench.jpg")
	file, err := os.Create(full)
	if err != nil {
		b.Fatalf("create: %v", err)
	}
	if err := jpeg.Encode(file, picture, &jpeg.Options{Quality: 90}); err != nil {
		file.Close()
		b.Fatalf("encode: %v", err)
	}
	if err := file.Close(); err != nil {
		b.Fatalf("close: %v", err)
	}
	return "bench.jpg"
}

// BenchmarkLoadPhoto is what opening a note cost before the view copies: read
// the camera original, decode it whole, scale it down, hand it to the canvas.
func BenchmarkLoadPhoto(b *testing.B) {
	workspace := b.TempDir()
	name := benchPicture(b, workspace, 4000, 3000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := Load(workspace, name, MaxDisplayEdge); err != nil {
			b.Fatalf("load: %v", err)
		}
	}
}

// BenchmarkLoadViewPhoto is what it costs now: decode the copy made once beside
// the picture.
func BenchmarkLoadViewPhoto(b *testing.B) {
	workspace := b.TempDir()
	name := benchPicture(b, workspace, 4000, 3000)
	if _, err := LoadView(workspace, name, MaxDisplayEdge); err != nil {
		b.Fatalf("warm: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := LoadView(workspace, name, MaxDisplayEdge); err != nil {
			b.Fatalf("load: %v", err)
		}
	}
}

// BenchmarkLoadViewPhotoCold is the first open of a picture that has no copy yet:
// the original's decode, its scale, the encode and the write. It is paid once per
// picture, off the paint.
func BenchmarkLoadViewPhotoCold(b *testing.B) {
	for range b.N {
		workspace := b.TempDir()
		name := benchPicture(b, workspace, 4000, 3000)
		b.StartTimer()
		if _, err := LoadView(workspace, name, MaxDisplayEdge); err != nil {
			b.Fatalf("load: %v", err)
		}
		b.StopTimer()
	}
}

// BenchmarkViewCopySplit says where the cold path's time goes. The scale is the
// bulk of it, which is why the copy exists: it is paid once instead of on every
// open.
func BenchmarkViewCopySplit(b *testing.B) {
	workspace := b.TempDir()
	name := benchPicture(b, workspace, 4000, 3000)
	picture, err := loadPicture(workspace, name)
	if err != nil {
		b.Fatalf("decode: %v", err)
	}
	b.Run("scale", func(b *testing.B) {
		for range b.N {
			fitEdge(picture, MaxDisplayEdge)
		}
	})
	b.Run("encode", func(b *testing.B) {
		scaled := fitEdge(picture, MaxDisplayEdge)
		b.ResetTimer()
		for range b.N {
			if _, err := EncodeJPEG(scaled, ViewJPEGQuality); err != nil {
				b.Fatalf("encode: %v", err)
			}
		}
	})
}
