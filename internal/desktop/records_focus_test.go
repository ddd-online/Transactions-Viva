package desktop

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// firstRecordRow is the table's first row: the only list options on the records
// page are its rows.
func firstRecordRow(driver *app.Driver) (gfx.Hit, bool) {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Role == gfx.RoleOption {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// focusByTab stops when the hit with id has the keyboard, pressing Tab at most
// limit times (the page's controls come before the table).
func focusByTab(t *testing.T, driver *app.Driver, id gfx.ID, limit int) bool {
	t.Helper()
	for press := 0; press < limit; press++ {
		if driver.Snapshot().Focused == id {
			return true
		}
		if err := driver.Key(gfx.KeyTab, 0); err != nil {
			t.Fatal(err)
		}
		if err := driver.Paint(); err != nil {
			t.Fatal(err)
		}
	}
	return driver.Snapshot().Focused == id
}

// TestRecordsRowsAreNamedAndShowFocus: a record's row is a list option with a
// spoken name — the columns read as the sentence the row paints — and the
// keyboard's focus on it is a 2px ring drawn inside the row. The row spans the
// table's full width, so a ring around it would be cut by the table's own edge.
func TestRecordsRowsAreNamedAndShowFocus(t *testing.T) {
	driver, model := open(t)
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	row, ok := firstRecordRow(driver)
	if !ok {
		t.Fatal("the records page shows no row")
	}
	if row.Label == "" {
		t.Fatal("the record row has no name for assistive technology")
	}
	if !strings.Contains(row.Label, "支出") && !strings.Contains(row.Label, "收入") &&
		!strings.Contains(row.Label, "转账") {
		t.Fatalf("row name %q names no transaction type", row.Label)
	}
	if !strings.HasPrefix(row.Label, "20") {
		t.Fatalf("row name %q does not start with the date", row.Label)
	}
	// The name describes the row, not the table: the painted cells' words are in
	// it (the empty-cell dash is not a word).
	for _, text := range driver.Snapshot().Texts {
		value := strings.TrimSpace(text.Value)
		if value == "" || value == "—" || text.Bounds.X < row.Bounds.X ||
			text.Bounds.X > row.Bounds.X+row.Bounds.Width ||
			text.Bounds.Y < row.Bounds.Y || text.Bounds.Y > row.Bounds.Y+row.Bounds.Height {
			continue
		}
		if !strings.Contains(row.Label, value) {
			t.Fatalf("row name %q leaves out %q, which the row paints", row.Label, value)
		}
	}
	if !focusByTab(t, driver, row.ID, 40) {
		t.Fatal("tabbing never reached the record row")
	}
	// The ring is inset: a ring drawn outside the row would be cut by the table's
	// own edge, since the row runs from that edge to the other.
	found := false
	for _, primitive := range driver.Snapshot().Primitives {
		if primitive.Color != model.theme.Ring {
			continue
		}
		bounds := primitive.Bounds
		if bounds.X < row.Bounds.X-4 || bounds.X > row.Bounds.X+row.Bounds.Width+4 ||
			bounds.Y < row.Bounds.Y-4 || bounds.Y > row.Bounds.Y+row.Bounds.Height+4 {
			continue
		}
		found = true
	}
	if !found {
		t.Fatal("the focused row paints no focus ring")
	}
}
