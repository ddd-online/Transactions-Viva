// Package datepicker is the app's date field and range picker: a trigger that
// shows the value next to a clock, and the panel under it with a day grid, a
// month grid or a decade of years. The parameters decide its chrome, so a
// toolbar's range and a form's single date are the same component.
//
// Pages take a toolbar's single date from Day: Viva's own date field wearing this
// package's words, one day and one month grid. A dialog's single date takes Panel:
// Viva's calendar in the panel the dialog floats itself, because Viva 0.6.10's
// popover takes no press inside a ui.Modal. Picker keeps its own grids for the
// range fields and for the dialogs Panel has not replaced yet.
package datepicker

import (
	"strconv"
	"time"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/icons"
)

// dateLayout is the local calendar date form the whole app stores dates in.
const dateLayout = "2006-01-02"

// Grain is the unit a date range is picked in: a day, a month or a year.
type Grain uint8

const (
	// ByDay picks single days.
	ByDay Grain = iota
	// ByMonth picks whole months and shows the panel as twelve cells.
	ByMonth
	// ByYear picks whole years and shows the panel as a decade.
	ByYear
)

// Name is the grain's caption in the picker's 日 / 月 / 年 choice.
func (grain Grain) Name() string {
	switch grain {
	case ByMonth:
		return "月"
	case ByYear:
		return "年"
	}
	return "日"
}

// dateGrains are the grain choices, in the picker's order.
var dateGrains = [...]Grain{ByDay, ByMonth, ByYear}

// dateWeekdays are the day grid's columns; the week starts on Monday.
var dateWeekdays = [gridColumns]string{"一", "二", "三", "四", "五", "六", "日"}

const (
	// gridColumns is the day grid's column count.
	gridColumns = 7
	// gridRows is how many weeks the day grid always shows, so the panel does not
	// change height with the month.
	gridRows = 6
	// datePresetCount is the most shortcuts a grain offers.
	datePresetCount = 5
	// datePanelLayer puts the panel over the page and over a dialog's surface: a
	// dialog paints on a layer of its own, so a panel painted inside one only
	// shows and only takes presses above it. The tag picker uses the same layer.
	datePanelLayer uint16 = 20
)

// State is one picker's own state: whether its panel is open, the
// month it shows, and the identities the painted panel is hit by. Keep it in the
// page's model, one per picker; build it with NewState.
type State struct {
	// Open says whether the panel is showing.
	Open bool
	// Visible is the first day of the month the day grid shows.
	Visible time.Time

	cells   [gridColumns * gridRows]gfx.ID
	dismiss gfx.ID
	prev    gfx.ID
	next    gfx.ID
	// yearPrev and yearNext are the day grid's whole-year arrows; every grain
	// has the month (or period) arrows above the grid instead.
	yearPrev, yearNext gfx.ID
	presets            [datePresetCount]gfx.ID

	// columns and cellCount are the grid as the last paint laid it out. The
	// arrow keys read them, so a panel needs no key route of its own beyond the
	// page's: the page gives it the keys it would give any control.
	columns   int
	cellCount int
}

// NewState returns an empty picker state. Its cell identities are bound from
// the page on each paint, so the panel's buttons take presses like any other
// control.
func NewState() State { return State{} }

// Spec is what one picker shows and what a pick does.
type Spec struct {
	// Label names the control for assistive technology.
	Label string
	// Start and End are the chosen range as local YYYY-MM-DD dates; End is empty
	// while the range still waits for its second end.
	Start, End string
	// Grain is the unit the range is picked in. Grains shows the 日 / 月 / 年
	// choice, so the user can change it; Periods shows the arrows that move the
	// range a whole period. A form's single date leaves both off and keeps Grain
	// at ByDay, so it is one field and one month grid.
	Grain   Grain
	Grains  bool
	Periods bool
	// Fill stretches the trigger to its container, for a form's field row.
	Fill bool
	// Width fixes the trigger's width. Zero uses the widest range of any grain,
	// so a range field keeps one width as its value grows. A caller that shows
	// only one small value (a page's year) sets a narrower width.
	Width float32
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
	OnGrain func(grain Grain)
	OnRange func(start, end string)
}

