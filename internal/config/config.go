// Package config defines and persists the application configuration: the
// window frame, the last workspace, appearance, feature switches and backup
// policy. It is the Go port of the project's shared/types/config.ts.
//
// The file is ~/.transactions.json in production and ~/.transactions.dev.json
// while `viva dev` runs (its reload host sets VIVA_RELOAD_ADDRESS). Reads are
// lenient — a missing field, a wrongly typed value or a corrupt file falls back
// to the default for that field — and writes keep unknown top-level keys, so an
// older or newer build never erases what the other wrote.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// The configuration file's names. Development uses its own file so a reloading
// build never fights a release build over the same settings.
const (
	FileName    = ".transactions.json"
	DevFileName = ".transactions.dev.json"
)

// RecentWorkspaceLimit is how many workspaces the recent list keeps.
const RecentWorkspaceLimit = 5

// Path is the configuration file in home: the dev name when dev, else the
// production one.
func Path(home string, dev bool) string {
	name := FileName
	if dev {
		name = DevFileName
	}
	return filepath.Join(home, name)
}

// ProxyMode is how outbound requests reach the network. There is no SOCKS or
// PAC support, matching the application.
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

// FeatureKey names one of the optional features. Settings always exists.
type FeatureKey string

const (
	FeatureAccounting FeatureKey = "accounting"
	FeatureStock      FeatureKey = "stock"
	FeatureKeyEvent   FeatureKey = "keyEvent"
	FeatureDiary      FeatureKey = "diary"
	FeatureTodo       FeatureKey = "todo"
	FeatureNotes      FeatureKey = "notes"
)

// FeatureKeys are the features in the settings page's row order.
var FeatureKeys = [...]FeatureKey{
	FeatureAccounting, FeatureStock, FeatureKeyEvent, FeatureDiary, FeatureTodo, FeatureNotes,
}

// ErrFeatureKey is returned when a feature key is not one of FeatureKeys.
var ErrFeatureKey = errors.New("无法识别的功能开关")

// Valid reports whether the key is a known feature.
func (k FeatureKey) Valid() bool {
	for _, known := range FeatureKeys {
		if k == known {
			return true
		}
	}
	return false
}

// Enabled reports whether the feature is on. A key that is not a known feature
// is off: callers only pass FeatureKeys.
func (f FeatureFlags) Enabled(key FeatureKey) bool {
	switch key {
	case FeatureAccounting:
		return f.Accounting
	case FeatureStock:
		return f.Stock
	case FeatureKeyEvent:
		return f.KeyEvent
	case FeatureDiary:
		return f.Diary
	case FeatureTodo:
		return f.Todo
	case FeatureNotes:
		return f.Notes
	}
	return false
}

// set writes one feature and reports whether the key was a known one.
func (f *FeatureFlags) set(key FeatureKey, enabled bool) bool {
	switch key {
	case FeatureAccounting:
		f.Accounting = enabled
	case FeatureStock:
		f.Stock = enabled
	case FeatureKeyEvent:
		f.KeyEvent = enabled
	case FeatureDiary:
		f.Diary = enabled
	case FeatureTodo:
		f.Todo = enabled
	case FeatureNotes:
		f.Notes = enabled
	default:
		return false
	}
	return true
}

// ProxySetting is the outbound proxy configuration.
type ProxySetting struct {
	Mode ProxyMode `json:"mode"`
	URL  string    `json:"url"`
}

// FeatureFlags are the six optional features. Settings always exists.
type FeatureFlags struct {
	Accounting bool `json:"accounting"`
	Stock      bool `json:"stock"`
	KeyEvent   bool `json:"keyEvent"`
	Diary      bool `json:"diary"`
	Todo       bool `json:"todo"`
	Notes      bool `json:"notes"`
}

// AppConfig is everything the configuration file holds. Pointers keep "not
// set" distinct from a real 0.
type AppConfig struct {
	Width                 int                     `json:"width"`
	Height                int                     `json:"height"`
	X                     *int                    `json:"x"`
	Y                     *int                    `json:"y"`
	WorkspaceDir          string                  `json:"workspaceDir"`
	CloseBehavior         CloseBehavior           `json:"closeBehavior"`
	Appearance            Appearance              `json:"appearance"`
	Proxy                 ProxySetting            `json:"proxy"`
	Features              FeatureFlags            `json:"features"`
	LedgerFeatures        map[string]FeatureFlags `json:"ledgerFeatures"`
	KeyEventLinkedOpen    bool                    `json:"keyEventLinkedOpen"`
	ChartSummaryCollapsed bool                    `json:"chartSummaryCollapsed"`
	DataVersion           int                     `json:"dataVersion"`
	BackupDir             string                  `json:"backupDir"` // 空串 = 「文档」下的默认目录
	AutoBackup            bool                    `json:"autoBackup"`
	RecentWorkspaces      []string                `json:"recentWorkspaces"` // 最新的在前，最多 RecentWorkspaceLimit 条
}

