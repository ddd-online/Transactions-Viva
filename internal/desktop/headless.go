package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"transactions/internal/config"
)

// headlessConfig marks a build whose job is to drive the UI without a window
// (`go build -tags vivahead`). Such a build is a test instrument, not the
// shipped application: it works on the development configuration and on a
// throwaway workspace, and it must never read the production configuration or
// open the workspace named there. The main package's vivahead file sets it, so a
// release build cannot contain it.
var headlessConfig bool

// UseHeadlessConfig switches this process to the test contract above.
func UseHeadlessConfig() { headlessConfig = true }

// usesDevelopmentConfig reports whether a run reads the development
// configuration. A headless build always does: reading the production one is how
// a test run once migrated a person's real workspace and wrote test notes into
// it.
func usesDevelopmentConfig(dev, reloadHost bool) bool {
	return dev || reloadHost || headlessConfig
}

// guardWorkspace returns the configuration a test build may open. The production
// configuration names a person's real workspace; a development configuration
// pointed at the same directory would let a headless run migrate that database
// and write test data into it. The run continues without a workspace, so the
// picker still appears and the skipped directory is named on stderr.
func guardWorkspace(cfg config.AppConfig, home string) config.AppConfig {
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		return cfg
	}
	production := config.Load(config.Path(home, false)).Config()
	if !sameDirectory(cfg.WorkspaceDir, production.WorkspaceDir) {
		return cfg
	}
	fmt.Fprintf(os.Stderr, "headless: 配置的工作空间 %s 是生产工作空间，本次不打开\n", cfg.WorkspaceDir)
	cfg.WorkspaceDir = ""
	return cfg
}

// sameDirectory compares two configured directories the way the platform does,
// so "C:\ws" and "c:\ws\" are the same workspace.
func sameDirectory(a, b string) bool {
	a, b = filepath.Clean(strings.TrimSpace(a)), filepath.Clean(strings.TrimSpace(b))
	if a == "" || b == "" || a == "." || b == "." {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
