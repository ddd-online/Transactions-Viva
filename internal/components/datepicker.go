package components

import (
	"strconv"
	"time"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/icons"
)

// dateLayout is the local calendar date form the whole app stores dates in.
const dateLayout = "2006-01-02"

// DateGrain is the unit a date range is picked in: a day, a month or a year.
type DateGrain uint8

const (
	// DateByDay picks single days.
	DateByDay DateGrain = iota
	// DateByMonth picks whole months and shows the panel as twelve cells.
	DateByMonth
	// DateByYear picks whole years and shows the panel as a decade.
	DateByYear
)

// Name is the grain's caption in the picker's 日 / 月 / 年 choice.
func (grain DateGrain) Name() string {
	switch grain {
	case DateByMonth:
		return "月"
	case DateByYear:
		return "年"
	}
	return "日"
}

// dateGrains are the grain choices, in the old picker's order.
var dateGrains = [...]DateGrain{DateByDay, DateByMonth, DateByYear}

// dateWeekdays are the day grid's columns; the week starts on Monday.
var dateWeekdays = [calendarColumns]string{"一", "二", "三", "四", "五", "六", "日"}

const (
	// calendarColumns is the day grid's column count, as components.Calendar's.
	calendarColumns = 7
	// calendarRows is how many weeks the day grid always shows, so the panel does
	// not change height with the month.
	calendarRows = 6
	// datePresetCount is the most shortcuts a grain offers.
	datePresetCount = 5
	// datePanelLayer puts the panel over the page and over a dialog's surface: a
	// dialog paints on a layer of its own, so a panel painted inside one only
	// shows and only takes presses above it. The tag picker uses the same layer.
	datePanelLayer uint16 = 20
)

// DatePickerState is one picker's own state: whether its panel is open, the
// month it shows, and the identities the painted panel is hit by. Keep it in the
// page's model, one per picker; build it with NewDatePickerState.
type DatePickerState struct {
	// Open says whether the panel is showing.
	Open bool
	// Visible is the first day of the month the day grid shows.
	Visible time.Time

	tree    gfx.Tree
	cells   gfx.Scope
	dismiss gfx.ID
	prev    gfx.ID
	next    gfx.ID
	presets [datePresetCount]gfx.ID
	days    [calendarRows * calendarColumns]viva.CalendarDay
}

// NewDatePickerState builds a picker's state and the identities of its cells.
func NewDatePickerState() DatePickerState {
	var state DatePickerState
	feature := state.tree.Feature("date-picker")
	state.cells = state.tree.Sub(feature, state.tree.Name("cell"))
	return state
}

// DatePickerSpec is what one picker shows and what a pick does.
type DatePickerSpec struct {
	// Label names the control for assistive technology.
	Label string
	// ActionLabel names the button that opens a DateField's panel, such as
	// "打开日期选择".
	ActionLabel string
	// Start and End are the chosen range as local YYYY-MM-DD dates; End is empty
	// while the range still waits for its second end.
	Start, End string
	// Grain is the unit the range is picked in. Grains shows the 日 / 月 / 年
	// choice and the period arrows beside the trigger.
	Grain  DateGrain
	Grains bool
	// Placeholder is what the trigger says while nothing is chosen.
	Placeholder string
	// Viewport is the page's rectangle: a press anywhere in it closes the panel.
	Viewport gfx.Rect
	// MaxDate, when set, is the last day that may be picked.
	MaxDate string
	// CloseOnPick closes the panel as soon as a day is picked, as a form's
	// single-date field does.
	CloseOnPick bool
	// OnGrain and OnRange receive what the user picks.
	OnGrain func(grain DateGrain)
	OnRange func(start, end string)
}

// DateField is a form's date: a text field the user may also type in, with the
// button that opens the picker's panel under it.
func DateField(c ui.C, state *DatePickerState, value *string, spec DatePickerSpec) {
	state.bind(c)
	spec.Start = *value
	field := ui.TextField(c, value, spec.Label, spec.Placeholder).
		Action(icons.Calendar, spec.ActionLabel)
	if field.ActionClicked() {
		state.toggle(spec.Start)
	}
	field.DrawOver(func(d *ui.DrawContext) { state.paint(d, spec, d.Rect) })
}

