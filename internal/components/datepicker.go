package components

import (
	"strconv"
	"time"

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

// dateGrains are the grain choices, in the order the old picker showed them.
var dateGrains = [...]DateGrain{DateByDay, DateByMonth, DateByYear}

// The panel is the old project's `.ui-time__panel`: a 280 px card with 34 x 30
// day cells, the period arrows in its header and the presets in a row at the
// bottom.
const (
	datePanelWidth    float32 = 280
	datePanelPadding  float32 = 12
	dateCellWidth     float32 = 34
	dateCellHeight    float32 = 30
	dateCellGap       float32 = 2
	dateBlockGap      float32 = 8
	dateHeaderHeight  float32 = 26
	dateWeekdayHeight float32 = 18
	datePresetHeight  float32 = 26
	datePresetCount           = 5
	datePanelGap      float32 = 4
	// datePanelLayer puts the panel over the page and over a dialog's surface: a
	// dialog paints on a layer of its own, so a panel painted inside one only
	// shows and only takes presses above it. The tag picker uses the same layer.
	datePanelLayer uint16 = 20
)

// dateWeekdays are the columns of the day grid; the week starts on Monday.
var dateWeekdays = [7]string{"一", "二", "三", "四", "五", "六", "日"}

// DatePickerState is one picker's own state: whether its panel is open, the
// month the panel shows, and the identities the painted panel is hit by. Keep it
// in the page's model, one per picker; build it with NewDatePickerState.
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
// arrows, the value field, and the panel under it.
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
		if state.shiftButton(c, "上一周期", icons.ChevronLeft).Clicked() {
			state.shift(spec, -1)
		}
		border := theme.Input
		if state.Open {
			border = theme.Primary
		}
		field := ui.Row(c).Height(metrics.ControlHeight).MinWidth(190).
			PadX(metrics.PadControlX).Gap(6).Items(ui.Center).
			Border(1, border).Radius(metrics.RadiusControl).Background(theme.Background).
			Hit(ui.ID(c), spec.Label).Hover(ui.Look{}.Border(theme.Primary))
		field.Children(func() {
			ui.Icon(c, icons.Clock).Width(14).Height(14).Color(theme.MutedForeground)
			ui.Text(c, rangeText(spec.Start, spec.End, spec.Grain)).
				FontSize(metrics.FontSizeS).Mono().Ellipsis()
		})
		if field.Clicked() {
			state.toggle(spec.Start)
		}
		field.DrawOver(func(d *ui.DrawContext) { state.paint(d, spec, d.Rect) })
		if state.shiftButton(c, "下一周期", icons.ChevronRight).Clicked() {
			state.shift(spec, 1)
		}
	})
}

