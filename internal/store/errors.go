package store

import "errors"

// Opening a workspace can fail for reasons the UI must tell apart. The errors
// below are wrapped with the concrete detail, so callers use errors.Is.
var (
	// ErrCorrupt is a damaged database file; the user should restore from a
	// backup instead of the application creating a new one.
	ErrCorrupt = errors.New("工作空间数据库已损坏")
	// ErrNotWorkspace is a database with tables but no app_meta: it belongs to
	// something else, so it must not be opened as a workspace.
	ErrNotWorkspace = errors.New("不是 Transactions 工作空间")
	// ErrVersionTooNew is a database written by a newer application build.
	ErrVersionTooNew = errors.New("工作空间格式版本高于本程序支持的版本")
	// ErrWorkspaceMissing is a directory that no longer exists. The application
	// never recreates it silently: that would turn "missing" into "empty".
	ErrWorkspaceMissing = errors.New("工作空间目录不存在")
)
