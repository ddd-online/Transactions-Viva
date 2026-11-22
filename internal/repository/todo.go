package repository

import (
	"context"

	"transactions/internal/domain"
)

// TodoRepository is the workspace data the 待办 section uses. Cards, items and
// progress lines are all scoped by ledger_id, matching the old database.
type TodoRepository interface {
	// TodoCards returns the ledger's cards in sort_order with their doing items
	// and each item's progress in time order.
	TodoCards(ctx context.Context, ledgerID string) ([]domain.TodoCard, error)
	// TodoHistory returns completed items newest completion first, with the
	// card title and progress.
	TodoHistory(ctx context.Context, ledgerID string) ([]domain.TodoHistory, error)

	CreateTodoCard(ctx context.Context, ledgerID, title string) (domain.TodoCard, error)
	DeleteTodoCard(ctx context.Context, ledgerID, cardID string) error
	// ReorderTodoCards writes sort_order 1..n for the ids in this order.
	ReorderTodoCards(ctx context.Context, ledgerID string, cardIDs []string) error

	CreateTodoItem(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error)
	UpdateTodoItem(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error)
	SetTodoItemStatus(ctx context.Context, ledgerID, itemID, status string) error
	DeleteTodoItem(ctx context.Context, ledgerID, itemID string) error

	AddTodoProgress(ctx context.Context, ledgerID, itemID, content string) (domain.TodoProgress, error)
	UpdateTodoProgress(ctx context.Context, ledgerID, progressID, content string) (domain.TodoProgress, error)
	SetTodoProgressDone(ctx context.Context, ledgerID, progressID string, done bool) error
	DeleteTodoProgress(ctx context.Context, ledgerID, progressID string) error
}