// shiftButton declares one of the arrows that move the range a whole period.
func (state *DatePickerState) shiftButton(c ui.C, label string, icon gfx.Icon) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	return ui.Row(c).Height(metrics.ControlHeight).Width(28).
		Radius(metrics.RadiusControl).Center().Hit(ui.ID(c), label).
		Hover(ui.Look{}.Background(theme.Hover)).Children(func() {
		ui.Icon(c, icon).Width(14).Height(14).Color(theme.MutedForeground)
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

// paint draws the panel under its trigger and applies a press on it.
func (state *DatePickerState) paint(d *ui.DrawContext, spec DatePickerSpec, anchor gfx.Rect) {
	if !state.Open {
		return
	}
	canvas, theme := d.Canvas, *d.Theme
	bounds := gfx.Rect{
		X:      min(anchor.X, max(spec.Viewport.Right()-datePanelWidth, 0)),
		Y:      anchor.Y + anchor.Height + datePanelGap,
		Width:  datePanelWidth,
		Height: datePanelHeight(spec.Grain),
	}
	// The panel floats over a dialog's own layer. A press anywhere else closes it;
	// the panel's hits are declared after that backdrop, so they win over it.
	canvas.PushLayer(datePanelLayer)
	defer canvas.PopLayer()
	// A press outside the panel closes it and still reaches what it landed on,
	// as the old picker's document listener left the click to the page.
	canvas.HitWithOptions(state.dismiss, spec.Viewport, "关闭日期选择", gfx.HitOptions{
		Role: gfx.RoleButton, SkipFocus: true, Backdrop: true,
		PassPrimary: true, PassSecondary: true,
	})
	canvas.Elevate(bounds, theme.Metrics.RadiusSurface, gfx.ShadowMD)
	canvas.Rounded(bounds, theme.Metrics.RadiusSurface, theme.Popover)
	canvas.RoundedOutline(bounds, theme.Metrics.RadiusSurface, 1, theme.Border)

	inner := bounds.Inset(gfx.All(datePanelPadding))
	header, inner := inner.CutTop(dateHeaderHeight)
	state.paintHeader(canvas, &theme, spec, header)
	_, inner = inner.CutTop(dateBlockGap)
	grid, inner := inner.CutTop(dateGridHeight(spec.Grain))
	state.paintGrid(canvas, &theme, spec, grid)
	_, inner = inner.CutTop(dateBlockGap)
	divider, inner := inner.CutTop(1)
	canvas.Fill(divider, theme.Border)
	_, inner = inner.CutTop(dateBlockGap)
	state.paintPresets(canvas, &theme, spec, inner)
	state.handle(d, spec)
}

// paintHeader paints the period title between its two arrows.
func (state *DatePickerState) paintHeader(canvas *gfx.Canvas, theme *gfx.Theme, spec DatePickerSpec, bounds gfx.Rect) {
	visible := state.visibleMonth()
	title := ""
	switch spec.Grain {
	case DateByMonth:
		title = strconv.Itoa(visible.Year()) + " 年"
	case DateByYear:
		title = strconv.Itoa(state.decade()) + " – " + strconv.Itoa(state.decade()+9)
	default:
		title = strconv.Itoa(visible.Year()) + " 年 " + strconv.Itoa(int(visible.Month())) + " 月"
	}
	canvas.LabelCenter(title, bounds, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.PopoverForeground,
	})
	state.paintArrow(canvas, theme, state.prev, bounds, icons.ChevronLeft, state.stepLabel(spec, -1))
	state.paintArrow(canvas, theme, state.next, bounds, icons.ChevronRight, state.stepLabel(spec, 1))
}

// stepLabel is what a header arrow is called for the grain it steps.
func (state *DatePickerState) stepLabel(spec DatePickerSpec, direction int) string {
	switch spec.Grain {
	case DateByMonth:
		if direction < 0 {
			return "上一年"
		}
		return "下一年"
	case DateByYear:
		if direction < 0 {
			return "上一个十年"
		}
		return "下一个十年"
	}
	if direction < 0 {
		return "上个月"
	}
	return "下个月"
}

// paintArrow paints one header arrow at an end of bounds.
func (state *DatePickerState) paintArrow(canvas *gfx.Canvas, theme *gfx.Theme, id gfx.ID,
	bounds gfx.Rect, icon gfx.Icon, label string) {
	arrow := gfx.Rect{X: bounds.X, Y: bounds.Y, Width: dateCellWidth, Height: bounds.Height}
	if id == state.next {
		arrow.X = bounds.Right() - dateCellWidth
	}
	if canvas.Hovered() == id {
		canvas.Rounded(arrow, theme.Metrics.RadiusControl, theme.Hover)
	}
	canvas.DrawIcon(icon, arrow.Centered(14, 14), theme.MutedForeground)
	canvas.HitWithOptions(id, arrow, label, gfx.HitOptions{Role: gfx.RoleButton})
}

// paintGrid paints the day cells, the months or the decade of the grain.
func (state *DatePickerState) paintGrid(canvas *gfx.Canvas, theme *gfx.Theme, spec DatePickerSpec, bounds gfx.Rect) {
	if spec.Grain == DateByDay {
		weekdays, rest := bounds.CutTop(dateWeekdayHeight)
		for column, name := range dateWeekdays {
			cell := gfx.Rect{
				X: weekdays.X + float32(column)*(dateCellWidth+dateCellGap),
				Y: weekdays.Y, Width: dateCellWidth, Height: weekdays.Height,
			}
			canvas.LabelCenter(name, cell, gfx.TextStyle{
				Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground,
			})
		}
		bounds = rest
	}
	columns, rows := gridShape(spec.Grain)
	width := (bounds.Width - float32(columns-1)*dateCellGap) / float32(columns)
	today := dateText(time.Now())
	visible := state.visibleMonth()
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			index := row*columns + column
			cell := gfx.Rect{
				X:     bounds.X + float32(column)*(width+dateCellGap),
				Y:     bounds.Y + float32(row)*(dateCellHeight+dateCellGap),
				Width: width, Height: dateCellHeight,
			}
			state.paintCell(canvas, theme, spec, cell, index, visible, today)
		}
	}
}

