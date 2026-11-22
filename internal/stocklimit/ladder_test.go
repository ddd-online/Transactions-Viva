package stocklimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"transactions/internal/domain"
)

type fakeCache struct {
	days       map[string]*domain.StockLimitDay
	entries    map[string][]domain.StockLimitEntry
	quotes     map[string]map[string]domain.StockLimitQuote
	daySaves   []string
	entrySaves []string
	quoteSaves []string
	savedQuote map[string]map[string]domain.StockLimitQuote
	err        error
}

func newFakeCache() *fakeCache {
	return &fakeCache{
		days:       make(map[string]*domain.StockLimitDay),
		entries:    make(map[string][]domain.StockLimitEntry),
		quotes:     make(map[string]map[string]domain.StockLimitQuote),
		savedQuote: make(map[string]map[string]domain.StockLimitQuote),
	}
}

func (c *fakeCache) StockLimitDay(_ context.Context, date string) (*domain.StockLimitDay, error) {
	if c.err != nil {
		return nil, c.err
	}
	day, ok := c.days[date]
	if !ok {
		return nil, nil
	}
	copy := *day
	return &copy, nil
}

func (c *fakeCache) StockLimitEntries(_ context.Context, date string) ([]domain.StockLimitEntry, error) {
	if c.err != nil {
		return nil, c.err
	}
	return append([]domain.StockLimitEntry(nil), c.entries[date]...), nil
}

func (c *fakeCache) SaveStockLimitDay(_ context.Context, date, status string, count int, fetchedAt int64) error {
	if c.err != nil {
		return c.err
	}
	c.daySaves = append(c.daySaves, date)
	c.days[date] = &domain.StockLimitDay{TradeDate: date, Status: status, Count: count, FetchedAt: fetchedAt}
	c.entries[date] = nil
	return nil
}

func (c *fakeCache) SaveStockLimitEntries(_ context.Context, date string, entries []domain.StockLimitEntry, fetchedAt int64) error {
	if c.err != nil {
		return c.err
	}
	c.entrySaves = append(c.entrySaves, date)
	c.entries[date] = append([]domain.StockLimitEntry(nil), entries...)
	c.days[date] = &domain.StockLimitDay{TradeDate: date, Status: "ok", Count: len(entries), FetchedAt: fetchedAt}
	return nil
}

func (c *fakeCache) StockLimitQuotes(_ context.Context, date string) (map[string]domain.StockLimitQuote, error) {
	if c.err != nil {
		return nil, c.err
	}
	stored := make(map[string]domain.StockLimitQuote)
	for code, quote := range c.quotes[date] {
		stored[code] = quote
	}
	return stored, nil
}

func (c *fakeCache) SaveStockLimitQuotes(_ context.Context, date string, quotes map[string]domain.StockLimitQuote, _ int64) error {
	if c.err != nil {
		return c.err
	}
	c.quoteSaves = append(c.quoteSaves, date)
	copied := make(map[string]domain.StockLimitQuote, len(quotes))
	for code, quote := range quotes {
		copied[code] = quote
	}
	c.savedQuote[date] = copied
	if c.quotes[date] == nil {
		c.quotes[date] = make(map[string]domain.StockLimitQuote)
	}
	for code, quote := range quotes {
		c.quotes[date][code] = quote
	}
	return nil
}

func (c *fakeCache) seed(date, status string, entries []domain.StockLimitEntry) {
	c.days[date] = &domain.StockLimitDay{TradeDate: date, Status: status, Count: len(entries), FetchedAt: 1_700_000_000}
	c.entries[date] = append([]domain.StockLimitEntry(nil), entries...)
}

type fakeSource struct {
	entries map[string][]domain.StockLimitEntry
	calls   []string
	err     error
}

func (s *fakeSource) FetchDay(_ context.Context, date string) ([]domain.StockLimitEntry, error) {
	s.calls = append(s.calls, date)
	if s.err != nil {
		return nil, s.err
	}
	return append([]domain.StockLimitEntry(nil), s.entries[date]...), nil
}

type fakeQuotes struct {
	quotes map[string]domain.StockQuote
	calls  [][]string
	err    error
}

