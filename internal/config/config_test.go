package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPathUsesTransactionsNames(t *testing.T) {
	home := filepath.Join("home", "user")
	if got := Path(home, false); got != filepath.Join(home, ".transactions.json") {
		t.Fatalf("production path = %q", got)
	}
	if got := Path(home, true); got != filepath.Join(home, ".transactions.dev.json") {
		t.Fatalf("development path = %q", got)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	store := Load(filepath.Join(t.TempDir(), "absent.json"))
	if got, want := store.Config(), Default(); !sameConfig(got, want) {
		t.Fatalf("config = %+v, want %+v", got, want)
	}
}

func TestLoadNormalizesEveryFieldIndependently(t *testing.T) {
	path := writeConfig(t, `{
  "width": 1200,
  "height": "tall",
  "x": 40,
  "y": null,
  "workspaceDir": "/books",
  "closeBehavior": "nonsense",
  "appearance": "sepia",
  "proxy": {"mode": "manual", "url": "http://127.0.0.1:7890", "extra": true},
  "features": {"stock": false, "diary": "yes"},
  "ledgerFeatures": {"ledger-1": {"accounting": false}, "bad": 3},
  "keyEventLinkedOpen": false,
  "chartSummaryCollapsed": true,
  "dataVersion": 7,
  "backupDir": "/backups",
  "autoBackup": false,
  "recentWorkspaces": [" /a ", "/a", "", "/b", "/c", "/d", "/e", "/f"],
  "futureKey": {"nested": [1, 2]}
}`)
	store := Load(path)
	cfg := store.Config()

	if cfg.Width != 1200 || cfg.Height != 1000 {
		t.Fatalf("size = %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.X == nil || *cfg.X != 40 || cfg.Y != nil {
		t.Fatalf("position = %v,%v", cfg.X, cfg.Y)
	}
	if cfg.WorkspaceDir != "/books" || cfg.CloseBehavior != CloseAsk {
		t.Fatalf("workspace/close = %q/%q", cfg.WorkspaceDir, cfg.CloseBehavior)
	}
	if cfg.Appearance != AppearanceSystem {
		t.Fatalf("appearance = %q", cfg.Appearance)
	}
	if cfg.Proxy.Mode != ProxyManual || cfg.Proxy.URL != "http://127.0.0.1:7890" {
		t.Fatalf("proxy = %+v", cfg.Proxy)
	}
	if cfg.Features.Stock || !cfg.Features.Accounting || !cfg.Features.Diary {
		t.Fatalf("features = %+v", cfg.Features)
	}
	ledger, ok := cfg.LedgerFeatures["ledger-1"]
	if !ok || ledger.Accounting || !ledger.Stock || !ledger.Diary {
		t.Fatalf("ledger features = %+v", cfg.LedgerFeatures)
	}
	if _, ok := cfg.LedgerFeatures["bad"]; ok {
		t.Fatal("an invalid ledger feature entry was kept")
	}
	if cfg.KeyEventLinkedOpen || !cfg.ChartSummaryCollapsed || cfg.DataVersion != 7 {
		t.Fatalf("flags = %+v", cfg)
	}
	if cfg.BackupDir != "/backups" || cfg.AutoBackup {
		t.Fatalf("backup = %q/%v", cfg.BackupDir, cfg.AutoBackup)
	}
	wantRecent := []string{"/a", "/b", "/c", "/d", "/e"}
	if len(cfg.RecentWorkspaces) != len(wantRecent) {
		t.Fatalf("recent = %v", cfg.RecentWorkspaces)
	}
	for index, want := range wantRecent {
		if cfg.RecentWorkspaces[index] != want {
			t.Fatalf("recent = %v", cfg.RecentWorkspaces)
		}
	}

	// Saving keeps the unknown top-level key and writes pretty JSON.
	cfg.Appearance = AppearanceDark
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || data[0] != '{' || data[len(data)-1] != '\n' {
		t.Fatalf("file is not a pretty JSON object with a newline: %q", data)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["futureKey"]; !ok {
		t.Fatal("the unknown top-level key was dropped")
	}
	if raw["appearance"] != "dark" {
		t.Fatalf("appearance was not written: %v", raw["appearance"])
	}
	proxy, _ := raw["proxy"].(map[string]any)
	if _, ok := proxy["extra"]; ok {
		t.Fatal("an unknown nested proxy key survived; the old format rewrites the object")
	}

	reloaded := Load(path).Config()
	if reloaded.Appearance != AppearanceDark || reloaded.Width != 1200 ||
		len(reloaded.RecentWorkspaces) != 5 {
		t.Fatalf("reloaded = %+v", reloaded)
	}
}

func TestLoadCorruptFileReturnsDefaults(t *testing.T) {
	path := writeConfig(t, `{not json`)
	if got := Load(path).Config(); !sameConfig(got, Default()) {
		t.Fatalf("config = %+v, want defaults", got)
	}
}

func TestSetFeatureUsesTheDefaultsOrACopiedLedgerEntry(t *testing.T) {
	cfg := Default()
	flags, err := cfg.SetFeature("", FeatureStock, false)
	if err != nil || flags.Stock || cfg.Features.Stock {
		t.Fatalf("global feature = %+v / %v", flags, err)
	}

	// The first ledger change copies the current defaults (stock is already
	// off) and changes only the requested switch.
	flags, err = cfg.SetFeature("ledger-1", FeatureAccounting, false)
	if err != nil || flags.Accounting || flags.Stock || !flags.Diary {
		t.Fatalf("ledger feature = %+v / %v", flags, err)
	}
	if !cfg.Features.Accounting {
		t.Fatal("a ledger change also changed the defaults")
	}
	if got := cfg.EffectiveFeatures("ledger-1"); got.Accounting || got.Stock || !got.Diary {
		t.Fatalf("effective for the ledger = %+v", got)
	}
	if got := cfg.EffectiveFeatures("other"); !got.Accounting || got.Stock {
		t.Fatalf("effective for another ledger = %+v", got)
	}
	if _, err := cfg.SetFeature("", FeatureKey("nope"), true); err != ErrFeatureKey {
		t.Fatalf("invalid key error = %v", err)
	}
}

func TestSaveWithoutPathOnlyUpdatesMemory(t *testing.T) {
	store := Load("")
	cfg := store.Config()
	cfg.Width = 900
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if store.Config().Width != 900 {
		t.Fatal("the in-memory config was not updated")
	}
}

func writeConfig(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// sameConfig compares the fields that matter to the tests; maps and slices are
// compared by content through reflect.DeepEqual.
func sameConfig(a, b AppConfig) bool {
	return reflect.DeepEqual(a, b)
}
