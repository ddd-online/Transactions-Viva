package buildinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestReleaseNotesMatchVersion mirrors the old project's static guard: a release
// body may hold only the current version, or the updater shows an old changelog.
func TestReleaseNotesMatchVersion(t *testing.T) {
	path := filepath.Join("..", "..", "build", "release_notes.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read release notes: %v", err)
	}
	text := string(data)

	heading := regexp.MustCompile(`(?m)^##\s*\[([^\]]+)\]\s*-\s*(\d{4}-\d{2}-\d{2})`)
	match := heading.FindStringSubmatch(text)
	if match == nil {
		t.Fatal("release_notes.md must start with ## [version] - YYYY-MM-DD")
	}
	if match[1] != Version {
		t.Fatalf("release notes version = %q, buildinfo.Version = %q", match[1], Version)
	}
	if sections := regexp.MustCompile(`(?m)^##\s*\[`).FindAllString(text, -1); len(sections) != 1 {
		t.Fatalf("release_notes.md must contain exactly one version section, got %d", len(sections))
	}

	for _, script := range []string{"build.ps1", "clean.ps1", "release.ps1"} {
		path := filepath.Join("..", "..", "build", script)
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("release script missing or empty: %s (%v)", path, err)
		}
	}
	release, err := os.ReadFile(filepath.Join("..", "..", "build", "release.ps1"))
	if err != nil {
		t.Fatalf("read release.ps1: %v", err)
	}
	if !strings.Contains(string(release), "'--notes-file'") {
		t.Fatal("release.ps1 must upload the extracted single-version notes")
	}
	if !strings.Contains(string(release), "clean.ps1") {
		t.Fatal("release.ps1 must clean local artifacts after publishing (-SkipClean may skip)")
	}
}
