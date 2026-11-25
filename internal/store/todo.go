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

// TodoCards lists the workspace's themes with their doing items and progress.
func (w *Workspace) TodoCards(ctx context.Context) ([]domain.TodoCard, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	progress, err := todoProgressByItem(ctx, db)
	if err != nil {
		return nil, err
	}
	cards, err := todoCards(ctx, db)
	if err != nil {
		return nil, err
	}
	items, err := todoItemsByStatus(ctx, db, domain.TodoStatusDoing)
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

func todoCards(ctx context.Context, db *sql.DB) ([]domain.TodoCard, error) {
	rows, err := db.QueryContext(ctx, "SELECT "+todoCardColumns+" FROM todo_card "+
		"ORDER BY sort_order ASC, created_at ASC, id ASC")
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
	card.LedgerID = ""
	return card, err
}

func todoItemsByStatus(ctx context.Context, db *sql.DB, status string) ([]domain.TodoItem, error) {
	query := "SELECT " + todoItemColumns + " FROM todo_item WHERE status = ? "
	if status == domain.TodoStatusDone {
		query += "ORDER BY completed_at DESC, created_at DESC, id ASC"
	} else {
		// Newest first, and rowid breaks a same-second tie by insertion order,
		// so items made in one second still read as the order they were made.
		query += "ORDER BY created_at DESC, rowid DESC"
	}
	rows, err := db.QueryContext(ctx, query, status)
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
	item.LedgerID = ""
	return item, err
}

func todoProgressByItem(ctx context.Context, db *sql.DB) (map[string][]domain.TodoProgress, error) {
	rows, err := db.QueryContext(ctx, "SELECT "+todoProgressColumns+" FROM todo_progress "+
		"ORDER BY created_at ASC, id ASC")
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
	progress.LedgerID = ""
	return progress, err
}

// TodoHistory lists completed items newest first, with their card titles.
func (w *Workspace) TodoHistory(ctx context.Context) ([]domain.TodoHistory, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	cards, err := todoCards(ctx, db)
	if err != nil {
		return nil, err
	}
	titles := map[string]string{}
	for _, card := range cards {
		titles[card.ID] = card.Title
	}
	progress, err := todoProgressByItem(ctx, db)
	if err != nil {
		return nil, err
	}
	items, err := todoItemsByStatus(ctx, db, domain.TodoStatusDone)
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

// CreateTodoCard appends a theme at the end of the workspace's board.
func (w *Workspace) CreateTodoCard(ctx context.Context, title string) (domain.TodoCard, error) {
	title, err := todoText(title, domain.TodoCardTitleMax, "请填写主题")
	if err != nil {
		return domain.TodoCard{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.TodoCard{}, err
	}
	var next int
	if err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), 0) + 1 FROM todo_card").Scan(&next); err != nil {
		return domain.TodoCard{}, fmt.Errorf("读取待办排序失败：%w", err)
	}
	now := nowUnix()
	card := domain.TodoCard{
		ID: newID(), Title: title, CreatedAt: now, UpdatedAt: now, SortOrder: next,
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO todo_card ("+todoCardColumns+") VALUES (?, ?, ?, ?, ?, ?)",
		card.ID, "", card.Title, card.CreatedAt, card.UpdatedAt, card.SortOrder); err != nil {
		return domain.TodoCard{}, fmt.Errorf("创建待办主题失败：%w", err)
	}
	return card, nil
}

// DeleteTodoCard removes the card's progress, items and the card itself.
func (w *Workspace) DeleteTodoCard(ctx context.Context, cardID string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM todo_card WHERE id = ?",
			cardID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return notFoundf("主题不存在")
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM todo_progress WHERE item_id IN "+
			"(SELECT id FROM todo_item WHERE card_id = ?)", cardID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM todo_item WHERE card_id = ?", cardID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM todo_card WHERE id = ?", cardID)
		return err
	})
}

