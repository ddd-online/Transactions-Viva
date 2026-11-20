package desktop

import "github.com/vivaui/viva/app"

// addTray puts the application in the system's status area, so closing to the
// tray leaves a way back. The old application always had this icon; when the
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

// trayIcon is the placeholder mark for the status area: a T, opaque white on a
// transparent field, marked as a template so the bar tints it for light and
// dark. A packaged build replaces it with the product icon.
func trayIcon() app.StatusIcon {
	const size = 32
	pixels := make([]byte, size*size*4)
	fill := func(left, top, right, bottom int) {
		for y := top; y < bottom; y++ {
			for x := left; x < right; x++ {
				index := (y*size + x) * 4
				pixels[index+0] = 255
				pixels[index+1] = 255
				pixels[index+2] = 255
				pixels[index+3] = 255
			}
		}
	}
	// The bar and the stem of a T, with the stem centered under the bar.
	fill(7, 6, 25, 11)
	fill(13, 11, 19, 26)
	return app.StatusIcon{Size: size, RGBA: pixels, Template: true}
}
