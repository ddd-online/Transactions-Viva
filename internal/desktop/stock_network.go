package desktop

import (
	"net/http"
	"time"

	"transactions/internal/stocklimit"
	storepkg "transactions/internal/store"
)

// stockRequestTimeout is how long a quote or ladder request may take.
const stockRequestTimeout = 15 * time.Second

// stockHTTPClient builds the outbound client for quotes and the ladder. They
// connect directly, always: the proxy in the configuration belongs to the update
// check alone (关于软件 › 更新代理), so a proxy that cannot reach GitHub never takes
// the quotes down with it, and changing it never rebuilds these clients. The
// transport starts from the default one — its timeouts and connection limits — and
// drops the proxy, because http.DefaultTransport would otherwise read the proxy the
// environment names.
func stockHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport, Timeout: stockRequestTimeout}
}

// stockLimitService builds the ladder cache service for one workspace.
func (m *Model) stockLimitService(workspace *storepkg.Workspace) *stocklimit.Service {
	source := &stocklimit.EastmoneySource{Client: stockHTTPClient()}
	return stocklimit.NewService(workspace, source, m.stock.QuoteFetcher())
}