// Picker is a date field: the value, a clock icon, and the panel under it.
// The parameters decide its chrome: a toolbar's range shows the 日 / 月 / 年
// choice and the period arrows, a form's single date shows neither.
//
// It is built from the theme's own controls, so it looks like the buttons
// beside it.
func Picker(c ui.C, state *State, spec Spec) {
	state.bind(c)
	theme := c.Theme()
	metrics := theme.Metrics
	ui.Row(c).Gap(metrics.GapItem).Items(ui.Center).Children(func() {
		if spec.Grains {
			grain := spec.Grain
			if ui.Choice(c, &grain, dateGrains[:], Grain.Name).Changed() {
				start, end := normalizeRange(spec.Start, spec.End, grain)
				if spec.OnRange != nil && (start != spec.Start || end != spec.End) {
					spec.OnRange(start, end)
				}
				if spec.OnGrain != nil {
					spec.OnGrain(grain)
				}
			}
		}
		if spec.Periods {
			if ui.Button(c, "上一周期").Ghost().Icon(icons.ChevronLeft).IconOnly().Clicked() {
				state.shift(spec, -1)
			}
		}
		field := ui.Button(c, triggerText(spec)).Outline().Icon(icons.Calendar)
		if spec.Fill {
			field = field.FillWidth()
		} else if spec.Width > 0 {
			field = field.Width(spec.Width)
		} else {
			field = field.Width(fieldWidth(c.Canvas(), theme))
		}
		if field.Clicked() {
			state.toggle(spec.Start)
		}
		field.DrawOver(func(d *ui.DrawContext) { state.paint(d, spec, d.Rect) })
		if spec.Periods {
			if ui.Button(c, "下一周期").Ghost().Icon(icons.ChevronRight).IconOnly().Clicked() {
				state.shift(spec, 1)
			}
		}
	})
}

// fieldWidth is the width the widest value of any grain fits in, so the field
// keeps one width while the range grows or the grain changes.
func fieldWidth(canvas *gfx.Canvas, theme *gfx.Theme) float32 {
	width := float32(0)
	for _, sample := range [...]string{
		"2026-11-04 ~ 2026-11-30",
		"2026-11 ~ 2026-12",
		"2026 ~ 2027",
	} {
		button := viva.BeginButton(canvas, *theme, gfx.ID{}, gfx.Rect{}).
			WithIcon(icons.Calendar).SizeToFit(sample)
		width = max(width, button.BoundsWidth())
	}
	return width
}

// bind names the panel's hit identities for this paint, including the grid
// cells, so their button hits belong to the page tree.
func (state *State) bind(c ui.C) {
	state.dismiss = ui.ID(c)
	state.prev = ui.ID(c)
	state.next = ui.ID(c)
	state.yearPrev = ui.ID(c)
	state.yearNext = ui.ID(c)
	for index := range state.presets {
		state.presets[index] = ui.ID(c)
	}
	for index := range state.cells {
		state.cells[index] = ui.ID(c)
	}
}

// toggle opens or closes the panel, showing the month of the chosen date.
func (state *State) toggle(value string) {
	state.Open = !state.Open
	if state.Open {
		state.Visible = monthOf(value)
	}
}

// shift moves the range one period of its grain back or forward.
func (state *State) shift(spec Spec, direction int) {
	if spec.Start == "" || spec.End == "" || spec.OnRange == nil {
		return
	}
	start, end := shiftPeriod(spec.Start, spec.End, spec.Grain, direction)
	if spec.MaxDate != "" && (start > spec.MaxDate || end > spec.MaxDate) {
		return
	}
	spec.OnRange(start, end)
}

// layout is where the panel's blocks go, measured from the theme's own controls.
type layout struct {
	bounds  gfx.Rect // the panel surface
	inner   gfx.Rect // inside its padding
	caption gfx.Rect // the title between the arrows
	grid    gfx.Rect // the cells, weekday row included
	cell    float32  // a cell's height
	rowStep float32  // a cell and the gap under it
	gap     float32  // between columns and rows
	presets gfx.Rect // the shortcuts row
}

// paint draws the panel under its trigger and applies a press on it.
func (state *State) paint(d *ui.DrawContext, spec Spec, anchor gfx.Rect) {
	if !state.Open {
		return
	}
	canvas, theme := d.Canvas, *d.Theme
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

	state.paintCaption(canvas, theme, spec, place)
	state.paintGrid(canvas, theme, spec, place)
	state.paintPresets(canvas, theme, spec, place)
	state.handle(d, spec)
}

