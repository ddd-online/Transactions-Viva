package desktop

import (
	"time"

	"transactions/internal/config"
	"transactions/internal/pages/settings"
	"transactions/internal/proxy"
)

func (m *Model) proxyResolved() string {
	return proxy.Resolve(m.config.Proxy)
}

// saveProxy validates the update proxy, stores it, and asks for a paint so the
// update row shows the new setting. Nothing else is rebuilt: quotes and the ladder
// connect directly, and the update builds its client when it runs.
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
