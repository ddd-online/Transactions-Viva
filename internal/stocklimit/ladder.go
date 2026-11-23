// Package stocklimit turns the Eastmoney limit-up pool into the 连板天梯.
//
// Cache discipline (the project's behaviour):
//   - a closed day (a past day, or today after 15:00) is read from the local
//     cache and only fetched when missing or explicitly refreshed; a fetched
//     closed day is written back;
//   - an intraday snapshot (today before 15:00) is rendered but never written:
//     the pool changes every minute;
//   - today before 09:30 is preopen (no request, no cache write);
//   - a future date is an error;
//   - weekends are offday and never request the pool (the source would answer
//     with the latest trading day's pool).
//
// The previous trading day is looked back up to fifteen calendar days and
// cached too; it supplies the broken rows and the promotion rate.
package stocklimit

import (
	"context"
	"fmt"
	"time"

	"transactions/internal/domain"
	"transactions/internal/stockquote"
)

const (
	// limitOpenMinute is 09:30 local time.
	limitOpenMinute = 9*60 + 30
	// limitCloseMinute is 15:00 local time.
	limitCloseMinute = 15 * 60
	// retentionDays is the source's approximate history; older dates with no
	// local cache are unavailable rather than an empty limit-up day.
	retentionDays = 45
	// previousLookbackDays covers the National Day and Spring Festival breaks.
	previousLookbackDays = 15
	// quoteMaxAgeDays bounds how far back live quotes are asked for: the quote
	// endpoint only has the latest snapshot, so older ladders read stored
	// snapshots only.
	quoteMaxAgeDays = 4
)

// Source fetches one trading day's limit-up pool.
type Source interface {
	FetchDay(ctx context.Context, date string) ([]domain.StockLimitEntry, error)
}

// Cache is the workspace's ladder cache. *store.Workspace satisfies it; the
// service only depends on this narrow shape.
type Cache interface {
	StockLimitDay(ctx context.Context, date string) (*domain.StockLimitDay, error)
	StockLimitEntries(ctx context.Context, date string) ([]domain.StockLimitEntry, error)
	SaveStockLimitDay(ctx context.Context, date, status string, count int, fetchedAt int64) error
	SaveStockLimitEntries(ctx context.Context, date string, entries []domain.StockLimitEntry, fetchedAt int64) error
	StockLimitQuotes(ctx context.Context, date string) (map[string]domain.StockLimitQuote, error)
	SaveStockLimitQuotes(ctx context.Context, date string, quotes map[string]domain.StockLimitQuote, fetchedAt int64) error
}

// Service assembles one day's ladder from a pool source, the workspace cache
// and the quote source. Cache and Quotes may be nil in a test or a workspace
// whose market tables are not open yet.
type Service struct {
	Cache  Cache
	Source Source
	Quotes stockquote.Fetcher
	// Clock is the current local time; nil uses time.Now.
	Clock func() time.Time
}

