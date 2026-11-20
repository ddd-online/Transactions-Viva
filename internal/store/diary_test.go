package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"transactions/internal/repository"
)

// testDiaryBook creates one diary book for the tests that write entries.
func testDiaryBook(t *testing.T, workspace *Workspace) string {
	t.Helper()
	book, err := workspace.CreateDiaryBook(context.Background(), "家庭日记", "")
	if err != nil {
		t.Fatalf("CreateDiaryBook: %v", err)
	}
	return book.ID
}

func TestDiarySaveReadAndDelete(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	bookID := testDiaryBook(t, workspace)

	saved, err := workspace.SaveDiary(ctx, bookID, "2026-01-15", "今天很好", "😐")
	if err != nil {
		t.Fatalf("SaveDiary: %v", err)
	}
	if saved.Content != "今天很好" || saved.WordCount != 4 || saved.Mood != "😐" {
		t.Fatalf("saved = %+v", saved)
	}
	if saved.ID == "" || saved.CreatedAt == 0 || saved.BookID != bookID {
		t.Fatalf("saved entry has no id, timestamp or book: %+v", saved)
	}

	read, err := workspace.DiaryEntry(ctx, bookID, "2026-01-15")
	if err != nil {
		t.Fatalf("DiaryEntry: %v", err)
	}
	if read.ID != saved.ID || read.WordCount != 4 {
		t.Fatalf("read = %+v, saved = %+v", read, saved)
	}

	// Saving the same day again replaces the text but keeps the row's identity
	// and, as the old DAO did, its timestamps.
	again, err := workspace.SaveDiary(ctx, bookID, "2026-01-15", "今天很好，明天继续", "😊")
	if err != nil {
		t.Fatalf("SaveDiary again: %v", err)
	}
	if again.ID != saved.ID {
		t.Fatalf("id changed on update: %q then %q", saved.ID, again.ID)
	}
	if again.WordCount != 9 || again.Mood != "😊" {
		t.Fatalf("updated = %+v", again)
	}
	if again.CreatedAt != saved.CreatedAt || again.UpdatedAt != saved.UpdatedAt {
		t.Fatalf("timestamps changed on update: %+v then %+v", saved, again)
	}

	dates, err := workspace.DiaryDates(ctx, bookID)
	if err != nil || len(dates) != 1 || dates[0].Date != "2026-01-15" || dates[0].WordCount != 9 {
		t.Fatalf("dates = %+v, err=%v", dates, err)
	}

	if err := workspace.DeleteDiary(ctx, bookID, "2026-01-15"); err != nil {
		t.Fatalf("DeleteDiary: %v", err)
	}
	if _, err := workspace.DiaryEntry(ctx, bookID, "2026-01-15"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("after delete error = %v", err)
	}
	// Deleting a day twice is not an error.
	if err := workspace.DeleteDiary(ctx, bookID, "2026-01-15"); err != nil {
		t.Fatalf("second DeleteDiary: %v", err)
	}
}

