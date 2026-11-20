package store

import (
	"context"
	"errors"
	"testing"

	"transactions/internal/repository"
)

func TestDiarySaveReadAndDelete(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)

	saved, err := workspace.SaveDiary(ctx, ledger.ID, "2026-01-15", "今天很好", "😐")
	if err != nil {
		t.Fatalf("SaveDiary: %v", err)
	}
	if saved.Content != "今天很好" || saved.WordCount != 4 || saved.Mood != "😐" {
		t.Fatalf("saved = %+v", saved)
	}
	if saved.ID == "" || saved.CreatedAt == 0 {
		t.Fatalf("saved entry has no id or timestamp: %+v", saved)
	}

	read, err := workspace.DiaryEntry(ctx, ledger.ID, "2026-01-15")
	if err != nil {
		t.Fatalf("DiaryEntry: %v", err)
	}
	if read.ID != saved.ID || read.WordCount != 4 {
		t.Fatalf("read = %+v, saved = %+v", read, saved)
	}

	// Saving the same day again replaces the text but keeps the row's identity
	// and, as the old DAO did, its timestamps.
	again, err := workspace.SaveDiary(ctx, ledger.ID, "2026-01-15", "今天很好，明天继续", "😊")
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

	dates, err := workspace.DiaryDates(ctx, ledger.ID)
	if err != nil || len(dates) != 1 || dates[0].Date != "2026-01-15" || dates[0].WordCount != 9 {
		t.Fatalf("dates = %+v, err=%v", dates, err)
	}

	if err := workspace.DeleteDiary(ctx, ledger.ID, "2026-01-15"); err != nil {
		t.Fatalf("DeleteDiary: %v", err)
	}
	if _, err := workspace.DiaryEntry(ctx, ledger.ID, "2026-01-15"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("after delete error = %v", err)
	}
	// Deleting a day twice is not an error.
	if err := workspace.DeleteDiary(ctx, ledger.ID, "2026-01-15"); err != nil {
		t.Fatalf("second DeleteDiary: %v", err)
	}
}

func TestDiaryIsPerLedger(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	home := testLedger(t, workspace)
	travel, err := workspace.CreateLedger(ctx, "旅行账本", "")
	if err != nil {
		t.Fatalf("CreateLedger: %v", err)
	}

	if _, err := workspace.SaveDiary(ctx, home.ID, "2026-01-15", "家里的日记", ""); err != nil {
		t.Fatalf("SaveDiary home: %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, travel.ID, "2026-01-15", "路上的日记", ""); err != nil {
		t.Fatalf("SaveDiary travel: %v", err)
	}
	homeEntry, err := workspace.DiaryEntry(ctx, home.ID, "2026-01-15")
	if err != nil || homeEntry.Content != "家里的日记" {
		t.Fatalf("home entry = %+v, err=%v", homeEntry, err)
	}
	travelEntry, err := workspace.DiaryEntry(ctx, travel.ID, "2026-01-15")
	if err != nil || travelEntry.Content != "路上的日记" {
		t.Fatalf("travel entry = %+v, err=%v", travelEntry, err)
	}

	// Deleting one ledger's day leaves the other's alone.
	if err := workspace.DeleteDiary(ctx, home.ID, "2026-01-15"); err != nil {
		t.Fatalf("DeleteDiary: %v", err)
	}
	if _, err := workspace.DiaryEntry(ctx, travel.ID, "2026-01-15"); err != nil {
		t.Fatalf("travel entry after home delete: %v", err)
	}

	dates, err := workspace.DiaryDates(ctx, travel.ID)
	if err != nil || len(dates) != 1 {
		t.Fatalf("travel dates = %+v, err=%v", dates, err)
	}
}

func TestDiaryDatesNewestFirstAndEntriesOldestFirst(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)

	for _, date := range []string{"2026-01-15", "2025-12-31", "2026-02-01"} {
		if _, err := workspace.SaveDiary(ctx, ledger.ID, date, date, ""); err != nil {
			t.Fatalf("SaveDiary %s: %v", date, err)
		}
	}
	dates, err := workspace.DiaryDates(ctx, ledger.ID)
	if err != nil {
		t.Fatalf("DiaryDates: %v", err)
	}
	want := []string{"2026-02-01", "2026-01-15", "2025-12-31"}
	if len(dates) != len(want) {
		t.Fatalf("dates = %+v", dates)
	}
	for index, date := range want {
		if dates[index].Date != date {
			t.Fatalf("dates[%d] = %q, want %q", index, dates[index].Date, date)
		}
	}

	entries, err := workspace.DiaryEntries(ctx, ledger.ID)
	if err != nil {
		t.Fatalf("DiaryEntries: %v", err)
	}
	if len(entries) != 3 || entries[0].Date != "2025-12-31" || entries[2].Date != "2026-02-01" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestDiaryRejectsBadRequests(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)

	if _, err := workspace.SaveDiary(ctx, "", "2026-01-15", "x", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty ledger error = %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, ledger.ID, "2026-1-15", "x", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("bad date error = %v", err)
	}
	if _, err := workspace.SaveDiary(ctx, ledger.ID, "2026-02-29", "x", ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("leap day error = %v", err)
	}
	if _, err := workspace.DiaryDates(ctx, ""); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty ledger list error = %v", err)
	}
}
