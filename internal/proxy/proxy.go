// Package proxy holds the HTTP proxy rules: the three modes, manual address
// normalization, automatic detection (environment, then the platform's system
// proxy) and one outbound client for stocks and update checks.
package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"transactions/internal/config"
)

// Effective maps an unknown mode to auto, matching the configuration reader.
func Effective(setting config.ProxySetting) config.ProxyMode {
	switch setting.Mode {
	case config.ProxyOff, config.ProxyManual:
		return setting.Mode
	}
	return config.ProxyAuto
}

// Validate trims the mode / URL pair and normalizes a manual HTTP address. The
// error message is the sentence the settings page shows.
func Validate(mode config.ProxyMode, raw string) (config.ProxySetting, error) {
	switch mode {
	case config.ProxyOff, config.ProxyAuto:
		return config.ProxySetting{Mode: mode, URL: strings.TrimSpace(raw)}, nil
	case config.ProxyManual:
		normalized, err := NormalizeHTTP(raw)
		if err != nil {
			return config.ProxySetting{}, err
		}
		return config.ProxySetting{Mode: config.ProxyManual, URL: normalized}, nil
	}
	return config.ProxySetting{}, fmt.Errorf("代理模式无法识别：%s", mode)
}

// NormalizeHTTP accepts host:port and http://[user:pass@]host:port and returns
// the canonical http:// form. Only http:// is supported; https:// and socks://
// are rejected instead of being silently retried as HTTP.
func NormalizeHTTP(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("请填写代理地址（形如 http://127.0.0.1:7890）")
	}
	authority := trimmed
	if index := strings.Index(trimmed, "://"); index >= 0 {
		scheme := trimmed[:index]
		if !strings.EqualFold(scheme, "http") {
			return "", fmt.Errorf("仅支持 http:// 代理，暂不支持 %s://（如需请手动填写本机转发端口）", scheme)
		}
		authority = trimmed[index+3:]
	}
	for _, cut := range []string{"/", "?", "#"} {
		if index := strings.Index(authority, cut); index >= 0 {
			authority = authority[:index]
		}
	}
	authority = strings.TrimSpace(authority)
	if authority == "" {
		return "", fmt.Errorf("代理地址缺少主机名")
	}
	if strings.ContainsAny(authority, " \t\r\n") {
		return "", fmt.Errorf("代理地址含空格，请去掉空格再试")
	}
	colon := strings.LastIndex(authority, ":")
	if colon < 0 {
		return "", fmt.Errorf("代理地址缺少端口（形如 http://127.0.0.1:7890）")
	}
	host := authority[:colon]
	port, err := strconv.Atoi(strings.TrimPrefix(authority[colon+1:], "+"))
	if host == "" {
		return "", fmt.Errorf("代理地址缺少主机名")
	}
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("代理端口无效（应为 1-65535 的数字）")
	}
	return fmt.Sprintf("http://%s:%d", host, port), nil
}

// ManualURL is the normalized manual address, or "" for the other modes.
func ManualURL(setting config.ProxySetting) string {
	if Effective(setting) != config.ProxyManual {
		return ""
	}
	normalized, err := NormalizeHTTP(setting.URL)
	if err != nil {
		return ""
	}
	return normalized
}

// HasEnvProxy reports whether any of the environment variables that
// http.ProxyFromEnvironment reads is set.
func HasEnvProxy() bool {
	for _, key := range []string{
		"ALL_PROXY", "all_proxy", "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy",
	} {
		if strings.TrimSpace(env(key)) != "" {
			return true
		}
	}
	return false
}

// AutoProxy resolves the effective proxy for one request. Environment variables
// (with their NO_PROXY rules) win; the platform's system proxy is the fallback,
// and no proxy means a direct connection.
func AutoProxy(request *http.Request) (*url.URL, error) {
	if HasEnvProxy() {
		return http.ProxyFromEnvironment(request)
	}
	if address := systemProxy(); address != "" {
		normalized, err := NormalizeHTTP(address)
		if err == nil {
			return url.Parse(normalized)
		}
	}
	return nil, nil
}

// Resolve is the address the settings page shows: a proxy URL or "直连".
func Resolve(setting config.ProxySetting) string {
	switch Effective(setting) {
	case config.ProxyOff:
		return "直连"
	case config.ProxyManual:
		if address := ManualURL(setting); address != "" {
			return address
		}
		return "未配置（直连）"
	default:
		request, _ := http.NewRequest(http.MethodGet, "https://api.github.com/", nil)
		resolved, err := AutoProxy(request)
		if err != nil || resolved == nil {
			return "直连"
		}
		return resolved.String()
	}
}

// Client builds the shared outbound HTTP client. off disables even environment
// proxies; manual uses the normalized address; auto follows environment then
// the system proxy.
func Client(setting config.ProxySetting, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	switch Effective(setting) {
	case config.ProxyOff:
		transport.Proxy = nil
	case config.ProxyManual:
		if address := ManualURL(setting); address != "" {
			parsed, err := url.Parse(address)
			if err == nil {
				transport.Proxy = http.ProxyURL(parsed)
			}
		} else {
			transport.Proxy = nil
		}
	default:
		transport.Proxy = AutoProxy
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}

// TestResult is one proxy check, as the settings page reports it.
type TestResult struct {
	OK      bool
	Status  int
	Message string
}

// Test fetches target through the setting. A status below 500 counts as
// connected; an HTTP 4xx still proves the request reached a proxy.
func Test(setting config.ProxySetting, target string, timeout time.Duration) TestResult {
	if strings.TrimSpace(target) == "" {
		target = "https://api.github.com/zen"
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return TestResult{Message: "代理连接失败：" + err.Error()}
	}
	request.Header.Set("User-Agent", "Transactions-proxy-test")
	response, err := Client(setting, timeout).Do(request)
	if err != nil {
		return TestResult{Message: "代理连接失败：" + err.Error()}
	}
	defer response.Body.Close()
	elapsed := time.Since(started).Milliseconds()
	if response.StatusCode >= 500 {
		return TestResult{Status: response.StatusCode,
			Message: fmt.Sprintf("代理已连接，但目标返回 HTTP %d", response.StatusCode)}
	}
	return TestResult{OK: true, Status: response.StatusCode,
		Message: fmt.Sprintf("代理连通正常（%d ms）", elapsed)}
}

var env = os.Getenv