// Default returns the configuration a fresh install starts from. The values
// match the project's defaultConfig().
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
		RecentWorkspaces:      []string{},
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
		Notes:      true,
	}
}

// EffectiveFeatures is the flags in force for a ledger: its own entry when it
// has one, else the global defaults. An empty ledger id means the defaults.
func (c AppConfig) EffectiveFeatures(ledgerID string) FeatureFlags {
	if ledgerID != "" {
		if flags, ok := c.LedgerFeatures[ledgerID]; ok {
			return flags
		}
	}
	return c.Features
}

// SetFeature turns a feature on or off for a ledger, or for the defaults when
// ledgerID is empty, and returns the flags it wrote. The first change to a
// ledger copies the current defaults into its entry, as the service did, so
// "this ledger differs in one switch" stays predictable; ledgers never changed
// still follow the defaults.
func (c *AppConfig) SetFeature(ledgerID string, key FeatureKey, enabled bool) (FeatureFlags, error) {
	if !key.Valid() {
		return FeatureFlags{}, ErrFeatureKey
	}
	if ledgerID == "" {
		c.Features.set(key, enabled)
		return c.Features, nil
	}
	flags, ok := c.LedgerFeatures[ledgerID]
	if !ok {
		flags = c.Features
	}
	flags.set(key, enabled)
	if c.LedgerFeatures == nil {
		c.LedgerFeatures = map[string]FeatureFlags{}
	}
	c.LedgerFeatures[ledgerID] = flags
	return flags, nil
}

// Store owns one configuration file. Load it once at startup, read Config, and
// pass the new value to Save after a change.
type Store struct {
	path   string
	config AppConfig
	raw    map[string]json.RawMessage // the file's top-level object, unknown keys included
}

// Load reads path, falling back to Default for a missing, corrupt or wrongly
// typed file. A read failure is not an error: the configuration is a cache of
// user choices, and losing it must not keep the application from starting.
func Load(path string) *Store {
	store := &Store{path: path, raw: map[string]json.RawMessage{}}
	data, err := os.ReadFile(path)
	if err == nil {
		var raw map[string]json.RawMessage
		if json.Unmarshal(data, &raw) == nil && raw != nil {
			store.raw = raw
		}
	}
	store.config = normalize(store.raw)
	return store
}

// Path is the file the store reads and writes; empty means memory only.
func (s *Store) Path() string { return s.path }

// Config is the loaded configuration.
func (s *Store) Config() AppConfig { return s.config }

// Save writes cfg and remembers it. The write is atomic: a temporary file in
// the same directory replaces the one, so a crash cannot leave half a
// configuration behind. Unknown top-level keys in the file survive.
func (s *Store) Save(cfg AppConfig) error {
	if s.path == "" {
		s.config = cfg
		return nil
	}
	data, err := s.encode(cfg)
	if err != nil {
		return err
	}
	if err := writeFile(s.path, data); err != nil {
		return err
	}
	s.config = cfg
	return nil
}

// encode merges the known fields over the file's original object, keeping keys
// this build does not know.
func (s *Store) encode(cfg AppConfig) ([]byte, error) {
	raw := make(map[string]json.RawMessage, len(s.raw)+1)
	for key, value := range s.raw {
		raw[key] = value
	}
	known, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(known, &fields); err != nil {
		return nil, err
	}
	for key, value := range fields {
		raw[key] = value
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// writeFile writes data to path through path.tmp, falling back to a direct
// write where a rename cannot replace the target.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		os.Remove(temporary)
		return os.WriteFile(path, data, 0o644)
	}
	return nil
}

