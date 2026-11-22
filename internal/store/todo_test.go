package store_test

import (
	"context"
	"errors"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
	"transactions/internal/store/storetest"
)

func TestTodoCardsItemsProgressAndHistory(t *testing.T) {
	fixture := storetest.Open(t)
	ctx := context.Background()
	workspace := fixture.Workspace
	card, err := workspace.CreateTodoCard(ctx, " 本周 ")
	if err != nil {
		t.Fatal(err)
	}
	if card.Title != "本周" || card.SortOrder != 1 {
		t.Fatalf("card = %+v", card)
	}
	item, err := workspace.CreateTodoItem(ctx, domain.TodoItem{
		LedgerID: fixture.Home.ID, CardID: card.ID, Title: "写周报",
		StartDate: "2026-01-01", DueDate: "2026-01-02",
		Urgency: 3, Importance: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	progress, err := workspace.AddTodoProgress(ctx, item.ID, "先列大纲")
	if err != nil {
		t.Fatal(err)
	}
	if progress.Done {
		t.Fatal("new progress is done")
	}
	if err := workspace.SetTodoProgressDone(ctx, progress.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.UpdateTodoProgress(ctx, progress.ID, "列完大纲"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SetTodoItemStatus(ctx, item.ID, domain.TodoStatusDone); err != nil {
		t.Fatal(err)
	}
	cards, err := workspace.TodoCards(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || len(cards[0].Items) != 0 {
		t.Fatalf("doing cards = %+v", cards)
	}
	history, err := workspace.TodoHistory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].CardTitle != "本周" || history[0].Item.Title != "写周报" {
		t.Fatalf("history = %+v", history)
	}
	if len(history[0].Item.Progress) != 1 || !history[0].Item.Progress[0].Done ||
		history[0].Item.Progress[0].Content != "列完大纲" {
		t.Fatalf("history progress = %+v", history[0].Item.Progress)
	}
	if err := workspace.SetTodoItemStatus(ctx, item.ID, domain.TodoStatusDoing); err != nil {
		t.Fatal(err)
	}
	cards, err = workspace.TodoCards(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || len(cards[0].Items) != 1 || cards[0].Items[0].CompletedAt != 0 {
		t.Fatalf("reopened cards = %+v", cards)
	}
	if cards[0].LedgerID != "" || cards[0].Items[0].LedgerID != "" {
		t.Fatalf("todo leaked a ledger scope: %+v", cards[0])
	}
}

func TestTodoReorderAndCascade(t *testing.T) {
	fixture := storetest.Open(t)
	ctx := context.Background()
	workspace := fixture.Workspace
	first, _ := workspace.CreateTodoCard(ctx, "一")
	second, _ := workspace.CreateTodoCard(ctx, "二")
	if err := workspace.ReorderTodoCards(ctx, []string{second.ID, first.ID}); err != nil {
		t.Fatal(err)
	}
	cards, err := workspace.TodoCards(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || cards[0].ID != second.ID {
		t.Fatalf("reordered = %+v", cards)
	}
	item, err := workspace.CreateTodoItem(ctx, domain.TodoItem{
		LedgerID: fixture.Home.ID, CardID: first.ID, Title: "事项", Urgency: -1, Importance: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.AddTodoProgress(ctx, item.ID, "进度"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.DeleteTodoCard(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	history, err := workspace.TodoHistory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("history after card delete = %+v", history)
	}
	if _, err := workspace.AddTodoProgress(ctx, item.ID, "x"); err == nil {
		t.Fatal("progress added to a deleted item")
	}
}

func TestTodoValidation(t *testing.T) {
	fixture := storetest.Open(t)
	ctx := context.Background()
	workspace := fixture.Workspace
	card, _ := workspace.CreateTodoCard(ctx, "主题")
	for _, item := range []domain.TodoItem{
		{LedgerID: fixture.Home.ID, CardID: card.ID, Title: " ", Urgency: -1, Importance: -1},
		{LedgerID: fixture.Home.ID, CardID: card.ID, Title: "日期", StartDate: "2026-02-30", Urgency: -1, Importance: -1},
		{LedgerID: fixture.Home.ID, CardID: card.ID, Title: "顺序", StartDate: "2026-02-02", DueDate: "2026-02-01", Urgency: -1, Importance: -1},
		{LedgerID: fixture.Home.ID, CardID: card.ID, Title: "档位", Urgency: 0, Importance: -1},
		{LedgerID: fixture.Home.ID, CardID: card.ID, Title: "档位", Urgency: -1, Importance: 4},
	} {
		if _, err := workspace.CreateTodoItem(ctx, item); err == nil {
			t.Fatalf("accepted invalid item %+v", item)
		}
	}
	if _, err := workspace.CreateTodoItem(ctx, domain.TodoItem{
		LedgerID: fixture.Home.ID, CardID: "missing", Title: "卡片不存在", Urgency: -1, Importance: -1,
	}); err == nil {
		t.Fatal("accepted item under a missing card")
	}
	if err := workspace.SetTodoItemStatus(ctx, "missing", domain.TodoStatusDone); err == nil {
		t.Fatal("accepted missing item status")
	}
	if _, err := workspace.CreateTodoCard(ctx, ""); err == nil {
		t.Fatal("accepted empty card title")
	}
	if err := workspace.DeleteTodoCard(ctx, "missing"); err == nil {
		t.Fatal("deleted missing card")
	}
	if err := workspace.DeleteTodoItem(ctx, "missing"); err == nil {
		t.Fatal("deleted missing item")
	}
	if err := workspace.SetTodoProgressDone(ctx, "missing", true); err == nil {
		t.Fatal("updated missing progress")
	}
	if _, err := workspace.UpdateTodoProgress(ctx, "missing", "x"); err == nil {
		t.Fatal("updated missing progress")
	}
	if err := workspace.DeleteTodoProgress(ctx, "missing"); err == nil {
		t.Fatal("deleted missing progress")
	}
	if _, err := workspace.AddTodoProgress(ctx, "missing", "x"); err == nil {
		t.Fatal("added progress to missing item")
	}
	if err := workspace.ReorderTodoCards(ctx, []string{"missing"}); err == nil {
		t.Fatal("reordered missing card")
	}
}

func TestTodoProgressNotFoundIsNotFound(t *testing.T) {
	fixture := storetest.Open(t)
	if _, err := fixture.Workspace.UpdateTodoProgress(context.Background(), "missing", "x"); err == nil {
		t.Fatal("expected error")
	} else if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}
