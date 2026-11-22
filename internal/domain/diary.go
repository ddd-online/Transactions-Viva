package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// 日记：每个账本每个本地日期一篇，正文是纯文本，心情存一个 emoji。
//
// 三条规则：
//
//   - 字数按 Unicode 码点算（不是字节，也不是字素簇），与
//     `Array.from(text).length` 同口径；
//   - 现有数据里的中文心情（烦躁 / 焦虑 / 难过 / 平静 / 开心）读出时映射到同一套
//     emoji，写回时一律写 emoji；
//   - 日期是严格的 `YYYY-MM-DD`：恰好十个 ASCII 字符、零填充、日期真实存在
//     （`2026-1-01`、`2026/01/01`、`2026-02-29` 都不合法）。

// DiaryDateLayout is the local calendar date form a diary entry is keyed by.
const DiaryDateLayout = "2006-01-02"

// DiaryMood is one choice of the mood picker: the emoji that is stored and the
// Chinese name the button and the tooltip read.
type DiaryMood struct {
	Emoji string
	Label string
}

// DiaryMoods are the five moods in the picker's order. The emoji is what
// the database stores; the label is only ever shown.
var DiaryMoods = [...]DiaryMood{
	{Emoji: "😤", Label: "烦躁"},
	{Emoji: "😰", Label: "焦虑"},
	{Emoji: "😢", Label: "难过"},
	{Emoji: "😐", Label: "平静"},
	{Emoji: "😊", Label: "开心"},
}

// chineseDiaryMoods maps stored Chinese mood labels onto the emoji the
// picker shows. It is read-only history: writes always store the emoji.
var chineseDiaryMoods = map[string]string{
	"烦躁": "😤",
	"焦虑": "😰",
	"难过": "😢",
	"平静": "😐",
	"开心": "😊",
}

// NormalizeDiaryMood resolves a stored mood to the emoji the page paints: a
// Chinese name becomes its emoji, anything else (an emoji, an unknown
// string, "") is returned unchanged.
func NormalizeDiaryMood(value string) string {
	if emoji, ok := chineseDiaryMoods[value]; ok {
		return emoji
	}
	return value
}

// DiaryMoodLabel is the Chinese name of an emoji, for accessible names and
// tooltips. An unknown mood returns "".
func DiaryMoodLabel(emoji string) string {
	emoji = NormalizeDiaryMood(emoji)
	for _, mood := range DiaryMoods {
		if mood.Emoji == emoji {
			return mood.Label
		}
	}
	return ""
}

// DiaryWordCount is the text's length in Unicode code points, the figure the
// page counted beside the editor.
func DiaryWordCount(text string) int { return utf8.RuneCountInString(text) }

// ParseDiaryDate reads a strict local date. ok is false for anything but an
// exactly ten character YYYY-MM-DD naming a real day, so a hand-typed date in
// an import file name or a URL cannot reach the database.
func ParseDiaryDate(value string) (time.Time, bool) {
	if len(value) != len(DiaryDateLayout) {
		return time.Time{}, false
	}
	parsed, err := time.ParseInLocation(DiaryDateLayout, value, time.Local)
	if err != nil || parsed.Format(DiaryDateLayout) != value {
		return time.Time{}, false
	}
	return parsed, true
}

// IsDiaryDate reports whether value is a strict local date.
func IsDiaryDate(value string) bool {
	_, ok := ParseDiaryDate(value)
	return ok
}

// DiaryDateCn names a date the way the editor's heading does: 2026年1月15日.
// A date that does not parse is returned as written.
func DiaryDateCn(value string) string {
	parsed, ok := ParseDiaryDate(value)
	if !ok {
		return value
	}
	return fmt.Sprintf("%d年%d月%d日", parsed.Year(), int(parsed.Month()), parsed.Day())
}

// diaryWeekdays are the week's Chinese names, indexed by time.Weekday.
var diaryWeekdays = [...]string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}

// DiaryWeekdayCn is the weekday of a date, 星期四 style; "" for a date that
// does not parse.
func DiaryWeekdayCn(value string) string {
	parsed, ok := ParseDiaryDate(value)
	if !ok {
		return ""
	}
	return diaryWeekdays[int(parsed.Weekday())]
}

// DiaryToday is today's local date in the stored form.
func DiaryToday() string { return time.Now().Format(DiaryDateLayout) }

// DiaryMonthKey is the `YYYY-MM` a date falls in; "" when the date is invalid.
func DiaryMonthKey(value string) string {
	parsed, ok := ParseDiaryDate(value)
	if !ok {
		return ""
	}
	return parsed.Format("2006-01")
}

// DiaryYearKey is the `YYYY` a date falls in; "" when the date is invalid.
func DiaryYearKey(value string) string {
	parsed, ok := ParseDiaryDate(value)
	if !ok {
		return ""
	}
	return parsed.Format("2006")
}

// DiaryExportScope is what an export takes: the whole ledger, one year or one
// month of it.
type DiaryExportScope uint8

const (
	// DiaryScopeAll exports every entry.
	DiaryScopeAll DiaryExportScope = iota
	// DiaryScopeYear exports one calendar year.
	DiaryScopeYear
	// DiaryScopeMonth exports one calendar month.
	DiaryScopeMonth
)

// DiaryScopeLabel is the segmented control's caption.
func (scope DiaryExportScope) Label() string {
	switch scope {
	case DiaryScopeYear:
		return "按年"
	case DiaryScopeMonth:
		return "按月"
	}
	return "全部"
}

// DiaryExportScopeNames are the choices in the page's order.
var DiaryExportScopeNames = [...]DiaryExportScope{
	DiaryScopeAll, DiaryScopeYear, DiaryScopeMonth,
}

// DiaryExportQuery selects the entries an export writes. Year and Month are 0
// when the scope does not bind them; Month only matters together with a year.
type DiaryExportQuery struct {
	Scope DiaryExportScope
	Year  int
	Month int
}

// Includes reports whether an entry date passes the query. A date that does not
// parse is never exported, as in the service.
func (query DiaryExportQuery) Includes(date string) bool {
	parsed, ok := ParseDiaryDate(date)
	if !ok {
		return false
	}
	if query.Scope == DiaryScopeAll {
		return true
	}
	if query.Year > 0 && parsed.Year() != query.Year {
		return false
	}
	if query.Scope == DiaryScopeMonth && query.Month > 0 && int(parsed.Month()) != query.Month {
		return false
	}
	return true
}

// DiaryFileDate reads the date from a `YYYY-MM-DD.txt` diary file name. Only
// the lowercase `.txt` extension is accepted, and the stem must be a real date,
// so a scanned directory imports exactly the files the export wrote.
func DiaryFileDate(name string) (string, bool) {
	stem, ok := strings.CutSuffix(name, ".txt")
	if !ok || !IsDiaryDate(stem) {
		return "", false
	}
	return stem, true
}

// DiaryFileName is the file an entry exports to.
func DiaryFileName(date string) string { return date + ".txt" }
