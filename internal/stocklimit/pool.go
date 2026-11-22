package stocklimit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"transactions/internal/domain"
)

const (
	eastmoneyBaseURL  = "https://push2ex.eastmoney.com"
	eastmoneyPath     = "/getTopicZTPool"
	eastmoneyToken    = "7eea3edcaed734bea9cbfc24409ed989"
	eastmoneyPageSize = 1000
	eastmoneyTimeout  = 8 * time.Second
)

var limitCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

// EastmoneySource reads the Eastmoney limit-up pool for one trading day.
//
// The endpoint is public but undocumented: requests must carry the public ut
// token and a browser user agent. A response whose qdate is not the requested
// date (a weekend, a holiday, or a date outside the retention window) is an
// empty pool, not the nearest trading day's pool. Every actual failure is an
// error, so the service can tell "no limit-ups" from "could not fetch".
type EastmoneySource struct {
	// Client overrides the HTTP client; a nil client uses an eight-second one.
	Client *http.Client
	// BaseURL overrides the endpoint; a trailing slash is allowed.
	BaseURL string
	// Now supplies the cache-busting query parameter (tests may fix it).
	Now func() time.Time
}

var _ Source = (*EastmoneySource)(nil)

// NewEastmoneySource returns the production limit-up pool source.
func NewEastmoneySource() *EastmoneySource { return &EastmoneySource{} }

// FetchDay fetches one day's limit-up pool. date is a local YYYY-MM-DD date.
func (s *EastmoneySource) FetchDay(ctx context.Context, date string) ([]domain.StockLimitEntry, error) {
	queryDate := strings.ReplaceAll(date, "-", "")
	if !strictDate(date) {
		return nil, fmt.Errorf("日期格式无效")
	}
	params := url.Values{}
	params.Set("ut", eastmoneyToken)
	params.Set("dpt", "wz.ztzt")
	params.Set("Pageindex", "0")
	params.Set("pagesize", strconv.Itoa(eastmoneyPageSize))
	params.Set("sort", "fbt:asc")
	params.Set("date", queryDate)
	params.Set("_", strconv.FormatInt(s.now().UnixMilli(), 10))
	target := s.baseURL() + eastmoneyPath + "?" + params.Encode()

	requestCtx, cancel := context.WithTimeout(ctx, eastmoneyTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("查询涨停池失败（网络）：%w", err)
	}
	request.Header.Set("Accept", "application/json, text/plain, */*")
	request.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	response, err := s.client().Do(request)
	if err != nil {
		return nil, fmt.Errorf("查询涨停池失败（网络）：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查询涨停池失败（HTTP %d）", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取涨停池响应失败：%w", err)
	}
	payload, err := parseLimitPoolPayload(string(body))
	if err != nil {
		return nil, err
	}
	// The source answers holidays and dates beyond its retention window with the
	// latest trading day's pool; that must not be shown as this day's data.
	if payload.tradeDate != "" && payload.tradeDate != date {
		return []domain.StockLimitEntry{}, nil
	}
	return payload.entries, nil
}

func (s *EastmoneySource) baseURL() string {
	base := strings.TrimSpace(s.BaseURL)
	if base == "" {
		return eastmoneyBaseURL
	}
	return strings.TrimRight(base, "/")
}

func (s *EastmoneySource) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: eastmoneyTimeout}
}

func (s *EastmoneySource) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// flexNumber reads a number that the undocumented endpoint might send as a
// JSON number, a numeric string, null or a malformed value. Malformed values
// become 0, matching the old client's finiteNumber fallback.
type flexNumber float64

func (n *flexNumber) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*n = 0
		return nil
	}
	if text[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			*n = 0
			return nil
		}
		text = strings.TrimSpace(value)
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		*n = 0
		return nil
	}
	*n = flexNumber(value)
	return nil
}

// flexString reads a JSON string; a missing, null or non-string value is "".
type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" || text[0] != '"' {
		*s = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		*s = ""
		return nil
	}
	*s = flexString(value)
	return nil
}

