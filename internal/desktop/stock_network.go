package desktop

import (
	"net/http"
	"time"

	"transactions/internal/config"
	"transactions/internal/proxy"
	"transactions/internal/stocklimit"
	storepkg "transactions/internal/store"
)

// stockHTTPClient builds the shared outbound client for quotes and the ladder.
func stockHTTPClient(setting config.ProxySetting) *http.Client {
	return proxy.Client(setting, 15*time.Second)
}

// stockLimitService builds the ladder cache service for one workspace.
func (m *Model) stockLimitService(workspace *storepkg.Workspace) *stocklimit.Service {
	source := &stocklimit.EastmoneySource{Client: stockHTTPClient(m.config.Proxy)}
	return stocklimit.NewService(workspace, source, m.stock.QuoteFetcher())
}