// measure works out where each block of the panel goes, from the theme's button
// and text metrics.
func (state *State) measure(canvas *gfx.Canvas, theme gfx.Theme, spec Spec, anchor gfx.Rect) layout {
	metrics := theme.Metrics
	cell := viva.BeginButton(nil, theme, gfx.ID{}, gfx.Rect{}).BoundsHeight()
	gap := metrics.GapItem
	rowStep := cell + gap
	_, rows := gridShape(spec.Grain)
	gridHeight := float32(rows)*rowStep - gap
	if spec.Grain == ByDay {
		gridHeight += metrics.FontSizeS + gap // the weekday row
	}

	cellWidth := float32(metrics.ControlHeight)
	// Two-digit days must fit inside a cell's button padding, not just inside
	// the column: a 32px cell with the default 10px side padding leaves 12px
	// for the label and clips "28" / "30". Widen the whole panel to the label's
	// measured width once, and every grain uses those wider cells.
	if sample := viva.BeginButton(canvas, theme, gfx.ID{}, gfx.Rect{}).SizeToFit("00"); sample.BoundsWidth() > cellWidth {
		cellWidth = sample.BoundsWidth()
	}
	width := float32(gridColumns)*cellWidth + float32(gridColumns-1)*gap
	presetsHeight := viva.BeginButton(nil, theme, gfx.ID{}, gfx.Rect{}).Small().BoundsHeight()
	presetsWidth := float32(0)
	for _, preset := range datePresets(spec.Grain, spec.CloseOnPick) {
		if preset.label == "" {
			break
		}
		presetsWidth += viva.BeginButton(canvas, theme, gfx.ID{}, gfx.Rect{Width: 4096, Height: presetsHeight}).
			Small().SizeToFit(preset.label).BoundsWidth()
		presetsWidth += gap
	}
	width = max(width, presetsWidth-gap) + 2*metrics.PadSurface
	height := 2*metrics.PadSurface + cell + gap + gridHeight + gap + presetsHeight
	y := anchor.Y + anchor.Height + gap
	if spec.Viewport.Height > 0 {
		if y+height > spec.Viewport.Bottom() {
			// A form above a short window would otherwise open the panel off
			// screen; flip it over the field and keep it inside the viewport.
			y = anchor.Y - gap - height
		}
		y = max(spec.Viewport.Y, min(y, spec.Viewport.Bottom()-height))
	}
	bounds := gfx.Rect{
		X:      min(anchor.X, max(spec.Viewport.Right()-width, 0)),
		Y:      y,
		Width:  width,
		Height: height,
	}
	inner := bounds.Inset(gfx.All(metrics.PadSurface))
	return layout{
		bounds:  bounds,
		inner:   inner,
		caption: gfx.Rect{X: inner.X, Y: inner.Y, Width: inner.Width, Height: cell},
		grid:    gfx.Rect{X: inner.X, Y: inner.Y + cell + gap, Width: inner.Width, Height: gridHeight},
		cell:    cell,
		rowStep: rowStep,
		gap:     gap,
		presets: gfx.Rect{X: inner.X, Y: bounds.Bottom() - metrics.PadSurface - presetsHeight, Width: inner.Width, Height: presetsHeight},
	}
}

