package desktop

import (
	"path/filepath"
	"strings"

	"transactions/internal/config"
	"transactions/internal/pages/settings"
)

// settingsSpec is what the settings page paints and the commands it calls. The
// shell owns the configuration, the theme and the navigation, so every write
// goes through here.
func (m *Model) settingsSpec() settings.Spec {
	return settings.Spec{
		Workspace:       m.config.WorkspaceDir,
		Appearance:      m.config.Appearance,
		CloseBehavior:   m.config.CloseBehavior,
		Features:        m.config.Features,
		TrayAvailable:   m.trayOK,
		OnPickWorkspace: m.pickWorkspaceDir,
		OnAppearance:    m.setAppearance,
		OnCloseBehavior: m.setCloseBehavior,
		OnFeature:       m.setFeature,
	}
}

// setAppearance pins the look or follows the system, repaints, and saves. A
// failed write restores the previous appearance so the page keeps showing what
// the file holds.
func (m *Model) setAppearance(value config.Appearance) error {
	previous := m.config.Appearance
	m.config.Appearance = value
	m.refreshTheme()
	if err := m.persist(); err != nil {
		m.config.Appearance = previous
		m.refreshTheme()
		return err
	}
	return nil
}

// setCloseBehavior stores what the window's close button does next time.
func (m *Model) setCloseBehavior(value config.CloseBehavior) error {
	previous := m.config.CloseBehavior
	m.config.CloseBehavior = value
	if err := m.persist(); err != nil {
		m.config.CloseBehavior = previous
		return err
	}
	return nil
}

// setFeature writes a global feature switch, hides or shows the section, and
// saves. A ledger-specific override is not used yet: the shell has no open
// ledger, so this changes the defaults every ledger without an entry follows.
func (m *Model) setFeature(key config.FeatureKey, enabled bool) error {
	previous := m.config.Features
	if _, err := m.config.SetFeature("", key, enabled); err != nil {
		return err
	}
	m.ensureVisibleSection()
	if err := m.persist(); err != nil {
		m.config.Features = previous
		m.ensureVisibleSection()
		return err
	}
	return nil
}

// feature is the switch that shows the section; settings has none.
func (s Section) feature() config.FeatureKey {
	switch s {
	case SectionAccounting:
		return config.FeatureAccounting
	case SectionStock:
		return config.FeatureStock
	case SectionTodo:
		return config.FeatureTodo
	case SectionEvents:
		return config.FeatureKeyEvent
	case SectionDiary:
		return config.FeatureDiary
	}
	return ""
}

// sectionEnabled reports whether a feature switch lets the section show.
// Settings is always available, as in the old application.
func (m *Model) sectionEnabled(section Section) bool {
	if section == SectionSettings {
		return true
	}
	key := section.feature()
	return key != "" && m.config.Features.Enabled(key)
}

// firstEnabledSection is the fallback when the current section is switched off:
// the first feature still on, else settings.
func (m *Model) firstEnabledSection() Section {
	for _, entry := range navSections {
		if m.sectionEnabled(entry.section) {
			return entry.section
		}
	}
	return SectionSettings
}

// ensureVisibleSection leaves a section that a switch just turned off, landing
// on the first enabled feature (or settings when none is).
func (m *Model) ensureVisibleSection() {
	if m.sectionEnabled(m.section) {
		return
	}
	m.openSection(m.firstEnabledSection())
}

// pickWorkspaceDir asks for a directory and adopts it as the workspace. It is
// the only command here that can be cancelled: an empty choice changes nothing.
func (m *Model) pickWorkspaceDir() error {
	if m.pickWorkspace == nil {
		return nil
	}
	directory, err := m.pickWorkspace()
	if err != nil {
		return err
	}
	directory = strings.TrimSpace(directory)
	if directory == "" || directory == m.config.WorkspaceDir {
		return nil
	}
	previousDir, previousRecent := m.config.WorkspaceDir, m.config.RecentWorkspaces
	m.config.WorkspaceDir = directory
	m.config.RecentWorkspaces = prependWorkspace(directory, previousRecent)
	if err := m.persist(); err != nil {
		m.config.WorkspaceDir = previousDir
		m.config.RecentWorkspaces = previousRecent
		return err
	}
	return nil
}

// prependWorkspace puts directory first in the recent list, without
// duplicates and at most RecentWorkspaceLimit entries.
func prependWorkspace(directory string, recent []string) []string {
	next := make([]string, 0, min(len(recent)+1, config.RecentWorkspaceLimit))
	next = append(next, directory)
	for _, path := range recent {
		if path == directory {
			continue
		}
		next = append(next, path)
		if len(next) == config.RecentWorkspaceLimit {
			break
		}
	}
	return next
}

// workspaceLabel is the footer's workspace name: the last path element, or the
// not-opened state.
func (m *Model) workspaceLabel() string {
	if m.config.WorkspaceDir == "" {
		return "尚未打开工作空间"
	}
	name := filepath.Base(m.config.WorkspaceDir)
	if name == "." || name == string(filepath.Separator) {
		return m.config.WorkspaceDir
	}
	return name
}

// persist writes the configuration through the store Run loaded. A model
// without one (tests, New) keeps its configuration in memory.
func (m *Model) persist() error {
	if m.configStore == nil {
		return nil
	}
	return m.configStore.Save(m.config)
}

// invalidate asks for a frame after a change that did not come from a click.
func (m *Model) invalidate() {
	if m.invalidator != nil {
		m.invalidator.Invalidate()
	}
}
