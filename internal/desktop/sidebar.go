package desktop

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/icons"
)

// sidebar is the navigation column. Viva's Sidebar component paints the
// surface, the group label, the menu items and the footer; the page owns the
// selection and the fixed/open behaviour.
func (m *Model) sidebar(c ui.C) {
	ui.Control(c, "主导航", &m.navHandlers).Width(204).FillHeight().Draw(m.drawNav)
}

// drawSidebar declares the component's content for this paint. It runs inside
// the control's Draw callback, so every item is rebuilt in order as the
// component documentation asks.
func (m *Model) drawSidebar(d *ui.DrawContext) {
	theme := *d.Theme
	// The workspace card's rounded border is the boundary between the rail and
	// the content; a second line on the rail would double it. Paint the rail's
	// edge in its own surface color so only the card's border shows.
	theme.Border = theme.CardElevated
	nav := components.BeginSidebarWithOptions(d.Canvas, theme, d.Rect, "", components.SidebarOptions{
		Side:        components.SidebarLeft,
		Variant:     components.SidebarStandard,
		Collapsible: components.SidebarFixed,
	})
	for index, entry := range navSections {
		nav.AddMenuItem(components.SidebarMenuItem{
			ID:         m.navIDs[index],
			Label:      entry.label,
			VectorIcon: entry.icon,
			Active:     m.section == entry.section,
		})

		// Only the expanded section declares a sub-menu; the component draws
		// that list under its parent, and an item without sub-functions simply
		// has none.
		if !m.expandedOpen || m.expanded != entry.section {
			continue
		}
		labels := m.pageTabs(entry.section)
		if len(labels) == 0 {
			continue
		}
		selected := m.pageTab(entry.section)
		m.subItems = m.subItems[:0]
		for tabIndex, label := range labels {
			m.subItems = append(m.subItems, components.SidebarSubItem{
				ID:     m.subIDs[entry.section][tabIndex],
				Label:  label,
				Active: tabIndex == selected,
			})
		}
		nav.AddSubMenu(m.subItems)
	}
	nav.AddFooterMenuItem(components.SidebarMenuItem{
		ID:         m.navSettingsID,
		Label:      SectionSettings.Label(),
		VectorIcon: icons.Settings,
		Active:     m.section == SectionSettings,
	})

	// The component names its items with identities of our own; a click on one
	// is delivered to this paint. Changing the model after the paint keeps the
	// declaration read-only, and the runtime repaints once the change is in.
	if id, ok := d.Canvas.ClickTarget(); ok {
		d.Canvas.AfterPaint(func() { m.selectNav(id) })
	}
}

// selectNav applies a click on one of the sidebar's identities.
func (m *Model) selectNav(id gfx.ID) {
	if id == m.navSettingsID {
		m.section = SectionSettings
		m.expandedOpen = false
		return
	}
	for index, entry := range navSections {
		if m.navIDs[index] != id {
			continue
		}
		m.section = entry.section
		if len(m.pageTabs(entry.section)) > 0 {
			// A section with sub-functions opens them and closes any other
			// section's list; its current sub-function stays selected.
			m.expanded, m.expandedOpen = entry.section, true
		} else {
			m.expandedOpen = false
		}
		return
	}
	if !m.expandedOpen {
		return
	}
	labels := m.pageTabs(m.expanded)
	for tabIndex := range labels {
		if m.subIDs[m.expanded][tabIndex] == id {
			m.section = m.expanded
			m.selectTab(m.expanded, tabIndex)
			return
		}
	}
}
