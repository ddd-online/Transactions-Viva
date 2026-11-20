package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"transactions/internal/repository"
)

// newID returns a RFC 4122 version 4 UUID string, the id shape the old
// application used for ledgers, records and templates.
func newID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		// crypto/rand failing is fatal for data integrity; fall back to a
		// timestamp-based value rather than returning an empty id.
		now := time.Now().UnixNano()
		for index := range bytes {
			bytes[index] = byte(now >> (8 * (index % 8)))
		}
	}
	bytes[6] = bytes[6]&0x0f | 0x40
	bytes[8] = bytes[8]&0x3f | 0x80
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], bytes[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], bytes[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], bytes[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], bytes[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], bytes[10:16])
	return string(encoded)
}

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
