package datepicker

import (
	"strconv"
	"time"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/icons"
)

// calendarPadding is the room components.Calendar keeps inside the bounds it is
// given, so the panel leaves exactly that around the grid it paints.
const calendarPadding float32 = 8

// Panel is the single-date field of a dialog: Viva's calendar inside a panel the
// application floats over the dialog itself.
//
// A page's field takes Day, Viva's own date picker. A dialog's cannot: in Viva
// 0.6.10 the ui.Popover that picker opens tags its hits with a focus scope of its
// own, and the router only accepts the hits of the dialog's trapped scope, so a
// press on a day dismisses the whole dialog. This panel paints on the layer a
// dialog's select list and tag picker already use, so its day cells are the
// page's own identities and take presses inside the dialog. What it shows is
// still Viva's calendar: the month caption and its arrows, the weekday row and
// the day grid are components.Calendar's. Keep it in the model beside the value
// it edits; its zero value is ready to declare.
type Panel struct {
	// Open says whether the panel is showing.
	Open bool
	// Visible is the first day of the month the grid shows.
	Visible time.Time

	dismiss gfx.ID
	prev    gfx.ID
	next    gfx.ID
	cells   [gridColumns * gridRows]gfx.ID
}

// PanelSpec is what one dialog date field shows and what a pick does.
type PanelSpec struct {
	// Placeholder is what the field says while no day is chosen.
	Placeholder string
	// Fill stretches the field to its form row; Width fixes it instead.
	Fill  bool
	Width float32
	// Viewport is the page the panel must stay inside.
	Viewport gfx.Rect
	// OnPick receives the day the user picked, after value holds it, for a
	// dialog that has to load or filter what belongs to that day.
	OnPick func(day string)
}

// Field declares the field showing *value and writes back the day the user picks.
// It is one control high, like every field in a form row.
func (panel *Panel) Field(c ui.C, value *string, spec PanelSpec) ui.Box {
	panel.bind(c)
	text := spec.Placeholder
	if *value != "" {
		text = *value
	}
	field := ui.Button(c, text).Outline().Icon(icons.Calendar)
	if *value == "" {
		field = field.Muted()
	}
	switch {
	case spec.Fill:
		field = field.FillWidth()
	case spec.Width > 0:
		field = field.Width(spec.Width)
	}
	if field.Clicked() {
		panel.toggle(*value)
	}
	field.DrawOver(func(d *ui.DrawContext) { panel.paint(d, spec, value, d.Rect) })
	return field
}

// Key moves the grid's focus by the arrows, as the panel's own controls would.
// The dialog gives it the keys it gives its other controls, before it handles
// them itself.
func (panel *Panel) Key(event gfx.KeyEvent) gfx.EventResult {
	if !panel.Open || event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}
	}
	at := -1
	for index := range panel.cells {
		if panel.cells[index] == event.Target {
			at = index
			break
		}
	}
	if at < 0 {
		return gfx.EventResult{}
	}
	columns := gridColumns
	next := at
	switch event.Key {
	case gfx.KeyLeft:
		next = max(at-1, at-at%columns)
	case gfx.KeyRight:
		next = min(at+1, at-at%columns+columns-1)
	case gfx.KeyUp:
		next = max(at-columns, 0)
	case gfx.KeyDown:
		next = min(at+columns, len(panel.cells)-1)
	case gfx.KeyHome:
		next = at - at%columns
	case gfx.KeyEnd:
		next = min(at-at%columns+columns-1, len(panel.cells)-1)
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true, Focus: panel.cells[next], FocusVisible: true}
}

// Close forgets the open month, for a dialog that closes while the panel is
// showing: the day it would reopen on is stale there.
func (panel *Panel) Close() { panel.Open = false }

// bind names the panel's hit identities for this paint, including the grid
// cells, so their hits belong to the page tree.
func (panel *Panel) bind(c ui.C) {
	panel.dismiss = ui.ID(c)
	panel.prev = ui.ID(c)
	panel.next = ui.ID(c)
	for index := range panel.cells {
		panel.cells[index] = ui.ID(c)
	}
}

// toggle opens or closes the panel, showing the month of the chosen date.
func (panel *Panel) toggle(value string) {
	panel.Open = !panel.Open
	if panel.Open {
		panel.Visible = monthOf(value)
	}
}

