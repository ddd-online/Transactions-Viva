package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

var _ repository.TodoRepository = (*Workspace)(nil)

const todoCardColumns = "id, ledger_id, title, created_at, updated_at, sort_order"
const todoItemColumns = "id, ledger_id, card_id, title, start_date, due_date, urgency, importance, status, completed_at, created_at, updated_at"
const todoProgressColumns = "id, ledger_id, item_id, content, created_at, done"

// TodoCards lists the ledger's themes with their doing items and progress.
func (w *Workspace) TodoCards(ctx context.Context, ledgerID string) ([]domain.TodoCard, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	progress, err := todoProgressByItem(ctx, db, ledgerID)
	if err != nil {
		return nil, err
	}
	cards, err := todoCards(ctx, db, ledgerID)
	if err != nil {
		return nil, err
	}
	items, err := todoItemsByStatus(ctx, db, ledgerID, domain.TodoStatusDoing)
	if err != nil {
		return nil, err
	}
	byCard := map[string][]domain.TodoItem{}
	for _, item := range items {
		item.Progress = progress[item.ID]
		byCard[item.CardID] = append(byCard[item.CardID], item)
	}
	for index := range cards {
		cards[index].Items = byCard[cards[index].ID]
	}
	return cards, nil
}

func todoCards(ctx context.Context, db *sql.DB, ledgerID string) ([]domain.TodoCard, error) {
	rows, err := db.QueryContext(ctx, "SELECT "+todoCardColumns+" FROM todo_card WHERE ledger_id = ? "+
		"ORDER BY sort_order ASC, created_at ASC, id ASC", ledgerID)
	if err != nil {
		return nil, fmt.Errorf("读取待办主题失败：%w", err)
	}
	defer rows.Close()
	var cards []domain.TodoCard
	for rows.Next() {
		card, err := scanTodoCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, rows.Err()
}

func scanTodoCard(rows interface{ Scan(...any) error }) (domain.TodoCard, error) {
	var card domain.TodoCard
	err := rows.Scan(&card.ID, &card.LedgerID, &card.Title, &card.CreatedAt, &card.UpdatedAt, &card.SortOrder)
	return card, err
}

func todoItemsByStatus(ctx context.Context, db *sql.DB, ledgerID, status string) ([]domain.TodoItem, error) {
	query := "SELECT " + todoItemColumns + " FROM todo_item WHERE ledger_id = ? AND status = ? "
	if status == domain.TodoStatusDone {
		query += "ORDER BY completed_at DESC, created_at DESC, id ASC"
	} else {
		query += "ORDER BY created_at ASC, id ASC"
	}
	rows, err := db.QueryContext(ctx, query, ledgerID, status)
	if err != nil {
		return nil, fmt.Errorf("读取待办事项失败：%w", err)
	}
	defer rows.Close()
	var items []domain.TodoItem
	for rows.Next() {
		item, err := scanTodoItem(rows)
		if err != nil {
			return nil, err
		}
		item.Urgency = domain.SnapTodoLevel(item.Urgency)
		item.Importance = domain.SnapTodoLevel(item.Importance)
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanTodoItem(rows interface{ Scan(...any) error }) (domain.TodoItem, error) {
	var item domain.TodoItem
	err := rows.Scan(&item.ID, &item.LedgerID, &item.CardID, &item.Title, &item.StartDate,
		&item.DueDate, &item.Urgency, &item.Importance, &item.Status, &item.CompletedAt,
		&item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func todoProgressByItem(ctx context.Context, db *sql.DB, ledgerID string) (map[string][]domain.TodoProgress, error) {
	rows, err := db.QueryContext(ctx, "SELECT "+todoProgressColumns+" FROM todo_progress WHERE ledger_id = ? "+
		"ORDER BY created_at ASC, id ASC", ledgerID)
	if err != nil {
		return nil, fmt.Errorf("读取待办进度失败：%w", err)
	}
	defer rows.Close()
	byItem := map[string][]domain.TodoProgress{}
	for rows.Next() {
		progress, err := scanTodoProgress(rows)
		if err != nil {
			return nil, err
		}
		byItem[progress.ItemID] = append(byItem[progress.ItemID], progress)
	}
	return byItem, rows.Err()
}

func scanTodoProgress(rows interface{ Scan(...any) error }) (domain.TodoProgress, error) {
	var progress domain.TodoProgress
	err := rows.Scan(&progress.ID, &progress.LedgerID, &progress.ItemID, &progress.Content,
		&progress.CreatedAt, &progress.Done)
	return progress, err
}

// TodoHistory lists completed items newest first, with their card titles.
func (w *Workspace) TodoHistory(ctx context.Context, ledgerID string) ([]domain.TodoHistory, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	cards, err := todoCards(ctx, db, ledgerID)
	if err != nil {
		return nil, err
	}
	titles := map[string]string{}
	for _, card := range cards {
		titles[card.ID] = card.Title
	}
	progress, err := todoProgressByItem(ctx, db, ledgerID)
	if err != nil {
		return nil, err
	}
	items, err := todoItemsByStatus(ctx, db, ledgerID, domain.TodoStatusDone)
	if err != nil {
		return nil, err
	}
	history := make([]domain.TodoHistory, 0, len(items))
	for _, item := range items {
		item.Progress = progress[item.ID]
		history = append(history, domain.TodoHistory{Item: item, CardTitle: titles[item.CardID]})
	}
	return history, nil
}

// CreateTodoCard appends a theme at the end of the ledger's board.
func (w *Workspace) CreateTodoCard(ctx context.Context, ledgerID, title string) (domain.TodoCard, error) {
	title, err := todoText(title, domain.TodoCardTitleMax, "请输入卡片主题")
	if err != nil {
		return domain.TodoCard{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.TodoCard{}, err
	}
	var next int
	if err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), 0) + 1 FROM todo_card WHERE ledger_id = ?",
		ledgerID).Scan(&next); err != nil {
		return domain.TodoCard{}, fmt.Errorf("读取待办排序失败：%w", err)
	}
	now := nowUnix()
	card := domain.TodoCard{
		ID: newID(), LedgerID: ledgerID, Title: title, CreatedAt: now, UpdatedAt: now, SortOrder: next,
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO todo_card ("+todoCardColumns+") VALUES (?, ?, ?, ?, ?, ?)",
		card.ID, card.LedgerID, card.Title, card.CreatedAt, card.UpdatedAt, card.SortOrder); err != nil {
		return domain.TodoCard{}, fmt.Errorf("创建待办主题失败：%w", err)
	}
	return card, nil
}

// DeleteTodoCard removes the card's progress, items and the card itself.
func (w *Workspace) DeleteTodoCard(ctx context.Context, ledgerID, cardID string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM todo_card WHERE id = ? AND ledger_id = ?",
			cardID, ledgerID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("%w：卡片不存在", repository.ErrNotFound)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM todo_progress WHERE item_id IN "+
			"(SELECT id FROM todo_item WHERE card_id = ? AND ledger_id = ?)", cardID, ledgerID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM todo_item WHERE card_id = ? AND ledger_id = ?", cardID, ledgerID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM todo_card WHERE id = ? AND ledger_id = ?", cardID, ledgerID)
		return err
	})
}

