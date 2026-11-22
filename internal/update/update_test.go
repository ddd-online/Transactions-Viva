package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.2.0", "0.1.9", true},
		{"v0.2.0", "0.2", false},
		{"0.2.0", "0.2.0", false},
		{"0.2", "0.2.0", false},
		{"0.1.9", "0.2.0", false},
	}
	for _, test := range cases {
		if got := IsNewer(test.latest, test.current); got != test.want {
			t.Fatalf("IsNewer(%q, %q) = %v, want %v", test.latest, test.current, got, test.want)
		}
	}
}

func TestCheckReadsReleaseAndPickExe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":   "v0.2.0",
			"prerelease": false,
			"body":       "## 更新内容\n\n- 修复问题",
			"assets": []map[string]any{
				{"name": "notes.txt", "browser_download_url": "https://github.com/x/notes.txt"},
				{"name": "Transactions.exe", "browser_download_url": "https://github.com/x/Transactions.exe", "digest": "sha256:ABC"},
			},
		})
	}))
	defer server.Close()
	result, err := CheckAt(context.Background(), server.Client(), server.URL, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasUpdate || result.Version != "0.2.0" {
		t.Fatalf("result = %+v", result)
	}
	if result.DownloadURL != "https://github.com/x/Transactions.exe" || result.Digest != "sha256:ABC" {
		t.Fatalf("asset = %+v", result)
	}
	if !strings.Contains(result.Body, "修复问题") {
		t.Fatalf("body = %q", result.Body)
	}
}

func TestCheckNoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.1.0"})
	}))
	defer server.Close()
	result, err := CheckAt(context.Background(), server.Client(), server.URL, "0.1.0")
	if err != nil || result.HasUpdate {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestDownloadVerifiesDigestAndReportsProgress(t *testing.T) {
	content := []byte(strings.Repeat("transactions-installer\n", 1000))
	sum := sha256.Sum256(content)
	digest := "sha256:" + strings.ToUpper(hex.EncodeToString(sum[:]))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()
	directory := t.TempDir()
	progress := 0
	path, err := Download(context.Background(), server.Client(), server.URL+"/Transactions.exe", digest, directory, true, func(p Progress) {
		if p.Percent > progress {
			progress = p.Percent
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "Transactions.exe" {
		t.Fatalf("path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(content) {
		t.Fatalf("downloaded content mismatch: %v", err)
	}
	if progress != 100 {
		t.Fatalf("progress = %d", progress)
	}
}

func TestDownloadRejectsForeignHost(t *testing.T) {
	if _, err := Download(context.Background(), http.DefaultClient, "https://example.com/a.exe", "", t.TempDir(), false, nil); err == nil {
		t.Fatal("foreign host accepted")
	}
}

func TestAllowedDownloadHost(t *testing.T) {
	for _, good := range []string{
		"https://github.com/x/a.exe",
		"https://objects.githubusercontent.com/x/a.exe",
	} {
		if !AllowedDownloadHost(good, false) {
			t.Fatalf("rejected %q", good)
		}
	}
	for _, bad := range []string{
		"http://github.com/a.exe",
		"https://example.com/a.exe",
	} {
		if AllowedDownloadHost(bad, false) {
			t.Fatalf("accepted %q", bad)
		}
	}
}
