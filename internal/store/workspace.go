package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

// Workspace owns one opened SQLite database and its directory. It implements
// repository.Repository; callers must Close it when switching away or quitting.
type Workspace struct {
	directory string
	database  string
	db        *sql.DB
}

// Open opens (or creates) the workspace database in directory, checks it and
// applies migrations. The directory must already exist: silently recreating a
// deleted workspace would turn "gone" into "empty".
func Open(directory string) (*Workspace, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil, fmt.Errorf("%w：路径为空", ErrWorkspaceMissing)
	}
	info, err := os.Stat(directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w：%s", ErrWorkspaceMissing, directory)
		}
		return nil, fmt.Errorf("读取工作空间目录失败：%w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w：%s 不是目录", ErrWorkspaceMissing, directory)
	}

	databasePath := databasePathOf(directory)
	_, statErr := os.Stat(databasePath)
	existed := statErr == nil
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("打开工作空间数据库失败：%w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	closeOnError := func(openErr error) (*Workspace, error) {
		_ = db.Close()
		return nil, openErr
	}

	if err := applyPragmas(db); err != nil {
		return closeOnError(fmt.Errorf("%w：%s（%v）", ErrCorrupt, databasePath, err))
	}
	if existed {
		if err := quickCheck(db); err != nil {
			return closeOnError(fmt.Errorf("%w：%s：%v", ErrCorrupt, databasePath, err))
		}
	}

	legacy, err := tableExists(db, "tbl_billadm_schema_migration")
	if err != nil {
		return closeOnError(fmt.Errorf("%w：%v", ErrCorrupt, err))
	}
	if legacy {
		return closeOnError(ErrLegacyWorkspace)
	}

	version, hasMeta, err := schemaVersion(db)
	if err != nil {
		return closeOnError(fmt.Errorf("%w：%v", ErrCorrupt, err))
	}
	if !hasMeta {
		count, err := userTableCount(db)
		if err != nil {
			return closeOnError(fmt.Errorf("%w：%v", ErrCorrupt, err))
		}
		if count > 0 {
			if isOldAccountingDB(db) {
				return closeOnError(ErrOldWorkspace)
			}
			return closeOnError(ErrNotWorkspace)
		}
		version = 0
	} else if version < 1 {
		return closeOnError(ErrNotWorkspace)
	}
	if version > targetVersion() {
		if isOldAccountingDB(db) {
			return closeOnError(fmt.Errorf("%w（库格式版本 %d）", ErrOldWorkspace, version))
		}
		return closeOnError(fmt.Errorf("%w：库格式版本 %d，本程序支持到 %d", ErrVersionTooNew, version, targetVersion()))
	}
	if err := applyMigrations(db, databasePath, migrations, version); err != nil {
		return closeOnError(err)
	}
	return &Workspace{directory: directory, database: databasePath, db: db}, nil
}

// applyPragmas sets the old application's connection contract. WAL keeps reads
// and writes from blocking each other; the app uses a single connection.
func applyPragmas(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	return nil
}

// quickCheck runs PRAGMA quick_check and requires every returned row to say ok.
func quickCheck(db *sql.DB) error {
	rows, err := db.Query("PRAGMA quick_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	var problems []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(value), "ok") {
			problems = append(problems, value)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "；"))
	}
	return nil
}

// isOldAccountingDB recognizes the old application's database so opening it can
// point at the import tool instead of a generic version error.
func isOldAccountingDB(db *sql.DB) bool {
	for _, table := range []string{"stock_account", "chart", "key_event", "diary_entry"} {
		exists, err := tableExists(db, table)
		if err == nil && exists {
			return true
		}
	}
	return false
}

// Directory is the workspace directory.
func (w *Workspace) Directory() string { return w.directory }

// DatabasePath is the SQLite file inside the workspace.
func (w *Workspace) DatabasePath() string { return w.database }

// Close releases the connection. It is safe to call more than once.
func (w *Workspace) Close() error {
	if w == nil || w.db == nil {
		return nil
	}
	err := w.db.Close()
	w.db = nil
	return err
}

// dbHandle is the connection; methods call it after an Open check.
func (w *Workspace) dbHandle() (*sql.DB, error) {
	if w == nil || w.db == nil {
		return nil, errors.New("工作空间尚未打开")
	}
	return w.db, nil
}