// DateRangePicker is a toolbar's time range: the 日 / 月 / 年 choice, the period
// arrows, the value, and the panel under it. It is built from the theme's own
// controls, so it looks like the buttons beside it.
func DateRangePicker(c ui.C, state *DatePickerState, spec DatePickerSpec) {
	state.bind(c)
	theme := c.Theme()
	metrics := theme.Metrics
	ui.Row(c).Gap(metrics.GapItem).Items(ui.Center).Children(func() {
		if spec.Grains {
			grain := spec.Grain
			if ui.Choice(c, &grain, dateGrains[:], DateGrain.Name).Changed() {
				start, end := normalizeRange(spec.Start, spec.End, grain)
				if spec.OnRange != nil && (start != spec.Start || end != spec.End) {
					spec.OnRange(start, end)
				}
				if spec.OnGrain != nil {
					spec.OnGrain(grain)
				}
			}
		}
		if ui.Button(c, "上一周期").Ghost().Icon(icons.ChevronLeft).IconOnly().Clicked() {
			state.shift(spec, -1)
		}
		field := ui.Button(c, rangeText(spec.Start, spec.End, spec.Grain)).
			Outline().Icon(icons.Clock)
		if field.Clicked() {
			state.toggle(spec.Start)
		}
		field.DrawOver(func(d *ui.DrawContext) { state.paint(d, spec, d.Rect) })
		if ui.Button(c, "下一周期").Ghost().Icon(icons.ChevronRight).IconOnly().Clicked() {
			state.shift(spec, 1)
		}
	})
}

// bind names the panel's hit identities for this paint.
func (state *DatePickerState) bind(c ui.C) {
	state.dismiss = ui.ID(c)
	state.prev = ui.ID(c)
	state.next = ui.ID(c)
	for index := range state.presets {
		state.presets[index] = ui.ID(c)
	}
}

// toggle opens or closes the panel, showing the month of the chosen date.
func (state *DatePickerState) toggle(value string) {
	state.Open = !state.Open
	if state.Open {
		state.Visible = monthOf(value)
	}
}

// shift moves the range one period of its grain back or forward.
func (state *DatePickerState) shift(spec DatePickerSpec, direction int) {
	if spec.Start == "" || spec.End == "" || spec.OnRange == nil {
		return
	}
	start, end := shiftPeriod(spec.Start, spec.End, spec.Grain, direction)
	if spec.MaxDate != "" && (start > spec.MaxDate || end > spec.MaxDate) {
		return
	}
	spec.OnRange(start, end)
}

// layout is where the panel's blocks go, measured from the theme and from
// components.Calendar's own grid.
type layout struct {
	bounds   gfx.Rect // the panel surface
	calendar gfx.Rect // where components.Calendar is asked to paint
	caption  gfx.Rect // the month or decade title between the arrows
	grid     gfx.Rect // where the month or year cells go
	cell     gfx.Rect // one day cell, as the calendar sizes it
	rowStep  float32  // a day cell plus the gap under it
	presets  gfx.Rect // the shortcuts row
}

// paint draws the panel under its trigger and applies a press on it.
func (state *DatePickerState) paint(d *ui.DrawContext, spec DatePickerSpec, anchor gfx.Rect) {
	if !state.Open {
		return
	}
	canvas, theme := d.Canvas, *d.Theme
	state.fillDays(spec)
	place := state.measure(canvas, theme, spec, anchor)
	// The panel floats over a dialog's own layer. A press anywhere else closes it;
	// the panel's own hits are declared after that backdrop, so they win over it.
	canvas.PushLayer(datePanelLayer)
	defer canvas.PopLayer()
	canvas.HitWithOptions(state.dismiss, spec.Viewport, "关闭日期选择", gfx.HitOptions{
		Role: gfx.RoleButton, SkipFocus: true, Backdrop: true,
		PassPrimary: true, PassSecondary: true,
	})

	// The surface is the framework's popup look, so the panel matches a menu.
	look := viva.PopupRecipe(theme)
	canvas.Elevate(place.bounds, look.Radius, look.Shadow)
	canvas.Rounded(place.bounds, look.Radius, look.Fill)
	canvas.RoundedOutline(place.bounds, look.Radius, 1, look.Border)

	if spec.Grain == DateByDay {
		viva.BeginCalendar(canvas, theme, place.calendar, viva.CalendarOptions{
			MonthTitle:    monthTitle(state.visibleMonth()),
			Weekdays:      dateWeekdays,
			Days:          state.days[:],
			PreviousID:    state.prev,
			NextID:        state.next,
			PreviousLabel: "上个月",
			NextLabel:     "下个月",
		}).Paint()
	} else {
		state.paintPeriods(canvas, theme, spec, place)
	}
	state.paintPresets(canvas, theme, spec, place)
	state.handle(d, spec)
}

