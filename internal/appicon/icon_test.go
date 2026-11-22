package appicon

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestIconLoadsAndMatchesPackaging(t *testing.T) {
	icon, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if icon.Size < 128 || icon.Size > 1024 {
		t.Fatalf("size = %d", icon.Size)
	}
	if len(icon.RGBA) != icon.Size*icon.Size*4 {
		t.Fatalf("RGBA length = %d, want %d", len(icon.RGBA), icon.Size*icon.Size*4)
	}
	// The mark is opaque in the middle; a blank decode would make the taskbar
	// icon empty even though a PNG exists.
	center := (icon.Size/2*icon.Size + icon.Size/2) * 4
	if icon.RGBA[center+3] == 0 {
		t.Fatal("icon center is transparent")
	}

	toml, err := os.ReadFile(filepath.Join("..", "..", "viva.toml"))
	if err != nil {
		t.Fatalf("read viva.toml: %v", err)
	}
	match := regexp.MustCompile(`(?m)^\s*icon\s*=\s*"([^"]+)"`).FindStringSubmatch(string(toml))
	if match == nil {
		t.Fatal("viva.toml has no [app] icon")
	}
	if match[1] != "internal/appicon/icon.png" {
		t.Fatalf("viva.toml icon = %q, want internal/appicon/icon.png", match[1])
	}
	if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(match[1]))); err != nil {
		t.Fatalf("packaging icon: %v", err)
	}
}