// ReorderTodoCards writes sort_order 1..n for the given cards.
func (w *Workspace) ReorderTodoCards(ctx context.Context, ledgerID string, cardIDs []string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		for index, cardID := range cardIDs {
			result, err := tx.ExecContext(ctx, "UPDATE todo_card SET sort_order = ?, updated_at = ? WHERE id = ? AND ledger_id = ?",
				index+1, nowUnix(), cardID, ledgerID)
			if err != nil {
				return err
			}
			if affected, err := result.RowsAffected(); err != nil {
				return err
			} else if affected == 0 {
				return fmt.Errorf("%w：卡片不存在", repository.ErrNotFound)
			}
		}
		return nil
	})
}

// CreateTodoItem adds a doing item under an existing card.
func (w *Workspace) CreateTodoItem(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error) {
	item, err := normalizeTodoItem(item)
	if err != nil {
		return domain.TodoItem{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.TodoItem{}, err
	}
	var cardExists int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM todo_card WHERE id = ? AND ledger_id = ?",
		item.CardID, item.LedgerID).Scan(&cardExists); err != nil {
		return domain.TodoItem{}, err
	}
	if cardExists == 0 {
		return domain.TodoItem{}, fmt.Errorf("%w：卡片不存在", repository.ErrNotFound)
	}
	now := nowUnix()
	item.ID = newID()
	item.Status = domain.TodoStatusDoing
	item.CompletedAt = 0
	item.CreatedAt = now
	item.UpdatedAt = now
	if _, err := db.ExecContext(ctx, "INSERT INTO todo_item ("+todoItemColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		item.ID, item.LedgerID, item.CardID, item.Title, item.StartDate, item.DueDate,
		item.Urgency, item.Importance, item.Status, item.CompletedAt, item.CreatedAt, item.UpdatedAt); err != nil {
		return domain.TodoItem{}, fmt.Errorf("创建待办事项失败：%w", err)
	}
	item.Progress = nil
	return item, nil
}

