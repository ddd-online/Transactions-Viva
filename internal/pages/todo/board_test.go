package todo

import (
	"context"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// workspaceRepo answers none of the workspace reads: the board only touches the
// todo repository.
type workspaceRepo struct {
	repository.Repository
}

// boardRepo is a workspace of cards and items in memory. Completing an item moves
// it out of its card and into the history, as the store does, so the page sees a
// shorter board the moment it reloads; restoring one moves it back.
type boardRepo struct {
	repository.TodoRepository
	cards   []domain.TodoCard
	history []domain.TodoHistory
}

func (r *boardRepo) TodoCards(context.Context) ([]domain.TodoCard, error) { return r.cards, nil }

func (r *boardRepo) TodoHistory(context.Context) ([]domain.TodoHistory, error) {
	return r.history, nil
}

func (r *boardRepo) SetTodoItemStatus(_ context.Context, itemID, status string) error {
	if status != domain.TodoStatusDone {
		return r.restore(itemID, status)
	}
	for cardIndex := range r.cards {
		card := &r.cards[cardIndex]
		for itemIndex, item := range card.Items {
			if item.ID != itemID {
				continue
			}
			card.Items = append(card.Items[:itemIndex], card.Items[itemIndex+1:]...)
			item.Status, item.CompletedAt = status, 1
			r.history = append([]domain.TodoHistory{{Item: item, CardTitle: card.Title}}, r.history...)
			return nil
		}
	}
	return nil
}

// restore moves one history entry back into its theme.
func (r *boardRepo) restore(itemID, status string) error {
	for index, entry := range r.history {
		if entry.Item.ID != itemID {
			continue
		}
		item := entry.Item
		item.Status, item.CompletedAt = status, 0
		r.history = append(r.history[:index], r.history[index+1:]...)
		for cardIndex := range r.cards {
			if r.cards[cardIndex].ID == item.CardID {
				r.cards[cardIndex].Items = append(r.cards[cardIndex].Items, item)
			}
		}
		return nil
	}
	return nil
}

// paint returns the page's driver over a board of three rows in two themes, on
// the tab the test asks for: 0 is the board, 1 the history. The shell owns the
// second-level navigation, so the test picks the tab the way the shell does.
func paint(t *testing.T, tab int, history []domain.TodoHistory) (*app.Driver, *boardRepo) {
	t.Helper()
	repo := &boardRepo{
		cards: []domain.TodoCard{
			{ID: "c1", Title: "工作", Items: []domain.TodoItem{
				{ID: "i1", CardID: "c1", Title: "第一件", CreatedAt: 3},
				{ID: "i2", CardID: "c1", Title: "第二件", CreatedAt: 2},
			}},
			{ID: "c2", Title: "家里", Items: []domain.TodoItem{
				{ID: "i3", CardID: "c2", Title: "第三件", CreatedAt: 1},
			}},
		},
		history: history,
	}
	model := New()
	if err := model.SetRepository(workspaceRepo{}, repo); err != nil {
		t.Fatalf("SetRepository: %v", err)
	}
	model.SelectTab(tab)
	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: model.View, Theme: &theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatalf("NewDriver: %v", err)
	}
	t.Cleanup(driver.Close)
	return driver, repo
}

// click is one press in the middle of the first control whose label is label.
func click(driver *app.Driver, label string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label != label || hit.Disabled {
			continue
		}
		bounds := hit.Bounds
		return driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2) == nil
	}
	return false
}

// TestCompletingAnItemKeepsTheBoardPainting is the crash that came with marking an
// item done: the completion reloads the board, and the board was painting the
// list it had already started, so the next row's index ran past the shorter one.
func TestCompletingAnItemKeepsTheBoardPainting(t *testing.T) {
	driver, repo := paint(t, 0, nil)
	if got := len(driver.Snapshot().Hits); got == 0 {
		t.Fatal("the board painted no controls")
	}
	if !click(driver, "标记完成：第一件") {
		t.Fatal("the first row has no completion control")
	}
	// The paint after the click is the one that used to panic.
	if len(driver.Snapshot().Hits) == 0 {
		t.Fatal("the board painted nothing after completing an item")
	}
	if len(repo.cards[0].Items) != 1 || repo.cards[0].Items[0].ID != "i2" {
		t.Fatalf("the store kept %+v, want the second item only", repo.cards[0].Items)
	}
	if click(driver, "标记完成：第一件") {
		t.Fatal("the completed item is still on the board")
	}
}

// TestRestoringAHistoryItemKeepsTheHistoryPainting is the same hazard on the other
// list: restoring reloads it under its own loop.
func TestRestoringAHistoryItemKeepsTheHistoryPainting(t *testing.T) {
	driver, repo := paint(t, 1, []domain.TodoHistory{
		{Item: domain.TodoItem{ID: "d1", CardID: "c1", Title: "做完的一件", CompletedAt: 1}, CardTitle: "工作"},
		{Item: domain.TodoItem{ID: "d2", CardID: "c1", Title: "做完的两件", CompletedAt: 2}, CardTitle: "工作"},
	})
	if got := len(driver.Snapshot().Hits); got == 0 {
		t.Fatal("the history painted no controls")
	}
	if !click(driver, "恢复") {
		t.Fatal("the first history row has no restore control")
	}
	if len(driver.Snapshot().Hits) == 0 {
		t.Fatal("the history painted nothing after restoring an item")
	}
	if len(repo.history) != 1 {
		t.Fatalf("the store kept %d history entries, want 1", len(repo.history))
	}
}
