package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"transactions/internal/ids"
	"transactions/internal/repository"
)

// newID is the id shape the store writes: the UUID the application used.
func newID() string { return ids.New() }

func nowUnix() int64 { return time.Now().Unix() }

// inTx runs fn in a transaction and commits only when it returns nil.
func inTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// duplicateError maps a SQLite unique-key failure to the repository sentinel.
func duplicateError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "constraint failed") {
		return fmt.Errorf("%w：%v", repository.ErrDuplicate, err)
	}
	return err
}