// paintCaption paints the period title between the arrows every grain has. The
// day grid also gets a pair of double chevrons outside them: the single arrows
// move a month, the double ones a whole year.
func (state *State) paintCaption(canvas *gfx.Canvas, theme gfx.Theme, spec Spec, place layout) {
	visible := state.visibleMonth()
	title, previous, next := "", "上个月", "下个月"
	switch spec.Grain {
	case ByMonth:
		title, previous, next = strconv.Itoa(visible.Year())+" 年", "上一年", "下一年"
	case ByYear:
		title = strconv.Itoa(state.decade()) + " – " + strconv.Itoa(state.decade()+9)
		previous, next = "上一个十年", "下一个十年"
	default:
		title = strconv.Itoa(visible.Year()) + " 年 " + strconv.Itoa(int(visible.Month())) + " 月"
	}
	arrow := gfx.Rect{Y: place.caption.Y, Width: place.cell, Height: place.caption.Height}
	arrowGap := place.gap / 2
	titleInset := place.cell
	arrow.X = place.caption.X
	if spec.Grain == ByDay {
		viva.BeginButton(canvas, theme, state.yearPrev, arrow).
			Icon().WithIcon(icons.ChevronsLeft).Ghost("上一年")
		arrow.X += place.cell + arrowGap
		titleInset += place.cell + arrowGap
	}
	viva.BeginButton(canvas, theme, state.prev, arrow).
		Icon().WithIcon(icons.ChevronLeft).Ghost(previous)
	arrow.X = place.caption.Right() - place.cell
	if spec.Grain == ByDay {
		viva.BeginButton(canvas, theme, state.yearNext, arrow).
			Icon().WithIcon(icons.ChevronsRight).Ghost("下一年")
		arrow.X -= place.cell + arrowGap
	}
	viva.BeginButton(canvas, theme, state.next, arrow).
		Icon().WithIcon(icons.ChevronRight).Ghost(next)
	canvas.LabelCenter(title, gfx.Rect{
		X: place.caption.X + titleInset, Y: place.caption.Y,
		Width: place.caption.Width - 2*titleInset, Height: place.caption.Height,
	}, gfx.TextStyle{Size: typography.SizeBody, Weight: gfx.WeightMedium, Color: theme.Foreground})
}

// paintGrid paints the one grid of the grain: days under their weekdays, or the
// months, or the decade. Every cell is the theme's button recipe, so the three
// grains share one look and one set of states.
func (state *State) paintGrid(canvas *gfx.Canvas, theme gfx.Theme, spec Spec, place layout) {
	metrics := theme.Metrics
	columns, rows := gridShape(spec.Grain)
	cellWidth := (place.grid.Width - float32(columns-1)*place.gap) / float32(columns)
	top := place.grid.Y
	if spec.Grain == ByDay {
		weekdays := gfx.Rect{X: place.grid.X, Y: top, Width: place.grid.Width, Height: metrics.FontSizeS + place.gap}
		for column, name := range dateWeekdays {
			cell := gfx.Rect{
				X: weekdays.X + float32(column)*(cellWidth+place.gap),
				Y: weekdays.Y, Width: cellWidth, Height: weekdays.Height,
			}
			canvas.LabelCenter(name, cell, gfx.TextStyle{
				Size: typography.SizeCaption, Color: theme.MutedForeground,
			})
		}
		top += weekdays.Height
	}
	visible := state.visibleMonth()
	today := dateText(time.Now())
	state.columns, state.cellCount = columns, columns*rows
	// One stop of Tab for the whole grid: the cell the keyboard is in, else the
	// chosen end, else today. The arrows walk the rest (see Key).
	tab := state.tabCell(canvas, spec, columns*rows)
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			index := row*columns + column
			cell := gfx.Rect{
				X:     place.grid.X + float32(column)*(cellWidth+place.gap),
				Y:     top + float32(row)*place.rowStep,
				Width: cellWidth, Height: place.cell,
			}
			from, to, label := state.cellValue(spec.Grain, index, visible)
			state.paintCell(canvas, theme, spec, cell, index, from, to, label, today, index != tab)
		}
	}
}

// paintCell paints one cell as the theme's button: filled for the ends of the
// range, muted inside it, bordered for today, plain otherwise.
func (state *State) paintCell(canvas *gfx.Canvas, theme gfx.Theme, spec Spec,
	cell gfx.Rect, index int, from, to, label, today string, skipTab bool) {
	// The button paints; the hit is the cell's, declared below. A ghost cell's
	// own hit would be as wide as its label, so pressing beside the digits would
	// miss — and the cell's name would be a bare number.
	id := state.cells[index]
	button := viva.BeginButton(canvas, theme, gfx.ID{}, cell).WithState(viva.ButtonState{
		Hovered: canvas.Hovered() == id,
		Pressed: canvas.Pressed() == id,
		Focused: canvas.FocusVisible() == id,
	})
	disabled := false
	switch {
	case spec.MaxDate != "" && to > spec.MaxDate:
		disabled = true
		button.Disabled().Ghost(label)
	case from == spec.Start || to == spec.End:
		button.Default(label)
	case spec.Start != "" && spec.End != "" && from > spec.Start && to < spec.End:
		button.Secondary(label)
	case spec.Grain == ByDay && from == today:
		button.Outline(label)
	default:
		button.Ghost(label)
	}
	canvas.HitWithOptions(id, cell, state.cellName(spec.Grain, from, label), gfx.HitOptions{
		Role: gfx.RoleButton, Disabled: disabled, SkipTab: skipTab,
	})
}

