//go:build windows || linux

package desktop

import "github.com/vivaui/viva/app"

// windowButtonStrip is the room Viva's window buttons take at the end of a
// custom titlebar: the development reload host paints them for the window it
// owns, and the page it drives has to keep its own titlebar content clear of
// them. macOS keeps its traffic lights at the leading end and draws them itself,
// which is the other file's case.
func windowButtonStrip() (leading, trailing float32) {
	return 0, 3 * app.WindowControlWidth
}
