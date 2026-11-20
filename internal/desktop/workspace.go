package desktop

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

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
	}
}

// adoptWorkspace makes ws the open workspace and reloads the accounting pages.
// The previous workspace is closed only after the new one loaded; if loading
// fails, the previous one is restored.
func (m *Model) adoptWorkspace(workspace *storepkg.Workspace) error {
	if workspace == nil {
		m.accounting.SetRepository(nil)
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
		}
		return err
	}
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
			ui.Text(c, "打开工作空间").FontSize(22).Bold()
			ui.Paragraph(c, "账本、记录、分类标签与模板都保存在本地工作空间的 transactions.db 里。选择一个目录开始使用。", 3).Muted()
			if m.workspaceErr != "" {
				ui.Text(c, "打开失败："+m.workspaceErr).FontSize(12).Color(theme.Danger)
			}
			if ui.Button(c, "选择工作空间").Clicked() {
				if err := m.pickWorkspaceDir(); err != nil {
					m.workspaceErr = err.Error()
				}
			}
			ui.Text(c, "也可以从左侧进入「应用设置 → 工作空间」选择目录。").FontSize(12).Muted()
		})
	})
}
