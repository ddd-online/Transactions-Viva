package diary

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/store/storetest"
)

func TestDecodeTextFallsBackThroughEncodings(t *testing.T) {
	t.Run("utf-8 with a BOM is kept", func(t *testing.T) {
		raw := append([]byte{0xef, 0xbb, 0xbf}, []byte("今天很好")...)
		if got := DecodeText(raw); got != "\ufeff今天很好" {
			t.Fatalf("decoded = %q", got)
		}
	})

	t.Run("utf-16 little endian with a BOM", func(t *testing.T) {
		if got := DecodeText(utf16LE("今天很好")); got != "今天很好" {
			t.Fatalf("decoded = %q", got)
		}
	})

	t.Run("utf-16 big endian with a BOM", func(t *testing.T) {
		if got := DecodeText(utf16BE("今天很好")); got != "今天很好" {
			t.Fatalf("decoded = %q", got)
		}
	})

	t.Run("gbk", func(t *testing.T) {
		// "今天" in GBK.
		if got := DecodeText([]byte{0xbd, 0xf1, 0xcc, 0xec}); got != "今天" {
			t.Fatalf("decoded = %q", got)
		}
	})

	t.Run("invalid bytes become question marks", func(t *testing.T) {
		// A stray byte is one replacement; a character cut off by the end of the
		// file is one for the cut sequence and one for its orphaned tail, which
		// is how the old decoder counted them.
		raw := []byte{'a', 0xff, 'b', 0xe4, 0xbd}
		got := DecodeText(raw)
		if got != "a?b??" {
			t.Fatalf("decoded = %q", got)
		}
	})
}

// utf16LE encodes text as UTF-16LE with a byte-order mark.
func utf16LE(text string) []byte {
	units := encodeUTF16(text)
	out := []byte{0xff, 0xfe}
	for _, unit := range units {
		out = append(out, byte(unit), byte(unit>>8))
	}
	return out
}

// utf16BE encodes text as UTF-16BE with a byte-order mark.
func utf16BE(text string) []byte {
	units := encodeUTF16(text)
	out := []byte{0xfe, 0xff}
	for _, unit := range units {
		out = append(out, byte(unit>>8), byte(unit))
	}
	return out
}

func TestScanFindsOnlyDatedFiles(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "2026-01-15.txt"), "a")
	write(t, filepath.Join(root, "notes.txt"), "b")
	write(t, filepath.Join(root, "2026-02-29.txt"), "c")
	write(t, filepath.Join(root, "2026-01-15.md"), "d")
	if err := os.MkdirAll(filepath.Join(root, "2025"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "2025", "2025-12-31.txt"), "e")

	files, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %+v", files)
	}
	if files[0].Date != "2025-12-31" || files[1].Date != "2026-01-15" {
		t.Fatalf("files are not oldest first: %+v", files)
	}
	if _, err := Scan(filepath.Join(root, "missing")); err == nil {
		t.Fatal("scanning a missing directory should fail")
	}
}

func TestImportWritesEveryFileIntoTheLedger(t *testing.T) {
	fixture := storetest.Open(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "2026-01-15.txt"), "今天很好")
	write(t, filepath.Join(root, "2026-01-16.txt"), "导入的日记")

	result, err := Import(context.Background(), fixture.Workspace, fixture.Home.ID, root)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Imported != 2 || result.Total != 2 || len(result.Failures) != 0 {
		t.Fatalf("result = %+v", result)
	}
	entry, err := fixture.Workspace.DiaryEntry(context.Background(), fixture.Home.ID, "2026-01-15")
	if err != nil || entry.Content != "今天很好" || entry.WordCount != 4 {
		t.Fatalf("imported = %+v, err=%v", entry, err)
	}
	// The other ledger saw nothing.
	dates, err := fixture.Workspace.DiaryDates(context.Background(), fixture.Travel.ID)
	if err != nil || len(dates) != 0 {
		t.Fatalf("travel dates = %+v, err=%v", dates, err)
	}
}

func TestExportWritesFilesAndRoundTrips(t *testing.T) {
	fixture := storetest.Open(t)
	ctx := context.Background()
	for _, date := range []string{"2026-01-15", "2026-02-01", "2025-12-31"} {
		if _, err := fixture.Workspace.SaveDiary(ctx, fixture.Home.ID, date, "内容 "+date, "😊"); err != nil {
			t.Fatalf("SaveDiary %s: %v", date, err)
		}
	}

	target := filepath.Join(t.TempDir(), "out")
	result, err := Export(ctx, fixture.Workspace, fixture.Home.ID, target, domain.DiaryExportQuery{Scope: domain.DiaryScopeAll})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if result.Written != 3 || result.Total != 3 {
		t.Fatalf("result = %+v", result)
	}
	raw, err := os.ReadFile(filepath.Join(target, "2026-01-15.txt"))
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if string(raw) != "内容 2026-01-15" {
		t.Fatalf("exported = %q", raw)
	}

	// A month export takes only that month.
	monthly := filepath.Join(t.TempDir(), "month")
	result, err = Export(ctx, fixture.Workspace, fixture.Home.ID, monthly,
		domain.DiaryExportQuery{Scope: domain.DiaryScopeMonth, Year: 2026, Month: 2})
	if err != nil || result.Written != 1 {
		t.Fatalf("monthly result = %+v, err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(monthly, "2026-02-01.txt")); err != nil {
		t.Fatalf("monthly file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(monthly, "2026-01-15.txt")); !os.IsNotExist(err) {
		t.Fatalf("a month export wrote another month: %v", err)
	}

	// What the export wrote scans and imports again.
	files, err := Scan(monthly)
	if err != nil || len(files) != 1 {
		t.Fatalf("rescan = %+v, err=%v", files, err)
	}
	if _, err := Import(ctx, fixture.Workspace, fixture.Travel.ID, monthly); err != nil {
		t.Fatalf("reimport: %v", err)
	}
	entry, err := fixture.Workspace.DiaryEntry(ctx, fixture.Travel.ID, "2026-02-01")
	if err != nil || entry.Content != "内容 2026-02-01" {
		t.Fatalf("reimported = %+v, err=%v", entry, err)
	}
}

func TestImportReportsUnreadableFilesAndKeepsGoing(t *testing.T) {
	fixture := storetest.Open(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "2026-01-15.txt"), "可以导入")
	missing := filepath.Join(root, "2026-01-16.txt")
	result, err := ImportFiles(context.Background(), fixture.Workspace, fixture.Home.ID, []File{
		{Date: "2026-01-15", Path: filepath.Join(root, "2026-01-15.txt")},
		{Date: "2026-01-16", Path: missing},
	})
	if err != nil {
		t.Fatalf("ImportFiles: %v", err)
	}
	if result.Imported != 1 || result.Total != 2 || len(result.Failures) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.Failures[0].Date != "2026-01-16" {
		t.Fatalf("failure = %+v", result.Failures[0])
	}
	entry, err := fixture.Workspace.DiaryEntry(context.Background(), fixture.Home.ID, "2026-01-15")
	if err != nil || entry.Content != "可以导入" {
		t.Fatalf("the readable file did not import: %+v, err=%v", entry, err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// encodeUTF16 turns text into UTF-16 code units.
func encodeUTF16(text string) []uint16 {
	runes := []rune(text)
	units := make([]uint16, 0, len(runes))
	for _, value := range runes {
		if value < 0x10000 {
			units = append(units, uint16(value))
			continue
		}
		value -= 0x10000
		units = append(units, uint16(0xd800+(value>>10)), uint16(0xdc00+(value&0x3ff)))
	}
	return units
}