// ReorderTodoCards writes sort_order 1..n for the given cards.
func (w *Workspace) ReorderTodoCards(ctx context.Context, cardIDs []string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		for index, cardID := range cardIDs {
			result, err := tx.ExecContext(ctx, "UPDATE todo_card SET sort_order = ?, updated_at = ? WHERE id = ?",
				index+1, nowUnix(), cardID)
			if err != nil {
				return err
			}
			if affected, err := result.RowsAffected(); err != nil {
				return err
			} else if affected == 0 {
				return notFoundf("主题不存在")
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
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM todo_card WHERE id = ?",
		item.CardID).Scan(&cardExists); err != nil {
		return domain.TodoItem{}, err
	}
	if cardExists == 0 {
		return domain.TodoItem{}, notFoundf("主题不存在")
	}
	now := nowUnix()
	item.ID = newID()
	item.Status = domain.TodoStatusDoing
	item.CompletedAt = 0
	item.CreatedAt = now
	item.UpdatedAt = now
	if _, err := db.ExecContext(ctx, "INSERT INTO todo_item ("+todoItemColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		item.ID, "", item.CardID, item.Title, item.StartDate, item.DueDate,
		item.Urgency, item.Importance, item.Status, item.CompletedAt, item.CreatedAt, item.UpdatedAt); err != nil {
		return domain.TodoItem{}, fmt.Errorf("创建待办事项失败：%w", err)
	}
	item.LedgerID = ""
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
	existing, err := todoItem(ctx, db, item.ID)
	if err != nil {
		return domain.TodoItem{}, err
	}
	// The form lets an edit move the item to another theme; an empty CardID
	// keeps the one it already had.
	cardID := existing.CardID
	if item.CardID != "" && item.CardID != existing.CardID {
		var exists int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM todo_card WHERE id = ?",
			item.CardID).Scan(&exists); err != nil {
			return domain.TodoItem{}, err
		}
		if exists == 0 {
			return domain.TodoItem{}, notFoundf("主题不存在")
		}
		cardID = item.CardID
	}
	item.CardID = cardID
	item.Status = existing.Status
	item.CompletedAt = existing.CompletedAt
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = nowUnix()
	if _, err := db.ExecContext(ctx, "UPDATE todo_item SET card_id = ?, title = ?, start_date = ?, due_date = ?, urgency = ?, "+
		"importance = ?, updated_at = ? WHERE id = ?",
		item.CardID, item.Title, item.StartDate, item.DueDate, item.Urgency, item.Importance, item.UpdatedAt,
		item.ID); err != nil {
		return domain.TodoItem{}, fmt.Errorf("保存待办事项失败：%w", err)
	}
	if cardID != existing.CardID {
		// Moving the last item to another theme empties the old one; it goes the
		// way a theme goes when its last item is deleted.
		if err := pruneEmptyCard(ctx, db, existing.CardID); err != nil {
			return domain.TodoItem{}, err
		}
	}
	progress, err := todoProgressByItem(ctx, db)
	if err != nil {
		return domain.TodoItem{}, err
	}
	item.Progress = progress[item.ID]
	return item, nil
}

// SetTodoItemStatus moves an item between doing and done.
func (w *Workspace) SetTodoItemStatus(ctx context.Context, itemID, status string) error {
	if status != domain.TodoStatusDoing && status != domain.TodoStatusDone {
		return invalidf("待办状态只能是进行中或已完成")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	item, err := todoItem(ctx, db, itemID)
	if err != nil {
		return err
	}
	completedAt := int64(0)
	if status == domain.TodoStatusDone {
		completedAt = nowUnix()
	}
	_, err = db.ExecContext(ctx, "UPDATE todo_item SET status = ?, completed_at = ?, updated_at = ? "+
		"WHERE id = ?", status, completedAt, nowUnix(), item.ID)
	if err != nil {
		return fmt.Errorf("更新待办状态失败：%w", err)
	}
	return nil
}

// DeleteTodoItem removes an item and its progress. The last item to leave a theme
// takes the theme with it: a theme exists to hold items, and the board has no row
// for one that holds none (see pruneEmptyCard).
func (w *Workspace) DeleteTodoItem(ctx context.Context, itemID string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		var cardID string
		if err := tx.QueryRowContext(ctx, "SELECT card_id FROM todo_item WHERE id = ?",
			itemID).Scan(&cardID); err != nil {
			if err == sql.ErrNoRows {
				return notFoundf("事项不存在")
			}
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM todo_progress WHERE item_id = ?", itemID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM todo_item WHERE id = ?", itemID); err != nil {
			return err
		}
		return pruneEmptyCard(ctx, tx, cardID)
	})
}

// sqlExecer is what pruneEmptyCard needs: the workspace outside a transaction and
// the transaction inside one.
type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// pruneEmptyCard deletes a theme no item belongs to any more — the last item was
// deleted, or moved to another theme. Completed items count: a theme whose items
// are all done still holds them, and the history names them.
func pruneEmptyCard(ctx context.Context, execer sqlExecer, cardID string) error {
	if cardID == "" {
		return nil
	}
	_, err := execer.ExecContext(ctx,
		"DELETE FROM todo_card WHERE id = ? AND NOT EXISTS (SELECT 1 FROM todo_item WHERE card_id = ?)",
		cardID, cardID)
	return err
}