// NewService wires the production sources.
func NewService(cache Cache, source Source, quotes stockquote.Fetcher) *Service {
	return &Service{Cache: cache, Source: source, Quotes: quotes}
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

// Ladder returns one day's ladder. refresh forces a fetch even when a closed
// cache exists, but never turns an intraday snapshot into a write.
func (s *Service) Ladder(ctx context.Context, date string, refresh bool) (domain.StockLimitLadder, error) {
	now := s.now()
	day, ok := parseDay(date)
	if !ok {
		return domain.StockLimitLadder{}, fmt.Errorf("日期要写成 YYYY-MM-DD")
	}
	today := now.Format("2006-01-02")
	if date > today {
		return domain.StockLimitLadder{}, fmt.Errorf("不能查看未来日期")
	}
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		return emptyLadder(date, now.Unix(), domain.StockLimitOffday), nil
	}
	if date == today && now.Before(atMinute(day, limitOpenMinute)) {
		return emptyLadder(date, now.Unix(), domain.StockLimitPreopen), nil
	}
	closed := date < today || (date == today && !now.Before(atMinute(day, limitCloseMinute)))

	cached, err := s.cacheDay(ctx, date)
	if err != nil {
		return domain.StockLimitLadder{}, err
	}
	cacheUsable := cached != nil && closed

	var entries []domain.StockLimitEntry
	fetchedAt := now.Unix()
	stale := false
	fromCache := false
	if !refresh && cacheUsable {
		entries, err = s.cachedEntries(ctx, cached, date)
		if err != nil {
			return domain.StockLimitLadder{}, err
		}
		fetchedAt = cached.FetchedAt
		fromCache = true
	} else {
		if s.Source == nil {
			return domain.StockLimitLadder{}, fmt.Errorf("涨停池数据源未配置")
		}
		entries, err = s.Source.FetchDay(ctx, date)
		if err != nil {
			// A closed day with a cache is still usable when the refresh fails:
			// show the cache and mark it stale instead of failing the page.
			if cacheUsable {
				entries, readErr := s.cachedEntries(ctx, cached, date)
				if readErr != nil {
					return domain.StockLimitLadder{}, readErr
				}
				return s.finish(ctx, date, entries, ladderContext{
					now: now, fetchedAt: cached.FetchedAt, temporary: false,
					fromCache: true, stale: true,
				})
			}
			return domain.StockLimitLadder{}, err
		}
		fetchedAt = now.Unix()
		if closed {
			switch {
			case len(entries) == 0 && cached != nil && cached.Status == string(domain.StockLimitOK):
				// A refresh that lost the data keeps the stored pool.
				entries, err = s.cachedEntries(ctx, cached, date)
				if err != nil {
					return domain.StockLimitLadder{}, err
				}
				fetchedAt = cached.FetchedAt
				fromCache = true
				stale = true
			case len(entries) == 0 && isBeyondRetention(date, today):
				return emptyLadder(date, now.Unix(), domain.StockLimitUnavailable), nil
			default:
				if err := s.saveDay(ctx, date, entries, now.Unix()); err != nil {
					return domain.StockLimitLadder{}, err
				}
			}
		}
		// Intraday: render only, never write.
	}

	return s.finish(ctx, date, entries, ladderContext{
		now: now, fetchedAt: fetchedAt, temporary: !closed,
		fromCache: fromCache, stale: stale,
	})
}

type ladderContext struct {
	now       time.Time
	fetchedAt int64
	temporary bool
	fromCache bool
	stale     bool
}

func (s *Service) finish(ctx context.Context, date string, entries []domain.StockLimitEntry, context ladderContext) (domain.StockLimitLadder, error) {
	previous, err := s.loadPreviousDay(ctx, date, context.now)
	if err != nil {
		return domain.StockLimitLadder{}, err
	}
	hasPrevious := previous.date != ""
	data := domain.BuildLimitLadder(entries, previous.entries, hasPrevious)
	status := domain.StockLimitEmpty
	if len(entries) > 0 {
		status = domain.StockLimitOK
	}
	ladder := domain.StockLimitLadder{
		Date:         date,
		PreviousDate: previous.date,
		Status:       status,
		Temporary:    context.temporary,
		FromCache:    context.fromCache,
		Stale:        context.stale,
		FetchedAt:    context.fetchedAt,
		Source:       "eastmoney",
		Rungs:        data.Rungs,
		Metrics:      data.Metrics,
	}
	return s.attachQuotes(ctx, ladder, context.now)
}

type previousDay struct {
	date    string
	entries []domain.StockLimitEntry
}

