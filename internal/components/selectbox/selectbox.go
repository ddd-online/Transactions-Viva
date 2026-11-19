// Package selectbox wraps ui.Select for the application.
package selectbox

import "github.com/vivaui/viva/ui"

// Select is ui.Select with the entrance of its open list kept to the list.
//
// The framework begins that entrance in the select's paint and never ends it
// (see the gap noted in docs/architecture.md): the next EndPresence of the frame
// — a dialog's, which runs once its whole body is painted — then closes the
// list's group, so the list's 8 px slide is applied to everything painted after
// the select. A Select in a form row made the rest of the dialog jump for a
// frame each time the list opened.
//
// DrawOver runs once the select itself is painted, which is where the group
// belongs; ending it there keeps the motion on the list and leaves the page
// still. Remove this wrapper when the framework ends the group itself.
func Select[T comparable](c ui.C, value *T, choices []T, label func(T) string) ui.Box {
	box := ui.Select(c, value, choices, label)
	if _, open := box.Popup(); open {
		box.DrawOver(func(d *ui.DrawContext) { d.Canvas.EndPresence() })
	}
	return box
}
