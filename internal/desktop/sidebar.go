package desktop

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/icons"
)

// sidebarWidth is the navigation rail's fixed width, the same column width the
// reference application uses.
const sidebarWidth = float32(204)

// subItemIndent is how far a sub-function row is pushed in from its section
// row's leading edge. There is no matching inset on the trailing edge: both
// rows share the same right edge, so the indented list reads as one column with
// the section it belongs to.
const subItemIndent = float32(26)

// sidebar is the navigation column: one row per section, the expanded section's
// sub-functions under it, and the settings entry against the bottom edge.
func (m *Model) sidebar(c ui.C) {
	theme := c.Theme()
	metrics := theme.Metrics
	gap := metrics.GapItem / 2
	ui.Column(c).Width(sidebarWidth).FillHeight().
		Background(theme.CardElevated).Pad(metrics.GapItem).Gap(gap).Children(func() {
		// The sections scroll when the window is too short for them; the
		// settings entry stays against the bottom edge.
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(gap).Children(func() {
				m.navList(c)
			})
		})
		settings := m.navRow(c, len(navSections), SectionSettings.Label(),
			icons.Settings, m.section == SectionSettings)
		if settings.Clicked() {
			m.openSection(SectionSettings)
		}
	})
}

// navList declares the rail's sections and the sub-functions of the one that is
// expanded.
func (m *Model) navList(c ui.C) {
	metrics := c.Theme().Metrics
	gap := metrics.GapItem / 2
	for _, entry := range navSections {
		if !m.sectionEnabled(entry.section) {
			continue
		}
		row := m.navRow(c, int(entry.section), entry.label, entry.icon, m.section == entry.section)
		if row.Clicked() {
			m.openSection(entry.section)
		}

		// Only the expanded section declares a sub-menu: one row per
		// sub-function, directly under the section it belongs to.
		if !m.expandedOpen || m.expanded != entry.section {
			continue
		}
		labels := m.pageTabs(entry.section)
		if len(labels) == 0 {
			continue
		}
		section, selected := entry.section, m.pageTab(entry.section)
		// The list is keyed apart from the rows and the footer entry, so
		// opening another section does not hand its state to this column.
		c.KeyInt(len(navSections)*2 + int(section))
		ui.Column(c).Gap(gap).PadY(metrics.GapItem / 4).
			MarginLeft(subItemIndent).Children(func() {
			for tabIndex, label := range labels {
				sub := m.navSubRow(c, tabIndex, label, tabIndex == selected)
				if sub.Clicked() {
					m.openTab(section, tabIndex)
				}
			}
		})
	}
}

// navRow is one section row: its icon and label, the highlighted background
// while it is the current section, and the hover, press and focus looks the
// rest of the application's rows use. The caller reads Clicked from the row it
// declared, so the paint itself changes no state.
func (m *Model) navRow(c ui.C, key int, label string, icon gfx.Icon, active bool) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	c.KeyInt(key)
	row := ui.Row(c).Height(metrics.ControlHeight).FillWidth().
		PadX(metrics.GapItem).Gap(metrics.GapItem).Items(ui.Center).
		Radius(metrics.RadiusItem()).
		HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleMenuItem, Selected: active}).
		Hover(ui.Look{}.Background(theme.Accent)).
		Pressed(ui.Look{}.Background(theme.Accent)).
		Focus(ui.Look{}.Ring(3, theme.Ring))
	if active {
		row = row.Background(theme.Accent)
	}
	foreground := theme.Foreground
	if active {
		foreground = theme.AccentForeground
	}
	weight := gfx.WeightRegular
	if active {
		weight = gfx.WeightMedium
	}
	row.Children(func() {
		ui.Icon(c, icon).Color(foreground)
		ui.Text(c, label).FontSize(metrics.FontSizeM).Weight(weight).Color(foreground)
	})
	return row
}

// navSubRow is one sub-function row: shorter than a section row and without an
// icon, but the same highlight and interaction.
func (m *Model) navSubRow(c ui.C, key int, label string, active bool) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	c.KeyInt(key)
	row := ui.Row(c).Height(metrics.ControlHeight-4).FillWidth().
		PadX(metrics.GapItem).Items(ui.Center).
		Radius(metrics.RadiusItem()).
		HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleMenuItem, Selected: active}).
		Hover(ui.Look{}.Background(theme.Accent)).
		Pressed(ui.Look{}.Background(theme.Accent)).
		Focus(ui.Look{}.Ring(3, theme.Ring))
	if active {
		row = row.Background(theme.Accent)
	}
	foreground := theme.Foreground
	if active {
		foreground = theme.AccentForeground
	}
	row.Children(func() {
		ui.Text(c, label).FontSize(metrics.FontSizeM).Color(foreground)
	})
	return row
}
