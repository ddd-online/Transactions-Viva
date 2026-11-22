package desktop

import (
	"context"
	"fmt"
	"strings"

	"github.com/vivaui/viva/app"

	"transactions/internal/backup"
	"transactions/internal/pages/settings"
	storepkg "transactions/internal/store"
)

// backupDir is the resolved directory shown in 数据备份: the configured path or
// the default under Documents.
func (m *Model) backupDir() string {
	return backup.ResolveDir(m.config.BackupDir, backup.DocumentsDir())
}

func (m *Model) createManualBackup() (settings.BackupResult, error) {
	if m.workspace == nil {
		return settings.BackupResult{}, fmt.Errorf("还没有打开工作空间")
	}
	result, err := m.backups.CreateManual(context.Background(), m.workspace, m.backupDir())
	if err != nil {
		return settings.BackupResult{}, err
	}
	return settings.BackupResult{
		Name: result.Name, SizeBytes: result.SizeBytes, Duration: result.Duration,
	}, nil
}

func (m *Model) listBackups() ([]settings.BackupEntry, error) {
	entries := m.backups.List(m.backupDir())
	result := make([]settings.BackupEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, settings.BackupEntry{
			Path: entry.Path, Name: entry.Name, Kind: string(entry.Kind),
			CreatedAt: entry.CreatedAt, SizeBytes: entry.SizeBytes,
			SchemaVersion: entry.SchemaVersion, WorkspaceDir: entry.WorkspaceDir,
			HasAssets: entry.HasAssets, Restorable: entry.Restorable, Problem: entry.Problem,
		})
	}
	return result, nil
}

func (m *Model) pickRestoreBackup() (settings.BackupEntry, bool, error) {
	if m.pickBackup == nil {
		return settings.BackupEntry{}, false, nil
	}
	path, err := m.pickBackup()
	if err != nil || strings.TrimSpace(path) == "" {
		return settings.BackupEntry{}, false, err
	}
	entry, err := m.backups.EntryAt(path)
	if err != nil {
		return settings.BackupEntry{}, false, err
	}
	return settings.BackupEntry{
		Path: entry.Path, Name: entry.Name, Kind: string(entry.Kind),
		CreatedAt: entry.CreatedAt, SizeBytes: entry.SizeBytes,
		SchemaVersion: entry.SchemaVersion, WorkspaceDir: entry.WorkspaceDir,
		HasAssets: entry.HasAssets, Restorable: entry.Restorable, Problem: entry.Problem,
	}, true, nil
}

// restoreBackup replaces the workspace database and reopens it in place, so the
// pages pick up the restored data without restarting the process.
func (m *Model) restoreBackup(entry settings.BackupEntry) error {
	if m.workspace == nil {
		return fmt.Errorf("还没有打开工作空间")
	}
	directory := m.workspace.Directory()
	pre, err := m.backups.Restore(context.Background(), m.workspace, m.backupDir(), entry.Path)
	if pre == "" && err != nil {
		return err
	}
	// Restore closes the workspace before it swaps the files, so even a failed
	// copy has left this handle unusable. Reopen whatever is on disk now.
	m.workspace = nil
	workspace, openErr := storepkg.Open(directory)
	if openErr != nil {
		m.workspaceErr = openErr.Error()
		if err != nil {
			return fmt.Errorf("%w；重新打开工作空间失败：%v", err, openErr)
		}
		return fmt.Errorf("恢复后重新打开工作空间失败：%w", openErr)
	}
	if adoptErr := m.adoptWorkspace(workspace); adoptErr != nil {
		m.workspaceErr = adoptErr.Error()
		return adoptErr
	}
	m.workspaceErr = ""
	if err != nil {
		return err
	}
	_ = pre
	return nil
}

func (m *Model) changeBackupDir() error {
	directory, err := m.pickDirectory("选择备份位置")
	if err != nil {
		return err
	}
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil
	}
	if m.workspace != nil {
		if problem := backup.DirProblem(directory, m.workspace.Directory()); problem != "" {
			return fmt.Errorf("%s", problem)
		}
	}
	previous := m.config.BackupDir
	m.config.BackupDir = directory
	if err := m.persist(); err != nil {
		m.config.BackupDir = previous
		return err
	}
	return nil
}

func (m *Model) setAutoBackup(enabled bool) error {
	previous := m.config.AutoBackup
	m.config.AutoBackup = enabled
	if err := m.persist(); err != nil {
		m.config.AutoBackup = previous
		return err
	}
	return nil
}

func (m *Model) openRecentWorkspace(directory string) error {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil
	}
	if m.workspace != nil && strings.EqualFold(m.workspace.Directory(), directory) {
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
		m.config.WorkspaceDir, m.config.RecentWorkspaces = previousDir, previousRecent
		return err
	}
	m.autoBackupIfNeeded()
	return nil
}

// autoBackupIfNeeded runs at most one automatic snapshot per day. A failure is
// deliberately silent: opening a workspace must never depend on the backup
// directory being writable.
func (m *Model) autoBackupIfNeeded() {
	if m.workspace == nil || !m.config.AutoBackup {
		return
	}
	_, _ = m.backups.AutoBackupIfNeeded(context.Background(), m.workspace, m.backupDir(), true)
}

// openPath is the shell's "open this folder" command for 数据备份.
func (m *Model) openPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return app.OpenPath(path)
}