// fillDays is the calendar's month snapshot: the six weeks around the visible
// month, with the range and today marked for it to paint.
func (state *DatePickerState) fillDays(spec DatePickerSpec) {
	visible := state.visibleMonth()
	start := state.gridStart(visible)
	today := dateText(time.Now())
	for index := range state.days {
		day := start.AddDate(0, 0, index)
		date := dateText(day)
		state.days[index] = viva.CalendarDay{
			ID:          state.cells.At(index),
			Label:       strconv.Itoa(day.Day()),
			Value:       date,
			Hint:        date,
			Outside:     day.Month() != visible.Month(),
			Today:       date == today,
			Selected:    date == spec.Start && date == spec.End,
			RangeStart:  date == spec.Start && date != spec.End,
			RangeEnd:    date == spec.End && date != spec.Start,
			RangeMiddle: spec.Start != "" && spec.End != "" && date > spec.Start && date < spec.End,
			Disabled:    spec.MaxDate != "" && date > spec.MaxDate,
		}
	}
}

// measure works out where each block of the panel goes. The day grid is the
// reference: components.Calendar decides the cell size and the room around it,
// and the panel takes the width the calendar and the shortcuts need.
func (state *DatePickerState) measure(canvas *gfx.Canvas, theme gfx.Theme, spec DatePickerSpec, anchor gfx.Rect) layout {
	metrics := theme.Metrics
	scratch := viva.BeginCalendar(nil, theme, gfx.Rect{Width: 4096}, viva.CalendarOptions{Days: state.days[:]})
	cell := scratch.DayBounds(0)
	rowStep := scratch.DayBounds(calendarColumns).Y - cell.Y
	side := cell.X - scratch.Bounds().X
	width := 2*side + float32(calendarColumns)*cell.Width

	presetsHeight := viva.BeginButton(nil, theme, gfx.ID{}, gfx.Rect{}).Small().BoundsHeight()
	presetsWidth := float32(0)
	for _, preset := range datePresets(spec.Grain, spec.CloseOnPick) {
		if preset.label == "" {
			break
		}
		presetsWidth += viva.BeginButton(canvas, theme, gfx.ID{}, gfx.Rect{Width: 4096, Height: presetsHeight}).
			Small().SizeToFit(preset.label).BoundsWidth()
		presetsWidth += metrics.GapItem
	}
	presetsWidth -= metrics.GapItem
	width = max(width, presetsWidth+2*side)

	bounds := gfx.Rect{
		X:      min(anchor.X, max(spec.Viewport.Right()-width, 0)),
		Y:      anchor.Y + anchor.Height + metrics.GapItem,
		Width:  width,
		Height: scratch.Bounds().Height + metrics.GapItem + presetsHeight + side,
	}
	inner := bounds.Inset(gfx.All(side))
	return layout{
		bounds:   bounds,
		calendar: gfx.Rect{X: inner.X, Y: inner.Y, Width: inner.Width},
		caption:  gfx.Rect{X: inner.X, Y: inner.Y, Width: inner.Width, Height: cell.Height},
		grid:     gfx.Rect{X: inner.X, Y: inner.Y + rowStep, Width: inner.Width},
		cell:     cell,
		rowStep:  rowStep,
		presets:  gfx.Rect{X: inner.X, Y: bounds.Bottom() - side - presetsHeight, Width: inner.Width, Height: presetsHeight},
	}
}

