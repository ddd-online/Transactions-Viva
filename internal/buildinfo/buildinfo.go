// Package buildinfo is what this copy of the program is: the version from
// viva.toml, the platform it was built for, the commit it came from and the Go
// runtime under it. 关于软件 shows these facts, and 复制版本信息 puts them on the
// clipboard for a bug report.
package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Version is the application version. It is the same string as viva.toml's
// version — viva package reads that file, this binary reads this constant, and a
// test keeps the two equal.
const Version = "0.6.0"

// Facts is one built copy of the program. Every field is a finished string: the
// About page repaints on every pointer move and must not build them again.
type Facts struct {
	// Version is the version without the leading v ("0.1.0").
	Version string
	// Platform names the system and the architecture as a person reads them
	// ("Windows · x86-64").
	Platform string
	// Build is the commit and its date ("d60c48f · 2026-11-19"), or empty when
	// the build recorded none.
	Build string
	// Runtime is the Go runtime that built it ("go1.26.0").
	Runtime string
}

// Read returns the facts of the running binary. It reads the build info once:
// a settings page repaints on every pointer move, so the strings are made here
// rather than on a paint.
var Read = sync.OnceValue(func() Facts {
	info, _ := debug.ReadBuildInfo()
	return factsFrom(info, runtime.GOOS, runtime.GOARCH, runtime.Version())
})

// factsFrom builds the facts from the given build info and platform names. The
// pieces arrive as arguments so a test can describe a build it does not have.
func factsFrom(info *debug.BuildInfo, goos, goarch, goVersion string) Facts {
	facts := Facts{
		Version:  Version,
		Platform: platformName(goos, goarch),
		Runtime:  goVersion,
	}
	if info == nil {
		return facts
	}
	var revision, when string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			when = setting.Value
		}
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if revision == "" {
		return facts
	}
	facts.Build = revision
	if stamp, err := time.Parse(time.RFC3339, when); err == nil {
		facts.Build += " · " + stamp.Format("2006-01-02")
	}
	return facts
}

// platformName names the operating system and architecture as a person reads
// them, keeping the runtime's own name for anything unrecognised.
func platformName(goos, goarch string) string {
	system := goos
	switch goos {
	case "darwin":
		system = "macOS"
	case "windows":
		system = "Windows"
	case "linux":
		system = "Linux"
	}
	architecture := goarch
	switch goarch {
	case "amd64":
		architecture = "x86-64"
	case "386":
		architecture = "x86"
	case "arm64":
		architecture = "arm64"
	}
	return system + " · " + architecture
}

// VersionText is the version as the page shows it.
func (f Facts) VersionText() string { return "v" + f.Version }

// BuildText is the build line, with a word of its own when none was recorded.
func (f Facts) BuildText() string {
	if f.Build == "" {
		return "未记录"
	}
	return f.Build
}

// CopyText is the same facts as one block for a bug report.
func (f Facts) CopyText() string {
	var report strings.Builder
	report.WriteString("Transactions " + f.Version + "\n")
	report.WriteString("平台：" + f.Platform + "\n")
	report.WriteString("运行时：" + f.Runtime + "\n")
	report.WriteString("构建：" + f.BuildText() + "\n")
	return report.String()
}
