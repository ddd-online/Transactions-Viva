package domain

import (
	"testing"
)

func TestDiaryWordCountCountsCodePoints(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"今天很好", 4},
		{"hello", 5},
		{"a b", 3},
		{"😐", 1},
		{"😤😰😢😐😊", 5},
		// A newline is a code point: the page counted it too.
		{"a\nb", 3},
	}
	for _, one := range cases {
		if got := DiaryWordCount(one.text); got != one.want {
			t.Fatalf("DiaryWordCount(%q) = %d, want %d", one.text, got, one.want)
		}
	}
}

func TestDiaryDateIsStrict(t *testing.T) {
	valid := []string{"2026-01-15", "2024-02-29", "2000-02-29"}
	for _, value := range valid {
		if !IsDiaryDate(value) {
			t.Fatalf("%q should be a diary date", value)
		}
	}
	invalid := []string{
		"", "2026-1-15", "2026-01-5", "2026/01/15", "26-01-15",
		"2026-13-01", "2026-00-10", "2026-02-29", "1900-02-29",
		"2026-01-15 ", " 2026-01-15", "２０２６-01-15",
	}
	for _, value := range invalid {
		if IsDiaryDate(value) {
			t.Fatalf("%q should not be a diary date", value)
		}
	}
}

func TestDiaryMoodNormalization(t *testing.T) {
	cases := map[string]string{
		"烦躁": "😤", "焦虑": "😰", "难过": "😢", "平静": "😐", "开心": "😊",
		"😊":  "😊",
		"":   "",
		"??": "??",
	}
	for value, want := range cases {
		if got := NormalizeDiaryMood(value); got != want {
			t.Fatalf("NormalizeDiaryMood(%q) = %q, want %q", value, got, want)
		}
	}
	if DiaryMoodLabel("😢") != "难过" {
		t.Fatalf("label = %q", DiaryMoodLabel("😢"))
	}
	if DiaryMoodLabel("开心") != "开心" {
		t.Fatalf("Chinese label = %q", DiaryMoodLabel("开心"))
	}
	if DiaryMoodLabel("x") != "" {
		t.Fatalf("unknown label = %q", DiaryMoodLabel("x"))
	}
}

func TestDiaryDateLabels(t *testing.T) {
	if got := DiaryDateCn("2026-01-15"); got != "2026年1月15日" {
		t.Fatalf("cn = %q", got)
	}
	if got := DiaryDateCn("2026-01-15 "); got != "2026-01-15 " {
		t.Fatalf("invalid cn = %q, want it as written", got)
	}
	// 2026-01-15 is a Thursday.
	if got := DiaryWeekdayCn("2026-01-15"); got != "星期四" {
		t.Fatalf("weekday = %q", got)
	}
	if got := DiaryWeekdayCn("2026-02-29"); got != "" {
		t.Fatalf("invalid weekday = %q", got)
	}
}

func TestDiaryExportQueryIncludes(t *testing.T) {
	query := DiaryExportQuery{Scope: DiaryScopeAll}
	if !query.Includes("2026-01-15") || query.Includes("2026-1-15") {
		t.Fatal("the all scope takes every valid date and nothing else")
	}
	query = DiaryExportQuery{Scope: DiaryScopeYear, Year: 2026}
	if !query.Includes("2026-12-31") || query.Includes("2025-12-31") {
		t.Fatal("the year scope takes one calendar year")
	}
	query = DiaryExportQuery{Scope: DiaryScopeMonth, Year: 2026, Month: 2}
	if !query.Includes("2026-02-01") || query.Includes("2026-03-01") || query.Includes("2025-02-01") {
		t.Fatal("the month scope takes one calendar month of one year")
	}
	// A month without a year binds nothing, as the service's 0 meant.
	query = DiaryExportQuery{Scope: DiaryScopeMonth, Month: 2}
	if !query.Includes("2019-02-01") {
		t.Fatal("a month scope without a year should not filter the year")
	}
}

func TestDiaryFileDate(t *testing.T) {
	cases := map[string]string{
		"2026-01-15.txt": "2026-01-15",
		"2026-01-15.md":  "",
		"2026-01-15.TXT": "",
		"2026-02-29.txt": "",
		"notes.txt":      "",
		"2026-01-15":     "",
	}
	for name, want := range cases {
		date, ok := DiaryFileDate(name)
		if ok != (want != "") || date != want {
			t.Fatalf("DiaryFileDate(%q) = %q, %v; want %q", name, date, ok, want)
		}
	}
	if DiaryFileName("2026-01-15") != "2026-01-15.txt" {
		t.Fatalf("file name = %q", DiaryFileName("2026-01-15"))
	}
}