func (q *fakeQuotes) Quotes(_ context.Context, codes []string) (map[string]domain.StockQuote, error) {
	q.calls = append(q.calls, append([]string(nil), codes...))
	if q.err != nil {
		return nil, q.err
	}
	return q.quotes, nil
}

func (q *fakeQuotes) Name(context.Context, string) (string, error) { return "", nil }

func clockAt(value string) func() time.Time {
	return func() time.Time {
		parsed, err := time.ParseInLocation("2006-01-02 15:04", value, time.Local)
		if err != nil {
			panic(err)
		}
		return parsed
	}
}

func testEntry(code string, level, seal int, board string) domain.StockLimitEntry {
	return domain.StockLimitEntry{
		StockCode: code, StockName: code, PriceMilli: 10_000, Level: level,
		FirstSeal: seal, Industry: board,
	}
}

func TestLadderPreopenOffdayFuture(t *testing.T) {
	cache := newFakeCache()
	source := &fakeSource{}
	service := NewService(cache, source, nil)
	service.Clock = clockAt("2026-11-20 09:00")

	ladder, err := service.Ladder(context.Background(), "2026-11-20", false)
	if err != nil {
		t.Fatalf("preopen: %v", err)
	}
	if ladder.Status != domain.StockLimitPreopen || len(source.calls) != 0 {
		t.Fatalf("preopen = %+v, calls = %v", ladder.Status, source.calls)
	}

	service.Clock = clockAt("2026-11-21 10:00")
	ladder, err = service.Ladder(context.Background(), "2026-11-21", false)
	if err != nil {
		t.Fatalf("offday: %v", err)
	}
	if ladder.Status != domain.StockLimitOffday || len(source.calls) != 0 {
		t.Fatalf("offday = %+v, calls = %v", ladder.Status, source.calls)
	}

	service.Clock = clockAt("2026-11-20 10:00")
	if _, err := service.Ladder(context.Background(), "2026-11-21", false); err == nil {
		t.Fatal("future date should be an error")
	}
}

func TestLadderClosedReadsAndWritesCache(t *testing.T) {
	cache := newFakeCache()
	cache.seed("2026-11-19", "ok", []domain.StockLimitEntry{
		testEntry("600001", 1, 93_000, "银行"),
		testEntry("600002", 1, 94_000, "银行"),
	})
	source := &fakeSource{entries: map[string][]domain.StockLimitEntry{
		"2026-11-20": {testEntry("600001", 2, 95_000, "银行")},
	}}
	service := NewService(cache, source, nil)
	service.Clock = clockAt("2026-11-20 16:00")

	ladder, err := service.Ladder(context.Background(), "2026-11-20", false)
	if err != nil {
		t.Fatalf("Ladder: %v", err)
	}
	if ladder.Status != domain.StockLimitOK || ladder.Temporary || ladder.FromCache {
		t.Fatalf("ladder = %+v", ladder)
	}
	if len(source.calls) != 1 || source.calls[0] != "2026-11-20" {
		t.Fatalf("source calls = %v", source.calls)
	}
	if len(cache.entrySaves) != 1 || cache.entrySaves[0] != "2026-11-20" {
		t.Fatalf("entry saves = %v", cache.entrySaves)
	}
	if ladder.PreviousDate != "2026-11-19" {
		t.Fatalf("previous date = %q", ladder.PreviousDate)
	}
	if ladder.Metrics.MaxLevel == nil || *ladder.Metrics.MaxLevel != 2 {
		t.Fatalf("max level = %v", ladder.Metrics.MaxLevel)
	}
	if len(ladder.Rungs) != 2 || len(ladder.Rungs[0].Broken) != 1 {
		t.Fatalf("rungs = %+v", ladder.Rungs)
	}

	// A closed day is read from the cache; no second request.
	again, err := service.Ladder(context.Background(), "2026-11-20", false)
	if err != nil {
		t.Fatalf("cached Ladder: %v", err)
	}
	if !again.FromCache || again.Stale {
		t.Fatalf("cached ladder = %+v", again)
	}
	if len(source.calls) != 1 {
		t.Fatalf("source calls after cache read = %v", source.calls)
	}
	if again.FetchedAt == 0 {
		t.Fatal("cached ladder lost fetchedAt")
	}

	// refresh forces a new request.
	if _, err := service.Ladder(context.Background(), "2026-11-20", true); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(source.calls) != 2 {
		t.Fatalf("source calls after refresh = %v", source.calls)
	}
}

