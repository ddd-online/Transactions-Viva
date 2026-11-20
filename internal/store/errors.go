package store

import "errors"

// Opening a workspace can fail for reasons the UI must tell apart. The errors
// below are wrapped with the concrete detail, so callers use errors.Is.
var (
	// ErrCorrupt is a damaged database file; the user should restore from a
	// backup instead of the application creating a new one.
	ErrCorrupt = errors.New("工作空间数据库已损坏")
	// ErrNotWorkspace is a database with tables but no app_meta: it belongs to
	// something else (or predates the v1 schema), so it must not be migrated.
	ErrNotWorkspace = errors.New("不是 Transactions 工作空间")
	// ErrVersionTooNew is a database written by a newer application build.
	ErrVersionTooNew = errors.New("工作空间格式版本高于本程序支持的版本")
	// ErrLegacyWorkspace is the pre-v1 source database, which needs the import
	// tool rather than a direct open.
	ErrLegacyWorkspace = errors.New("这是更早版本的工作空间")
	// ErrOldWorkspace is an old application database (schema versions 1-12)
	// whose accounting tables match ours but whose schema history is not ours.
	// It is import-only; opening it as a new workspace would misread versions.
	ErrOldWorkspace = errors.New("这是旧版 Transactions 工作空间")
	// ErrWorkspaceMissing is a directory that no longer exists. The application
	// never recreates it silently: that would turn "missing" into "empty".
	ErrWorkspaceMissing = errors.New("工作空间目录不存在")
)
