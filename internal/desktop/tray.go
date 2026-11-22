package desktop

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
	"sync"

	"github.com/vivaui/viva/app"
)

// trayIconPNG is the status-area icon: the same mark the About page shows —
// the primary plate with the white "Tr" — rasterized at 64 px so the densest
// bars stay sharp. The plate is colored rather than a template, because Windows
// paints a template as-is; the blue plate keeps the white mark visible on both
// light and dark taskbars.
//
// It is derived from icons.AppMark, not drawn by hand: a rounded plate 60 px
// wide (radius 14) in the theme's primary at the centre of the 64 px canvas,
// with the glyph 34 px wide in the primary foreground. Rasterize the same
// shape again (4× and downsample) when the mark or the plate's colour changes.
//
//go:embed tray.png
var trayIconPNG []byte

// trayIcon decodes the embedded image once. A decode failure yields an empty
// icon: the status area then takes no icon, and closing to the tray falls back
// to quitting.
var trayIcon = sync.OnceValue(func() app.StatusIcon {
	img, err := png.Decode(bytes.NewReader(trayIconPNG))
	if err != nil {
		return app.StatusIcon{}
	}
	bounds := img.Bounds()
	straight := image.NewNRGBA(bounds)
	draw.Draw(straight, bounds, img, bounds.Min, draw.Src)
	rgba := make([]byte, bounds.Dx()*bounds.Dy()*4)
	stride := bounds.Dx() * 4
	for y := 0; y < bounds.Dy(); y++ {
		copy(rgba[y*stride:(y+1)*stride], straight.Pix[y*straight.Stride:])
	}
	return app.StatusIcon{Size: bounds.Dx(), RGBA: rgba}
})

// addTray puts the application in the system's status area, so closing to the
// tray leaves a way back. The application always had this icon; when the
// platform has no status area the close behavior falls back to quitting.
func (m *Model) addTray(application *app.Application) bool {
	item, err := application.AddStatusItem(app.StatusItemConfig{
		Tooltip: "Transactions",
		Icon:    trayIcon(),
		OnClick: func(app.StatusClick) { m.showWindow() },
		Menu: func() []app.MenuItem {
			return []app.MenuItem{
				{Title: "显示主窗口", OnSelect: m.showWindow},
				{Separator: true},
				{Title: "关闭程序", OnSelect: func() {
					m.saveWindowBounds()
					application.Quit()
				}},
			}
		},
	})
	if err != nil {
		return false
	}
	m.tray = item
	return true
}

// showWindow brings a window hidden in the status area back.
func (m *Model) showWindow() {
	if m.window == nil || m.window.Closed() {
		return
	}
	m.window.Show()
}