func TestDiaryIsPerBookNotPerLedger(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	// Two ledgers prove nothing here: the diaries hang off books now.
	if _, err := workspace.CreateLedger(ctx, "旅行账本", ""); err != nil {
		t.Fatalf("CreateLedger: %v", err)
	}
	home := testDiaryBook(t, workspace)
	travel, err := workspace.CreateDiaryBook(ctx, "旅行日记", "出差与旅行")
	if err != nil {
		t.Fatalf("CreateDiaryBook: %v", err)
	}

	if _, err := workspace.SaveDiary(ctx, home, "2026-01-15", "家里的日记", ""); err != nil {
		t.Fatalf("SaveDiary home: %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, travel.ID, "2026-01-15", "路上的日记", ""); err != nil {
		t.Fatalf("SaveDiary travel: %v", err)
	}
	homeEntry, err := workspace.DiaryEntry(ctx, home, "2026-01-15")
	if err != nil || homeEntry.Content != "家里的日记" {
		t.Fatalf("home entry = %+v, err=%v", homeEntry, err)
	}
	travelEntry, err := workspace.DiaryEntry(ctx, travel.ID, "2026-01-15")
	if err != nil || travelEntry.Content != "路上的日记" {
		t.Fatalf("travel entry = %+v, err=%v", travelEntry, err)
	}

	// Deleting one book's day leaves the other's alone.
	if err := workspace.DeleteDiary(ctx, home, "2026-01-15"); err != nil {
		t.Fatalf("DeleteDiary: %v", err)
	}
	if _, err := workspace.DiaryEntry(ctx, travel.ID, "2026-01-15"); err != nil {
		t.Fatalf("travel entry after home delete: %v", err)
	}
}

func TestDiaryBookCommands(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)

	if _, err := workspace.CreateDiaryBook(ctx, "  ", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty name error = %v", err)
	}
	// A fresh workspace already carries the 默认日记本 the v4 migration seeds.
	first, err := workspace.CreateDiaryBook(ctx, "第一本", "说明")
	if err != nil {
		t.Fatalf("CreateDiaryBook: %v", err)
	}
	second, err := workspace.CreateDiaryBook(ctx, "第二本", "")
	if err != nil {
		t.Fatalf("CreateDiaryBook second: %v", err)
	}
	books, err := workspace.DiaryBooks(ctx)
	if err != nil || len(books) != 3 {
		t.Fatalf("books = %+v, err=%v", books, err)
	}
	if books[1].ID != first.ID || books[2].ID != second.ID {
		t.Fatalf("books are not in creation order: %+v", books)
	}
	if books[1].Description != "说明" {
		t.Fatalf("description = %q", books[1].Description)
	}

	if err := workspace.UpdateDiaryBook(ctx, first.ID, "改名", "新说明"); err != nil {
		t.Fatalf("UpdateDiaryBook: %v", err)
	}
	books, err = workspace.DiaryBooks(ctx)
	if err != nil || books[1].Name != "改名" || books[1].Description != "新说明" {
		t.Fatalf("after update = %+v, err=%v", books, err)
	}
	if err := workspace.UpdateDiaryBook(ctx, first.ID, "", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty rename error = %v", err)
	}
	if err := workspace.UpdateDiaryBook(ctx, "missing", "x", ""); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("missing rename error = %v", err)
	}

	// The last book stays, as the last ledger does.
	if err := workspace.DeleteDiaryBook(ctx, second.ID); err != nil {
		t.Fatalf("DeleteDiaryBook: %v", err)
	}
	if err := workspace.DeleteDiaryBook(ctx, first.ID); err != nil {
		t.Fatalf("DeleteDiaryBook second: %v", err)
	}
	if err := workspace.DeleteDiaryBook(ctx, books[0].ID); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("deleting the last book error = %v", err)
	}
}

