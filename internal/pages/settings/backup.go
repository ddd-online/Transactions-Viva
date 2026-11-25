package settings

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/pages/settings/controls"
)

// BackupEntry is one row of 数据备份 · 从备份恢复. The shell maps it to and from
// internal/backup.Entry; the page only formats it.
type BackupEntry struct {
	Path          string
	Name          string
	Kind          string
	CreatedAt     int64
	SizeBytes     int64
	SchemaVersion int
	WorkspaceDir  string
	HasAssets     bool
	Restorable    bool
	Problem       string
}

// BackupResult is the receipt of 立即备份.
type BackupResult struct {
	Name      string
	SizeBytes int64
	Duration  time.Duration
}

func (m *Model) dataBackup(c ui.C, spec Spec) {
	if !m.backupLoaded {
		m.backupLoaded = true
		m.backupError = ""
		if spec.OnListBackups != nil {
			entries, err := spec.OnListBackups()
			if err != nil {
				m.backupError = err.Error()
			} else {
				m.backupEntries = entries
			}
		}
	}
	controls.Page(c, "数据备份",
		controls.GroupSpec{Title: "手动备份", Body: func() { m.backupManualGroup(c, spec) }},
		controls.GroupSpec{Title: "备份设置", Body: func() { m.backupSettingsGroup(c, spec) }},
		controls.GroupSpec{Title: "从备份恢复", Body: func() { m.backupRestoreGroup(c, spec) }},
		controls.GroupSpec{Title: "最近工作空间", Body: func() { m.backupRecentGroup(c, spec) }},
	)
	if m.backupError != "" {
		typography.Caption(c, m.backupError).Color(c.Theme().Danger)
	}
}

func (m *Model) backupManualGroup(c ui.C, spec Spec) {
	controls.Row(c, "备份当前工作空间", "数据库与图片资产，保留最近 2 份。", func() {
		button := ui.Button(c, "立即备份").Secondary()
		if spec.OnBackupNow == nil || spec.Workspace == "" {
			button = button.Disabled(true)
		}
		if button.Clicked() {
			m.runManualBackup(spec)
		}
	})
}

func (m *Model) backupSettingsGroup(c ui.C, spec Spec) {
	controls.Row(c, "备份位置", spec.BackupDir, func() {
		change := ui.Button(c, "更改").Secondary()
		if spec.OnChangeBackupDir == nil {
			change = change.Disabled(true)
		}
		if change.Clicked() {
			pick := spec.OnChangeBackupDir
			c.After(func() {
				m.report("更改备份位置失败", pick)
				m.backupLoaded = false
			})
		}
		open := ui.Button(c, "打开").Secondary()
		if spec.BackupDir == "" || spec.OnOpenPath == nil {
			open = open.Disabled(true)
		}
		if open.Clicked() {
			m.report("打开备份位置失败", func() error { return spec.OnOpenPath(spec.BackupDir) })
		}
	})
	ui.Divider(c)
	controls.Row(c, "工作空间目录", "", func() {
		open := ui.Button(c, "打开").Secondary()
		if spec.Workspace == "" || spec.OnOpenPath == nil {
			open = open.Disabled(true)
		}
		if open.Clicked() {
			m.report("打开工作空间失败", func() error { return spec.OnOpenPath(spec.Workspace) })
		}
	})
	ui.Divider(c)
	controls.Row(c, "自动备份", "每天第一次打开时存一份只含数据库的快照，保留最近 2 份。", func() {
		value := spec.AutoBackup
		control := ui.Switch(c, &value, "")
		if spec.OnSetAutoBackup == nil {
			control = control.Disabled(true)
		}
		if control.Label("自动备份").Changed() {
			m.report("保存自动备份设置失败", func() error { return spec.OnSetAutoBackup(value) })
		}
	})
}