// loadPreviousDay reads the previous trading day's pool from the cache, probing
// earlier days when the cache has only empty markers (weekends and holidays).
// A source failure stops the probe; the ladder still renders without broken
// rows and the promotion rate.
func (s *Service) loadPreviousDay(ctx context.Context, date string, now time.Time) (previousDay, error) {
	cursor := date
	for step := 0; step < previousLookbackDays; step++ {
		next, ok := shiftDay(cursor, -1)
		if !ok {
			break
		}
		cursor = next
		cached, err := s.cacheDay(ctx, cursor)
		if err != nil {
			return previousDay{}, err
		}
		if cached != nil {
			if cached.Status == string(domain.StockLimitOK) {
				entries, err := s.cachedEntries(ctx, cached, cursor)
				if err != nil {
					return previousDay{}, err
				}
				return previousDay{date: cursor, entries: entries}, nil
			}
			continue
		}
		day, _ := parseDay(cursor)
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			if err := s.saveDay(ctx, cursor, nil, now.Unix()); err != nil {
				return previousDay{}, err
			}
			continue
		}
		if s.Source == nil {
			break
		}
		entries, err := s.Source.FetchDay(ctx, cursor)
		if err != nil {
			break
		}
		if len(entries) > 0 {
			if err := s.saveDay(ctx, cursor, entries, now.Unix()); err != nil {
				return previousDay{}, err
			}
			return previousDay{date: cursor, entries: entries}, nil
		}
		if err := s.saveDay(ctx, cursor, nil, now.Unix()); err != nil {
			return previousDay{}, err
		}
	}
	return previousDay{}, nil
}

// attachQuotes fills the open percentage on every row and the closing price /
// percentage on broken rows. Only a quote whose own QuoteDate equals the
// ladder date is trusted; a quote failure simply leaves those labels absent.
// A finalized ladder stores the snapshot so history does not depend on the
// live quote endpoint still having that day.
func (s *Service) attachQuotes(ctx context.Context, ladder domain.StockLimitLadder, now time.Time) (domain.StockLimitLadder, error) {
	limitCodes := make([]string, 0)
	brokenCodes := make([]string, 0)
	seen := make(map[string]bool)
	for _, rung := range ladder.Rungs {
		for _, entry := range rung.Entries {
			if !seen[entry.StockCode] {
				seen[entry.StockCode] = true
				limitCodes = append(limitCodes, entry.StockCode)
			}
		}
		for _, entry := range rung.Broken {
			if !seen[entry.StockCode] {
				seen[entry.StockCode] = true
				brokenCodes = append(brokenCodes, entry.StockCode)
			}
		}
	}
	if len(limitCodes)+len(brokenCodes) == 0 {
		return ladder, nil
	}
	brokenSet := make(map[string]bool, len(brokenCodes))
	for _, code := range brokenCodes {
		brokenSet[code] = true
	}

	values := make(map[string]domain.StockLimitQuote)
	if s.Cache != nil {
		stored, err := s.Cache.StockLimitQuotes(ctx, ladder.Date)
		if err != nil {
			return domain.StockLimitLadder{}, err
		}
		for code, quote := range stored {
			values[code] = quote
		}
	}
	missing := make([]string, 0, len(limitCodes)+len(brokenCodes))
	for _, codes := range [][]string{limitCodes, brokenCodes} {
		for _, code := range codes {
			if _, ok := values[code]; !ok {
				missing = append(missing, code)
			}
		}
	}
	if len(missing) == 0 || s.Quotes == nil {
		applyQuotes(&ladder, values)
		return ladder, nil
	}
	today := now.Format("2006-01-02")
	age, ok := daysBetween(ladder.Date, today)
	if !ok || age > quoteMaxAgeDays {
		applyQuotes(&ladder, values)
		return ladder, nil
	}
	fetched, err := s.Quotes.Quotes(ctx, missing)
	if err != nil || fetched == nil {
		applyQuotes(&ladder, values)
		return ladder, nil
	}
	persist := make(map[string]domain.StockLimitQuote)
	for _, code := range missing {
		quote, ok := fetched[code]
		if !ok || quote.QuoteDate != ladder.Date {
			continue
		}
		value := domain.StockLimitQuote{TradeDate: ladder.Date, StockCode: code}
		if quote.OpenPrice > 0 {
			value.OpenPct = domain.QuoteChangePercent(quote.OpenPrice, quote.PrevClose)
		}
		if brokenSet[code] {
			value.ClosePct = domain.QuoteChangePercent(quote.LatestPrice, quote.PrevClose)
			if quote.LatestPrice > 0 {
				price := quote.LatestPrice
				value.ClosePriceMilli = &price
			}
		}
		values[code] = value
		empty := value.OpenPct == nil && value.ClosePct == nil && value.ClosePriceMilli == nil
		if !ladder.Temporary && !empty {
			persist[code] = value
		}
	}
	if len(persist) > 0 && s.Cache != nil {
		if err := s.Cache.SaveStockLimitQuotes(ctx, ladder.Date, persist, now.Unix()); err != nil {
			return domain.StockLimitLadder{}, err
		}
	}
	applyQuotes(&ladder, values)
	return ladder, nil
}

