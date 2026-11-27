package domain

import (
	"strings"
	"testing"
)

func TestNoteTitleAndDisplay(t *testing.T) {
	if got := NoteTitle("  周会记录  "); got != "周会记录" {
		t.Fatalf("NoteTitle = %q", got)
	}
	if got := NoteTitle("   "); got != "" {
		t.Fatalf("blank NoteTitle = %q", got)
	}
	long := NoteTitle(strings.Repeat("字", NoteTitleMaxRunes+10))
	if len([]rune(long)) != NoteTitleMaxRunes {
		t.Fatalf("long title has %d runes, want %d", len([]rune(long)), NoteTitleMaxRunes)
	}
	if NoteDisplayTitle("") != NoteDefaultTitle || NoteDisplayTitle("  ") != NoteDefaultTitle {
		t.Fatal("blank titles should show the default name")
	}
	if NoteDisplayTitle(" 周会 ") != "周会" {
		t.Fatal("display title should be trimmed")
	}
}

func TestNoteHeadingLevel(t *testing.T) {
	for given, want := range map[int]int{-2: 1, 0: 1, 1: 1, 3: 3, 5: 5, 9: 5} {
		if got := NoteHeadingLevel(given); got != want {
			t.Fatalf("NoteHeadingLevel(%d) = %d, want %d", given, got, want)
		}
	}
}

// TestNoteWordCount: 字数 is the count a person writing 中文 expects — one per Han
// character, one per run of Latin letters or digits, and nothing for the spaces and
// punctuation between them.
func TestNoteWordCount(t *testing.T) {
	cases := map[string]int{
		"":              0,
		"   ":           0,
		"，。！":           0,
		"你好世界":          4,
		"hello world":   2,
		"hello，world":   2,
		"第1章":           3,
		"a1b2":          1,
		"中文 English 混排": 5,
		"中abc文":         3,
		"2026年11月27日":   6,
		"😐":             0,
		"好😐坏":           2,
		"一句话，两个词。":      6,
	}
	for text, want := range cases {
		if got := NoteWordCount(text); got != want {
			t.Fatalf("NoteWordCount(%q) = %d, want %d", text, got, want)
		}
	}
}

func TestNoteImageSize(t *testing.T) {
	// A small image keeps its pixel size at scale 1.
	if width, height := NoteImageSize(400, 300, 1, 720); width != 400 || height != 300 {
		t.Fatalf("natural size = %vx%v", width, height)
	}
	// Scaling up stops at the content width.
	if width, height := NoteImageSize(400, 300, 2, 720); width != 720 || height != 540 {
		t.Fatalf("capped size = %vx%v", width, height)
	}
	// A wide image shrinks to the content width at scale 1.
	if width, height := NoteImageSize(2000, 1000, 1, 720); width != 720 || height != 360 {
		t.Fatalf("shrunk size = %vx%v", width, height)
	}
	// A tall image narrower than the content keeps its width.
	if width, height := NoteImageSize(400, 2000, 1, 720); width != 400 || height != 2000 {
		t.Fatalf("tall size = %vx%v", width, height)
	}
	if width, height := NoteImageSize(0, 300, 1, 720); width != 0 || height != 0 {
		t.Fatalf("unknown size = %vx%v", width, height)
	}
}

func TestNoteImageScale(t *testing.T) {
	if NoteImageScale(0) != NoteImageDefaultScale {
		t.Fatal("zero scale should mean natural size")
	}
	if NoteImageScale(NoteImageMinScale/2) != NoteImageMinScale {
		t.Fatal("tiny scale should be raised to the minimum")
	}
	if NoteImageScale(99) != NoteImageMaxScale {
		t.Fatal("huge scale should be lowered to the maximum")
	}
	if NoteImageScale(1.5) != 1.5 {
		t.Fatal("a normal scale should be kept")
	}
}
