package repository

import (
	"context"

	"transactions/internal/domain"
)

// TodoRepository is the workspace data the 待办 section uses. Cards, items and
// progress lines belong to the workspace, not to a ledger: one workspace has
// one todo board.
type TodoRepository interface {
	// TodoCards returns the workspace's cards in sort_order with their doing
	// items and each item's progress in time order.
	TodoCards(ctx context.Context) ([]domain.TodoCard, error)
	// TodoHistory returns completed items newest completion first, with the
	// card title and progress.
	TodoHistory(ctx context.Context) ([]domain.TodoHistory, error)

	CreateTodoCard(ctx context.Context, title string) (domain.TodoCard, error)
	DeleteTodoCard(ctx context.Context, cardID string) error
	// ReorderTodoCards writes sort_order 1..n for the ids in this order.
	ReorderTodoCards(ctx context.Context, cardIDs []string) error

	CreateTodoItem(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error)
	UpdateTodoItem(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error)
	SetTodoItemStatus(ctx context.Context, itemID, status string) error
	DeleteTodoItem(ctx context.Context, itemID string) error

	AddTodoProgress(ctx context.Context, itemID, content string) (domain.TodoProgress, error)
	UpdateTodoProgress(ctx context.Context, progressID, content string) (domain.TodoProgress, error)
	SetTodoProgressDone(ctx context.Context, progressID string, done bool) error
	DeleteTodoProgress(ctx context.Context, progressID string) error
}