// visibleMonth is the month the panel shows, in the place the grid's dates are
// read in.
func (panel *Panel) visibleMonth() time.Time {
	if panel.Visible.IsZero() {
		return monthOf("")
	}
	return time.Date(panel.Visible.Year(), panel.Visible.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// paint draws the panel under its trigger and applies a press on it.
func (panel *Panel) paint(d *ui.DrawContext, spec PanelSpec, value *string, anchor gfx.Rect) {
	if !panel.Open {
		return
	}
	canvas, theme := d.Canvas, *d.Theme
	metrics := theme.Metrics
	days := panel.days(*value)
	bounds := panel.place(canvas, theme, days, spec.Viewport, anchor)
	// The panel floats over the dialog's own layer. A press anywhere else closes
	// it; the panel's own hits are declared after that backdrop, so they win.
	canvas.PushLayer(datePanelLayer)
	defer canvas.PopLayer()
	canvas.HitWithOptions(panel.dismiss, spec.Viewport, "关闭日期选择", gfx.HitOptions{
		Role: gfx.RoleButton, SkipFocus: true, Backdrop: true,
		PassPrimary: true, PassSecondary: true,
	})

	// The surface is the framework's popup look, so the panel matches a menu.
	look := viva.PopupRecipe(theme)
	canvas.Elevate(bounds, look.Radius, look.Shadow)
	canvas.Rounded(bounds, look.Radius, look.Fill)
	canvas.RoundedOutline(bounds, look.Radius, 1, look.Border)

	viva.BeginCalendar(canvas, theme, bounds.Inset(gfx.All(metrics.PadSurface)), viva.CalendarOptions{
		MonthTitle:    panel.title(),
		Weekdays:      dateWeekdays,
		Days:          days,
		PreviousID:    panel.prev,
		NextID:        panel.next,
		PreviousLabel: "上个月",
		NextLabel:     "下个月",
	}).Paint()
	panel.handle(d, spec, value)
}

// days is the month the panel shows, as Viva's calendar takes it: six weeks
// starting on the Monday of the week the month begins in, so the panel never
// changes height with the month.
func (panel *Panel) days(value string) []viva.CalendarDay {
	visible := panel.visibleMonth()
	start := dayGridStart(visible)
	today := dateText(time.Now())
	days := make([]viva.CalendarDay, 0, gridColumns*gridRows)
	for index := 0; index < gridColumns*gridRows; index++ {
		date := start.AddDate(0, 0, index)
		text := dateText(date)
		days = append(days, viva.CalendarDay{
			ID:                 panel.cells[index],
			Label:              strconv.Itoa(date.Day()),
			AccessibilityLabel: spokenDay(date),
			Value:              text,
			Selected:           text == value,
			Today:              text == today,
			Outside:            date.Month() != visible.Month() || date.Year() != visible.Year(),
		})
	}
	return days
}

// place works out where the panel goes: under its trigger, over it when the
// window has no room below, and inside the viewport.
func (panel *Panel) place(canvas *gfx.Canvas, theme gfx.Theme, days []viva.CalendarDay,
	viewport, anchor gfx.Rect) gfx.Rect {
	metrics := theme.Metrics
	cell := max(metrics.ControlHeight-4, 0)
	width := float32(gridColumns)*cell + 2*calendarPadding + 2*metrics.PadSurface
	// The calendar knows how tall a month is at the width it is given; the panel
	// adds its own padding around it.
	height := viva.BeginCalendar(canvas, theme, gfx.Rect{Width: width - 2*metrics.PadSurface},
		viva.CalendarOptions{Days: days}).Bounds().Height + 2*metrics.PadSurface
	y := anchor.Y + anchor.Height + metrics.GapItem
	if viewport.Height > 0 {
		if y+height > viewport.Bottom() {
			y = anchor.Y - metrics.GapItem - height
		}
		y = max(viewport.Y, min(y, viewport.Bottom()-height))
	}
	x := anchor.X
	if viewport.Width > 0 {
		x = min(x, max(viewport.Right()-width, viewport.X))
	}
	return gfx.Rect{X: x, Y: y, Width: width, Height: height}
}

// title is the month the grid shows, in the words of the application's other
// calendars: a number and its unit, then the year.
func (panel *Panel) title() string {
	visible := panel.visibleMonth()
	return strconv.Itoa(int(visible.Month())) + "月 " + strconv.Itoa(visible.Year())
}

// spokenDay is a day spelled out for a screen reader: a grid of "5"s says
// nothing about which month it is in.
func spokenDay(day time.Time) string {
	return strconv.Itoa(day.Year()) + " 年 " + strconv.Itoa(int(day.Month())) + " 月 " +
		strconv.Itoa(day.Day()) + " 日"
}

// dayGridStart is the Monday of the week the month begins in.
func dayGridStart(month time.Time) time.Time {
	return month.AddDate(0, 0, -((int(month.Weekday()) + 6) % 7))
}

// handle applies the press this paint delivered, if any.
func (panel *Panel) handle(d *ui.DrawContext, spec PanelSpec, value *string) {
	canvas := d.Canvas
	changed := false
	switch {
	case canvas.Clicked(panel.dismiss):
		panel.Open, changed = false, true
	case canvas.Clicked(panel.prev):
		panel.Visible = panel.visibleMonth().AddDate(0, -1, 0)
		changed = true
	case canvas.Clicked(panel.next):
		panel.Visible = panel.visibleMonth().AddDate(0, 1, 0)
		changed = true
	default:
		start := dayGridStart(panel.visibleMonth())
		for index := range panel.cells {
			if !canvas.Clicked(panel.cells[index]) {
				continue
			}
			*value = dateText(start.AddDate(0, 0, index))
			panel.Open = false
			if spec.OnPick != nil {
				spec.OnPick(*value)
			}
			changed = true
			break
		}
	}
	if changed {
		canvas.RequestFrame()
	}
}
