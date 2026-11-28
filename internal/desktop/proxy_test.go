package desktop

import (
	"net/http"
	"testing"

	"transactions/internal/config"
)

// TestStockClientsIgnoreTheUpdateProxy: quotes and the ladder connect directly —
// the configured proxy is the update check's alone — and a proxy named by the
// environment does not reach them either.
func TestStockClientsIgnoreTheUpdateProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9")
	client := stockHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatalf("the stock client keeps a proxy: %+v", client.Transport)
	}
	if client.Timeout != stockRequestTimeout {
		t.Fatalf("stock timeout = %v, want %v", client.Timeout, stockRequestTimeout)
	}
}

func TestProxyModeAndManualAddress(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "设置")
	click(t, driver, "关于软件")
	click(t, driver, "手动")
	click(t, driver, "代理地址")
	driver.Type("127.0.0.1:7890")
	click(t, driver, "保存")
	if model.config.Proxy.Mode != config.ProxyManual || model.config.Proxy.URL != "http://127.0.0.1:7890" {
		t.Fatalf("manual proxy = %+v", model.config.Proxy)
	}
	if !shows(driver, "http://127.0.0.1:7890") {
		t.Fatal("saved manual address is not shown")
	}
	click(t, driver, "不使用")
	if model.config.Proxy.Mode != config.ProxyOff {
		t.Fatalf("off proxy = %+v", model.config.Proxy)
	}
	if model.config.Proxy.URL != "http://127.0.0.1:7890" {
		t.Fatalf("off dropped the manual address: %+v", model.config.Proxy)
	}
}
