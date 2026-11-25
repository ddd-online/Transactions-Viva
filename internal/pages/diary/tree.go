package diary

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

// rowKind is what one visible tree row is: a year, a month or a day.
type rowKind uint8

const (
	rowYear rowKind = iota
	rowMonth
	rowDay
)

// row is one visible row of the date tree. A year and a month carry their count
// of entries; a day carries its word count and mood, which the row shows.
type row struct {
	kind      rowKind
	key       string // "2026", "2026-01" or "2026-01-15"
	year      int
	month     int
	day       int
	count     int
	date      string
	wordCount int
	mood      string
}

// node describes row i to the tree: its depth, whether it has children, whether
// they show, and whether it is the selected day.
func (m *Model) node(index int) ui.TreeNode {
	if index < 0 || index >= len(m.rows) {
		return ui.TreeNode{}
	}
	one := m.rows[index]
	node := ui.TreeNode{Depth: int(one.kind)}
	switch one.kind {
	case rowYear, rowMonth:
		node.Branch = true
		node.Expanded = m.expanded[one.key]
	case rowDay:
		node.Selected = one.date == m.selected
	}
	return node
}

// rowLabel is the row's accessible name: the words read out when it takes the
// focus, which the painted row can leave implicit.
func (m *Model) rowLabel(index int) string {
	if index < 0 || index >= len(m.rows) {
		return ""
	}
	one := m.rows[index]
	switch one.kind {
	case rowYear:
		return fmt.Sprintf("%d年，%d 篇", one.year, one.count)
	case rowMonth:
		return fmt.Sprintf("%d月，%d 篇", one.month, one.count)
	}
	label := domain.DiaryDateCn(one.date)
	if mood := domain.DiaryMoodLabel(one.mood); mood != "" {
		label += "，" + mood
	}
	return fmt.Sprintf("%s，%d 字", label, one.wordCount)
}

// rowContent paints one row inside the tree's own row box (already indented for
// its depth and padded for the chevron):
//
//	2026年            12 篇
//	  1月              3 篇
//	    15   😊        42 字
//
// The year and the month keep the count at the trailing edge, the day puts the
// mood beside its number and the word count after it, so a column of days reads
// down the numbers and across the moods.
func (m *Model) rowContent(c ui.C, index int) {
	if index < 0 || index >= len(m.rows) {
		return
	}
	one := m.rows[index]
	theme := c.Theme()
	// A figure is mono in this application: the counts and the day numbers line
	// up down the tree the way an amount lines up in a table.
	number := func(text string) ui.Box {
		return ui.Text(c, text).FontSize(typography.SizeCaption).
			Color(theme.MutedForeground).Mono()
	}
	selected := one.kind == rowDay && one.date == m.selected
	if selected {
		// MutedForeground on the selection fill is 4.24:1 at 12px, under the
		// floor; the selected row's figure is the foreground instead.
		number = func(text string) ui.Box {
			return ui.Text(c, text).FontSize(typography.SizeCaption).
				Color(theme.AccentForeground).Mono()
		}
	}
	switch one.kind {
	case rowYear:
		typography.Body(c, fmt.Sprintf("%d年", one.year)).Grow(1).Ellipsis()
		number(fmt.Sprintf("%d 篇", one.count))
	case rowMonth:
		typography.Body(c, fmt.Sprintf("%d月", one.month)).Grow(1).Ellipsis()
		number(fmt.Sprintf("%d 篇", one.count))
	case rowDay:
		// The day number sits on its own right-aligned column and the word count
		// on the same trailing edge as the counts above it, so a column of days
		// reads down two straight lines.
		ui.Text(c, strconv.Itoa(one.day)).FontSize(typography.SizeBody).Mono().
			Width(dayNumberWidth).TextEnd()
		mood := domain.NormalizeDiaryMood(one.mood)
		if mood == "" {
			blank(c, moodWidth)
		} else {
			ui.Text(c, mood).FontSize(typography.SizeBody).Width(moodWidth)
		}
		number(fmt.Sprintf("%d 字", one.wordCount)).Grow(1).TextEnd()
	}
	m.rowTail(c, index, one)
}

// blank is an empty box that holds a column open. ui.Spacer is wrong here: it
// grows, so an empty slot would take the room from the column beside it and a day
// without a mood would push its word count out of line with the days that have one.
func blank(c ui.C, width float32) ui.Box {
	return ui.Row(c).Width(width).Shrink(0)
}

