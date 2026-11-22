package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// Diary books: a workspace's notebooks. They are independent of ledgers, so a
// writer can keep a separate notebook for each part of life.

const diaryBookColumns = "book_id, name, description, sort_order, created_at, updated_at"

// DiaryBooks lists the workspace's diary books in creation order.
func (w *Workspace) DiaryBooks(ctx context.Context) ([]domain.DiaryBook, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+diaryBookColumns+" FROM diary_book ORDER BY sort_order, created_at, rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var books []domain.DiaryBook
	for rows.Next() {
		var book domain.DiaryBook
		if err := rows.Scan(&book.ID, &book.Name, &book.Description, &book.SortOrder,
			&book.CreatedAt, &book.UpdatedAt); err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, rows.Err()
}

// CreateDiaryBook inserts a diary book at the end of the list.
func (w *Workspace) CreateDiaryBook(ctx context.Context, name, description string) (domain.DiaryBook, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.DiaryBook{}, fmt.Errorf("%w：日记本名称不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.DiaryBook{}, err
	}
	var next int
	if err := db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sort_order), -1) + 1 FROM diary_book").Scan(&next); err != nil {
		return domain.DiaryBook{}, err
	}
	now := nowUnix()
	book := domain.DiaryBook{
		ID: newID(), Name: name, Description: strings.TrimSpace(description),
		SortOrder: next, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO diary_book ("+diaryBookColumns+") VALUES (?, ?, ?, ?, ?, ?)",
		book.ID, book.Name, book.Description, book.SortOrder, book.CreatedAt, book.UpdatedAt,
	); err != nil {
		return domain.DiaryBook{}, fmt.Errorf("创建日记本失败：%w", err)
	}
	return book, nil
}

// UpdateDiaryBook rewrites a book's name and description; an empty name is
// refused, an empty description clears it, and a missing book is reported.
func (w *Workspace) UpdateDiaryBook(ctx context.Context, id, name, description string) error {
	id, name = strings.TrimSpace(id), strings.TrimSpace(name)
	if id == "" {
		return fmt.Errorf("%w：日记本 id 不能为空", repository.ErrInvalid)
	}
	if name == "" {
		return fmt.Errorf("%w：日记本名称不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx,
		"UPDATE diary_book SET name = ?, description = ?, updated_at = ? WHERE book_id = ?",
		name, strings.TrimSpace(description), nowUnix(), id)
	if err != nil {
		return fmt.Errorf("重命名日记本失败：%w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("%w：找不到该日记本", repository.ErrNotFound)
	}
	return nil
}

// DeleteDiaryBook removes one book and its entries in the same transaction. The
// last book stays: the diary page always has a book to show, and a workspace
// without one would look as if the deletion had failed.
func (w *Workspace) DeleteDiaryBook(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w：日记本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM diary_book").Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return fmt.Errorf("%w：至少保留一本日记本", repository.ErrInvalid)
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM diary_entry WHERE book_id = ?", id); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, "DELETE FROM diary_book WHERE book_id = ?", id)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w：找不到该日记本", repository.ErrNotFound)
		}
		return nil
	})
}

const diaryColumns = "id, date, content, word_count, mood, created_at, updated_at, book_id"