// paintPeriods paints the month or year cells of a coarse grain, with the title
// and arrows the calendar paints for days.
func (state *DatePickerState) paintPeriods(canvas *gfx.Canvas, theme gfx.Theme, spec DatePickerSpec, place layout) {
	metrics := theme.Metrics
	visible := state.visibleMonth()
	title := strconv.Itoa(visible.Year()) + " 年"
	columns, rows := 3, 4
	if spec.Grain == DateByYear {
		title = strconv.Itoa(state.decade()) + " – " + strconv.Itoa(state.decade()+9)
		columns, rows = 2, 5
	}
	viva.BeginButton(canvas, theme, state.prev, gfx.Rect{X: place.caption.X, Y: place.caption.Y, Width: place.cell.Width, Height: place.caption.Height}).
		Icon().WithIcon(icons.ChevronLeft).Ghost("上一年")
	viva.BeginButton(canvas, theme, state.next, gfx.Rect{X: place.caption.Right() - place.cell.Width, Y: place.caption.Y, Width: place.cell.Width, Height: place.caption.Height}).
		Icon().WithIcon(icons.ChevronRight).Ghost("下一年")
	canvas.LabelCenter(title, gfx.Rect{
		X: place.caption.X + place.cell.Width, Y: place.caption.Y,
		Width: place.caption.Width - 2*place.cell.Width, Height: place.caption.Height,
	}, gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground})

	width := (place.grid.Width - float32(columns-1)*metrics.GapItem) / float32(columns)
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			index := row*columns + column
			cell := gfx.Rect{
				X:      place.grid.X + float32(column)*(width+metrics.GapItem),
				Y:      place.grid.Y + float32(row)*place.rowStep,
				Width:  width,
				Height: place.cell.Height,
			}
			from, to, label := state.cellValue(spec.Grain, index, visible)
			button := viva.BeginButton(canvas, theme, state.cells.At(index), cell)
			switch {
			case from == spec.Start || to == spec.End:
				button.Default(label)
			case spec.Start != "" && spec.End != "" && from > spec.Start && to < spec.End:
				button.Secondary(label)
			default:
				button.Ghost(label)
			}
		}
	}
}

// paintPresets paints the shortcuts under the grid as the theme's link buttons.
func (state *DatePickerState) paintPresets(canvas *gfx.Canvas, theme gfx.Theme, spec DatePickerSpec, place layout) {
	x := place.presets.X
	for index, preset := range datePresets(spec.Grain, spec.CloseOnPick) {
		if preset.label == "" {
			break
		}
		button := viva.BeginButton(canvas, theme, state.presets[index], gfx.Rect{
			X: x, Y: place.presets.Y, Width: place.presets.Width, Height: place.presets.Height,
		}).Small().SizeToFit(preset.label)
		width := button.BoundsWidth()
		button.Link(preset.label)
		x += width + theme.Metrics.GapItem
	}
}

// handle applies the press this paint delivered, if any.
func (state *DatePickerState) handle(d *ui.DrawContext, spec DatePickerSpec) {
	canvas := d.Canvas
	changed := false
	switch {
	case canvas.Clicked(state.dismiss):
		state.Open, changed = false, true
	case canvas.Clicked(state.prev):
		state.step(spec, -1)
		changed = true
	case canvas.Clicked(state.next):
		state.step(spec, 1)
		changed = true
	default:
		if index, ok := canvas.ClickedIn(state.cells); ok {
			state.pickCell(spec, index)
			changed = true
		}
		for index, preset := range datePresets(spec.Grain, spec.CloseOnPick) {
			if preset.label == "" {
				break
			}
			if canvas.Clicked(state.presets[index]) {
				state.pickPreset(spec, preset.from, preset.to)
				changed = true
			}
		}
	}
	if changed {
		canvas.RequestFrame()
	}
}

// step moves the panel one period of the grain.
func (state *DatePickerState) step(spec DatePickerSpec, direction int) {
	visible := state.visibleMonth()
	switch spec.Grain {
	case DateByMonth:
		state.Visible = visible.AddDate(direction, 0, 0)
	case DateByYear:
		state.Visible = visible.AddDate(direction*10, 0, 0)
	default:
		state.Visible = visible.AddDate(0, direction, 0)
	}
}

// pickCell applies a press on one cell of the grid.
func (state *DatePickerState) pickCell(spec DatePickerSpec, index int) {
	visible := state.visibleMonth()
	from, to, _ := state.cellValue(spec.Grain, index, visible)
	if spec.Grain == DateByDay {
		day, ok := parseDate(from)
		if ok && (day.Month() != visible.Month() || day.Year() != visible.Year()) {
			state.Visible = monthOf(from)
		}
	}
	state.pickRange(spec, from, to)
}

