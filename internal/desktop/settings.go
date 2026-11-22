package desktop

import (
	"context"
	"errors"
	"strings"

	"github.com/vivaui/viva/app"

	"transactions/internal/config"
	diarysrv "transactions/internal/diary"
	"transactions/internal/domain"
	"transactions/internal/pages/settings"
	storepkg "transactions/internal/store"
)

// errNoDiaryWorkspace is what the 日记配置 commands report while no workspace is
// open; the section disables them, so this only guards a stale click.
var errNoDiaryWorkspace = errors.New("还没有打开工作空间")

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
		Ledgers:         m.accounting.Ledgers(),
		ActiveLedger:    m.accounting.ActiveLedger(),
		OnPickWorkspace: m.pickWorkspaceDir,
		OnAppearance:    m.setAppearance,
		OnCloseBehavior: m.setCloseBehavior,
		OnFeature:       m.setFeature,
		OnCreateLedger:  m.createLedger,
		OnUpdateLedger:  m.updateLedger,
		OnDeleteLedger:  m.deleteLedger,
		OpenURL:         m.openExternalURL,
		CopyText:        m.copyToClipboard,

		StockAccounts:        m.stock.Accounts(),
		ActiveStockAccount:   m.stock.ActiveAccount(),
		OnCreateStockAccount: m.createStockAccount,
		OnUpdateStockAccount: m.updateStockAccount,
		OnDeleteStockAccount: m.deleteStockAccount,
		ImportDiary:          m.importDiary,
		ExportDiary:          m.exportDiary,

		DiaryBooks:        m.diary.Books(),
		ActiveDiaryBook:   m.diary.ActiveBook(),
		OnCreateDiaryBook: m.createDiaryBook,
		OnUpdateDiaryBook: m.updateDiaryBook,
		OnDeleteDiaryBook: m.deleteDiaryBook,
	}
}

// The diary book commands belong to the diary section, which owns the books the
// 日记 page reads; the shell only repaints.
func (m *Model) createDiaryBook(name, description string) error {
	if err := m.diary.CreateBook(name, description); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

func (m *Model) updateDiaryBook(id, name, description string) error {
	if err := m.diary.UpdateBook(id, name, description); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

func (m *Model) deleteDiaryBook(id string) error {
	if err := m.diary.DeleteBook(id); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

// diaryDirectory asks the user for the directory an import reads or an export
// writes. The empty string is a dismissed picker, which is not an error.
func (m *Model) diaryDirectory(title string) (string, error) {
	directory, err := m.pickDirectory(title)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(directory), nil
}

// importDiary scans a picked directory and writes its diary files into the
// ledger the 日记 page shows. The page is reloaded afterwards so the imported
// days appear in its tree without a second visit.
func (m *Model) importDiary() (settings.DiaryImport, error) {
	if m.workspace == nil || m.diary.ActiveBook() == "" {
		return settings.DiaryImport{}, errNoDiaryWorkspace
	}
	directory, err := m.diaryDirectory("选择日记导入目录")
	if err != nil {
		return settings.DiaryImport{}, err
	}
	if directory == "" {
		return settings.DiaryImport{Canceled: true}, nil
	}
	result, err := diarysrv.Import(context.Background(), m.workspace, m.diary.ActiveBook(), directory)
	if err != nil {
		return settings.DiaryImport{}, err
	}
	m.diary.Reload()
	m.invalidate()
	failures := make([]string, 0, len(result.Failures))
	for _, failure := range result.Failures {
		failures = append(failures, failure.Date+": "+failure.Message)
	}
	return settings.DiaryImport{
		Imported: result.Imported, Total: result.Total, Failures: failures,
	}, nil
}

// exportDiary writes the shown ledger's diary to a picked directory, filtered by
// the section's scope. period is one day of the year or month to take.
func (m *Model) exportDiary(scope domain.DiaryExportScope, period string) (settings.DiaryExport, error) {
	if m.workspace == nil || m.diary.ActiveBook() == "" {
		return settings.DiaryExport{}, errNoDiaryWorkspace
	}
	directory, err := m.diaryDirectory("选择日记导出目录")
	if err != nil {
		return settings.DiaryExport{}, err
	}
	if directory == "" {
		return settings.DiaryExport{Canceled: true}, nil
	}
	query := domain.DiaryExportQuery{Scope: scope}
	if parsed, ok := domain.ParseDiaryDate(period); ok {
		if scope != domain.DiaryScopeAll {
			query.Year = parsed.Year()
		}
		if scope == domain.DiaryScopeMonth {
			query.Month = int(parsed.Month())
		}
	}
	result, err := diarysrv.Export(context.Background(), m.workspace, m.diary.ActiveBook(), directory, query)
	if err != nil {
		return settings.DiaryExport{}, err
	}
	failures := make([]string, 0, len(result.Failures))
	for _, failure := range result.Failures {
		failures = append(failures, failure.Date+": "+failure.Message)
	}
	return settings.DiaryExport{
		Written: result.Written, Total: result.Total, Failures: failures,
	}, nil
}

// openExternalURL opens a place outside the window: the About page's project
// links and the framework's colophon. The platform refuses anything that is not
// http, https or mailto, and the page shows that refusal as a toast.
func (m *Model) openExternalURL(target string) error {
	return app.OpenURL(target)
}

// copyToClipboard puts the About page's build facts on the clipboard. It goes
// through the driver under a test, so a run never touches the real clipboard.
func (m *Model) copyToClipboard(value string) error {
	return app.SetClipboardText(value)
}

// The ledger commands belong to the accounting section, which owns the shared
// ledger store the four pages read. The shell only repaints: a ledger created in
// settings is immediately the one the records page shows.
func (m *Model) createLedger(name, description string) error {
	if err := m.accounting.CreateLedger(name, description); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

func (m *Model) updateLedger(id, name, description string) error {
	if err := m.accounting.UpdateLedger(id, name, description); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

func (m *Model) deleteLedger(id string) error {
	if err := m.accounting.DeleteLedger(id); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

func (m *Model) createStockAccount(name, description string) error {
	if err := m.stock.CreateAccount(name, description); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

func (m *Model) updateStockAccount(id, name, description string) error {
	if err := m.stock.UpdateAccount(id, name, description); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

func (m *Model) deleteStockAccount(id string) error {
	if err := m.stock.DeleteAccount(id); err != nil {
		return err
	}
	m.invalidate()
	return nil
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
	if directory == "" {
		return nil
	}
	if m.workspace != nil && directory == m.workspace.Directory() {
		return nil
	}
	workspace, err := storepkg.Open(directory)
	if err != nil {
		m.workspaceErr = err.Error()
		return err
	}
	if err := m.adoptWorkspace(workspace); err != nil {
		m.workspaceErr = err.Error()
		return err
	}
	previousDir, previousRecent := m.config.WorkspaceDir, m.config.RecentWorkspaces
	m.config.WorkspaceDir = directory
	m.config.RecentWorkspaces = prependWorkspace(directory, previousRecent)
	if err := m.persist(); err != nil {
		m.config.WorkspaceDir = previousDir
		m.config.RecentWorkspaces = previousRecent
		return err
	}
	m.workspaceErr = ""
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
