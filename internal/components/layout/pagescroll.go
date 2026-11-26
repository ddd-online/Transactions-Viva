package layout

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// PageScroll is the keyboard's handle on a page's own scroll area. A page wraps
// its main scroll in Area, then calls Key from the Key method the shell hands it:
//
//	area := m.scroll.Area(ui.Scroll(c).Grow(1))
//	area.Children(...)
//
// PageUp and PageDown move the area by a viewport. Every other key is left to the
// control that has the focus — arrows belong to lists, Home and End to a caret —
// so the page never takes a key from the thing the user is working in.
//
// Without it a page scrolls only under the pointer. That is not only a pointer
// convenience: Viva clips the hits of a scrolled area, so a control below the
// fold cannot take the focus either, and a keyboard user cannot reach it at all.
type PageScroll struct {
	area *ui.ScrollState
}

// Area records the area as the page's own and hands it back, so a page writes it
// in place of the ui.Scroll call itself.
func (p *PageScroll) Area(b ui.Box) ui.Box {
	p.area = b.ScrollState()
	return b
}

// Key gives the area PageUp and PageDown. A recognized key is reported as handled
// even when the area is already at that end, so the page stops looking for
// another route for it.
func (p *PageScroll) Key(event gfx.KeyEvent) gfx.EventResult {
	if p.area == nil || event.Release {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyPageUp, gfx.KeyPageDown:
		return p.area.Key(event, 0)
	}
	return gfx.EventResult{}
}