// UpdateTodoItem rewrites the editable fields and leaves status / completion
// untouched.
func (w *Workspace) UpdateTodoItem(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error) {
	item, err := normalizeTodoItem(item)
	if err != nil {
		return domain.TodoItem{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.TodoItem{}, err
	}
	existing, err := todoItem(ctx, db, item.LedgerID, item.ID)
	if err != nil {
		return domain.TodoItem{}, err
	}
	item.CardID = existing.CardID
	item.Status = existing.Status
	item.CompletedAt = existing.CompletedAt
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = nowUnix()
	if _, err := db.ExecContext(ctx, "UPDATE todo_item SET title = ?, start_date = ?, due_date = ?, urgency = ?, "+
		"importance = ?, updated_at = ? WHERE id = ? AND ledger_id = ?",
		item.Title, item.StartDate, item.DueDate, item.Urgency, item.Importance, item.UpdatedAt,
		item.ID, item.LedgerID); err != nil {
		return domain.TodoItem{}, fmt.Errorf("保存待办事项失败：%w", err)
	}
	progress, err := todoProgressByItem(ctx, db, item.LedgerID)
	if err != nil {
		return domain.TodoItem{}, err
	}
	item.Progress = progress[item.ID]
	return item, nil
}

// SetTodoItemStatus moves an item between doing and done.
func (w *Workspace) SetTodoItemStatus(ctx context.Context, ledgerID, itemID, status string) error {
	if status != domain.TodoStatusDoing && status != domain.TodoStatusDone {
		return fmt.Errorf("%w：待办状态只能是进行中或已完成", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	item, err := todoItem(ctx, db, ledgerID, itemID)
	if err != nil {
		return err
	}
	completedAt := int64(0)
	if status == domain.TodoStatusDone {
		completedAt = nowUnix()
	}
	_, err = db.ExecContext(ctx, "UPDATE todo_item SET status = ?, completed_at = ?, updated_at = ? "+
		"WHERE id = ? AND ledger_id = ?", status, completedAt, nowUnix(), item.ID, ledgerID)
	if err != nil {
		return fmt.Errorf("更新待办状态失败：%w", err)
	}
	return nil
}

// DeleteTodoItem removes an item and its progress.
func (w *Workspace) DeleteTodoItem(ctx context.Context, ledgerID, itemID string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM todo_item WHERE id = ? AND ledger_id = ?",
			itemID, ledgerID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("%w：事项不存在", repository.ErrNotFound)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM todo_progress WHERE item_id = ? AND ledger_id = ?", itemID, ledgerID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM todo_item WHERE id = ? AND ledger_id = ?", itemID, ledgerID)
		return err
	})
}

