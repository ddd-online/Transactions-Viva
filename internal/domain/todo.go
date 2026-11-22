package domain

import "time"

// TodoDateLayout is the only accepted date shape for todo start / due dates.
const TodoDateLayout = "2006-01-02"

// TodoCard is one theme column. Cards belong to a ledger and are ordered by
// sort_order; a card with no items still exists.
type TodoCard struct {
	ID        string
	LedgerID  string
	Title     string
	CreatedAt int64
	UpdatedAt int64
	SortOrder int
	Items     []TodoItem
}

// TodoItem is one task. Dates are local YYYY-MM-DD; an empty date means unset.
// Status is TodoStatusDoing or TodoStatusDone; completed_at is 0 while doing.
type TodoItem struct {
	ID          string
	LedgerID    string
	CardID      string
	Title       string
	StartDate   string
	DueDate     string
	Urgency     int
	Importance  int
	Status      string
	CompletedAt int64
	CreatedAt   int64
	UpdatedAt   int64
	Progress    []TodoProgress
}

// TodoProgress is one log line under an item.
type TodoProgress struct {
	ID        string
	LedgerID  string
	ItemID    string
	Content   string
	Done      bool
	CreatedAt int64
}

// TodoHistory is a completed item with the title of its card.
type TodoHistory struct {
	Item      TodoItem
	CardTitle string
}

const (
	TodoStatusDoing = TodoDoing
	TodoStatusDone  = TodoDone

	TodoCardTitleMax = 200
	TodoItemTitleMax = 500
	TodoProgressMax  = 2000
)

// TodoDefaultLevel is the neutral stop used by a new item.
const TodoDefaultLevel = DefaultTodoLevel

// TodoDueState is how a due date reads today: overdue, today, future, or none.
func TodoDueState(dueDate, today string) string {
	if dueDate == "" {
		return ""
	}
	switch {
	case dueDate < today:
		return "overdue"
	case dueDate == today:
		return "today"
	default:
		return "future"
	}
}

// ParseTodoDate accepts exactly YYYY-MM-DD and a real calendar date. An empty
// string is allowed (no date) and returns ("", true).
func ParseTodoDate(value string) (string, bool) {
	if value == "" {
		return "", true
	}
	parsed, err := time.ParseInLocation(TodoDateLayout, value, time.Local)
	if err != nil || parsed.Format(TodoDateLayout) != value {
		return "", false
	}
	return value, true
}

// TodoDoneCount counts completed progress lines.
func TodoDoneCount(items []TodoProgress) int {
	done := 0
	for _, item := range items {
		if item.Done {
			done++
		}
	}
	return done
}
