package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DBProbe is what opening a candidate snapshot read-only tus. A probe never
// repairs or changes the file: a failed open is reported, not created.
type DBProbe struct {
	Opened        bool
	HasAppMeta    bool
	SchemaVersion int
	WorkspaceDir  string
	QuickCheck    string
}

// CurrentSchemaVersion is the schema this build writes.
func CurrentSchemaVersion() int {
	if len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].version
}

// ProbeDatabase reads the few facts the backup list needs. deep uses
// integrity_check; the quick path uses quick_check. It never returns an error:
// a probe that cannot open is Opened=false with the reason in QuickCheck.
func ProbeDatabase(path string, deep bool) DBProbe {
	probe := DBProbe{}
	if _, err := os.Stat(path); err != nil {
		probe.QuickCheck = err.Error()
		return probe
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		probe.QuickCheck = err.Error()
		return probe
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	probe.Opened = true
	if err := db.Ping(); err != nil {
		probe.Opened = false
		probe.QuickCheck = err.Error()
		return probe
	}
	hasMeta, err := tableExists(db, "app_meta")
	if err != nil {
		probe.QuickCheck = err.Error()
		return probe
	}
	probe.HasAppMeta = hasMeta
	if hasMeta {
		_ = db.QueryRow("SELECT value FROM app_meta WHERE key = 'schema_version'").Scan(&probe.SchemaVersion)
		_ = db.QueryRow("SELECT value FROM app_meta WHERE key = 'workspace_dir'").Scan(&probe.WorkspaceDir)
	}
	check := "quick_check"
	if deep {
		check = "integrity_check"
	}
	var result string
	if err := db.QueryRow("PRAGMA " + check).Scan(&result); err != nil {
		probe.QuickCheck = err.Error()
		return probe
	}
	probe.QuickCheck = result
	return probe
}

// SnapshotTo writes a consistent copy of the running database to target and
// stamps the source workspace in app_meta. The target must not be the current
// database; callers give it a unique name.
func (w *Workspace) SnapshotTo(ctx context.Context, target string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("备份路径为空")
	}
	if samePath(target, w.database) {
		return fmt.Errorf("不能覆盖当前正在使用的数据库")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("创建备份目录失败：%w", err)
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理备份文件失败：%w", err)
	}
	escaped := strings.ReplaceAll(target, "'", "''")
	if _, err := w.db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil {
		return fmt.Errorf("写入备份快照失败：%w", err)
	}
	snapshot, err := sql.Open("sqlite", target)
	if err != nil {
		_ = os.Remove(target)
		return fmt.Errorf("打开备份快照失败：%w", err)
	}
	if _, err := snapshot.ExecContext(ctx,
		"INSERT INTO app_meta(key, value) VALUES('workspace_dir', ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value", w.directory); err != nil {
		_ = snapshot.Close()
		_ = os.Remove(target)
		return fmt.Errorf("标记备份来源失败：%w", err)
	}
	_ = snapshot.Close()
	probe := ProbeDatabase(target, true)
	if !probe.Opened || probe.QuickCheck != "ok" {
		_ = os.Remove(target)
		return fmt.Errorf("备份快照未通过完整性检查：%s", probe.QuickCheck)
	}
	return nil
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}