// normalize builds a complete configuration from the file's raw object. Every
// field falls back to its default independently, so one bad value does not
// reset the rest.
func normalize(raw map[string]json.RawMessage) AppConfig {
	cfg := Default()
	if len(raw) == 0 {
		return cfg
	}
	if value, ok := numberValue(raw["width"]); ok && value > 0 {
		cfg.Width = int(value)
	}
	if value, ok := numberValue(raw["height"]); ok && value > 0 {
		cfg.Height = int(value)
	}
	if value, ok := numberValue(raw["x"]); ok {
		x := int(value)
		cfg.X = &x
	}
	if value, ok := numberValue(raw["y"]); ok {
		y := int(value)
		cfg.Y = &y
	}
	if value, ok := stringValue(raw["workspaceDir"]); ok {
		cfg.WorkspaceDir = value
	}
	if value, ok := stringValue(raw["closeBehavior"]); ok {
		cfg.CloseBehavior = normalizeCloseBehavior(value)
	}
	if value, ok := stringValue(raw["appearance"]); ok {
		cfg.Appearance = normalizeAppearance(value)
	}
	if proxy, ok := objectValue(raw["proxy"]); ok {
		cfg.Proxy = normalizeProxy(proxy)
	}
	if features, ok := objectValue(raw["features"]); ok {
		cfg.Features = normalizeFlags(features, cfg.Features)
	}
	if ledgers, ok := objectValue(raw["ledgerFeatures"]); ok {
		cfg.LedgerFeatures = normalizeLedgerFeatures(ledgers)
	}
	if value, ok := boolValue(raw["keyEventLinkedOpen"]); ok {
		cfg.KeyEventLinkedOpen = value
	}
	if value, ok := boolValue(raw["chartSummaryCollapsed"]); ok {
		cfg.ChartSummaryCollapsed = value
	}
	if value, ok := numberValue(raw["dataVersion"]); ok {
		cfg.DataVersion = int(value)
	}
	if value, ok := stringValue(raw["backupDir"]); ok {
		cfg.BackupDir = value
	}
	if value, ok := boolValue(raw["autoBackup"]); ok {
		cfg.AutoBackup = value
	}
	if list, ok := listValue(raw["recentWorkspaces"]); ok {
		cfg.RecentWorkspaces = normalizeRecent(list, cfg.RecentWorkspaces)
	}
	return cfg
}

func normalizeProxy(raw map[string]json.RawMessage) ProxySetting {
	setting := ProxySetting{Mode: ProxyAuto}
	if value, ok := stringValue(raw["mode"]); ok {
		switch ProxyMode(value) {
		case ProxyOff, ProxyManual:
			setting.Mode = ProxyMode(value)
		}
	}
	if value, ok := stringValue(raw["url"]); ok {
		setting.URL = value
	}
	return setting
}

func normalizeFlags(raw map[string]json.RawMessage, fallback FeatureFlags) FeatureFlags {
	flags := fallback
	for _, key := range FeatureKeys {
		field := string(key)
		if value, ok := boolValue(raw[field]); ok {
			flags.set(key, value)
		}
	}
	return flags
}

func normalizeLedgerFeatures(raw map[string]json.RawMessage) map[string]FeatureFlags {
	ledgers := make(map[string]FeatureFlags, len(raw))
	for ledgerID, value := range raw {
		flags, ok := objectValue(value)
		if !ok {
			continue
		}
		ledgers[ledgerID] = normalizeFlags(flags, DefaultFeatures())
	}
	return ledgers
}

// normalizeRecent keeps only non-empty, distinct strings, newest first, up to
// RecentWorkspaceLimit; a bad item is dropped rather than resetting the list.
func normalizeRecent(raw []json.RawMessage, fallback []string) []string {
	recent := make([]string, 0, min(len(raw), RecentWorkspaceLimit))
	for _, item := range raw {
		value, ok := stringValue(item)
		if !ok {
			continue
		}
		path := strings.TrimSpace(value)
		if path == "" {
			continue
		}
		duplicate := false
		for _, kept := range recent {
			if kept == path {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		recent = append(recent, path)
		if len(recent) == RecentWorkspaceLimit {
			break
		}
	}
	if len(recent) == 0 && len(raw) == 0 {
		return fallback
	}
	return recent
}

func normalizeCloseBehavior(value string) CloseBehavior {
	switch CloseBehavior(value) {
	case CloseQuit, CloseTray, CloseAsk:
		return CloseBehavior(value)
	}
	return CloseAsk
}

// normalizeAppearance treats an unknown appearance as following the system;
// the application did the same when it applied the mode.
func normalizeAppearance(value string) Appearance {
	switch Appearance(value) {
	case AppearanceLight, AppearanceDark, AppearanceSystem:
		return Appearance(value)
	}
	return AppearanceSystem
}

// numberValue reads a finite JSON number.
func numberValue(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, false
	}
	return value, true
}

func boolValue(raw json.RawMessage) (bool, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return false, false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, false
	}
	return value, true
}

func stringValue(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// objectValue reads a JSON object; null, an array or a scalar is not one.
func objectValue(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, false
	}
	return value, true
}

// listValue reads a JSON array as its items.
func listValue(raw json.RawMessage) ([]json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var value []json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, false
	}
	return value, true
}