// applyQuotes mutates the ladder in place; absent values stay absent rather than
// inventing a number.
func applyQuotes(ladder *domain.StockLimitLadder, values map[string]domain.StockLimitQuote) {
	if len(values) == 0 {
		return
	}
	for i := range ladder.Rungs {
		for j := range ladder.Rungs[i].Entries {
			entry := &ladder.Rungs[i].Entries[j]
			if value, ok := values[entry.StockCode]; ok {
				entry.OpenPct = value.OpenPct
			}
		}
		for j := range ladder.Rungs[i].Broken {
			entry := &ladder.Rungs[i].Broken[j]
			value, ok := values[entry.StockCode]
			if !ok {
				continue
			}
			entry.OpenPct = value.OpenPct
			entry.TodayPct = value.ClosePct
			entry.TodayPrice = value.ClosePriceMilli
		}
	}
}

func (s *Service) cacheDay(ctx context.Context, date string) (*domain.StockLimitDay, error) {
	if s.Cache == nil {
		return nil, nil
	}
	return s.Cache.StockLimitDay(ctx, date)
}

func (s *Service) cachedEntries(ctx context.Context, day *domain.StockLimitDay, date string) ([]domain.StockLimitEntry, error) {
	if day == nil || s.Cache == nil {
		return nil, nil
	}
	if day.Status != string(domain.StockLimitOK) {
		return []domain.StockLimitEntry{}, nil
	}
	return s.Cache.StockLimitEntries(ctx, date)
}

func (s *Service) saveDay(ctx context.Context, date string, entries []domain.StockLimitEntry, fetchedAt int64) error {
	if s.Cache == nil {
		return nil
	}
	if len(entries) == 0 {
		return s.Cache.SaveStockLimitDay(ctx, date, string(domain.StockLimitEmpty), 0, fetchedAt)
	}
	return s.Cache.SaveStockLimitEntries(ctx, date, entries, fetchedAt)
}

func emptyLadder(date string, now int64, status domain.StockLimitStatus) domain.StockLimitLadder {
	return domain.StockLimitLadder{
		Date:      date,
		Status:    status,
		FetchedAt: now,
		Source:    "eastmoney",
	}
}

func atMinute(day time.Time, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), minute/60, minute%60, 0, 0, day.Location())
}

func parseDay(date string) (time.Time, bool) {
	if !strictDate(date) {
		return time.Time{}, false
	}
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

// strictDate accepts exactly YYYY-MM-DD; time.Parse alone also accepts a
// single-digit month or day, which would make lexical date comparison wrong.
func strictDate(date string) bool {
	if len(date) != 10 || date[4] != '-' || date[7] != '-' {
		return false
	}
	for i := 0; i < len(date); i++ {
		if i == 4 || i == 7 {
			continue
		}
		if date[i] < '0' || date[i] > '9' {
			return false
		}
	}
	return true
}

func shiftDay(date string, delta int) (string, bool) {
	day, ok := parseDay(date)
	if !ok {
		return "", false
	}
	return day.AddDate(0, 0, delta).Format("2006-01-02"), true
}

func daysBetween(from, to string) (int, bool) {
	if !strictDate(from) || !strictDate(to) {
		return 0, false
	}
	start, errStart := time.Parse("2006-01-02", from)
	end, errEnd := time.Parse("2006-01-02", to)
	if errStart != nil || errEnd != nil {
		return 0, false
	}
	return int(end.Sub(start).Hours() / 24), true
}

func isBeyondRetention(date, today string) bool {
	days, ok := daysBetween(date, today)
	return ok && days > retentionDays
}