func (m *Model) backupRestoreGroup(c ui.C, spec Spec) {
	controls.Row(c, "从文件恢复…", "选择已有的 transactions.db 或手动备份目录。", func() {
		button := ui.Button(c, "选择文件").Secondary()
		if spec.OnPickRestoreFile == nil || spec.Workspace == "" {
			button = button.Disabled(true)
		}
		if button.Clicked() {
			pick := spec.OnPickRestoreFile
			c.After(func() {
				entry, ok, err := pick()
				if err != nil {
					m.toaster.Show(err.Error(), "", ui.ToastError)
					return
				}
				if ok {
					m.backupRestore, m.backupRestoreOn = entry, true
				}
			})
		}
	})
	ui.Divider(c)
	if len(m.backupEntries) == 0 {
		controls.Row(c, "还没有备份", "点「立即备份」生成第一份；也可以从文件恢复。", nil)
		return
	}
	theme := c.Theme()
	metrics := theme.Metrics
	for index, entry := range m.backupEntries {
		if index > 0 {
			ui.Divider(c)
		}
		entry := entry
		ui.Row(c.Key("backup-" + entry.Path)).FillWidth().MinHeight(metrics.ControlHeight * 2).
			Shrink(0).PadX(metrics.PadSurface).Gap(metrics.GapItem * 2).
			Items(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Row(c).FillWidth().Gap(6).Items(ui.Center).Children(func() {
					// The badges keep their width: a long file name ellipsizes
					// instead, so "手动" and "含图片" are never cut in half on the
					// 960 px window.
					typography.Body(c, entry.Name).Grow(1).MinWidth(0).Ellipsis()
					if label := backupKindLabel(entry.Kind); label != "" {
						backupBadge(c, label)
					}
					if entry.HasAssets {
						backupBadge(c, "含图片")
					}
				})
				meta := fmt.Sprintf("%s · %s · 数据格式 v%d",
					time.Unix(entry.CreatedAt, 0).Format("2006-01-02 15:04"),
					backupBytes(entry.SizeBytes), entry.SchemaVersion)
				// Two lines rather than an ellipsis: at the 960 px window the
				// reading column is 469 px, and the schema version at the end of
				// the line is the part a reader checks before restoring.
				ui.Text(c, meta).FontSize(typography.SizeCaption).Mono().
					Color(theme.MutedForeground).MaxLines(2)
				if !entry.Restorable && entry.Problem != "" {
					typography.Caption(c, entry.Problem).Color(theme.Danger)
				}
			})
			if entry.Restorable {
				if ui.Button(c, "恢复").Secondary().Small().Clicked() {
					m.backupRestore, m.backupRestoreOn = entry, true
				}
			} else {
				typography.Caption(c, "不可恢复").Muted()
			}
			remove := ui.Button(c, "删除").Secondary().Small()
			if spec.OnDeleteBackup == nil {
				remove = remove.Disabled(true)
			}
			if remove.Clicked() {
				m.backupDelete, m.backupDeleteOn = entry, true
			}
		})
	}
}

func (m *Model) backupRecentGroup(c ui.C, spec Spec) {
	if len(spec.RecentWorkspaces) == 0 {
		controls.Row(c, "还没有最近工作空间", "选择工作空间后会出现在这里。", nil)
	} else {
		for index, directory := range spec.RecentWorkspaces {
			if index > 0 {
				ui.Divider(c)
			}
			directory := directory
			current := sameDirectory(directory, spec.Workspace)
			c.Key("recent-" + directory)
			controls.Row(c, filepath.Base(directory), directory, func() {
				if current {
					typography.Caption(c, "当前")
				} else {
					button := ui.Button(c, "切换").Link()
					if spec.OnOpenRecentWorkspace == nil {
						button = button.Disabled(true)
					}
					if button.Clicked() {
						m.report("切换工作空间失败", func() error { return spec.OnOpenRecentWorkspace(directory) })
					}
				}
				remove := ui.Button(c, "移除").Secondary().Small()
				if spec.OnRemoveRecentWorkspace == nil {
					remove = remove.Disabled(true)
				}
				if remove.Clicked() {
					m.report("移除最近工作空间失败", func() error { return spec.OnRemoveRecentWorkspace(directory) })
				}
			})
		}
	}
	ui.Divider(c)
	ui.Row(c).Height(c.Theme().Metrics.ControlHeight + 2*c.Theme().Metrics.GapItem).FillWidth().
		PadX(c.Theme().Metrics.PadSurface).Items(ui.Center).Children(func() {
		button := ui.Button(c, "选择工作空间…").Ghost()
		if spec.OnPickWorkspace == nil {
			button = button.Disabled(true)
		}
		if button.Clicked() {
			// The picker is a native modal: opened during the paint, its message
			// loop re-dispatches this very click, and the dialog opens twice.
			pick := spec.OnPickWorkspace
			c.After(func() {
				m.report("打开工作空间失败", pick)
				m.backupLoaded = false
			})
		}
	})
}