// paintCell paints one cell of the grid and names it for the pointer.
func (state *DatePickerState) paintCell(canvas *gfx.Canvas, theme *gfx.Theme, spec DatePickerSpec,
	cell gfx.Rect, index int, visible time.Time, today string) {
	from, to, label := state.cellValue(spec.Grain, index, visible)
	style := gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.PopoverForeground}
	selected := from == spec.Start || to == spec.End
	inRange := spec.Start != "" && spec.End != "" && from > spec.Start && to < spec.End
	disabled := spec.MaxDate != "" && to > spec.MaxDate
	circleRadius := theme.Metrics.RadiusControl
	switch {
	case disabled:
		style.Color = theme.MutedForeground.WithAlpha(0.5)
	case selected:
		canvas.Rounded(cell, circleRadius, theme.Primary)
		style.Color, style.Weight = theme.PrimaryForeground, gfx.WeightMedium
	case inRange:
		canvas.Rounded(cell, circleRadius, theme.Primary.WithAlpha(0.12))
		style.Color = theme.Primary
	case canvas.Hovered() == state.cells.At(index):
		canvas.Rounded(cell, circleRadius, theme.Hover)
	}
	if !selected && !inRange && !disabled && spec.Grain == DateByDay && from == today {
		canvas.RoundedOutline(cell, circleRadius, 1, theme.Primary)
	}
	if spec.Grain == DateByDay && !state.inVisibleMonth(from, visible) {
		if !selected {
			style.Color = theme.MutedForeground
		}
	}
	canvas.LabelCenter(label, cell, style)
	canvas.HitWithOptions(state.cells.At(index), cell, label, gfx.HitOptions{
		Role: gfx.RoleButton, Disabled: disabled, Selected: selected, Value: from,
	})
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

// paintPresets paints the shortcuts under the grid.
func (state *DatePickerState) paintPresets(canvas *gfx.Canvas, theme *gfx.Theme, spec DatePickerSpec, bounds gfx.Rect) {
	presets := datePresets(spec.Grain, spec.CloseOnPick)
	x := bounds.X
	for index, preset := range presets {
		if preset.label == "" {
			break
		}
		width := canvas.TextWidthWeight(preset.label, theme.Metrics.FontSizeS, gfx.WeightRegular) + 12
		cell := gfx.Rect{X: x, Y: bounds.Y, Width: width, Height: bounds.Height}
		active := spec.Start == preset.from && spec.End == preset.to
		style := gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.Primary}
		switch {
		case active:
			canvas.Rounded(cell, theme.Metrics.RadiusControl, theme.Primary.WithAlpha(0.12))
		case canvas.Hovered() == state.presets[index]:
			canvas.Rounded(cell, theme.Metrics.RadiusControl, theme.Hover)
		}
		canvas.LabelCenter(preset.label, cell, style)
		canvas.HitWithOptions(state.presets[index], cell, preset.label, gfx.HitOptions{
			Role: gfx.RoleButton, Selected: active,
		})
		x += width + dateBlockGap
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

// inVisibleMonth reports whether a date belongs to the month the grid shows.
func (state *DatePickerState) inVisibleMonth(value string, visible time.Time) bool {
	day, ok := parseDate(value)
	return ok && day.Year() == visible.Year() && day.Month() == visible.Month()
}

// gridShape is how many columns and rows a grain's grid has.
func gridShape(grain DateGrain) (columns, rows int) {
	switch grain {
	case DateByMonth:
		return 3, 4
	case DateByYear:
		return 2, 5
	}
	return 7, 6
}

// dateGridHeight is the height of one grain's grid, weekdays included.
func dateGridHeight(grain DateGrain) float32 {
	_, rows := gridShape(grain)
	return float32(rows)*(dateCellHeight+dateCellGap) - dateCellGap + weekdayHeight(grain)
}

// weekdayHeight is the header row above the day grid.
func weekdayHeight(grain DateGrain) float32 {
	if grain == DateByDay {
		return dateWeekdayHeight
	}
	return 0
}

// datePanelHeight is the whole panel's height for a grain.
func datePanelHeight(grain DateGrain) float32 {
	return 2*datePanelPadding + dateHeaderHeight + dateBlockGap + dateGridHeight(grain) +
		dateBlockGap + 1 + dateBlockGap + datePresetHeight
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
	if start > end {
		return start, end
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
