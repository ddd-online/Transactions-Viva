//go:build windows

package proxy

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// systemProxy reads the static HTTP proxy from Windows Internet Options. PAC
// scripts are not resolved; a machine that only has AutoConfigURL is direct.
func systemProxy() string {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return ""
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return ""
	}
	server = strings.TrimSpace(server)
	if server == "" {
		return ""
	}
	if !strings.Contains(server, "=") {
		return server
	}
	fallback := ""
	for _, part := range strings.Split(server, ";") {
		equals := strings.Index(part, "=")
		if equals < 0 {
			continue
		}
		scheme := strings.TrimSpace(part[:equals])
		value := strings.TrimSpace(part[equals+1:])
		switch {
		case strings.EqualFold(scheme, "http"):
			return value
		case strings.EqualFold(scheme, "https") && fallback == "":
			fallback = value
		}
	}
	return fallback
}
