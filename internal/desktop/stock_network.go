package desktop

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"transactions/internal/config"
	"transactions/internal/stocklimit"
	storepkg "transactions/internal/store"
)

// stockHTTPClient builds the shared outbound client for quotes and the ladder.
// The settings UI still marks the proxy row as unsupported, but a configured
// proxy (off / auto / manual) is honoured for the stock network paths.
func stockHTTPClient(proxy config.ProxySetting) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	switch proxy.Mode {
	case config.ProxyOff:
		transport.Proxy = nil
	case config.ProxyManual:
		if parsed, err := url.Parse(strings.TrimSpace(proxy.URL)); err == nil &&
			(parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			transport.Proxy = http.ProxyURL(parsed)
		} else {
			transport.Proxy = http.ProxyFromEnvironment
		}
	default:
		transport.Proxy = http.ProxyFromEnvironment
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second}
}

// stockLimitService builds the ladder cache service for one workspace.
func (m *Model) stockLimitService(workspace *storepkg.Workspace) *stocklimit.Service {
	source := &stocklimit.EastmoneySource{Client: stockHTTPClient(m.config.Proxy)}
	return stocklimit.NewService(workspace, source, m.stock.QuoteFetcher())
}