func (m *Model) runManualBackup(spec Spec) {
	if spec.OnBackupNow == nil {
		return
	}
	result, err := spec.OnBackupNow()
	if err != nil {
		m.toaster.Show(err.Error(), "", ui.ToastError)
		return
	}
	m.backupLoaded = false
	m.toaster.Show(fmt.Sprintf("已备份 %s（%s，用时 %s）",
		result.Name, backupBytes(result.SizeBytes), result.Duration.Round(time.Millisecond)), "", ui.ToastSuccess)
}

func (m *Model) backupRestoreDialog(c ui.C, spec Spec) {
	if !m.backupRestoreOn {
		return
	}
	entry := m.backupRestore
	ui.ModalWith(c, &m.backupRestoreOn, "恢复这份备份？", ui.ModalSpec{
		Width:       460,
		Description: "当前数据会先另存为恢复前备份，然后替换当前工作空间。恢复后工作空间会在当前窗口中重新打开。",
	}, func() {
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			typography.Body(c, entry.Name)
			typography.Caption(c, time.Unix(entry.CreatedAt, 0).Format("2006-01-02 15:04:05")).
				Mono()
			typography.Caption(c, entry.Path).Mono().Ellipsis()
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.backupRestoreOn = false
		}
		if ui.Button(c, "恢复这份备份").Destructive().Clicked() {
			if spec.OnRestoreBackup == nil {
				return
			}
			if err := spec.OnRestoreBackup(entry); err != nil {
				m.toaster.Show(err.Error(), "", ui.ToastError)
				return
			}
			m.backupRestoreOn = false
			m.backupLoaded = false
			m.toaster.Show("已从备份恢复，恢复前的数据已另存一份", "", ui.ToastSuccess)
		}
	})
}

func (m *Model) backupDeleteDialog(c ui.C, spec Spec) {
	if !m.backupDeleteOn {
		return
	}
	entry := m.backupDelete
	ui.ModalWith(c, &m.backupDeleteOn, "删除这份备份？", ui.ModalSpec{
		Width:       460,
		Description: "只删除这一份备份，不会影响当前工作空间。",
	}, func() {
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			typography.Body(c, entry.Name)
			typography.Caption(c, entry.Path).Mono().Ellipsis()
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.backupDeleteOn = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			if spec.OnDeleteBackup == nil {
				return
			}
			if err := spec.OnDeleteBackup(entry); err != nil {
				m.toaster.Show(err.Error(), "", ui.ToastError)
				return
			}
			m.backupDeleteOn = false
			m.backupLoaded = false
			m.toaster.Show("备份已删除", "", ui.ToastSuccess)
		}
	})
}

func backupKindLabel(kind string) string {
	switch kind {
	case "auto":
		return "自动"
	case "pre-restore":
		return "恢复前"
	case "manual":
		return "手动"
	}
	return ""
}

func backupBadge(c ui.C, label string) {
	theme := c.Theme()
	ui.Text(c, label).FontSize(typography.SizeCaption).PadX(6).PadY(1).Radius(5.5).
		Shrink(0).Background(theme.Muted).Color(theme.MutedForeground)
}

func backupBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", value)
}

func sameDirectory(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
