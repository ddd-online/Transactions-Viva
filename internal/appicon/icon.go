// Package appicon is the application's picture: the source for the executable,
// installer, desktop shortcut and the running window's taskbar icon.
package appicon

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"sync"

	"github.com/vivaui/viva/app"
)

//go:embed icon.png
var source []byte

// Load decodes the embedded PNG into the straight-alpha pixels Viva asks for.
// The result is cached: the window config may be built on more than one paint.
var Load = sync.OnceValues(func() (app.WindowIcon, error) {
	img, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		return app.WindowIcon{}, fmt.Errorf("appicon: decode: %w", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != bounds.Dy() {
		return app.WindowIcon{}, fmt.Errorf("appicon: icon must be square, got %dx%d", bounds.Dx(), bounds.Dy())
	}
	size := bounds.Dx()
	if size < 128 || size > 1024 {
		return app.WindowIcon{}, fmt.Errorf("appicon: icon size %d outside 128..1024", size)
	}
	// image.RGBA is premultiplied; Viva's WindowIcon wants straight alpha, which
	// is exactly image.NRGBA's layout.
	nrgba := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(nrgba, nrgba.Bounds(), img, bounds.Min, draw.Src)
	return app.WindowIcon{Size: size, RGBA: nrgba.Pix}, nil
})
