package domain

import (
	"strings"
	"testing"
)

func TestKeyEventTitleFallsBackToTheFirstBodyLine(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		content string
		want    string
	}{
		{"typed title wins", "  年会  ", "抽奖与聚餐\n再一行", "年会"},
		{"first non-empty line", "", "\n  抽奖与聚餐  \n再一行", "抽奖与聚餐"},
		{"no title and no body", "   ", " \n\t ", ""},
		{"title is trimmed", "   ", "", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := KeyEventTitle(test.title, test.content); got != test.want {
				t.Fatalf("KeyEventTitle(%q, %q) = %q, want %q", test.title, test.content, got, test.want)
			}
		})
	}
}

func TestKeyEventTitleKeepsTwoHundredCodePoints(t *testing.T) {
	long := strings.Repeat("会", KeyEventTitleLimit+20)
	got := KeyEventTitle(long, "")
	if runes := []rune(got); len(runes) != KeyEventTitleLimit {
		t.Fatalf("title keeps %d code points, want %d", len(runes), KeyEventTitleLimit)
	}
	if !strings.HasPrefix(long, got) {
		t.Fatal("truncated title is not a prefix of the input")
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := TruncateRunes("普通文字", 2); got != "普通" {
		t.Fatalf("TruncateRunes = %q", got)
	}
	if got := TruncateRunes("short", 10); got != "short" {
		t.Fatalf("TruncateRunes = %q", got)
	}
	if got := TruncateRunes("short", 0); got != "" {
		t.Fatalf("TruncateRunes with a zero limit = %q", got)
	}
}
