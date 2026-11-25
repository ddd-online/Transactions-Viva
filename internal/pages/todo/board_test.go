package todo

import (
	"context"
	"strings"
	"testing"
	"time"

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
	return paintCards(t, []domain.TodoCard{
		{ID: "c1", Title: "工作", Items: []domain.TodoItem{
			{ID: "i1", CardID: "c1", Title: "第一件", CreatedAt: 3},
			{ID: "i2", CardID: "c1", Title: "第二件", CreatedAt: 2},
		}},
		{ID: "c2", Title: "家里", Items: []domain.TodoItem{
			{ID: "i3", CardID: "c2", Title: "第三件", CreatedAt: 1},
		}},
	}, history, tab)
}

// paintCards is paint over the cards a test names.
func paintCards(t *testing.T, cards []domain.TodoCard, history []domain.TodoHistory, tab int) (*app.Driver, *boardRepo) {
	t.Helper()
	repo := &boardRepo{cards: cards, history: history}
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

// TestProgressRecordsWrapInsteadOfTruncating pins the fix for the progress log: a
// record is a sentence, so it wraps inside its row instead of running under the
// time and the two buttons. A wrapped line is painted as its own piece of text, so
// the dialog's copy of a long record arrives as several pieces that join back into
// the record; a truncated one arrives whole and clipped.
func TestProgressRecordsWrapInsteadOfTruncating(t *testing.T) {
	record := "2121想加上公休啊SHUOXHO爱生活小哦AHIOSXHUIO阿，再补一句更长的说明看看会不会换行而不是被截断"
	driver, _ := paintCards(t, []domain.TodoCard{{
		ID: "c1", Title: "工作", Items: []domain.TodoItem{{
			ID: "i1", CardID: "c1", Title: "第一件", CreatedAt: 1,
			Progress: []domain.TodoProgress{{ID: "p1", ItemID: "i1", Content: record, CreatedAt: 1}},
		}},
	}}, nil, 0)

	// The row's own expansion, then the progress dialog.
	if !click(driver, "展开进度 第一件") {
		t.Fatal("the board row has no expansion control")
	}
	if !click(driver, "编辑记录") {
		t.Fatal("the expanded row has no progress dialog")
	}

	var pieces []string
	for _, painted := range driver.Snapshot().Texts {
		// The expanded row prints the whole record on one line of its own; the
		// dialog is the copy this test is about.
		if painted.Value == record || painted.Value == "" {
			continue
		}
		if strings.Contains(record, painted.Value) {
			pieces = append(pieces, painted.Value)
		}
	}
	if len(pieces) < 2 {
		t.Fatalf("the dialog painted the record as %d piece(s): it was clipped, not wrapped", len(pieces))
	}
	if joined := strings.Join(pieces, ""); joined != record {
		t.Fatalf("the wrapped lines join to %q, want %q", joined, record)
	}
}

// TestProgressTimesStayAligned is the time column of the progress dialog: a record
// that wraps — a long run the line cannot break — must not squeeze the time beside
// it. Every row's time keeps its full width and ends on the same right edge.
func TestProgressTimesStayAligned(t *testing.T) {
	record := "2121想加上公休啊SHUOXHO爱生活小哦!AHIOSXHUO阿水好像UI爱生活IXBHIUJA时候"
	long, short := int64(1792000000), int64(1791000000)
	driver, _ := paintCards(t, []domain.TodoCard{{
		ID: "c1", Title: "工作", Items: []domain.TodoItem{{
			ID: "i1", CardID: "c1", Title: "第一件", CreatedAt: 1,
			Progress: []domain.TodoProgress{
				{ID: "p1", ItemID: "i1", Content: record, CreatedAt: long},
				{ID: "p2", ItemID: "i1", Content: "2121", CreatedAt: short},
			},
		}},
	}}, nil, 0)

	if !click(driver, "展开进度 第一件") {
		t.Fatal("the board row has no expansion control")
	}
	if !click(driver, "编辑记录") {
		t.Fatal("the expanded row has no progress dialog")
	}

	want := []string{
		time.Unix(long, 0).Format("01-02 15:04"),
		time.Unix(short, 0).Format("01-02 15:04"),
	}
	width := make([]float32, len(want))
	right := make([]float32, len(want))
	found := make([]bool, len(want))
	for _, painted := range driver.Snapshot().Texts {
		for index, value := range want {
			if painted.Value != value {
				continue
			}
			width[index], right[index], found[index] = painted.Bounds.Width, painted.Bounds.X+painted.Bounds.Width, true
		}
	}
	for index := range want {
		if !found[index] {
			t.Fatalf("the time %q was not painted: it was cut off", want[index])
		}
	}
	if width[0] != width[1] {
		t.Fatalf("the times are %.1f and %.1f wide: the long record squeezed one", width[0], width[1])
	}
	if right[0] != right[1] {
		t.Fatalf("the times end at %.1f and %.1f: they do not line up", right[0], right[1])
	}
}