// rowTail is the trailing slot every row keeps: the day's delete key while the row
// is lit, empty space the rest of the time. Year and month rows hold nothing to
// delete, but they keep the slot too, so the counts above and below stay on one
// right edge whether a key is showing or not.
func (m *Model) rowTail(c ui.C, index int, one row) {
	if one.kind != rowDay {
		blank(c, rowTailWidth)
		return
	}
	theme := c.Theme()
	// The key's identity is named before it is declared, so that the row stays lit
	// while the pointer is on the key: without it, moving onto the key un-hovers
	// the row, the key disappears, and the pointer falls back on the row — the two
	// flicker against each other and the click never lands.
	c.Key("diary-row-delete-" + one.date)
	keyID := ui.ID(c)
	// The key is quiet until the row is lit: the pointer is on it, it has the
	// keyboard, or it is the day the editor shows — the toolbar used to carry the
	// delete command, and this is where the entry lives.
	lit := one.date == m.selected || c.Canvas().Hovered() == keyID
	if rowID := m.tree.RowID(index); !rowID.IsZero() {
		lit = lit || c.Canvas().Hovered() == rowID || c.Canvas().FocusVisible() == rowID
	}
	if !lit {
		blank(c, rowTailWidth)
		return
	}
	// A delete key, so it wears the destructive palette: the danger ink on the
	// danger wash, a step stronger under the pointer.
	button := ui.Row(c).Size(rowTailWidth, rowTailWidth).Center().
		Radius(theme.Metrics.RadiusControl).
		Background(theme.Danger.WithAlpha(0.10)).
		HitWith(keyID, "删除日记 "+domain.DiaryDateCn(one.date), gfx.HitOptions{
			Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
		}).
		Hover(ui.Look{}.Background(theme.Danger.WithAlpha(0.20))).
		Pressed(ui.Look{}.Background(theme.Danger.WithAlpha(0.30))).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Tooltip("删除日记")
	button.Children(func() {
		ui.Icon(c, icons.Trash2).Size(15, 15).Color(theme.Danger)
	})
	if button.Clicked() {
		m.askDeleteDate(one.date)
	}
}

// dayNumberWidth and moodWidth keep the day rows in one column: the numbers are
// two characters at most, the mood is one emoji.
const (
	dayNumberWidth float32 = 20
	moodWidth      float32 = 20
	// rowTailWidth is the trailing slot of a row: the delete key's box.
	rowTailWidth float32 = 24
)

// rebuildRows flattens the dates into the visible rows: years newest first,
// months newest first inside a year, days newest first inside a month, with a
// branch's children only while it is expanded.
func (m *Model) rebuildRows() {
	type monthGroup struct {
		month int
		key   string
		days  []domain.DiaryDateItem
	}
	type yearGroup struct {
		year   int
		key    string
		months []monthGroup
	}

	years := make([]yearGroup, 0, 4)
	for _, item := range m.dates {
		year, ok := domain.ParseDiaryDate(item.Date)
		if !ok {
			continue
		}
		yearKey := domain.DiaryYearKey(item.Date)
		monthKey := domain.DiaryMonthKey(item.Date)
		index := sort.Search(len(years), func(i int) bool { return years[i].year <= year.Year() })
		if index == len(years) || years[index].year != year.Year() {
			years = append(years, yearGroup{})
			copy(years[index+1:], years[index:])
			years[index] = yearGroup{year: year.Year(), key: yearKey}
		}
		group := &years[index]
		monthIndex := sort.Search(len(group.months), func(i int) bool { return group.months[i].month <= int(year.Month()) })
		if monthIndex == len(group.months) || group.months[monthIndex].month != int(year.Month()) {
			group.months = append(group.months, monthGroup{})
			copy(group.months[monthIndex+1:], group.months[monthIndex:])
			group.months[monthIndex] = monthGroup{month: int(year.Month()), key: monthKey}
		}
		month := &group.months[monthIndex]
		month.days = append(month.days, item)
	}

	m.rows = m.rows[:0]
	for _, year := range years {
		count := 0
		for _, month := range year.months {
			count += len(month.days)
		}
		m.rows = append(m.rows, row{kind: rowYear, key: year.key, year: year.year, count: count})
		if !m.expanded[year.key] {
			continue
		}
		for _, month := range year.months {
			m.rows = append(m.rows, row{
				kind: rowMonth, key: month.key, year: year.year, month: month.month,
				count: len(month.days),
			})
			if !m.expanded[month.key] {
				continue
			}
			for _, day := range month.days {
				parsed, _ := domain.ParseDiaryDate(day.Date)
				m.rows = append(m.rows, row{
					kind: rowDay, key: day.Date, date: day.Date, year: parsed.Year(),
					month: int(parsed.Month()), day: parsed.Day(),
					wordCount: day.WordCount, mood: day.Mood,
				})
			}
		}
	}
}

// toggleRow opens or closes the year or month at a row index.
func (m *Model) toggleRow(index int) {
	if index < 0 || index >= len(m.rows) {
		return
	}
	one := m.rows[index]
	if one.kind == rowDay {
		return
	}
	m.expanded[one.key] = !m.expanded[one.key]
	m.rebuildRows()
}

// chooseRow handles a click or Enter on a row: a day opens in the editor, a year
// or a month opens or closes, which is what clicking one should do in a tree of
// dates.
func (m *Model) chooseRow(index int) {
	if index < 0 || index >= len(m.rows) {
		return
	}
	one := m.rows[index]
	if one.kind == rowDay {
		m.selectDate(one.date)
		return
	}
	m.expanded[one.key] = !m.expanded[one.key]
	m.rebuildRows()
}

// openAround expands the year and month a date falls in, so a date just written
// or imported is visible in the tree without hunting for its branches.
func (m *Model) openAround(date string) {
	if !domain.IsDiaryDate(date) {
		return
	}
	year, month := domain.DiaryYearKey(date), domain.DiaryMonthKey(date)
	if len(m.dates) > 0 {
		// The newest entry's branches open too, so a ledger with entries shows
		// its latest day without a click.
		newest := m.dates[0].Date
		if domain.IsDiaryDate(newest) {
			m.expanded[domain.DiaryYearKey(newest)] = true
			m.expanded[domain.DiaryMonthKey(newest)] = true
		}
	}
	m.expanded[year] = true
	m.expanded[month] = true
}