type rawLimitEntry struct {
	Code      flexString  `json:"c"`
	Name      *flexString `json:"n"`
	Market    flexNumber  `json:"m"`
	Price     flexNumber  `json:"p"`
	Pct       flexNumber  `json:"zdp"`
	Amount    flexNumber  `json:"amount"`
	FloatMV   flexNumber  `json:"ltsz"`
	Turnover  flexNumber  `json:"hs"`
	Level     flexNumber  `json:"lbc"`
	FirstSeal flexNumber  `json:"fbt"`
	LastSeal  flexNumber  `json:"lbt"`
	Broken    flexNumber  `json:"zbc"`
	SealFund  flexNumber  `json:"fund"`
	Industry  flexString  `json:"hybk"`
	Stats     *struct {
		Days flexNumber `json:"days"`
		CT   flexNumber `json:"ct"`
	} `json:"zttj"`
}

type rawLimitPayload struct {
	RC   *flexNumber `json:"rc"`
	Data *struct {
		QDate flexNumber       `json:"qdate"`
		Pool  *[]rawLimitEntry `json:"pool"`
	} `json:"data"`
}

type limitPoolPayload struct {
	entries   []domain.StockLimitEntry
	tradeDate string
}

func integerValue(value flexNumber) int64 { return int64(math.Trunc(float64(value))) }

func roundTwo(value flexNumber) float64 {
	return math.Round(float64(value)*100) / 100
}

func poolTradeDate(value flexNumber) string {
	raw := integerValue(value)
	if raw < 10000101 || raw > 99991231 {
		return ""
	}
	text := strconv.FormatInt(raw, 10)
	return text[:4] + "-" + text[4:6] + "-" + text[6:8]
}

// parseLimitPoolPayload parses the pool JSON. A bad JSON body, a non-zero rc or
// a missing pool array is an error. Per-entry validation follows the old
// client: six-digit codes only, level below 1 becomes 1, duplicate codes keep
// the first row, and entries are sorted by first seal time then code.
func parseLimitPoolPayload(text string) (*limitPoolPayload, error) {
	var raw rawLimitPayload
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("涨停池接口返回异常")
	}
	if raw.RC == nil || integerValue(*raw.RC) != 0 {
		return nil, fmt.Errorf("涨停池接口返回异常")
	}
	if raw.Data == nil || raw.Data.Pool == nil {
		return nil, fmt.Errorf("涨停池接口返回异常")
	}
	payload := &limitPoolPayload{tradeDate: poolTradeDate(raw.Data.QDate)}
	seen := make(map[string]bool, len(*raw.Data.Pool))
	for _, item := range *raw.Data.Pool {
		code := strings.TrimSpace(string(item.Code))
		if !limitCodePattern.MatchString(code) || seen[code] {
			continue
		}
		seen[code] = true
		name := code
		if item.Name != nil {
			name = strings.TrimSpace(string(*item.Name))
		}
		level := int(integerValue(item.Level))
		if level < 1 {
			level = 1
		}
		entry := domain.StockLimitEntry{
			StockCode:     code,
			StockName:     name,
			Market:        int(integerValue(item.Market)),
			PriceMilli:    integerValue(item.Price),
			Pct:           roundTwo(item.Pct),
			AmountCents:   int64(math.Trunc(float64(item.Amount) * 100)),
			FloatMvCents:  int64(math.Trunc(float64(item.FloatMV) * 100)),
			Turnover:      roundTwo(item.Turnover),
			Level:         level,
			FirstSeal:     int(integerValue(item.FirstSeal)),
			LastSeal:      int(integerValue(item.LastSeal)),
			BrokenCount:   int(integerValue(item.Broken)),
			SealFundCents: int64(math.Trunc(float64(item.SealFund) * 100)),
			Industry:      strings.TrimSpace(string(item.Industry)),
		}
		if item.Stats != nil {
			entry.StatDays = int(integerValue(item.Stats.Days))
			entry.StatCount = int(integerValue(item.Stats.CT))
		}
		payload.entries = append(payload.entries, entry)
	}
	sortEntriesBySeal(payload.entries)
	return payload, nil
}

func sortEntriesBySeal(entries []domain.StockLimitEntry) {
	// Insertion sort keeps the function allocation-free and stable; pools are
	// small (a day seldom has more than a few hundred limit-ups).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			if entries[j-1].FirstSeal < entries[j].FirstSeal ||
				(entries[j-1].FirstSeal == entries[j].FirstSeal && entries[j-1].StockCode <= entries[j].StockCode) {
				break
			}
			entries[j-1], entries[j] = entries[j], entries[j-1]
		}
	}
}