// AddTodoProgress appends one log line under an existing item.
func (w *Workspace) AddTodoProgress(ctx context.Context, ledgerID, itemID, content string) (domain.TodoProgress, error) {
	content, err := todoText(content, domain.TodoProgressMax, "请输入进度")
	if err != nil {
		return domain.TodoProgress{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.TodoProgress{}, err
	}
	if _, err := todoItem(ctx, db, ledgerID, itemID); err != nil {
		return domain.TodoProgress{}, err
	}
	progress := domain.TodoProgress{ID: newID(), LedgerID: ledgerID, ItemID: itemID, Content: content, CreatedAt: nowUnix()}
	if _, err := db.ExecContext(ctx, "INSERT INTO todo_progress ("+todoProgressColumns+") VALUES (?, ?, ?, ?, ?, ?)",
		progress.ID, progress.LedgerID, progress.ItemID, progress.Content, progress.CreatedAt, progress.Done); err != nil {
		return domain.TodoProgress{}, fmt.Errorf("写入待办进度失败：%w", err)
	}
	return progress, nil
}

// UpdateTodoProgress corrects a progress line's words without touching its
// time or checked state.
func (w *Workspace) UpdateTodoProgress(ctx context.Context, ledgerID, progressID, content string) (domain.TodoProgress, error) {
	content, err := todoText(content, domain.TodoProgressMax, "请输入进度")
	if err != nil {
		return domain.TodoProgress{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.TodoProgress{}, err
	}
	progress, err := todoProgress(ctx, db, ledgerID, progressID)
	if err != nil {
		return domain.TodoProgress{}, err
	}
	progress.Content = content
	if _, err := db.ExecContext(ctx, "UPDATE todo_progress SET content = ? WHERE id = ? AND ledger_id = ?",
		progress.Content, progress.ID, ledgerID); err != nil {
		return domain.TodoProgress{}, fmt.Errorf("保存待办进度失败：%w", err)
	}
	return progress, nil
}

// SetTodoProgressDone checks or unchecks one progress line.
func (w *Workspace) SetTodoProgressDone(ctx context.Context, ledgerID, progressID string, done bool) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, "UPDATE todo_progress SET done = ? WHERE id = ? AND ledger_id = ?",
		done, progressID, ledgerID)
	if err != nil {
		return fmt.Errorf("更新待办进度失败：%w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("%w：进度记录不存在", repository.ErrNotFound)
	}
	return nil
}

// DeleteTodoProgress removes one progress line.
func (w *Workspace) DeleteTodoProgress(ctx context.Context, ledgerID, progressID string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, "DELETE FROM todo_progress WHERE id = ? AND ledger_id = ?", progressID, ledgerID)
	if err != nil {
		return fmt.Errorf("删除待办进度失败：%w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("%w：进度记录不存在", repository.ErrNotFound)
	}
	return nil
}

func todoItem(ctx context.Context, db *sql.DB, ledgerID, itemID string) (domain.TodoItem, error) {
	row := db.QueryRowContext(ctx, "SELECT "+todoItemColumns+" FROM todo_item WHERE id = ? AND ledger_id = ?", itemID, ledgerID)
	item, err := scanTodoItem(row)
	if err == sql.ErrNoRows {
		return domain.TodoItem{}, fmt.Errorf("%w：事项不存在", repository.ErrNotFound)
	}
	if err != nil {
		return domain.TodoItem{}, err
	}
	item.Urgency = domain.SnapTodoLevel(item.Urgency)
	item.Importance = domain.SnapTodoLevel(item.Importance)
	return item, nil
}

func todoProgress(ctx context.Context, db *sql.DB, ledgerID, progressID string) (domain.TodoProgress, error) {
	row := db.QueryRowContext(ctx, "SELECT "+todoProgressColumns+" FROM todo_progress WHERE id = ? AND ledger_id = ?",
		progressID, ledgerID)
	progress, err := scanTodoProgress(row)
	if err == sql.ErrNoRows {
		return domain.TodoProgress{}, fmt.Errorf("%w：进度记录不存在", repository.ErrNotFound)
	}
	return progress, err
}

func normalizeTodoItem(item domain.TodoItem) (domain.TodoItem, error) {
	title, err := todoText(item.Title, domain.TodoItemTitleMax, "请输入事项")
	if err != nil {
		return domain.TodoItem{}, err
	}
	start, ok := domain.ParseTodoDate(strings.TrimSpace(item.StartDate))
	if !ok {
		return domain.TodoItem{}, fmt.Errorf("%w：日期格式应为 YYYY-MM-DD", repository.ErrInvalid)
	}
	due, ok := domain.ParseTodoDate(strings.TrimSpace(item.DueDate))
	if !ok {
		return domain.TodoItem{}, fmt.Errorf("%w：日期格式应为 YYYY-MM-DD", repository.ErrInvalid)
	}
	if start != "" && due != "" && start > due {
		return domain.TodoItem{}, fmt.Errorf("%w：截止日期不能早于开始日期", repository.ErrInvalid)
	}
	if !todoLevel(item.Urgency) || !todoLevel(item.Importance) {
		return domain.TodoItem{}, fmt.Errorf("%w：紧急度 / 重要度不在六档内", repository.ErrInvalid)
	}
	item.Title = title
	item.StartDate = start
	item.DueDate = due
	return item, nil
}

func todoLevel(value int) bool {
	for _, level := range domain.TodoLevels {
		if level == value {
			return true
		}
	}
	return false
}

func todoText(raw string, max int, empty string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", fmt.Errorf("%w：%s", repository.ErrInvalid, empty)
	}
	runes := []rune(text)
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes), nil
}
