package desktop

import (
	"time"

	"transactions/internal/config"
	"transactions/internal/pages/settings"
	"transactions/internal/proxy"
	"transactions/internal/stockquote"
)

func (m *Model) proxyResolved() string {
	return proxy.Resolve(m.config.Proxy)
}

// saveProxy validates the pair, stores it, then rebuilds the stock quote and
// ladder clients so a proxy change takes effect without a restart.
func (m *Model) saveProxy(setting config.ProxySetting) (config.ProxySetting, error) {
	validated, err := proxy.Validate(setting.Mode, setting.URL)
	if err != nil {
		return config.ProxySetting{}, err
	}
	previous := m.config.Proxy
	m.config.Proxy = validated
	if err := m.persist(); err != nil {
		m.config.Proxy = previous
		return config.ProxySetting{}, err
	}
	m.refreshStockClients(validated)
	m.invalidate()
	return validated, nil
}

func (m *Model) testProxy(setting config.ProxySetting) (settings.ProxyTestResult, error) {
	validated, err := proxy.Validate(setting.Mode, setting.URL)
	if err != nil {
		return settings.ProxyTestResult{}, err
	}
	result := proxy.Test(validated, "", 8*time.Second)
	return settings.ProxyTestResult{OK: result.OK, Message: result.Message}, nil
}

func (m *Model) refreshStockClients(setting config.ProxySetting) {
	fetcher := &stockquote.TencentFetcher{Client: proxy.Client(setting, 15*time.Second)}
	m.stock.SetQuotes(fetcher)
	if m.workspace != nil {
		m.stock.SetLimitService(m.stockLimitService(m.workspace))
	}
}
