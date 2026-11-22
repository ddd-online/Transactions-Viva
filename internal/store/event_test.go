package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func TestKeyEventUpsertAndYearListing(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()

	first, err := workspace.SaveKeyEvent(ctx, "2026-01-15", "  年会  ", "抽奖与聚餐\n再一行", " #D9705A ")
	if err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	if first.Title != "年会" || first.Content != "抽奖与聚餐\n再一行" || first.Color != "#D9705A" {
		t.Fatalf("event = %+v", first)
	}
	if first.CreatedAt == 0 || first.UpdatedAt == 0 {
		t.Fatalf("timestamps = %+v", first)
	}

	// A second save of the same day updates the row instead of adding one.
	second, err := workspace.SaveKeyEvent(ctx, "2026-01-15", "", "\n  改过的正文  ", "")
	if err != nil {
		t.Fatalf("SaveKeyEvent twice: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("id changed from %s to %s", first.ID, second.ID)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("created_at changed from %d to %d", first.CreatedAt, second.CreatedAt)
	}
	if second.Title != "改过的正文" {
		t.Fatalf("title fallback = %q", second.Title)
	}
	if second.UpdatedAt < first.UpdatedAt {
		t.Fatalf("updated_at went backwards: %d then %d", first.UpdatedAt, second.UpdatedAt)
	}

	if _, err := workspace.SaveKeyEvent(ctx, "2026-03-02", "年会总结", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	if _, err := workspace.SaveKeyEvent(ctx, "2025-12-31", "去年", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	events, err := workspace.KeyEventsByYear(ctx, "2026")
	if err != nil {
		t.Fatalf("KeyEventsByYear: %v", err)
	}
	if len(events) != 2 || events[0].Date != "2026-03-02" || events[1].Date != "2026-01-15" {
		t.Fatalf("events = %+v", events)
	}

	if _, err := workspace.KeyEvent(ctx, "2024-01-01"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("missing event error = %v", err)
	}
	if _, err := workspace.KeyEventsByYear(ctx, "26"); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("bad year error = %v", err)
	}
}

func TestKeyEventTitleIsCutToTwoHundredCodePoints(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	long := strings.Repeat("会", 240)
	event, err := workspace.SaveKeyEvent(ctx, "2026-05-01", long, "", "")
	if err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	if runes := []rune(event.Title); len(runes) != domain.KeyEventTitleLimit {
		t.Fatalf("stored title keeps %d code points", len(runes))
	}
}

func TestKeyEventImagesKeepTheirOrderPerDay(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()

	first, err := workspace.CreateKeyEventImage(ctx, domain.KeyEventImage{
		EventDate: "2026-01-15", FilePath: "key_events/2026-01-15/a.png", ThumbPath: "key_events/2026-01-15/thumb_a.jpg",
	})
	if err != nil {
		t.Fatalf("CreateKeyEventImage: %v", err)
	}
	second, err := workspace.CreateKeyEventImage(ctx, domain.KeyEventImage{
		EventDate: "2026-01-15", FilePath: "key_events/2026-01-15/b.png",
	})
	if err != nil {
		t.Fatalf("CreateKeyEventImage: %v", err)
	}
	other, err := workspace.CreateKeyEventImage(ctx, domain.KeyEventImage{
		EventDate: "2026-01-16", FilePath: "key_events/2026-01-16/c.png",
	})
	if err != nil {
		t.Fatalf("CreateKeyEventImage: %v", err)
	}
	if first.ID == "" || second.ID == "" || first.SortOrder != 1 || second.SortOrder != 2 || other.SortOrder != 1 {
		t.Fatalf("images = %+v / %+v / %+v", first, second, other)
	}

	images, err := workspace.KeyEventImages(ctx, "2026-01-15")
	if err != nil || len(images) != 2 || images[0].ID != first.ID || images[1].ID != second.ID {
		t.Fatalf("images = %+v, err=%v", images, err)
	}

	removed, err := workspace.DeleteKeyEventImage(ctx, second.ID)
	if err != nil {
		t.Fatalf("DeleteKeyEventImage: %v", err)
	}
	if removed.ID != second.ID || removed.FilePath != "key_events/2026-01-15/b.png" {
		t.Fatalf("removed = %+v", removed)
	}
	// A repeated delete is the same request, not an error.
	if again, err := workspace.DeleteKeyEventImage(ctx, second.ID); err != nil || again.ID != "" {
		t.Fatalf("repeated delete = %+v, err=%v", again, err)
	}
	images, err = workspace.KeyEventImages(ctx, "2026-01-15")
	if err != nil || len(images) != 1 {
		t.Fatalf("images after delete = %+v, err=%v", images, err)
	}
}

func TestDeleteKeyEventRemovesImagesAndReturnsThem(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	if _, err := workspace.SaveKeyEvent(ctx, "2026-02-02", "聚会", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		if _, err := workspace.CreateKeyEventImage(ctx, domain.KeyEventImage{
			EventDate: "2026-02-02", FilePath: "key_events/2026-02-02/" + name + ".jpg",
		}); err != nil {
			t.Fatalf("CreateKeyEventImage: %v", err)
		}
	}
	images, err := workspace.DeleteKeyEvent(ctx, "2026-02-02")
	if err != nil {
		t.Fatalf("DeleteKeyEvent: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("returned images = %+v", images)
	}
	if _, err := workspace.KeyEvent(ctx, "2026-02-02"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("event after delete error = %v", err)
	}
	if left, err := workspace.KeyEventImages(ctx, "2026-02-02"); err != nil || len(left) != 0 {
		t.Fatalf("images after delete = %+v, err=%v", left, err)
	}
}

func TestDeletingALedgerKeepsWorkspaceEvents(t *testing.T) {
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)
	ctx := context.Background()
	if _, err := workspace.SaveKeyEvent(ctx, "2026-04-04", "工作空间事件", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	if _, err := workspace.CreateKeyEventImage(ctx, domain.KeyEventImage{
		EventDate: "2026-04-04", FilePath: "key_events/2026-04-04/a.jpg",
	}); err != nil {
		t.Fatalf("CreateKeyEventImage: %v", err)
	}
	if err := workspace.DeleteLedger(ctx, ledger.ID); err != nil {
		t.Fatalf("DeleteLedger: %v", err)
	}
	if _, err := workspace.KeyEvent(ctx, "2026-04-04"); err != nil {
		t.Fatalf("event did not survive its ledger: %v", err)
	}
	if images, err := workspace.KeyEventImages(ctx, "2026-04-04"); err != nil || len(images) != 1 {
		t.Fatalf("images after ledger delete = %+v, err=%v", images, err)
	}
}

func TestKeyEventRecordsSpanLedgers(t *testing.T) {
	workspace := testWorkspace(t)
	home := testLedger(t, workspace)
	travel, err := workspace.CreateLedger(context.Background(), "旅行账本", "")
	if err != nil {
		t.Fatalf("CreateLedger: %v", err)
	}
	ctx := context.Background()
	later := time.Date(2026, time.January, 15, 20, 0, 0, 0, time.Local).Unix()
	earlier := time.Date(2026, time.January, 15, 9, 0, 0, 0, time.Local).Unix()
	rows := []domain.TransactionRecord{
		{TransactionID: "home-1", LedgerID: home.ID, AmountCents: 3200, TransactionType: "expense",
			Category: "餐饮美食", Description: "午餐", KeyEventDate: "2026-01-15", TransactionAt: earlier},
		{TransactionID: "travel-1", LedgerID: travel.ID, AmountCents: 128000, TransactionType: "expense",
			Category: "交通出行", Description: "去程机票", KeyEventDate: "2026-01-15", TransactionAt: later},
		{TransactionID: "unrelated", LedgerID: home.ID, AmountCents: 100, TransactionType: "expense",
			Category: "餐饮美食", KeyEventDate: "2026-01-16", TransactionAt: later},
	}
	for _, record := range rows {
		if _, err := workspace.CreateRecord(ctx, record, []string{"日常"}); err != nil {
			t.Fatalf("CreateRecord: %v", err)
		}
	}
	linked, err := workspace.KeyEventRecords(ctx, "2026-01-15")
	if err != nil {
		t.Fatalf("KeyEventRecords: %v", err)
	}
	if len(linked) != 2 || linked[0].Record.TransactionID != "travel-1" || linked[1].Record.TransactionID != "home-1" {
		t.Fatalf("linked = %+v", linked)
	}
	if linked[0].Record.LedgerID != travel.ID || len(linked[0].Tags) != 1 || linked[0].Tags[0] != "日常" {
		t.Fatalf("linked record = %+v", linked[0])
	}
}
