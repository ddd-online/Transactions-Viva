package store

import (
	"errors"
	"fmt"

	"transactions/internal/repository"
)

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

// userError is the detail the interface shows: its message has no prefix, while
// Unwrap keeps the repository sentinel in the chain, so errors.Is still tells
// the four failure kinds apart. Formatting a sentinel into the message would
// print its internal wording ("无效的数据：…") in front of every validation
// error.
type userError struct {
	sentinel error
	message  string
}

func (e userError) Error() string { return e.message }

func (e userError) Unwrap() error { return e.sentinel }

// invalidf reports a request the store refuses (missing id, unknown enum).
func invalidf(format string, args ...any) error {
	return userError{sentinel: repository.ErrInvalid, message: fmt.Sprintf(format, args...)}
}

// duplicatef reports a name that is already taken.
func duplicatef(format string, args ...any) error {
	return userError{sentinel: repository.ErrDuplicate, message: fmt.Sprintf(format, args...)}
}

// notFoundf reports a row the caller expected but which no longer exists.
func notFoundf(format string, args ...any) error {
	return userError{sentinel: repository.ErrNotFound, message: fmt.Sprintf(format, args...)}
}
