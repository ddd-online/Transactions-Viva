package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"transactions/internal/config"
)

// writeWorkspaceConfig writes a configuration file that names one workspace.
func writeWorkspaceConfig(t *testing.T, home, name, directory string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"workspaceDir": directory})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, name), data, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestHeadlessBuildReadsTheDevelopmentConfig guards the accident this file
// exists for: a headless run read the production config, opened the real
// workspace and wrote test notes into it.
func TestHeadlessBuildReadsTheDevelopmentConfig(t *testing.T) {
	if usesDevelopmentConfig(false, false) {
		t.Fatal("a release run must not read the development config")
	}
	UseHeadlessConfig()
	t.Cleanup(func() { headlessConfig = false })
	if !usesDevelopmentConfig(false, false) {
		t.Fatal("a headless build must read the development config")
	}
	home := t.TempDir()
	if got, want := config.Path(home, usesDevelopmentConfig(false, false)), config.Path(home, true); got != want {
		t.Fatalf("headless config path = %q, want %q", got, want)
	}
}

// TestHeadlessBuildDropsTheProductionWorkspace covers the development config
// being pointed at the workspace the production config names: the run keeps
// going, but with no workspace open.
func TestHeadlessBuildDropsTheProductionWorkspace(t *testing.T) {
	home := t.TempDir()
	production := filepath.Join(t.TempDir(), "real")
	other := filepath.Join(t.TempDir(), "scratch")
	writeWorkspaceConfig(t, home, config.FileName, production)

	writeWorkspaceConfig(t, home, config.DevFileName, production)
	cfg := config.Load(config.Path(home, true)).Config()
	if got := guardWorkspace(cfg, home); got.WorkspaceDir != "" {
		t.Fatalf("production workspace kept: %q", got.WorkspaceDir)
	}

	writeWorkspaceConfig(t, home, config.DevFileName, other)
	cfg = config.Load(config.Path(home, true)).Config()
	if got := guardWorkspace(cfg, home); got.WorkspaceDir != other {
		t.Fatalf("scratch workspace = %q, want %q", got.WorkspaceDir, other)
	}
}

func TestSameDirectoryFollowsThePaths(t *testing.T) {
	directory := t.TempDir()
	if !sameDirectory(directory, directory+string(filepath.Separator)) {
		t.Fatal("a directory is not its own trailing-separator form")
	}
	if sameDirectory(directory, t.TempDir()) {
		t.Fatal("two different directories compare equal")
	}
	for _, empty := range []string{"", "  ", "."} {
		if sameDirectory(empty, empty) {
			t.Fatalf("%q compared equal to itself", empty)
		}
	}
}
