package layout

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// StatusLine paints a line of status text as a live region: the words are the
// page's, but the row they sit in is announced when they change. A save failure or
// a validation message is otherwise only visible — a screen reader passes over a
// painted line, and the user who needs it hears nothing.
//
// The region is polite: it waits for the user to be idle instead of interrupting,
// which is what a status wants. It never takes the keyboard.
func StatusLine(c ui.C, text string, paint func(ui.C)) {
	ui.Row(c).FillWidth().HitWith(ui.ID(c), text, gfx.HitOptions{
		Role: gfx.RoleStatus, Live: gfx.LivePolite, SkipFocus: true,
	}).Children(func() {
		paint(c)
	})
}
