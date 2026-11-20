package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed schema/*.sql
var schemaFiles embed.FS

var initSchema = mustReadSchema("schema/0001_init.sql")

func mustReadSchema(name string) string {
	data, err := schemaFiles.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// migration is one schema change. Versions are contiguous from 1; append a new
// entry for every structure change and never edit a released one.
type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{version: 1, name: "init", sql: initSchema},
}

func targetVersion() int { return migrations[len(migrations)-1].version }

// validateMigrations checks the contract the old project used: versions start
// at 1 and increase by one with no holes.
func validateMigrations(list []migration) error {
	if len(list) == 0 {
		return fmt.Errorf("store: migration list is empty")
	}
	for index, item := range list {
		if item.version != index+1 {
			return fmt.Errorf("store: migration versions must start at 1 and be contiguous: want %d, got %d", index+1, item.version)
		}
	}
	return nil
}

// schemaVersion reads app_meta.schema_version. exists is false when app_meta is
// absent; a missing or malformed value with the table present is version 0 and
// the caller treats that as an unknown workspace.
func schemaVersion(db *sql.DB) (version int, exists bool, err error) {
	hasTable, err := tableExists(db, "app_meta")
	if err != nil {
		return 0, false, err
	}
	if !hasTable {
		return 0, false, nil
	}
	var value string
	row := db.QueryRow("SELECT value FROM app_meta WHERE key = 'schema_version'")
	switch err := row.Scan(&value); {
	case err == sql.ErrNoRows:
		return 0, true, nil
	case err != nil:
		return 0, true, err
	}
	fmt.Sscanf(strings.TrimSpace(value), "%d", &version)
	return version, true, nil
}

func setSchemaVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec(
		"INSERT INTO app_meta(key, value) VALUES ('schema_version', ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		fmt.Sprintf("%d", version),
	)
	return err
}

// applyMigrations upgrades db to the target list. A non-zero current version is
// backed up first; each migration and its version update commit together.
func applyMigrations(db *sql.DB, databasePath string, list []migration, current int) error {
	if err := validateMigrations(list); err != nil {
		return err
	}
	target := list[len(list)-1].version
	if current > target {
		return fmt.Errorf("%w：库格式版本 %d，本程序支持到 %d", ErrVersionTooNew, current, target)
	}
	if current == target {
		return nil
	}
	if current < 0 {
		current = 0
	}
	if current > 0 {
		if err := backupDatabase(db, databasePath); err != nil {
			return fmt.Errorf("迁移前备份失败：%w", err)
		}
	}
	for _, item := range list {
		if item.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(item.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("应用迁移 %d (%s) 失败：%w", item.version, item.name, err)
		}
		if err := setSchemaVersion(tx, item.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("写入迁移版本 %d 失败：%w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 %d 失败：%w", item.version, err)
		}
	}
	return nil
}

// backupDatabase writes a single rollback copy next to the database. VACUUM
// INTO needs to run outside a transaction.
func backupDatabase(db *sql.DB, databasePath string) error {
	backupPath := databasePath + ".bak"
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	quoted := strings.ReplaceAll(backupPath, "'", "''")
	if _, err := db.Exec("VACUUM INTO '" + quoted + "'"); err != nil {
		return err
	}
	return nil
}

func tableExists(db *sql.DB, name string) (bool, error) {
	var found string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return found != "", nil
}

// userTableCount counts the workspace's own tables, ignoring SQLite internals.
func userTableCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'",
	).Scan(&count)
	return count, err
}

// databasePathOf is the database inside a workspace directory.
func databasePathOf(directory string) string { return filepath.Join(directory, "transactions.db") }
