package domain

import (
	"sort"
	"strings"
)

// 连板天梯的纯计算：今日涨停池 + 前一交易日涨停池 → 分层天梯与汇总指标。
// 数据是市场数据，不属于任何账户。

// StockLimitEntry 是涨停池里的一只股票。
type StockLimitEntry struct {
	StockCode     string
	StockName     string
	Market        int
	PriceMilli    int64
	Pct           float64
	AmountCents   int64
	FloatMvCents  int64
	Turnover      float64
	Level         int
	FirstSeal     int
	LastSeal      int
	BrokenCount   int
	SealFundCents int64
	Industry      string
	StatDays      int
	StatCount     int
	// OpenPct 是当天开盘涨跌幅（%），来自行情；没有时为 nil。
	OpenPct *float64
}

// StockLimitBrokenEntry 是昨日在这一层、今日没涨停的股票。
type StockLimitBrokenEntry struct {
	StockLimitEntry
	TodayPct   *float64
	TodayPrice *int64
}

// StockLimitRung 是天梯的一层。
type StockLimitRung struct {
	Level      int
	Entries    []StockLimitEntry
	Broken     []StockLimitBrokenEntry
	Candidates *int
	Rate       *float64
}

// StockLimitMetrics 是天梯顶部的汇总数字。
type StockLimitMetrics struct {
	LimitCount    int
	PreviousCount *int
	MaxLevel      *int
	PromotedCount *int
	Rate          *float64
	BrokenCount   *int
}

// StockLimitStatus 是一天的天梯状态。
type StockLimitStatus string

const (
	StockLimitOK          StockLimitStatus = "ok"
	StockLimitEmpty       StockLimitStatus = "empty"
	StockLimitPreopen     StockLimitStatus = "preopen"
	StockLimitUnavailable StockLimitStatus = "unavailable"
	StockLimitOffday      StockLimitStatus = "offday"
)

// StockLimitLadder 是某一天的天梯。
type StockLimitLadder struct {
	Date         string
	PreviousDate string // 空串 = 没有前一交易日
	Status       StockLimitStatus
	Temporary    bool
	FromCache    bool
	Stale        bool
	FetchedAt    int64
	Source       string
	Rungs        []StockLimitRung
	Metrics      StockLimitMetrics
}

// LimitLadderData 是不含日期 / 状态的天梯内容。
type LimitLadderData struct {
	Rungs   []StockLimitRung
	Metrics StockLimitMetrics
}

// OneWordSealTime 是集合竞价封板时间 09:25:00 的 HHMMSS 整数。
const OneWordSealTime = 92_500

// IsOneWordBoard 报告是否一字板：09:25 封死且全天没炸过板。
func IsOneWordBoard(entry StockLimitEntry) bool {
	return entry.FirstSeal == OneWordSealTime && entry.BrokenCount == 0
}

// LadderSealTime 把 HHMMSS 整数格式化成 HH:MM；缺值给破折号。
func LadderSealTime(value int) string {
	if value <= 0 {
		return "—"
	}
	text := "000000" + itoa(value)
	text = text[len(text)-6:]
	return text[:2] + ":" + text[2:4]
}

func byFirstSeal(left, right StockLimitEntry) bool {
	if left.FirstSeal != right.FirstSeal {
		return left.FirstSeal < right.FirstSeal
	}
	return left.StockCode < right.StockCode
}

func groupByLevel(entries []StockLimitEntry) map[int][]StockLimitEntry {
	grouped := make(map[int][]StockLimitEntry)
	for _, entry := range entries {
		grouped[entry.Level] = append(grouped[entry.Level], entry)
	}
	return grouped
}

func sharePercent(part, whole int) float64 {
	if whole <= 0 {
		return 0
	}
	return RoundRate(float64(part) / float64(whole) * 100)
}

