package proxy

import (
	"testing"

	"transactions/internal/config"
)

func TestNormalizeHTTP(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:7890":                "http://127.0.0.1:7890",
		"http://127.0.0.1:7890/":        "http://127.0.0.1:7890",
		" http://user:pass@host:8080/x": "http://user:pass@host:8080",
		"http://[::1]:1080":             "http://[::1]:1080",
	}
	for input, want := range cases {
		got, err := NormalizeHTTP(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeHTTP(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"", "127.0.0.1", "socks5://127.0.0.1:1080", "http://host:99999", "http://host:abc"} {
		if _, err := NormalizeHTTP(input); err == nil {
			t.Fatalf("NormalizeHTTP(%q) accepted", input)
		}
	}
}

func TestValidatePreservesURLForOffAndAuto(t *testing.T) {
	for _, mode := range []config.ProxyMode{config.ProxyOff, config.ProxyAuto} {
		setting, err := Validate(mode, " http://127.0.0.1:7890 ")
		if err != nil || setting.Mode != mode || setting.URL != "http://127.0.0.1:7890" {
			t.Fatalf("Validate(%s) = %+v, %v", mode, setting, err)
		}
	}
	setting, err := Validate(config.ProxyManual, "127.0.0.1:7890")
	if err != nil || setting.URL != "http://127.0.0.1:7890" {
		t.Fatalf("manual = %+v, %v", setting, err)
	}
}

func TestResolveAndClientForManual(t *testing.T) {
	setting := config.ProxySetting{Mode: config.ProxyManual, URL: "127.0.0.1:7890"}
	if got := Resolve(setting); got != "http://127.0.0.1:7890" {
		t.Fatalf("Resolve = %q", got)
	}
	if got := Resolve(config.ProxySetting{Mode: config.ProxyOff}); got != "直连" {
		t.Fatalf("off resolve = %q", got)
	}
	if client := Client(setting, 0); client == nil {
		t.Fatal("manual client is nil")
	}
}
