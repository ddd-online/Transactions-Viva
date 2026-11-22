package domain

import "testing"

func limitEntry(code string, level, firstSeal int, board string) StockLimitEntry {
	return StockLimitEntry{
		StockCode: code, StockName: code, Level: level,
		FirstSeal: firstSeal, Industry: board,
	}
}

func TestBuildLimitLadder(t *testing.T) {
	today := []StockLimitEntry{
		limitEntry("600001", 1, 92_500, "银行"),
		limitEntry("600002", 1, 100_000, "银行"),
		limitEntry("600003", 2, 95_000, "电网设备"),
	}
	previous := []StockLimitEntry{
		limitEntry("600001", 1, 92_500, "银行"),
		limitEntry("600002", 1, 93_000, "银行"),
		limitEntry("600003", 1, 94_000, "电网设备"),
		limitEntry("600004", 1, 95_000, "化学制药"),
		limitEntry("600005", 2, 96_000, "一般零售"),
	}
	data := BuildLimitLadder(today, previous, true)
	if len(data.Rungs) != 2 {
		t.Fatalf("rungs = %+v", data.Rungs)
	}
	if data.Rungs[0].Level != 1 || len(data.Rungs[0].Entries) != 2 || len(data.Rungs[0].Broken) != 1 {
		t.Fatalf("first rung = %+v", data.Rungs[0])
	}
	if data.Rungs[0].Candidates != nil || data.Rungs[0].Rate != nil {
		t.Fatalf("first board candidates = %v rate = %v", data.Rungs[0].Candidates, data.Rungs[0].Rate)
	}
	if data.Rungs[1].Level != 2 || len(data.Rungs[1].Entries) != 1 || len(data.Rungs[1].Broken) != 1 {
		t.Fatalf("second rung = %+v", data.Rungs[1])
	}
	if data.Rungs[1].Candidates == nil || *data.Rungs[1].Candidates != 4 {
		t.Fatalf("second board candidates = %v", data.Rungs[1].Candidates)
	}
	if data.Rungs[1].Rate == nil || *data.Rungs[1].Rate != 25 {
		t.Fatalf("second board rate = %v", data.Rungs[1].Rate)
	}
	if data.Metrics.LimitCount != 3 || data.Metrics.PreviousCount == nil || *data.Metrics.PreviousCount != 5 {
		t.Fatalf("metrics = %+v", data.Metrics)
	}
	if data.Metrics.MaxLevel == nil || *data.Metrics.MaxLevel != 2 {
		t.Fatalf("max level = %v", data.Metrics.MaxLevel)
	}
	if data.Metrics.PromotedCount == nil || *data.Metrics.PromotedCount != 3 || *data.Metrics.BrokenCount != 2 {
		t.Fatalf("promoted / broken = %+v", data.Metrics)
	}
	if data.Metrics.Rate == nil || *data.Metrics.Rate != 60 {
		t.Fatalf("overall rate = %v", data.Metrics.Rate)
	}

	if data := BuildLimitLadder(today, nil, false); len(data.Rungs) != 2 || data.Rungs[0].Broken != nil {
		t.Fatalf("no previous ladder = %+v", data)
	}
}

func TestLadderRowsGroupBoards(t *testing.T) {
	rung := StockLimitRung{
		Level: 1,
		Entries: []StockLimitEntry{
			limitEntry("600001", 1, 100_000, "银行"),
			limitEntry("600002", 1, 92_500, "电网设备"),
		},
		Broken: []StockLimitBrokenEntry{{
			StockLimitEntry: limitEntry("600003", 1, 95_000, "银行"),
		}},
	}
	rows := LadderRows(rung)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Entry.StockCode != "600002" || rows[1].Entry.StockCode != "600001" || rows[2].Entry.StockCode != "600003" {
		t.Fatalf("row order = %+v", rows)
	}
	if rows[2].Broken == nil {
		t.Fatal("last row should be broken")
	}
}

func TestBoardCounts(t *testing.T) {
	counts := BoardCounts([]StockLimitEntry{
		limitEntry("600001", 1, 0, "银行"),
		limitEntry("600002", 1, 0, "银行"),
		limitEntry("600003", 1, 0, "电网设备"),
		limitEntry("600004", 1, 0, ""),
	})
	if len(counts) != 2 || counts[0].Board != "银行" || counts[0].Count != 2 {
		t.Fatalf("counts = %+v", counts)
	}
}

func TestOneWordBoardAndSealTime(t *testing.T) {
	entry := StockLimitEntry{FirstSeal: 92_500, BrokenCount: 0}
	if !IsOneWordBoard(entry) {
		t.Fatal("one-word board not detected")
	}
	entry.BrokenCount = 1
	if IsOneWordBoard(entry) {
		t.Fatal("broken one-word board detected")
	}
	if got := LadderSealTime(92_500); got != "09:25" {
		t.Fatalf("seal time = %q", got)
	}
	if got := LadderSealTime(0); got != "—" {
		t.Fatalf("empty seal time = %q", got)
	}
}