func TestLadderIntradayRendersWithoutWriting(t *testing.T) {
	cache := newFakeCache()
	cache.seed("2026-11-19", "ok", []domain.StockLimitEntry{testEntry("600002", 1, 94_000, "银行")})
	source := &fakeSource{entries: map[string][]domain.StockLimitEntry{
		"2026-11-20": {testEntry("600001", 1, 95_000, "银行")},
	}}
	quotes := &fakeQuotes{quotes: map[string]domain.StockQuote{
		"600001": {StockCode: "600001", LatestPrice: 11_000, PrevClose: 10_000, OpenPrice: 10_200, QuoteDate: "2026-11-20"},
		"600002": {StockCode: "600002", LatestPrice: 9_800, PrevClose: 10_000, OpenPrice: 10_100, QuoteDate: "2026-11-20"},
	}}
	service := NewService(cache, source, quotes)
	service.Clock = clockAt("2026-11-20 10:00")

	ladder, err := service.Ladder(context.Background(), "2026-11-20", false)
	if err != nil {
		t.Fatalf("Ladder: %v", err)
	}
	if !ladder.Temporary || ladder.Status != domain.StockLimitOK {
		t.Fatalf("intraday ladder = %+v", ladder)
	}
	if len(quotes.calls) != 1 {
		t.Fatalf("intraday quotes should still be shown: %v", quotes.calls)
	}
	if len(ladder.Rungs[0].Entries) != 1 || ladder.Rungs[0].Entries[0].OpenPct == nil {
		t.Fatalf("intraday open pct = %+v", ladder.Rungs[0].Entries)
	}
	for _, date := range append(append([]string{}, cache.daySaves...), cache.entrySaves...) {
		if date == "2026-11-20" {
			t.Fatalf("intraday day was written: saves = %v", append(cache.daySaves, cache.entrySaves...))
		}
	}
	if len(cache.quoteSaves) != 0 {
		t.Fatalf("intraday quotes were written: %v", cache.quoteSaves)
	}
}

func TestLadderFallsBackToCacheWhenRefreshFails(t *testing.T) {
	cache := newFakeCache()
	cache.seed("2026-11-20", "ok", []domain.StockLimitEntry{testEntry("600001", 1, 95_000, "银行")})
	cache.seed("2026-11-19", "ok", []domain.StockLimitEntry{testEntry("600002", 1, 94_000, "银行")})
	source := &fakeSource{err: errors.New("network down")}
	service := NewService(cache, source, nil)
	service.Clock = clockAt("2026-11-20 16:00")

	ladder, err := service.Ladder(context.Background(), "2026-11-20", true)
	if err != nil {
		t.Fatalf("Ladder: %v", err)
	}
	if !ladder.FromCache || !ladder.Stale || ladder.Status != domain.StockLimitOK {
		t.Fatalf("fallback ladder = %+v", ladder)
	}
	if ladder.FetchedAt != 1_700_000_000 {
		t.Fatalf("fallback fetchedAt = %d", ladder.FetchedAt)
	}
}

func TestLadderUnavailableBeyondRetention(t *testing.T) {
	cache := newFakeCache()
	source := &fakeSource{}
	service := NewService(cache, source, nil)
	service.Clock = clockAt("2026-11-20 16:00")

	ladder, err := service.Ladder(context.Background(), "2026-09-30", false)
	if err != nil {
		t.Fatalf("Ladder: %v", err)
	}
	if ladder.Status != domain.StockLimitUnavailable {
		t.Fatalf("status = %q, want unavailable", ladder.Status)
	}
	if len(cache.daySaves) != 0 || len(cache.entrySaves) != 0 {
		t.Fatalf("unavailable day was cached: %v %v", cache.daySaves, cache.entrySaves)
	}
	if len(source.calls) != 1 || source.calls[0] != "2026-09-30" {
		t.Fatalf("source calls = %v", source.calls)
	}
}

