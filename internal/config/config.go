// Package config defines the application configuration: the window frame, the
// last workspace, appearance, feature switches and backup policy. It is the Go
// port of the old project's shared/types/config.ts.
//
// Persistence is not here yet. The on-disk form is ~/.tr.json (production) or
// ~/.tr.dev.json (development), and it preserves unknown keys so an older or
// newer build never erases what the other wrote. The JSON store lands together
// with the settings page.
package config

// ProxyMode is how outbound requests reach the network. There is no SOCKS or
// PAC support, matching the old application.
type ProxyMode string

const (
	ProxyAuto   ProxyMode = "auto"
	ProxyOff    ProxyMode = "off"
	ProxyManual ProxyMode = "manual"
)

// CloseBehavior says what the window's close button does.
type CloseBehavior string

const (
	CloseQuit CloseBehavior = "quit"
	CloseTray CloseBehavior = "tray"
	CloseAsk  CloseBehavior = "ask"
)

// Appearance is the chosen look: system follows the desktop, otherwise pinned.
type Appearance string

const (
	AppearanceSystem Appearance = "system"
	AppearanceLight  Appearance = "light"
	AppearanceDark   Appearance = "dark"
)

// ProxySetting is the outbound proxy configuration.
type ProxySetting struct {
	Mode ProxyMode
	URL  string
}

// FeatureFlags are the five optional features. Settings always exists.
type FeatureFlags struct {
	Accounting bool
	Stock      bool
	KeyEvent   bool
	Diary      bool
	Todo       bool
}

// AppConfig is everything ~/.tr.json holds. Pointers keep "not set" distinct
// from a real 0, as the old TypeScript x/y fields did.
type AppConfig struct {
	Width                 int
	Height                int
	X                     *int
	Y                     *int
	WorkspaceDir          string
	CloseBehavior         CloseBehavior
	Appearance            Appearance
	Proxy                 ProxySetting
	Features              FeatureFlags
	LedgerFeatures        map[string]FeatureFlags
	KeyEventLinkedOpen    bool
	ChartSummaryCollapsed bool
	DataVersion           int
	BackupDir             string // 空串 = 「文档」下的默认目录
	AutoBackup            bool
	RecentWorkspaces      []string // 最新的在前，最多 5 条
}

// Default returns the configuration a fresh install starts from. The values
// match the old project's defaultConfig().
func Default() AppConfig {
	return AppConfig{
		Width:                 1500,
		Height:                1000,
		CloseBehavior:         CloseAsk,
		Appearance:            AppearanceSystem,
		Proxy:                 ProxySetting{Mode: ProxyAuto},
		Features:              DefaultFeatures(),
		LedgerFeatures:        map[string]FeatureFlags{},
		KeyEventLinkedOpen:    true,
		ChartSummaryCollapsed: false,
		DataVersion:           2,
		AutoBackup:            true,
	}
}

// DefaultFeatures turns every optional feature on.
func DefaultFeatures() FeatureFlags {
	return FeatureFlags{
		Accounting: true,
		Stock:      true,
		KeyEvent:   true,
		Diary:      true,
		Todo:       true,
	}
}