// cellName is the day, month or year the cell stands for, spelled out: a grid of
// "5"s says nothing to a screen reader about which month it is in.
func (state *State) cellName(grain Grain, from, label string) string {
	day, ok := parseDate(from)
	if !ok {
		return label
	}
	switch grain {
	case ByYear:
		return strconv.Itoa(day.Year()) + " 年"
	case ByMonth:
		return strconv.Itoa(day.Year()) + " 年 " + strconv.Itoa(int(day.Month())) + " 月"
	}
	return strconv.Itoa(day.Year()) + " 年 " + strconv.Itoa(int(day.Month())) + " 月 " +
		strconv.Itoa(day.Day()) + " 日"
}

// tabCell is the grid's one stop of Tab: the cell the keyboard is on, else the
// chosen end, else today, else the first cell.
func (state *State) tabCell(canvas *gfx.Canvas, spec Spec, count int) int {
	for index := 0; index < count && index < len(state.cells); index++ {
		if canvas.FocusVisible() == state.cells[index] {
			return index
		}
	}
	visible := state.visibleMonth()
	today := dateText(time.Now())
	for index := 0; index < count && index < len(state.cells); index++ {
		from, to, _ := state.cellValue(spec.Grain, index, visible)
		if from == spec.Start || to == spec.End ||
			(spec.Grain == ByDay && from == today) || (spec.Grain != ByDay && from == spec.Start) {
			return index
		}
	}
	return 0
}

// Key moves the grid's focus by the arrows. The page gives it the keys it gives
// its other controls, before it handles them itself. Home and End go to the ends
// of the row, and a key at the edge keeps the focus where it is.
func (state *State) Key(event gfx.KeyEvent) gfx.EventResult {
	if !state.Open || event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}
	}
	at := -1
	for index := 0; index < state.cellCount && index < len(state.cells); index++ {
		if state.cells[index] == event.Target {
			at = index
			break
		}
	}
	if at < 0 {
		return gfx.EventResult{}
	}
	columns := max(state.columns, 1)
	count := state.cellCount
	next := at
	switch event.Key {
	case gfx.KeyLeft:
		next = max(at-1, at-at%columns)
	case gfx.KeyRight:
		next = min(at+1, at-at%columns+columns-1)
	case gfx.KeyUp:
		next = max(at-columns, 0)
	case gfx.KeyDown:
		next = min(at+columns, count-1)
	case gfx.KeyHome:
		next = at - at%columns
	case gfx.KeyEnd:
		next = min(at-at%columns+columns-1, count-1)
	default:
		return gfx.EventResult{}
	}
	if next < 0 || next >= len(state.cells) {
		next = at
	}
	return gfx.EventResult{Handled: true, Repaint: true, Focus: state.cells[next], FocusVisible: true}
}