// pickPreset applies a shortcut: it always names both ends, and a form's field
// takes the day it starts on and closes.
func (state *DatePickerState) pickPreset(spec DatePickerSpec, from, to string) {
	if spec.OnRange == nil {
		return
	}
	if spec.CloseOnPick {
		spec.OnRange(from, from)
		state.Open = false
		return
	}
	spec.OnRange(from, to)
}

// pickRange applies a picked period: a form's field takes it and closes, a range
// takes it as the first end, turns it around, or starts a new range with it.
func (state *DatePickerState) pickRange(spec DatePickerSpec, from, to string) {
	if spec.OnRange == nil {
		return
	}
	if spec.CloseOnPick {
		spec.OnRange(from, to)
		state.Open = false
		return
	}
	start, end := rangeAfterPick(spec.Start, spec.End, from, to)
	spec.OnRange(start, end)
}

// cellValue is the day, month or year cell index stands for, as the range it
// picks and the text it shows.
func (state *DatePickerState) cellValue(grain DateGrain, index int, visible time.Time) (string, string, string) {
	switch grain {
	case DateByMonth:
		from, to := monthSpan(visible.Year(), time.Month(index+1))
		return from, to, strconv.Itoa(index+1) + " 月"
	case DateByYear:
		from, to := yearSpan(state.decade() + index)
		return from, to, strconv.Itoa(state.decade()+index) + " 年"
	}
	day := state.gridStart(visible).AddDate(0, 0, index)
	date := dateText(day)
	return date, date, strconv.Itoa(day.Day())
}