func TestLadderPreviousLookbackSkipsEmptyMarkers(t *testing.T) {
	cache := newFakeCache()
	cache.seed("2026-11-19", "empty", nil)
	cache.seed("2026-11-18", "ok", []domain.StockLimitEntry{testEntry("600002", 1, 94_000, "银行")})
	source := &fakeSource{entries: map[string][]domain.StockLimitEntry{
		"2026-11-20": {testEntry("600001", 2, 95_000, "银行")},
	}}
	service := NewService(cache, source, nil)
	service.Clock = clockAt("2026-11-20 16:00")

	ladder, err := service.Ladder(context.Background(), "2026-11-20", false)
	if err != nil {
		t.Fatalf("Ladder: %v", err)
	}
	if ladder.PreviousDate != "2026-11-18" {
		t.Fatalf("previous date = %q, want 2026-11-18", ladder.PreviousDate)
	}
}

func TestLadderQuotesOnlyTrustTheirOwnDate(t *testing.T) {
	cache := newFakeCache()
	cache.seed("2026-11-19", "ok", []domain.StockLimitEntry{
		testEntry("600002", 1, 94_000, "银行"),
	})
	source := &fakeSource{entries: map[string][]domain.StockLimitEntry{
		"2026-11-20": {
			testEntry("600001", 2, 95_000, "银行"),
			testEntry("600003", 1, 96_000, "软件"),
		},
	}}
	quotes := &fakeQuotes{quotes: map[string]domain.StockQuote{
		"600001": {StockCode: "600001", LatestPrice: 11_000, PrevClose: 10_000, OpenPrice: 10_200, QuoteDate: "2026-11-20"},
		"600002": {StockCode: "600002", LatestPrice: 9_500, PrevClose: 10_000, OpenPrice: 10_100, QuoteDate: "2026-11-20"},
		"600003": {StockCode: "600003", LatestPrice: 12_000, PrevClose: 10_000, OpenPrice: 10_000, QuoteDate: "2026-11-19"},
	}}
	service := NewService(cache, source, quotes)
	service.Clock = clockAt("2026-11-20 16:00")

	ladder, err := service.Ladder(context.Background(), "2026-11-20", false)
	if err != nil {
		t.Fatalf("Ladder: %v", err)
	}
	if len(quotes.calls) != 1 {
		t.Fatalf("quote calls = %v", quotes.calls)
	}
	var limitEntry, wrongDate *domain.StockLimitEntry
	for i := range ladder.Rungs {
		for j := range ladder.Rungs[i].Entries {
			entry := &ladder.Rungs[i].Entries[j]
			switch entry.StockCode {
			case "600001":
				limitEntry = entry
			case "600003":
				wrongDate = entry
			}
		}
	}
	if limitEntry == nil || limitEntry.OpenPct == nil || *limitEntry.OpenPct != 2 {
		t.Fatalf("limit open pct = %+v", limitEntry)
	}
	if wrongDate == nil || wrongDate.OpenPct != nil {
		t.Fatalf("wrong-date quote should not be trusted: %+v", wrongDate)
	}
	var broken *domain.StockLimitBrokenEntry
	for i := range ladder.Rungs {
		for j := range ladder.Rungs[i].Broken {
			if ladder.Rungs[i].Broken[j].StockCode == "600002" {
				broken = &ladder.Rungs[i].Broken[j]
			}
		}
	}
	if broken == nil || broken.OpenPct == nil || *broken.OpenPct != 1 {
		t.Fatalf("broken open pct = %+v", broken)
	}
	if broken.TodayPct == nil || *broken.TodayPct != -5 {
		t.Fatalf("broken today pct = %v", broken.TodayPct)
	}
	if broken.TodayPrice == nil || *broken.TodayPrice != 9_500 {
		t.Fatalf("broken today price = %v", broken.TodayPrice)
	}

	saved := cache.savedQuote["2026-11-20"]
	if len(saved) != 2 {
		t.Fatalf("saved quotes = %+v", saved)
	}
	if _, ok := saved["600003"]; ok {
		t.Fatalf("wrong-date quote was persisted: %+v", saved)
	}

	// A finalized day reads its stored snapshot. The wrong-date code had no
	// trusted value to store, so it is retried (and still not trusted).
	before := len(quotes.calls)
	if _, err := service.Ladder(context.Background(), "2026-11-20", false); err != nil {
		t.Fatalf("cached Ladder: %v", err)
	}
	if len(quotes.calls) != before+1 {
		t.Fatalf("stored snapshot should cover the trusted codes: %v", quotes.calls)
	}
	if last := quotes.calls[len(quotes.calls)-1]; len(last) != 1 || last[0] != "600003" {
		t.Fatalf("second quote request = %v, want only the untrusted code", last)
	}
	if len(cache.savedQuote["2026-11-20"]) != 2 {
		t.Fatalf("untrusted quote was persisted: %+v", cache.savedQuote["2026-11-20"])
	}
}

