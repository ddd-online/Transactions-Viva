package store

import (
	"context"
	"errors"
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

// TestDiaryBookIsSeededAndIndependentOfLedgers verifies the single v1 schema:
// a fresh workspace already has the default book, and deleting a ledger must
// not take a diary with it.
func TestDiaryBookIsSeededAndIndependentOfLedgers(t *testing.T) {
	ctx := context.Background()
	workspace, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Close() })

	books, err := workspace.DiaryBooks(ctx)
	if err != nil || len(books) != 1 || books[0].Name != "默认日记本" {
		t.Fatalf("fresh workspace books = %+v, err=%v", books, err)
	}
	bookID := books[0].ID

	ledger, err := workspace.CreateLedger(ctx, "家庭账本", "")
	if err != nil {
		t.Fatalf("CreateLedger: %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, bookID, "2026-01-15", "家里的", "😐"); err != nil {
		t.Fatalf("SaveDiary: %v", err)
	}
	if err := workspace.DeleteLedger(ctx, ledger.ID); err != nil {
		t.Fatalf("DeleteLedger: %v", err)
	}
	entry, err := workspace.DiaryEntry(ctx, bookID, "2026-01-15")
	if err != nil || entry.Content != "家里的" {
		t.Fatalf("entry after deleting the ledger = %+v, err=%v", entry, err)
	}
}