// AddTodoProgress appends one log line under an existing item.
func (w *Workspace) AddTodoProgress(ctx context.Context, itemID, content string) (domain.TodoProgress, error) {
	content, err := todoText(content, domain.TodoProgressMax, "请填写进度")
	if err != nil {
		return domain.TodoProgress{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.TodoProgress{}, err
	}
	if _, err := todoItem(ctx, db, itemID); err != nil {
		return domain.TodoProgress{}, err
	}
	progress := domain.TodoProgress{ID: newID(), ItemID: itemID, Content: content, CreatedAt: nowUnix()}
	if _, err := db.ExecContext(ctx, "INSERT INTO todo_progress ("+todoProgressColumns+") VALUES (?, ?, ?, ?, ?, ?)",
		progress.ID, "", progress.ItemID, progress.Content, progress.CreatedAt, progress.Done); err != nil {
		return domain.TodoProgress{}, fmt.Errorf("写入待办进度失败：%w", err)
	}
	return progress, nil
}

// UpdateTodoProgress corrects a progress line's words without touching its
// time or checked state.
func (w *Workspace) UpdateTodoProgress(ctx context.Context, progressID, content string) (domain.TodoProgress, error) {
	content, err := todoText(content, domain.TodoProgressMax, "请填写进度")
	if err != nil {
		return domain.TodoProgress{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.TodoProgress{}, err
	}
	progress, err := todoProgress(ctx, db, progressID)
	if err != nil {
		return domain.TodoProgress{}, err
	}
	progress.Content = content
	if _, err := db.ExecContext(ctx, "UPDATE todo_progress SET content = ? WHERE id = ?",
		progress.Content, progress.ID); err != nil {
		return domain.TodoProgress{}, fmt.Errorf("保存待办进度失败：%w", err)
	}
	return progress, nil
}

// SetTodoProgressDone checks or unchecks one progress line.
func (w *Workspace) SetTodoProgressDone(ctx context.Context, progressID string, done bool) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, "UPDATE todo_progress SET done = ? WHERE id = ?",
		done, progressID)
	if err != nil {
		return fmt.Errorf("更新待办进度失败：%w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return notFoundf("进度记录不存在")
	}
	return nil
}

// DeleteTodoProgress removes one progress line.
func (w *Workspace) DeleteTodoProgress(ctx context.Context, progressID string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, "DELETE FROM todo_progress WHERE id = ?", progressID)
	if err != nil {
		return fmt.Errorf("删除待办进度失败：%w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return notFoundf("进度记录不存在")
	}
	return nil
}

func todoItem(ctx context.Context, db *sql.DB, itemID string) (domain.TodoItem, error) {
	row := db.QueryRowContext(ctx, "SELECT "+todoItemColumns+" FROM todo_item WHERE id = ?", itemID)
	item, err := scanTodoItem(row)
	if err == sql.ErrNoRows {
		return domain.TodoItem{}, notFoundf("事项不存在")
	}
	if err != nil {
		return domain.TodoItem{}, err
	}
	item.Urgency = domain.SnapTodoLevel(item.Urgency)
	item.Importance = domain.SnapTodoLevel(item.Importance)
	return item, nil
}

func todoProgress(ctx context.Context, db *sql.DB, progressID string) (domain.TodoProgress, error) {
	row := db.QueryRowContext(ctx, "SELECT "+todoProgressColumns+" FROM todo_progress WHERE id = ?",
		progressID)
	progress, err := scanTodoProgress(row)
	if err == sql.ErrNoRows {
		return domain.TodoProgress{}, notFoundf("进度记录不存在")
	}
	return progress, err
}

func normalizeTodoItem(item domain.TodoItem) (domain.TodoItem, error) {
	title, err := todoText(item.Title, domain.TodoItemTitleMax, "请填写事项")
	if err != nil {
		return domain.TodoItem{}, err
	}
	start, ok := domain.ParseTodoDate(strings.TrimSpace(item.StartDate))
	if !ok {
		return domain.TodoItem{}, invalidf("日期要写成 YYYY-MM-DD")
	}
	due, ok := domain.ParseTodoDate(strings.TrimSpace(item.DueDate))
	if !ok {
		return domain.TodoItem{}, invalidf("日期要写成 YYYY-MM-DD")
	}
	if start != "" && due != "" && start > due {
		return domain.TodoItem{}, invalidf("截止日期不能早于开始日期")
	}
	if !todoLevel(item.Urgency) || !todoLevel(item.Importance) {
		return domain.TodoItem{}, invalidf("紧急度 / 重要度不在六档内")
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
		return "", invalidf("%s", empty)
	}
	runes := []rune(text)
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes), nil
}
