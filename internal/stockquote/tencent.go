package stockquote

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"

	"transactions/internal/domain"
)

// TencentFetcher reads A-share quotes from the Tencent quote endpoint
// (qt.gtimg.cn). The response is GBK, split into one v_<market><code>="..."
// segment per stock; fields are separated by '~'.
//
// Quotes are temporary external data: every network, HTTP, decoding or parsing
// failure is an empty result, never an error a page would show.
type TencentFetcher struct {
	// Client overrides the HTTP client (tests point it at an httptest server).
	// A nil client uses a three-second client.
	Client *http.Client
	// BaseURL overrides the endpoint; a trailing slash is allowed.
	BaseURL string
	// Now supplies the fallback quote time when field 30 is missing or invalid.
	Now func() time.Time
}

var _ Fetcher = (*TencentFetcher)(nil)

const (
	tencentBaseURL = "http://qt.gtimg.cn"
	quoteTimeout   = 3 * time.Second
	// Field indexes in the Tencent payload, matching the old project.
	fieldName      = 1
	fieldCode      = 2
	fieldLatest    = 3
	fieldPrevClose = 4
	fieldOpen      = 5
	fieldQuoteTime = 30
)

var decimalPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

// NewTencentFetcher returns the production quote source.
func NewTencentFetcher() *TencentFetcher { return &TencentFetcher{} }

// Quotes fetches the requested codes in one request. Unknown or malformed codes
// are dropped before the request; a failed request returns an empty map.
func (f *TencentFetcher) Quotes(ctx context.Context, codes []string) (map[string]domain.StockQuote, error) {
	result := make(map[string]domain.StockQuote)
	requested := make([]string, 0, len(codes))
	query := make([]string, 0, len(codes))
	seen := make(map[string]bool, len(codes))
	for _, raw := range codes {
		code := strings.TrimSpace(raw)
		instrument, ok := domain.InstrumentOf(code)
		if !ok || seen[code] {
			continue
		}
		seen[code] = true
		requested = append(requested, code)
		query = append(query, marketPrefix(instrument)+code)
	}
	if len(query) == 0 {
		return result, nil
	}
	payload, err := f.fetch(ctx, f.baseURL()+"/q="+strings.Join(query, ","))
	if err != nil {
		return result, nil
	}
	parsed := parseTencentQuotes(payload, f.now().Unix())
	for _, code := range requested {
		if quote, ok := parsed[code]; ok {
			result[code] = quote
		}
	}
	return result, nil
}

// Name returns the name in the first quoted segment, or "" when the request or
// the payload gave none.
func (f *TencentFetcher) Name(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	instrument, ok := domain.InstrumentOf(code)
	if !ok {
		return "", nil
	}
	payload, err := f.fetch(ctx, f.baseURL()+"/q="+marketPrefix(instrument)+code)
	if err != nil {
		return "", nil
	}
	fields := allQuoteFields(payload)
	if len(fields) == 0 || len(fields[0]) <= fieldName {
		return "", nil
	}
	return strings.TrimSpace(fields[0][fieldName]), nil
}

func (f *TencentFetcher) baseURL() string {
	base := strings.TrimSpace(f.BaseURL)
	if base == "" {
		return tencentBaseURL
	}
	return strings.TrimRight(base, "/")
}

func (f *TencentFetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return &http.Client{Timeout: quoteTimeout}
}

func (f *TencentFetcher) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// fetch performs one request and decodes the GBK body. Any failure is returned
// to the caller, which turns it into the empty result the interface promises.
func (f *TencentFetcher) fetch(ctx context.Context, target string) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, quoteTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	response, err := f.client().Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("腾讯行情 HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	text, err := simplifiedchinese.GBK.NewDecoder().Bytes(body)
	if err != nil {
		// A malformed byte should not hide the rest of the payload; the old
		// client fell back to UTF-8 and kept the (possibly garbled) names.
		return string(body), nil
	}
	return string(text), nil
}

func marketPrefix(instrument domain.Instrument) string {
	switch instrument {
	case domain.InstrumentShanghaiStock, domain.InstrumentShanghaiFund:
		return "sh"
	default:
		return "sz"
	}
}

// allQuoteFields scans every v_xxx="..." segment in a payload. A batch reply is
// several lines; scanning (instead of a regexp) keeps malformed segments from
// swallowing the rest.
func allQuoteFields(payload string) [][]string {
	var result [][]string
	rest := payload
	for {
		start := strings.Index(rest, "v_")
		if start < 0 {
			break
		}
		candidate := rest[start+2:]
		equal := strings.IndexByte(candidate, '=')
		if equal < 0 {
			break
		}
		after := candidate[equal+1:]
		if len(after) == 0 || after[0] != '"' {
			rest = candidate
			continue
		}
		body := after[1:]
		end := strings.IndexByte(body, '"')
		if end < 0 {
			break
		}
		result = append(result, strings.Split(body[:end], "~"))
		rest = body[end+1:]
	}
	return result
}

// parseTencentQuotes parses every valid A-share quote. A stopped stock (latest
// price 0) or an illegal price is silently skipped. A missing open stays 0; a
// missing or invalid quote time falls back to now and leaves QuoteDate empty.
func parseTencentQuotes(payload string, now int64) map[string]domain.StockQuote {
	result := make(map[string]domain.StockQuote)
	for _, parts := range allQuoteFields(payload) {
		if len(parts) <= fieldPrevClose {
			continue
		}
		code := strings.TrimSpace(parts[fieldCode])
		if _, ok := domain.InstrumentOf(code); !ok {
			continue
		}
		latest, ok := parseDecimal(parts[fieldLatest])
		if !ok || latest <= 0 {
			continue
		}
		prevClose, ok := parseDecimal(parts[fieldPrevClose])
		if !ok || prevClose <= 0 {
			continue
		}
		var open float64
		if len(parts) > fieldOpen {
			if value, ok := parseDecimal(parts[fieldOpen]); ok && value > 0 {
				open = value
			}
		}
		quoteTime := now
		quoteDate := ""
		if len(parts) > fieldQuoteTime {
			raw := strings.TrimSpace(parts[fieldQuoteTime])
			if parsed, ok := parseQuoteTime(raw); ok {
				quoteTime = parsed
				if len(raw) == 14 {
					quoteDate = raw[:4] + "-" + raw[4:6] + "-" + raw[6:8]
				}
			}
		}
		result[code] = domain.StockQuote{
			StockCode:   code,
			LatestPrice: domain.PriceYuanToMilli(latest),
			PrevClose:   domain.PriceYuanToMilli(prevClose),
			OpenPrice:   domain.PriceYuanToMilli(open),
			QuoteTime:   quoteTime,
			QuoteDate:   quoteDate,
		}
	}
	return result
}

// parseDecimal accepts only ordinary decimal text: Go's ParseFloat also takes
// hexadecimal floats, which the quote endpoint never sends and which should be
// treated as an invalid price.
func parseDecimal(raw string) (float64, bool) {
	text := strings.TrimSpace(raw)
	if !decimalPattern.MatchString(text) {
		return 0, false
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

// parseQuoteTime parses YYYYMMDDHHMMSS. The old implementation read the
// timestamp as UTC (NaiveDateTime::and_utc), and the quote time is only used as
// "when this snapshot was taken", so the same interpretation is kept.
func parseQuoteTime(raw string) (int64, bool) {
	if len(raw) != 14 {
		return 0, false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return 0, false
		}
	}
	value, err := time.Parse("20060102150405", raw)
	if err != nil {
		return 0, false
	}
	return value.Unix(), true
}
