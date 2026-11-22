package desktop

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"

	storepkg "transactions/internal/store"
)

// openConfiguredWorkspace opens the workspace remembered in the configuration.
// A missing or unopenable workspace leaves the model without one and records
// why; the content area then shows the workspace gate.
func (m *Model) openConfiguredWorkspace() {
	directory := strings.TrimSpace(m.config.WorkspaceDir)
	if directory == "" {
		return
	}
	workspace, err := storepkg.Open(directory)
	if err != nil {
		m.workspaceErr = err.Error()
		return
	}
	if err := m.adoptWorkspace(workspace); err != nil {
		m.workspaceErr = err.Error()
		return
	}
	m.autoBackupIfNeeded()
}

// adoptWorkspace makes ws the open workspace and reloads the accounting pages.
// The previous workspace is closed only after the new one loaded; if loading
// fails, the previous one is restored.
func (m *Model) adoptWorkspace(workspace *storepkg.Workspace) error {
	if workspace == nil {
		m.accounting.SetRepository(nil)
		m.diary.SetDataSource(nil)
		m.events.SetDataSource(nil)
		_ = m.stock.SetRepository(nil)
		m.stock.SetLimitService(nil)
		if m.workspace != nil {
			_ = m.workspace.Close()
			m.workspace = nil
		}
		return nil
	}
	if err := m.accounting.SetRepository(workspace); err != nil {
		if workspace != m.workspace {
			_ = workspace.Close()
		}
		if m.workspace != nil {
			_ = m.accounting.SetRepository(m.workspace)
			m.diary.SetDataSource(m.workspace)
			m.events.SetDataSource(m.workspace)
			_ = m.stock.SetRepository(m.workspace)
			m.stock.SetLimitService(m.stockLimitService(m.workspace))
		}
		return err
	}
	// The 日记 page reads the same workspace and the same ledger list.
	m.diary.SetDataSource(workspace)
	// Events belong to the workspace itself, so the page reads it and nothing
	// about a ledger.
	m.events.SetDataSource(workspace)
	// Stock accounts belong to the workspace too; the section keeps its own
	// account list and reads no ledger.
	if err := m.stock.SetRepository(workspace); err != nil {
		if m.workspace != nil {
			_ = m.accounting.SetRepository(m.workspace)
			m.diary.SetDataSource(m.workspace)
			m.events.SetDataSource(m.workspace)
			_ = m.stock.SetRepository(m.workspace)
			m.stock.SetLimitService(m.stockLimitService(m.workspace))
		}
		if workspace != m.workspace {
			_ = workspace.Close()
		}
		return err
	}
	m.stock.SetLimitService(m.stockLimitService(workspace))
	previous := m.workspace
	m.workspace = workspace
	m.workspaceErr = ""
	if previous != nil && previous != workspace {
		_ = previous.Close()
	}
	return nil
}

// closeWorkspace releases the SQLite connection when the application exits.
func (m *Model) closeWorkspace() {
	if m.workspace == nil {
		return
	}
	_ = m.workspace.Close()
	m.workspace = nil
}

// workspaceGate is the content shown while no workspace is open: the app cannot
// read or write accounting data until the user chooses a directory. Settings
// stays reachable from the navigation rail.
func (m *Model) workspaceGate(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Fill().Center().Pad(32).Children(func() {
		ui.Column(c).Width(470).Gap(14).Pad(28).
			Radius(theme.Metrics.RadiusSurface).
			Border(1, theme.Border).
			Background(theme.Card).
			Shadow(gfx.ShadowMD).Children(func() {
			typography.Title(c, "打开工作空间")
			typography.BodyParagraph(c, "账本、记录、分类标签与模板都保存在本地工作空间的 transactions.db 里。选择一个目录开始使用。", 3).Muted()
			if m.workspaceErr != "" {
				// A backend message can be a sentence or three; let it wrap
				// instead of running off the card.
				typography.CaptionParagraph(c, "打开失败："+m.workspaceErr, 4).Color(theme.Danger)
			}
			if ui.Button(c, "选择工作空间").Clicked() {
				if err := m.pickWorkspaceDir(); err != nil {
					m.workspaceErr = err.Error()
				}
			}
			typography.CaptionParagraph(c, "也可以从左侧进入「设置 → 通用设置 → 工作空间」选择目录。", 2)
		})
	})
}
