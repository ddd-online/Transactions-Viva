package buildinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"
)

// TestVersionMatchesVivaToml keeps the constant and the packaging file equal:
// `viva package` names the release after viva.toml, and the About page names the
// running build after the constant. Two versions of one program would lie on one
// of the two screens.
func TestVersionMatchesVivaToml(t *testing.T) {
	path := filepath.Join("..", "..", "viva.toml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read viva.toml: %v", err)
	}
	match := regexp.MustCompile(`(?m)^\s*version\s*=\s*"([^"]+)"`).FindStringSubmatch(string(body))
	if match == nil {
		t.Fatal("viva.toml has no version")
	}
	if match[1] != Version {
		t.Fatalf("viva.toml version = %q, buildinfo.Version = %q", match[1], Version)
	}
}

// TestReadNamesTheRunningBuild checks the facts every page shows: a version, a
// platform a person can read, and the runtime the binary actually uses.
func TestReadNamesTheRunningBuild(t *testing.T) {
	facts := Read()
	if facts.Version != Version {
		t.Fatalf("version = %q", facts.Version)
	}
	if !strings.Contains(facts.Platform, " · ") {
		t.Fatalf("platform = %q, want system and architecture", facts.Platform)
	}
	if !strings.HasPrefix(facts.Runtime, "go") {
		t.Fatalf("runtime = %q", facts.Runtime)
	}
	if got := facts.VersionText(); got != "v"+Version {
		t.Fatalf("version text = %q", got)
	}
}

// TestFactsFromReadsTheCommit covers the build line: a commit is shortened to
// seven characters and dated, and a build without one says so instead of
// showing an empty tile.
func TestFactsFromReadsTheCommit(t *testing.T) {
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "d60c48f9a1b2c3"},
		{Key: "vcs.time", Value: "2026-11-19T08:30:00Z"},
	}}
	facts := factsFrom(info, "windows", "amd64", "go1.26.0")
	if facts.Build != "d60c48f · 2026-11-19" {
		t.Fatalf("build = %q", facts.Build)
	}
	if facts.Platform != "Windows · x86-64" {
		t.Fatalf("platform = %q", facts.Platform)
	}

	bare := factsFrom(&debug.BuildInfo{}, "linux", "arm64", "go1.26.0")
	if bare.BuildText() != "未记录" {
		t.Fatalf("bare build text = %q", bare.BuildText())
	}
	if tallied := factsFrom(nil, "darwin", "arm64", "go1.26.0"); tallied.Platform != "macOS · arm64" {
		t.Fatalf("platform with no build info = %q", tallied.Platform)
	}
}

// TestCopyTextIsOneBlockForABugReport pins the shape a user pastes into an issue.
func TestCopyTextIsOneBlockForABugReport(t *testing.T) {
	facts := Facts{Version: "0.1.0", Platform: "Windows · x86-64", Build: "abc1234 · 2026-11-19", Runtime: "go1.26.0"}
	want := "Transactions 0.1.0\n平台：Windows · x86-64\n运行时：go1.26.0\n构建：abc1234 · 2026-11-19\n"
	if got := facts.CopyText(); got != want {
		t.Fatalf("copy text = %q, want %q", got, want)
	}
}