// DiaryDates returns one book's entry dates, newest first, with the word count
// and mood the tree paints. The content is not read: the list only needs the
// three columns the date tree shows.
func (w *Workspace) DiaryDates(ctx context.Context, bookID string) ([]domain.DiaryDateItem, error) {
	if strings.TrimSpace(bookID) == "" {
		return nil, fmt.Errorf("%w：日记本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT date, word_count, mood FROM diary_entry WHERE book_id = ? ORDER BY date DESC", bookID)
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
func (w *Workspace) DiaryEntry(ctx context.Context, bookID, date string) (domain.DiaryEntry, error) {
	if strings.TrimSpace(bookID) == "" {
		return domain.DiaryEntry{}, fmt.Errorf("%w：日记本 id 不能为空", repository.ErrInvalid)
	}
	if !domain.IsDiaryDate(date) {
		return domain.DiaryEntry{}, fmt.Errorf("%w：日记日期 %q 不是 YYYY-MM-DD", repository.ErrInvalid, date)
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.DiaryEntry{}, err
	}
	row := db.QueryRowContext(ctx,
		"SELECT "+diaryColumns+" FROM diary_entry WHERE book_id = ? AND date = ?", bookID, date)
	entry, err := scanDiaryEntry(row)
	if err == sql.ErrNoRows {
		return domain.DiaryEntry{}, fmt.Errorf("%w：%s 还没有日记", repository.ErrNotFound, date)
	}
	return entry, err
}

// SaveDiary writes one day's entry and returns the row as stored.
//
// Same book and date is an update: the row keeps its id, created_at and
// updated_at (the DAO's ON CONFLICT clause listed only content, word_count
// and mood), and the word count is the text's length in code points. The mood is
// written as the page holds it; a Chinese mood is normalized by
// domain.NormalizeDiaryMood where it is shown, not here.
func (w *Workspace) SaveDiary(ctx context.Context, bookID, date, content, mood string) (domain.DiaryEntry, error) {
	if strings.TrimSpace(bookID) == "" {
		return domain.DiaryEntry{}, fmt.Errorf("%w：日记本 id 不能为空", repository.ErrInvalid)
	}
	if !domain.IsDiaryDate(date) {
		return domain.DiaryEntry{}, fmt.Errorf("%w：日记日期 %q 不是 YYYY-MM-DD", repository.ErrInvalid, date)
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.DiaryEntry{}, err
	}
	now := nowUnix()
	// A day is written into a book that exists: the schema has no foreign keys,
	// so the check lives here rather than in a constraint.
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM diary_book WHERE book_id = ?", bookID).
		Scan(&exists); err != nil {
		return domain.DiaryEntry{}, err
	}
	if exists == 0 {
		return domain.DiaryEntry{}, fmt.Errorf("%w：找不到该日记本", repository.ErrNotFound)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO diary_entry ("+diaryColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT(book_id, date) DO UPDATE SET "+
			"content = excluded.content, word_count = excluded.word_count, mood = excluded.mood",
		newID(), date, content, domain.DiaryWordCount(content), mood, now, now, bookID,
	); err != nil {
		return domain.DiaryEntry{}, err
	}
	row := db.QueryRowContext(ctx,
		"SELECT "+diaryColumns+" FROM diary_entry WHERE book_id = ? AND date = ?", bookID, date)
	return scanDiaryEntry(row)
}

// DeleteDiary removes one day's entry; another book's entry for the same day is
// untouched, and a day without an entry is not an error.
func (w *Workspace) DeleteDiary(ctx context.Context, bookID, date string) error {
	if strings.TrimSpace(bookID) == "" {
		return fmt.Errorf("%w：日记本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "DELETE FROM diary_entry WHERE book_id = ? AND date = ?", bookID, date)
	return err
}

// DiaryEntries returns one book's whole diary, oldest first, for an export.
func (w *Workspace) DiaryEntries(ctx context.Context, bookID string) ([]domain.DiaryEntry, error) {
	if strings.TrimSpace(bookID) == "" {
		return nil, fmt.Errorf("%w：日记本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+diaryColumns+" FROM diary_entry WHERE book_id = ? ORDER BY date ASC", bookID)
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
// the schema, so both come back as an empty string rather than failing.
func scanDiaryEntry(scanner diaryScanner) (domain.DiaryEntry, error) {
	var entry domain.DiaryEntry
	var content, mood sql.NullString
	if err := scanner.Scan(&entry.ID, &entry.Date, &content, &entry.WordCount, &mood,
		&entry.CreatedAt, &entry.UpdatedAt, &entry.BookID); err != nil {
		return domain.DiaryEntry{}, err
	}
	entry.Content = content.String
	entry.Mood = mood.String
	return entry, nil
}
