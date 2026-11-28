package datepicker

import (
	"time"

	"github.com/vivaui/viva/ui"
	vivaday "github.com/vivaui/viva/ui/datepicker"
)

// dayLabels are the single-date field's words: the month's name carries its
// number, the week starts on Monday, and the panel's arrows speak Chinese, as the
// rest of the application's calendar does.
var dayLabels = vivaday.Labels{
	Months: [12]string{
		"1月", "2月", "3月", "4月", "5月", "6月",
		"7月", "8月", "9月", "10月", "11月", "12月",
	},
	Weekdays: [7]string{"日", "一", "二", "三", "四", "五", "六"},
	Previous: "上个月",
	Next:     "下个月",
}

// Day is one single-date field: Viva's own date picker wearing this
// application's words and week start, bound to the local YYYY-MM-DD string the
// pages store. Keep it in the model beside the value it edits; its zero value is
// ready to declare.
//
// It is for a field on a page. A field inside a ui.Modal keeps Picker's own day
// grid: in Viva 0.6.10 a ui.Popover opened inside a dialog paints its panel but
// publishes no hit for it, so a press on a day only dismisses the panel.
type Day struct {
	picker *vivaday.DatePicker
	value  time.Time
	// text is the string this field last gave the page, so a paint can tell the
	// page's own change (a new draft, an edit) from its own written-back pick.
	text string
}

// DaySpec is what one single-date field shows and what it may pick.
type DaySpec struct {
	// Label names the field for assistive technology.
	Label string
	// Placeholder is what the field says while no day is chosen.
	Placeholder string
	// Fill stretches the field to its row; Width fixes it instead.
	Fill  bool
	Width float32
	// Min and Max are the first and last days that may be picked, as local
	// YYYY-MM-DD dates; an empty one leaves that end open.
	Min, Max string
	// OnPick receives the day the user picked, after value holds it, for a page
	// that has to load or filter what belongs to that day.
	OnPick func(day string)
}

// Field declares the field showing *value and writes back the day the user picks.
// It is one control high, like every field in a form row.
func (day *Day) Field(c ui.C, value *string, spec DaySpec) ui.Box {
	if day.picker == nil {
		day.picker = vivaday.New().
			Labels(dayLabels).
			FirstWeekday(time.Monday)
	}
	// The page's own string wins whenever it is not the one this field last wrote:
	// a new draft, an edited record, a cleared form. A pick the picker has not
	// reported yet leaves the value it chose alone.
	if *value != day.text {
		day.value, day.text = localDay(*value), *value
	}
	day.picker.
		Placeholder(spec.Placeholder).
		Layout(dateLayout).
		Min(localDay(spec.Min)).
		Max(localDay(spec.Max))
	field := day.picker.View(c, spec.Label, &day.value)
	// The picker reports a pick on the paint after the one that delivered it, and
	// the day it chose is in day.value: hand it to the page here.
	if day.picker.Changed() {
		*value = day.value.Format(dateLayout)
		day.text = *value
		if spec.OnPick != nil {
			spec.OnPick(*value)
		}
	}
	switch {
	case spec.Fill:
		field = field.FillWidth()
	case spec.Width > 0:
		field = field.Width(spec.Width)
	}
	return field
}

// Close forgets the open month, for a page that leaves the screen while the
// field's panel is showing: the date it would reopen on is stale there.
func (day *Day) Close() { day.picker = nil }

// localDay is a local YYYY-MM-DD date as a midnight in the same place, and the
// zero time for an empty or unreadable one.
func localDay(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	day, err := time.ParseInLocation(dateLayout, value, time.Local)
	if err != nil {
		return time.Time{}
	}
	return day
}