// BuildLimitLadder 组装天梯。hasPrevious 为 false 表示前一交易日缺失。
func BuildLimitLadder(today, previous []StockLimitEntry, hasPrevious bool) LimitLadderData {
	todayCodes := make(map[string]bool, len(today))
	for _, entry := range today {
		todayCodes[entry.StockCode] = true
	}
	todayByLevel := groupByLevel(today)
	var previousByLevel map[int][]StockLimitEntry
	if hasPrevious {
		previousByLevel = groupByLevel(previous)
	}

	brokenByLevel := make(map[int][]StockLimitBrokenEntry)
	if hasPrevious {
		for level, entries := range previousByLevel {
			broken := make([]StockLimitEntry, 0, len(entries))
			for _, entry := range entries {
				if !todayCodes[entry.StockCode] {
					broken = append(broken, entry)
				}
			}
			sort.SliceStable(broken, func(i, j int) bool { return byFirstSeal(broken[i], broken[j]) })
			if len(broken) == 0 {
				continue
			}
			list := make([]StockLimitBrokenEntry, len(broken))
			for i, entry := range broken {
				list[i] = StockLimitBrokenEntry{StockLimitEntry: entry}
			}
			brokenByLevel[level] = list
		}
	}

	levels := make([]int, 0, len(todayByLevel)+len(brokenByLevel))
	for level := range todayByLevel {
		levels = append(levels, level)
	}
	for level := range brokenByLevel {
		if _, ok := todayByLevel[level]; !ok {
			levels = append(levels, level)
		}
	}
	sort.Ints(levels)

	rungs := make([]StockLimitRung, 0, len(levels))
	for _, level := range levels {
		entries := append([]StockLimitEntry(nil), todayByLevel[level]...)
		sort.SliceStable(entries, func(i, j int) bool { return byFirstSeal(entries[i], entries[j]) })
		var candidates *int
		if hasPrevious && level > 1 {
			count := len(previousByLevel[level-1])
			candidates = &count
		}
		var rate *float64
		if candidates != nil && *candidates > 0 {
			value := sharePercent(len(entries), *candidates)
			rate = &value
		}
		rungs = append(rungs, StockLimitRung{
			Level:      level,
			Entries:    entries,
			Broken:     brokenByLevel[level],
			Candidates: candidates,
			Rate:       rate,
		})
	}

	metrics := StockLimitMetrics{LimitCount: len(today)}
	if len(todayByLevel) > 0 {
		maxLevel := 0
		for level := range todayByLevel {
			if level > maxLevel {
				maxLevel = level
			}
		}
		metrics.MaxLevel = &maxLevel
	}
	if hasPrevious {
		previousCount := len(previous)
		promoted := 0
		for _, entry := range previous {
			if todayCodes[entry.StockCode] {
				promoted++
			}
		}
		broken := previousCount - promoted
		metrics.PreviousCount = &previousCount
		metrics.PromotedCount = &promoted
		metrics.BrokenCount = &broken
		if previousCount > 0 {
			rate := sharePercent(promoted, previousCount)
			metrics.Rate = &rate
		}
	}
	return LimitLadderData{Rungs: rungs, Metrics: metrics}
}

// LadderRow 是天梯里的一行；Broken 表示昨日在这一层、今日没涨停。
type LadderRow struct {
	Entry  StockLimitEntry
	Broken *StockLimitBrokenEntry
}

// BoardOf 返回条目的板块名（东财行业字段）。
func BoardOf(entry StockLimitEntry) string { return strings.TrimSpace(entry.Industry) }

type boardGroup struct {
	board   string
	entries []StockLimitEntry
	broken  []StockLimitBrokenEntry
}

func compareBoardGroups(left, right boardGroup) bool {
	leftLive, rightLive := len(left.entries) > 0, len(right.entries) > 0
	if leftLive != rightLive {
		return leftLive
	}
	if (left.board == "") != (right.board == "") {
		return left.board != ""
	}
	sealOf := func(group boardGroup) int {
		source := group.entries
		if len(source) == 0 {
			source = make([]StockLimitEntry, len(group.broken))
			for i, entry := range group.broken {
				source[i] = entry.StockLimitEntry
			}
		}
		earliest := source[0].FirstSeal
		for _, entry := range source {
			if entry.FirstSeal < earliest {
				earliest = entry.FirstSeal
			}
		}
		return earliest
	}
	leftSeal, rightSeal := sealOf(left), sealOf(right)
	if leftSeal != rightSeal {
		return leftSeal < rightSeal
	}
	return left.board < right.board
}

// LadderRows 返回一层里从上到下的行序：先按板块聚在一起，板块内今日连板在前，
// 断板跟在同板块的连板后面。
func LadderRows(rung StockLimitRung) []LadderRow {
	groups := make(map[string]*boardGroup)
	groupOf := func(board string) *boardGroup {
		if group, ok := groups[board]; ok {
			return group
		}
		group := &boardGroup{board: board}
		groups[board] = group
		return group
	}
	for _, entry := range rung.Entries {
		group := groupOf(BoardOf(entry))
		group.entries = append(group.entries, entry)
	}
	for _, entry := range rung.Broken {
		group := groupOf(BoardOf(entry.StockLimitEntry))
		group.broken = append(group.broken, entry)
	}
	list := make([]boardGroup, 0, len(groups))
	for _, group := range groups {
		list = append(list, *group)
	}
	sort.SliceStable(list, func(i, j int) bool { return compareBoardGroups(list[i], list[j]) })

	var rows []LadderRow
	for _, group := range list {
		entries := append([]StockLimitEntry(nil), group.entries...)
		sort.SliceStable(entries, func(i, j int) bool { return byFirstSeal(entries[i], entries[j]) })
		for _, entry := range entries {
			rows = append(rows, LadderRow{Entry: entry})
		}
		broken := append([]StockLimitBrokenEntry(nil), group.broken...)
		sort.SliceStable(broken, func(i, j int) bool {
			return byFirstSeal(broken[i].StockLimitEntry, broken[j].StockLimitEntry)
		})
		for i := range broken {
			rows = append(rows, LadderRow{Entry: broken[i].StockLimitEntry, Broken: &broken[i]})
		}
	}
	return rows
}

// BoardCount 是板块汇总的一格。
type BoardCount struct {
	Board string
	Count int
}

// BoardCounts 按板块数今天的涨停家数；只数今日涨停，空板块跳过。
func BoardCounts(entries []StockLimitEntry) []BoardCount {
	counts := make(map[string]int)
	for _, entry := range entries {
		board := BoardOf(entry)
		if board == "" {
			continue
		}
		counts[board]++
	}
	result := make([]BoardCount, 0, len(counts))
	for board, count := range counts {
		result = append(result, BoardCount{Board: board, Count: count})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Board < result[j].Board
	})
	return result
}