// paintPresets paints the shortcuts under the grid as the theme's link buttons.
func (state *State) paintPresets(canvas *gfx.Canvas, theme gfx.Theme, spec Spec, place layout) {
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
func (state *State) handle(d *ui.DrawContext, spec Spec) {
	canvas := d.Canvas
	changed := false
	switch {
	case canvas.Clicked(state.dismiss):
		state.Open, changed = false, true
	case canvas.Clicked(state.yearPrev):
		state.stepYear(-1)
		changed = true
	case canvas.Clicked(state.yearNext):
		state.stepYear(1)
		changed = true
	case canvas.Clicked(state.prev):
		state.step(spec, -1)
		changed = true
	case canvas.Clicked(state.next):
		state.step(spec, 1)
		changed = true
	default:
		for index := range state.cells {
			if canvas.Clicked(state.cells[index]) {
				state.pickCell(spec, index)
				changed = true
				break
			}
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
func (state *State) step(spec Spec, direction int) {
	visible := state.visibleMonth()
	switch spec.Grain {
	case ByMonth:
		state.Visible = visible.AddDate(direction, 0, 0)
	case ByYear:
		state.Visible = visible.AddDate(direction*10, 0, 0)
	default:
		state.Visible = visible.AddDate(0, direction, 0)
	}
}

// stepYear moves the day grid a whole year, keeping the month it shows.
func (state *State) stepYear(direction int) {
	state.Visible = state.visibleMonth().AddDate(direction, 0, 0)
}

// pickCell applies a press on one cell of the grid.
func (state *State) pickCell(spec Spec, index int) {
	visible := state.visibleMonth()
	from, to, _ := state.cellValue(spec.Grain, index, visible)
	if spec.Grain == ByDay {
		day, ok := parseDate(from)
		if ok && (day.Month() != visible.Month() || day.Year() != visible.Year()) {
			state.Visible = monthOf(from)
		}
	}
	state.pickRange(spec, from, to)
}

// pickPreset applies a shortcut: it always names both ends, and a form's field
// takes the day it starts on and closes.
func (state *State) pickPreset(spec Spec, from, to string) {
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
func (state *State) pickRange(spec Spec, from, to string) {
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
func (state *State) cellValue(grain Grain, index int, visible time.Time) (string, string, string) {
	switch grain {
	case ByMonth:
		from, to := monthSpan(visible.Year(), time.Month(index+1))
		return from, to, strconv.Itoa(index+1) + " 月"
	case ByYear:
		from, to := yearSpan(state.decade() + index)
		return from, to, strconv.Itoa(state.decade()+index) + " 年"
	}
	day := state.gridStart(visible).AddDate(0, 0, index)
	date := dateText(day)
	return date, date, strconv.Itoa(day.Day())
}

// visibleMonth is the month the panel shows, or this month before it was set.
func (state *State) visibleMonth() time.Time {
	if state.Visible.IsZero() || state.Visible.Year() < 2 {
		return monthOf("")
	}
	return time.Date(state.Visible.Year(), state.Visible.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// decade is the first year of the ten the year grid shows.
func (state *State) decade() int {
	return state.visibleMonth().Year() / 10 * 10
}

// gridShape is how many columns and rows a grain's grid has.
func gridShape(grain Grain) (columns, rows int) {
	switch grain {
	case ByMonth:
		return 3, 4
	case ByYear:
		return 2, 5
	}
	return gridColumns, gridRows
}

// gridStart is the Monday of the first week the month's grid shows.
func (state *State) gridStart(visible time.Time) time.Time {
	offset := (int(visible.Weekday()) + 6) % 7
	return visible.AddDate(0, 0, -offset)
}

// datePreset is one shortcut under the grid: its label and the range it picks.
type datePreset struct {
	label, from, to string
}

// datePresets are the shortcuts of a grain, as the picker offered them. A
// field that picks one day offers 今天 alone.
func datePresets(grain Grain, single bool) [datePresetCount]datePreset {
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
	case ByMonth:
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
	case ByYear:
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
// finished range starts a new one with it (the picker's rule).
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
func normalizeRange(start, end string, grain Grain) (string, string) {
	if grain == ByDay || start == "" || end == "" {
		return start, end
	}
	from, fromOK := parseDate(start)
	to, toOK := parseDate(end)
	if !fromOK || !toOK {
		return start, end
	}
	switch grain {
	case ByMonth:
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
func shiftPeriod(start, end string, grain Grain, direction int) (string, string) {
	switch grain {
	case ByMonth, ByYear:
		from, fromOK := parseDate(start)
		to, toOK := parseDate(end)
		if !fromOK || !toOK {
			return start, end
		}
		if grain == ByMonth {
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

// triggerText is what the field shows: the picked value, or the placeholder
// while nothing is chosen. A range still waiting for its second end ends with a
// tilde; a single-date field always shows one day.
func triggerText(spec Spec) string {
	if spec.Start == "" {
		if spec.Placeholder != "" {
			return spec.Placeholder
		}
		return "请选择时间范围"
	}
	from := valueLabel(spec.Start, spec.Grain)
	if spec.CloseOnPick {
		return from
	}
	if spec.End == "" {
		return from + " ~"
	}
	to := valueLabel(spec.End, spec.Grain)
	if from == to {
		return from
	}
	return from + " ~ " + to
}

// valueLabel is one end of the range at the grain's precision.
func valueLabel(value string, grain Grain) string {
	day, ok := parseDate(value)
	if !ok {
		return value
	}
	switch grain {
	case ByMonth:
		return day.Format("2006-01")
	case ByYear:
		return strconv.Itoa(day.Year())
	}
	return value
}
