package stockquote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func quoteLine(name, code, latest, prevClose, open, stamp string) string {
	fields := make([]string, 31)
	fields[0] = "1"
	fields[1] = name
	fields[2] = code
	fields[3] = latest
	fields[4] = prevClose
	fields[5] = open
	fields[30] = stamp
	prefix := "sh"
	if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") {
		prefix = "sz"
	}
	return "v_" + prefix + code + "=\"" + strings.Join(fields, "~") + "\";"
}

func gbkPayload(t *testing.T, text string) []byte {
	t.Helper()
	encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatalf("encode GBK: %v", err)
	}
	return encoded
}

func TestTencentFetcherQuotes(t *testing.T) {
	payload := quoteLine("浦发银行", "600000", "10.50", "10.00", "10.20", "20261120150000") + "\n" +
		quoteLine("平安银行", "000001", "12.34", "12.00", "12.10", "20261120150000") + "\n" +
		quoteLine("停牌股", "600002", "0", "9.00", "0", "20261120150000")
	encoded := gbkPayload(t, payload)
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "text/plain; charset=GBK")
		_, _ = w.Write(encoded)
	}))
	defer server.Close()

	fetcher := NewTencentFetcher()
	fetcher.BaseURL = server.URL
	fetcher.Now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	quotes, err := fetcher.Quotes(context.Background(),
		[]string{"600000", "000001", "600000", "invalid", "600002"})
	if err != nil {
		t.Fatalf("Quotes: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("requests = %d, want 1", len(paths))
	}
	if want := "/q=sh600000,sz000001,sh600002"; paths[0] != want {
		t.Fatalf("path = %q, want %q", paths[0], want)
	}
	if len(quotes) != 2 {
		t.Fatalf("quotes = %+v, want two (a stopped stock is absent)", quotes)
	}
	quote := quotes["600000"]
	if quote.StockCode != "600000" || quote.LatestPrice != 10_500 || quote.PrevClose != 10_000 ||
		quote.OpenPrice != 10_200 {
		t.Fatalf("600000 quote = %+v", quote)
	}
	if quote.QuoteDate != "2026-11-20" {
		t.Fatalf("quote date = %q", quote.QuoteDate)
	}
	if want := time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC).Unix(); quote.QuoteTime != want {
		t.Fatalf("quote time = %d, want %d", quote.QuoteTime, want)
	}
	if _, ok := quotes["600002"]; ok {
		t.Fatalf("stopped stock should be dropped: %+v", quotes["600002"])
	}
}

func TestTencentFetcherName(t *testing.T) {
	encoded := gbkPayload(t, quoteLine("贵州茅台", "600519", "1500.00", "1490.00", "1495.00", "20261120150000"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(encoded)
	}))
	defer server.Close()

	fetcher := NewTencentFetcher()
	fetcher.BaseURL = server.URL
	name, err := fetcher.Name(context.Background(), "600519")
	if err != nil {
		t.Fatalf("Name: %v", err)
	}
	if name != "贵州茅台" {
		t.Fatalf("name = %q", name)
	}
	if name, err := fetcher.Name(context.Background(), "not-a-code"); err != nil || name != "" {
		t.Fatalf("invalid code name = %q, err = %v", name, err)
	}
}

func TestTencentFetcherFailureIsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	fetcher := NewTencentFetcher()
	fetcher.BaseURL = server.URL
	quotes, err := fetcher.Quotes(context.Background(), []string{"600000"})
	if err != nil {
		t.Fatalf("Quotes error = %v, want nil", err)
	}
	if len(quotes) != 0 {
		t.Fatalf("quotes = %+v, want empty", quotes)
	}
	if name, err := fetcher.Name(context.Background(), "600000"); err != nil || name != "" {
		t.Fatalf("Name = %q, %v; want empty, nil", name, err)
	}
}

func TestParseQuoteTimeRejectsMalformed(t *testing.T) {
	if _, ok := parseQuoteTime("20261304150000"); ok {
		t.Fatal("month 13 should be rejected")
	}
	if _, ok := parseQuoteTime("2026112015000"); ok {
		t.Fatal("13 digits should be rejected")
	}
}
