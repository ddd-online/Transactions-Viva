package controls

import (
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
)

// PageMaxWidth is the settings reading column. The left navigation and the page
// inset stay in the settings package; a section only fills this column.
const PageMaxWidth float32 = 760

const (
	pageTop    float32 = 32
	pageBottom         = layout.GroupGap
	pageSide           = layout.GroupGap
)

// GroupSpec is one block of a settings page. Raw draws the body without a card
// (an identity header, a colophon); every other group is a titled settings card.
// An empty Title gives an untitled card.
type GroupSpec struct {
	Title string
	Body  func()
	Raw   bool
}

// Page is the right pane of the settings page: a fixed section title over the
// groups, which live in one vertical scroller. Every group is separated by
// GroupGap, so a page cannot drift from the others by forgetting its own gap.
//
// The page owns the scroll viewport, not the window: a long section scrolls
// under the fixed title and the fixed left navigation.
func Page(c ui.C, scroll *layout.PageScroll, title string, groups ...GroupSpec) {
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// The section name stays put; only the groups below it scroll.
		ui.Column(c).FillWidth().Items(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(PageMaxWidth).
				MarginSides(pageTop, pageSide, 0, pageSide).
				MarginBottom(layout.GroupGap).
				Children(func() {
					typography.Title(c, title)
				})
		})
		scroll.Area(ui.Scroll(c).Grow(1).FillWidth().Items(ui.Center)).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(PageMaxWidth).
				MarginSides(0, pageSide, pageBottom, pageSide).
				Children(func() {
					ui.Column(c).FillWidth().Gap(layout.GroupGap).Children(func() {
						for _, group := range groups {
							group := group
							if group.Body == nil {
								continue
							}
							if group.Raw {
								group.Body()
								continue
							}
							Group(c, group.Title, group.Body)
						}
					})
				})
		})
	})
}
