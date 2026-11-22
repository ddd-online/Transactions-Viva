package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"transactions/internal/store"
)

func testBackup(t *testing.T) (*store.Workspace, string, string, *Service, *time.Time) {
	t.Helper()
	workspace, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Close() })
	backupDir := t.TempDir()
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.Local)
	service := &Service{SupportedSchema: store.CurrentSchemaVersion(), Now: func() time.Time { return now }}
	return workspace, backupDir, workspace.Directory(), service, &now
}

func TestManualBackupKeepsTwoAndCopiesAssets(t *testing.T) {
	workspace, backupDir, workspaceDir, service, now := testBackup(t)
	ctx := context.Background()
	if _, err := workspace.CreateLedger(ctx, "家庭账本", ""); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(workspaceDir, "data", "assets", "key_events", "2026-11-22", "a.jpg")
	if err := os.MkdirAll(filepath.Dir(asset), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(asset, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	for day := 1; day <= 3; day++ {
		*now = time.Date(2026, 11, 20+day, 10, 0, 0, 0, time.Local)
		if _, err := service.CreateManual(ctx, workspace, backupDir); err != nil {
			t.Fatal(err)
		}
	}
	entries := service.List(backupDir)
	manual := 0
	for _, entry := range entries {
		if entry.Kind == KindManual {
			manual++
			if !entry.Restorable || entry.Problem != "" {
				t.Fatalf("manual entry = %+v", entry)
			}
			if !entry.HasAssets {
				t.Fatalf("manual backup has no assets: %+v", entry)
			}
		}
	}
	if manual != 2 {
		t.Fatalf("manual backups = %d, want 2", manual)
	}
}

func TestAutoBackupOncePerDayAndKeepsTwo(t *testing.T) {
	workspace, backupDir, _, service, now := testBackup(t)
	ctx := context.Background()
	if _, err := workspace.CreateLedger(ctx, "家庭账本", ""); err != nil {
		t.Fatal(err)
	}
	*now = time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)
	if _, err := service.AutoBackupIfNeeded(ctx, workspace, backupDir, true); err != nil {
		t.Fatal(err)
	}
	*now = time.Date(2026, 11, 20, 18, 0, 0, 0, time.Local)
	if result, err := service.AutoBackupIfNeeded(ctx, workspace, backupDir, true); err != nil || result != nil {
		t.Fatalf("same-day auto = %+v, %v", result, err)
	}
	for day := 21; day <= 23; day++ {
		*now = time.Date(2026, 11, day, 9, 0, 0, 0, time.Local)
		if _, err := service.AutoBackupIfNeeded(ctx, workspace, backupDir, true); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for _, entry := range service.List(backupDir) {
		if entry.Kind == KindAuto {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("auto backups = %d, want 2", count)
	}
}

func TestRestoreReplacesDatabaseAndSavesPreRestore(t *testing.T) {
	workspace, backupDir, workspaceDir, service, _ := testBackup(t)
	ctx := context.Background()
	if _, err := workspace.CreateLedger(ctx, "家庭账本", ""); err != nil {
		t.Fatal(err)
	}
	result, err := service.CreateManual(ctx, workspace, backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreateLedger(ctx, "第二本", ""); err != nil {
		t.Fatal(err)
	}
	pre, err := service.Restore(ctx, workspace, backupDir, result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if pre == "" {
		t.Fatal("restore did not write a pre-restore copy")
	}
	reopened, err := store.Open(workspaceDir)
	if err != nil {
		t.Fatalf("reopen after restore: %v", err)
	}
	defer reopened.Close()
	ledgers, err := reopened.Ledgers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledgers) != 1 || ledgers[0].Name != "家庭账本" {
		t.Fatalf("restored ledgers = %+v", ledgers)
	}
}

func TestDeleteBackupRemovesOnlyInsideDirectory(t *testing.T) {
	workspace, backupDir, _, service, _ := testBackup(t)
	result, err := service.CreateManual(context.Background(), workspace, backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(backupDir, result.Path); err != nil {
		t.Fatalf("delete backup: %v", err)
	}
	if entries := service.List(backupDir); len(entries) != 0 {
		t.Fatalf("entries after delete = %+v", entries)
	}
	outside := filepath.Join(t.TempDir(), "outside.db")
	if err := os.WriteFile(outside, []byte("do not delete"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(backupDir, outside); err == nil {
		t.Fatal("Delete removed a path outside the backup directory")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was removed: %v", err)
	}
}

func TestDirProblemRejectsWorkspaceInside(t *testing.T) {
	workspace := t.TempDir()
	if DirProblem(workspace, workspace) == "" {
		t.Fatal("workspace itself is allowed")
	}
	if DirProblem(filepath.Join(workspace, "backup"), workspace) == "" {
		t.Fatal("workspace child is allowed")
	}
	if DirProblem(t.TempDir(), workspace) != "" {
		t.Fatal("separate backup directory rejected")
	}
}
