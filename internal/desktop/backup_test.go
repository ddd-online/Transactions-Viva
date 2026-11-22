package desktop

import (
	"context"
	"testing"

	"transactions/internal/pages/settings"
)

func TestDataBackupCreatesAndListsManualBackup(t *testing.T) {
	driver, model := open(t)
	directory := t.TempDir()
	model.config.BackupDir = directory
	click(t, driver, "设置")
	click(t, driver, "数据备份")
	click(t, driver, "立即备份")
	if entries := model.backups.List(directory); len(entries) != 1 {
		t.Fatalf("manual backups = %d, want 1", len(entries))
	}
	if !shows(driver, "手动") {
		t.Fatal("manual backup entry is not shown")
	}
}

func TestDataBackupRestoresAndReopensWorkspace(t *testing.T) {
	_, model := open(t)
	ctx := context.Background()
	directory := t.TempDir()
	model.config.BackupDir = directory
	if _, err := model.createManualBackup(); err != nil {
		t.Fatal(err)
	}
	before, err := model.workspace.Ledgers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.workspace.CreateLedger(ctx, "第二本", ""); err != nil {
		t.Fatal(err)
	}
	entries := model.backups.List(directory)
	if len(entries) != 1 {
		t.Fatalf("manual backups = %d", len(entries))
	}
	entry := entries[0]
	if err := model.restoreBackup(settings.BackupEntry{Path: entry.Path}); err != nil {
		t.Fatal(err)
	}
	ledgers, err := model.workspace.Ledgers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledgers) != len(before) {
		t.Fatalf("restored ledgers = %+v, want %+v", ledgers, before)
	}
	for _, ledger := range ledgers {
		if ledger.Name == "第二本" {
			t.Fatalf("post-backup ledger survived restore: %+v", ledgers)
		}
	}
}