func TestDeleteDiaryBookRemovesItsEntriesOnly(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	keep := testDiaryBook(t, workspace)
	drop, err := workspace.CreateDiaryBook(ctx, "要删的日记本", "")
	if err != nil {
		t.Fatalf("CreateDiaryBook: %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, keep, "2026-01-15", "留下", ""); err != nil {
		t.Fatalf("SaveDiary keep: %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, drop.ID, "2026-01-15", "删掉", ""); err != nil {
		t.Fatalf("SaveDiary drop: %v", err)
	}
	if err := workspace.DeleteDiaryBook(ctx, drop.ID); err != nil {
		t.Fatalf("DeleteDiaryBook: %v", err)
	}
	if _, err := workspace.DiaryEntry(ctx, keep, "2026-01-15"); err != nil {
		t.Fatalf("the kept book lost its entry: %v", err)
	}
	dates, err := workspace.DiaryDates(ctx, drop.ID)
	if err != nil {
		t.Fatalf("DiaryDates of the deleted book: %v", err)
	}
	if len(dates) != 0 {
		t.Fatalf("the deleted book still has dates: %+v", dates)
	}
}

func TestDiaryRejectsBadRequests(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	bookID := testDiaryBook(t, workspace)

	if _, err := workspace.SaveDiary(ctx, "", "2026-01-15", "x", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty book error = %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, bookID, "2026-1-15", "x", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("bad date error = %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, bookID, "2026-02-29", "x", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("leap day error = %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, "missing", "2026-01-15", "x", ""); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("missing book error = %v", err)
	}
	if _, err := workspace.DiaryDates(ctx, ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty book list error = %v", err)
	}
}

// TestDiaryBookMigrationKeepsEachLedgersDiary upgrades a v3 workspace, where a
// diary belonged to a ledger, and checks that every ledger's diary became its
// own book with its entries attached.
func TestDiaryBookMigrationKeepsEachLedgersDiary(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	path := databasePathOf(directory)

	// Build a v3 workspace by hand: the first three migrations and nothing else.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := applyPragmas(db); err != nil {
		t.Fatalf("pragmas: %v", err)
	}
	if err := applyMigrations(db, path, migrations[:3], 0); err != nil {
		t.Fatalf("apply v3: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO ledger (id, name, description, created_at, updated_at) VALUES ('l-home', '家庭账本', '', 1, 1), ('l-travel', '旅行账本', '', 1, 1)"); err != nil {
		t.Fatalf("insert ledgers: %v", err)
	}
	if _, err := db.Exec(
		"INSERT INTO diary_entry (id, date, content, word_count, mood, created_at, updated_at, ledger_id) " +
			"VALUES ('e1', '2026-01-15', '家里的', 3, '😐', 1, 1, 'l-home'), " +
			"('e2', '2026-01-16', '路上的', 3, '😊', 1, 1, 'l-travel')"); err != nil {
		t.Fatalf("insert entries: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	workspace, err := Open(directory)
	if err != nil {
		t.Fatalf("Open after v3: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Close() })
	if version, _, err := schemaVersion(workspace.db); err != nil || version != targetVersion() {
		t.Fatalf("version = %d, err=%v", version, err)
	}

	books, err := workspace.DiaryBooks(ctx)
	if err != nil {
		t.Fatalf("DiaryBooks: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("books = %+v, want one per ledger with entries", books)
	}
	names := map[string]string{}
	for _, book := range books {
		names[book.ID] = book.Name
	}
	if names["book-l-home"] != "家庭账本" || names["book-l-travel"] != "旅行账本" {
		t.Fatalf("book names = %v", names)
	}

	home, err := workspace.DiaryEntry(ctx, "book-l-home", "2026-01-15")
	if err != nil || home.Content != "家里的" {
		t.Fatalf("home entry = %+v, err=%v", home, err)
	}
	travel, err := workspace.DiaryEntry(ctx, "book-l-travel", "2026-01-16")
	if err != nil || travel.Content != "路上的" || travel.Mood != "😊" {
		t.Fatalf("travel entry = %+v, err=%v", travel, err)
	}

	// The ledger column is gone: deleting a ledger can no longer delete a diary.
	if _, err := workspace.db.Exec("DELETE FROM ledger WHERE id = 'l-home'"); err != nil {
		t.Fatalf("delete ledger: %v", err)
	}
	if _, err := workspace.DiaryEntry(ctx, "book-l-home", "2026-01-15"); err != nil {
		t.Fatalf("the diary went with the ledger: %v", err)
	}

	// A workspace with no diaries at all gets one 默认日记本 instead.
	emptyDir := t.TempDir()
	empty, err := Open(emptyDir)
	if err != nil {
		t.Fatalf("Open empty: %v", err)
	}
	t.Cleanup(func() { _ = empty.Close() })
	emptyBooks, err := empty.DiaryBooks(ctx)
	if err != nil || len(emptyBooks) != 1 || emptyBooks[0].Name != "默认日记本" {
		t.Fatalf("empty workspace books = %+v, err=%v", emptyBooks, err)
	}
	if _, err := os.Stat(filepath.Join(emptyDir, "transactions.db")); err != nil {
		t.Fatalf("workspace database: %v", err)
	}
}
