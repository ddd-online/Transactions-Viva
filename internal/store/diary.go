package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

const diaryColumns = "id, date, content, word_count, mood, created_at, updated_at, ledger_id"

// DiaryDates returns one ledger's entry dates, newest first, with the word count
// and mood the tree paints. The content is not read: the list only needs the
// three columns the old page's date tree showed.
func (w *Workspace) DiaryDates(ctx context.Context, ledgerID string) ([]domain.DiaryDateItem, error) {
	if strings.TrimSpace(ledgerID) == "" {
		return nil, fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT date, word_count, mood FROM diary_entry WHERE ledger_id = ? ORDER BY date DESC", ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dates []domain.DiaryDateItem
	for rows.Next() {
		var item domain.DiaryDateItem
		var mood sql.NullString
		if err := rows.Scan(&item.Date, &item.WordCount, &mood); err != nil {
			return nil, err
		}
		item.Mood = mood.String
		dates = append(dates, item)
	}
	return dates, rows.Err()
}

// DiaryEntry returns one day's entry. A day without an entry is ErrNotFound, not
// an empty entry: the page uses that to show the editor empty without pretending
// it read something.
func (w *Workspace) DiaryEntry(ctx context.Context, ledgerID, date string) (domain.DiaryEntry, error) {
	if strings.TrimSpace(ledgerID) == "" {
		return domain.DiaryEntry{}, fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	if !domain.IsDiaryDate(date) {
		return domain.DiaryEntry{}, fmt.Errorf("%w：日记日期 %q 不是 YYYY-MM-DD", repository.ErrInvalid, date)
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.DiaryEntry{}, err
	}
	row := db.QueryRowContext(ctx,
		"SELECT "+diaryColumns+" FROM diary_entry WHERE ledger_id = ? AND date = ?", ledgerID, date)
	entry, err := scanDiaryEntry(row)
	if err == sql.ErrNoRows {
		return domain.DiaryEntry{}, fmt.Errorf("%w：%s 还没有日记", repository.ErrNotFound, date)
	}
	return entry, err
}

// SaveDiary writes one day's entry and returns the row as stored.
//
// Same ledger and date is an update: the row keeps its id, created_at and
// updated_at (the old DAO's ON CONFLICT clause listed only content, word_count
// and mood), and the word count is the text's length in code points. The mood is
// written as the page holds it; a legacy Chinese mood is normalized by
// domain.NormalizeDiaryMood where it is shown, not here.
func (w *Workspace) SaveDiary(ctx context.Context, ledgerID, date, content, mood string) (domain.DiaryEntry, error) {
	if strings.TrimSpace(ledgerID) == "" {
		return domain.DiaryEntry{}, fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	if !domain.IsDiaryDate(date) {
		return domain.DiaryEntry{}, fmt.Errorf("%w：日记日期 %q 不是 YYYY-MM-DD", repository.ErrInvalid, date)
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.DiaryEntry{}, err
	}
	now := nowUnix()
	if _, err := db.ExecContext(ctx,
		"INSERT INTO diary_entry ("+diaryColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT(ledger_id, date) DO UPDATE SET "+
			"content = excluded.content, word_count = excluded.word_count, mood = excluded.mood",
		newID(), date, content, domain.DiaryWordCount(content), mood, now, now, ledgerID,
	); err != nil {
		return domain.DiaryEntry{}, err
	}
	row := db.QueryRowContext(ctx,
		"SELECT "+diaryColumns+" FROM diary_entry WHERE ledger_id = ? AND date = ?", ledgerID, date)
	return scanDiaryEntry(row)
}

// DeleteDiary removes one day's entry; another ledger's entry for the same day
// is untouched, and a day without an entry is not an error.
func (w *Workspace) DeleteDiary(ctx context.Context, ledgerID, date string) error {
	if strings.TrimSpace(ledgerID) == "" {
		return fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "DELETE FROM diary_entry WHERE ledger_id = ? AND date = ?", ledgerID, date)
	return err
}

// DiaryEntries returns one ledger's whole diary, oldest first, for an export.
func (w *Workspace) DiaryEntries(ctx context.Context, ledgerID string) ([]domain.DiaryEntry, error) {
	if strings.TrimSpace(ledgerID) == "" {
		return nil, fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+diaryColumns+" FROM diary_entry WHERE ledger_id = ? ORDER BY date ASC", ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []domain.DiaryEntry
	for rows.Next() {
		entry, err := scanDiaryEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// diaryScanner is what scanDiaryEntry reads: a *sql.Row and a *sql.Rows both
// scan one diary row.
type diaryScanner interface {
	Scan(dest ...any) error
}

// scanDiaryEntry reads one row of diaryColumns. content and mood are nullable in
// the old schema, so both come back as an empty string rather than failing.
func scanDiaryEntry(scanner diaryScanner) (domain.DiaryEntry, error) {
	var entry domain.DiaryEntry
	var content, mood sql.NullString
	if err := scanner.Scan(&entry.ID, &entry.Date, &content, &entry.WordCount, &mood,
		&entry.CreatedAt, &entry.UpdatedAt, &entry.LedgerID); err != nil {
		return domain.DiaryEntry{}, err
	}
	entry.Content = content.String
	entry.Mood = mood.String
	return entry, nil
}