// visibleMonth is the month the panel shows, or this month before it was set.
func (state *DatePickerState) visibleMonth() time.Time {
	if state.Visible.IsZero() || state.Visible.Year() < 2 {
		return monthOf("")
	}
	return time.Date(state.Visible.Year(), state.Visible.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// decade is the first year of the ten the year grid shows.
func (state *DatePickerState) decade() int {
	return state.visibleMonth().Year() / 10 * 10
}

// gridStart is the Monday of the first week the month's grid shows.
func (state *DatePickerState) gridStart(visible time.Time) time.Time {
	offset := (int(visible.Weekday()) + 6) % 7
	return visible.AddDate(0, 0, -offset)
}

// monthTitle is the calendar's caption for the visible month.
func monthTitle(visible time.Time) string {
	return strconv.Itoa(visible.Year()) + " 年 " + strconv.Itoa(int(visible.Month())) + " 月"
}

// datePreset is one shortcut under the grid: its label and the range it picks.
type datePreset struct {
	label, from, to string
}

// datePresets are the shortcuts of a grain, as the old picker offered them. A
// field that picks one day offers 今天 alone.
func datePresets(grain DateGrain, single bool) [datePresetCount]datePreset {
	var presets [datePresetCount]datePreset
	today := dateText(time.Now())
	now, ok := parseDate(today)
	if !ok {
		return presets
	}
	set := func(index int, label, from, to string) {
		presets[index] = datePreset{label: label, from: from, to: to}
	}
	if single {
		set(0, "今天", today, today)
		return presets
	}
	switch grain {
	case DateByMonth:
		from, to := monthSpan(now.Year(), now.Month())
		set(0, "本月", from, to)
		previous := now.AddDate(0, -1, 0)
		from, to = monthSpan(previous.Year(), previous.Month())
		set(1, "上月", from, to)
		sixAgo := now.AddDate(0, -5, 0)
		from, _ = monthSpan(sixAgo.Year(), sixAgo.Month())
		_, to = monthSpan(now.Year(), now.Month())
		set(2, "近 6 个月", from, to)
		from, to = yearSpan(now.Year())
		set(3, "今年", from, to)
	case DateByYear:
		from, to := yearSpan(now.Year())
		set(0, "今年", from, to)
		from, to = yearSpan(now.Year() - 1)
		set(1, "去年", from, to)
	default:
		set(0, "今天", today, today)
		set(1, "近 7 天", dateText(now.AddDate(0, 0, -6)), today)
		monday, sunday := weekBounds(today)
		set(2, "本周", monday, sunday)
		from, to := monthSpan(now.Year(), now.Month())
		set(3, "本月", from, to)
		previous := now.AddDate(0, -1, 0)
		from, to = monthSpan(previous.Year(), previous.Month())
		set(4, "上月", from, to)
	}
	return presets
}

// rangeAfterPick is the range a press on from..to leaves: an open range takes
// the pick as its start, a pick before the start turns the range around, and a
// finished range starts a new one with it (the old picker's rule).
func rangeAfterPick(start, end, from, to string) (string, string) {
	switch {
	case start == "" || end != "":
		return from, ""
	case from < start:
		return from, start
	}
	return start, to
}

// parseDate reads a local calendar date.
func parseDate(value string) (time.Time, bool) {
	day, err := time.ParseInLocation(dateLayout, value, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

// dateText writes a local calendar date.
func dateText(day time.Time) string { return day.Format(dateLayout) }

// monthOf is the first day of the month value falls in, or of this month.
func monthOf(value string) time.Time {
	day, ok := parseDate(value)
	if !ok {
		day = time.Now()
	}
	return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// monthSpan is the first and last day of a month.
func monthSpan(year int, month time.Month) (string, string) {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	return dateText(first), dateText(last)
}

// yearSpan is the first and last day of a year.
func yearSpan(year int) (string, string) {
	return dateText(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)),
		dateText(time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC))
}

// weekBounds is the Monday and Sunday of the week a day belongs to.
func weekBounds(day string) (string, string) {
	value, ok := parseDate(day)
	if !ok {
		return day, day
	}
	offset := (int(value.Weekday()) + 6) % 7
	monday := value.AddDate(0, 0, -offset)
	return dateText(monday), dateText(monday.AddDate(0, 0, 6))
}

// normalizeRange snaps both ends of a range to the grain; a range that does not
// survive the snap is left as it was.
func normalizeRange(start, end string, grain DateGrain) (string, string) {
	if grain == DateByDay || start == "" || end == "" {
		return start, end
	}
	from, fromOK := parseDate(start)
	to, toOK := parseDate(end)
	if !fromOK || !toOK {
		return start, end
	}
	switch grain {
	case DateByMonth:
		start, _ = monthSpan(from.Year(), from.Month())
		_, end = monthSpan(to.Year(), to.Month())
	default:
		start, _ = yearSpan(from.Year())
		_, end = yearSpan(to.Year())
	}
	return start, end
}

// shiftPeriod moves a range one period of its grain: whole weeks for a range of
// exactly seven days, single days otherwise.
func shiftPeriod(start, end string, grain DateGrain, direction int) (string, string) {
	switch grain {
	case DateByMonth, DateByYear:
		from, fromOK := parseDate(start)
		to, toOK := parseDate(end)
		if !fromOK || !toOK {
			return start, end
		}
		if grain == DateByMonth {
			movedFrom := from.AddDate(0, direction, 0)
			movedTo := to.AddDate(0, direction, 0)
			start, _ = monthSpan(movedFrom.Year(), movedFrom.Month())
			_, end = monthSpan(movedTo.Year(), movedTo.Month())
			return start, end
		}
		start, _ = yearSpan(from.Year() + direction)
		_, end = yearSpan(to.Year() + direction)
		return start, end
	}
	from, fromOK := parseDate(start)
	to, toOK := parseDate(end)
	if !fromOK || !toOK {
		return start, end
	}
	days := direction
	if int(to.Sub(from).Hours()/24) == 6 {
		days *= 7
	}
	return dateText(from.AddDate(0, 0, days)), dateText(to.AddDate(0, 0, days))
}

// rangeText is what the trigger shows: the day, the month or the years of the
// range, with a trailing tilde while its second end is still missing.
func rangeText(start, end string, grain DateGrain) string {
	if start == "" {
		return "请选择时间范围"
	}
	label := func(value string) string {
		day, ok := parseDate(value)
		if !ok {
			return value
		}
		switch grain {
		case DateByMonth:
			return day.Format("2006-01")
		case DateByYear:
			return strconv.Itoa(day.Year())
		}
		return value
	}
	from := label(start)
	if end == "" {
		return from + " ~"
	}
	to := label(end)
	if from == to {
		return from
	}
	return from + " ~ " + to
}
