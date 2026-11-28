//go:build !windows && !linux

package desktop

// windowButtonStrip is empty where the platform's own window buttons are not
// drawn at the end of the titlebar: macOS keeps its traffic lights at the
// leading end and the window system paints them, so a development host's page
// has nothing to keep clear of there.
func windowButtonStrip() (leading, trailing float32) { return 0, 0 }