func TestEastmoneySourceFetchDay(t *testing.T) {
	body := `{
		"rc": 0,
		"data": {"qdate": 20261120, "tc": 4, "pool": [
			{"c":"600001","n":"甲股票","m":1,"p":11000,"zdp":10.01,"amount":1234.5,
			 "ltsz":5678.5,"hs":3.45,"lbc":0,"fbt":93000,"lbt":145959,"zbc":1,
			 "fund":4567.5,"hybk":"银行","zttj":{"days":3,"ct":2}},
			{"c":"600001","n":"重复","p":12000,"fbt":100000},
			{"c":"300001","n":"乙股票","m":0,"p":22000,"zdp":20,"amount":1,
			 "ltsz":2,"hs":1,"lbc":2,"fbt":92500,"lbt":150000,"zbc":0,
			 "fund":3,"hybk":"软件"},
			{"c":"bad","n":"非法"}
		]}
	}`
	var gotDate string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDate = r.URL.Query().Get("date")
		if r.URL.Query().Get("ut") == "" {
			t.Errorf("request is missing the public token: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	source := &EastmoneySource{Client: server.Client(), BaseURL: server.URL}
	entries, err := source.FetchDay(context.Background(), "2026-11-20")
	if err != nil {
		t.Fatalf("FetchDay: %v", err)
	}
	if gotDate != "20261120" {
		t.Fatalf("query date = %q", gotDate)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].StockCode != "300001" || entries[0].Level != 2 || entries[0].PriceMilli != 22_000 {
		t.Fatalf("first entry = %+v", entries[0])
	}
	first := entries[1]
	if first.StockName != "甲股票" || first.Level != 1 {
		t.Fatalf("duplicate code should keep the first row with level at least 1: %+v", first)
	}
	if first.PriceMilli != 11_000 || first.AmountCents != 123_450 || first.FloatMvCents != 567_850 {
		t.Fatalf("price / amounts = %+v", first)
	}
	if first.SealFundCents != 456_750 || first.Turnover != 3.45 {
		t.Fatalf("seal fund / turnover = %+v", first)
	}
	if first.FirstSeal != 93_000 || first.LastSeal != 145_959 || first.BrokenCount != 1 {
		t.Fatalf("seal times = %+v", first)
	}
	if first.Industry != "银行" || first.StatDays != 3 || first.StatCount != 2 {
		t.Fatalf("board / stats = %+v", first)
	}
}

func TestEastmoneySourceRejectsOtherTradeDate(t *testing.T) {
	body := `{"rc":0,"data":{"qdate":20261119,"pool":[{"c":"600001","n":"甲","p":11000,"fbt":93000}]}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	source := &EastmoneySource{Client: server.Client(), BaseURL: server.URL}
	entries, err := source.FetchDay(context.Background(), "2026-11-20")
	if err != nil {
		t.Fatalf("FetchDay: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("qdate mismatch entries = %+v, want empty", entries)
	}
}

func TestEastmoneySourceReportsFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"rc":102,"data":null}`))
	}))
	defer server.Close()
	source := &EastmoneySource{Client: server.Client(), BaseURL: server.URL}
	if _, err := source.FetchDay(context.Background(), "2026-11-20"); err == nil {
		t.Fatal("rc 102 should be an error")
	}
	if _, err := source.FetchDay(context.Background(), "2026-11-20x"); err == nil {
		t.Fatal("invalid date should be an error")
	}
}

// The source must not return the previous trading day's pool for a weekend; the
// service also refuses to ask, so this pins the adapter half.
func TestPoolTradeDateFormatting(t *testing.T) {
	if got := poolTradeDate(20261120); got != "2026-11-20" {
		t.Fatalf("pool trade date = %q", got)
	}
	if got := poolTradeDate(0); got != "" {
		t.Fatalf("missing trade date = %q", got)
	}
}
