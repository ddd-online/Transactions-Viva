// Package update checks GitHub releases, downloads the installer and starts it.
// The settings page renders the state; this package only talks to the network
// and the file system.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ReleaseAPI is the latest-release endpoint of the public repository.
const ReleaseAPI = "https://api.github.com/repos/ddd-online/Transactions-Viva/releases/latest"

// CheckTimeout bounds one release lookup.
const CheckTimeout = 15 * time.Second

// Result is one release lookup. HasUpdate is false for "already latest" and
// for a release tagged as a prerelease.
type Result struct {
	HasUpdate   bool
	Version     string
	DownloadURL string
	Digest      string
	Body        string
}

// Progress is one download sample. Percent is 0..100; Speed is display-ready.
type Progress struct {
	Percent int
	Speed   string
}

type release struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Body       string `json:"body"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"`
	} `json:"assets"`
}

// Check reads the latest release and compares it with currentVersion. A
// network or parse failure is returned as an error; "no update" is not.
func Check(ctx context.Context, client *http.Client, currentVersion string) (Result, error) {
	return CheckAt(ctx, client, ReleaseAPI, currentVersion)
}

// CheckAt is Check against another endpoint, for tests and release rehearsals.
func CheckAt(ctx context.Context, client *http.Client, endpoint, currentVersion string) (Result, error) {
	if client == nil {
		client = &http.Client{Timeout: CheckTimeout}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Transactions-App")
	response, err := client.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("请求 GitHub API 失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("请求 GitHub API 失败：HTTP %d", response.StatusCode)
	}
	var payload release
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return Result{}, fmt.Errorf("解析 GitHub 发布信息失败：%w", err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(payload.TagName, "v"))
	if payload.Prerelease || version == "" || !IsNewer(version, currentVersion) {
		return Result{Version: version}, nil
	}
	for _, asset := range payload.Assets {
		if !strings.HasSuffix(strings.ToLower(asset.Name), ".exe") ||
			strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			continue
		}
		return Result{
			HasUpdate:   true,
			Version:     version,
			DownloadURL: asset.BrowserDownloadURL,
			Digest:      asset.Digest,
			Body:        payload.Body,
		}, nil
	}
	return Result{Version: version}, nil
}

// IsNewer compares dotted numeric versions, ignoring a leading v. Missing
// segments count as zero, so 0.2 == 0.2.0 and 0.2.1 > 0.2.
func IsNewer(latest, current string) bool {
	latestParts := versionParts(latest)
	currentParts := versionParts(current)
	length := len(latestParts)
	if len(currentParts) > length {
		length = len(currentParts)
	}
	for index := 0; index < length; index++ {
		var left, right uint64
		if index < len(latestParts) {
			left = latestParts[index]
		}
		if index < len(currentParts) {
			right = currentParts[index]
		}
		if left != right {
			return left > right
		}
	}
	return false
}

func versionParts(version string) []uint64 {
	version = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(version), "v"))
	fields := strings.Split(version, ".")
	parts := make([]uint64, 0, len(fields))
	for _, field := range fields {
		value, _ := strconv.ParseUint(field, 10, 64)
		parts = append(parts, value)
	}
	return parts
}

// NormalizeDigest strips the sha256: prefix and lowercases the hex text. An
// empty digest returns "" and skips verification.
func NormalizeDigest(digest string) string {
	digest = strings.TrimSpace(digest)
	for strings.HasPrefix(digest, "sha256:") {
		digest = strings.TrimPrefix(digest, "sha256:")
	}
	return strings.ToLower(strings.TrimSpace(digest))
}

// DigestMatches reports whether an expected digest (possibly empty) matches.
func DigestMatches(expected, actual string) bool {
	normalized := NormalizeDigest(expected)
	if normalized == "" {
		return true
	}
	return normalized == NormalizeDigest(actual)
}

// AllowedDownloadHost limits downloads to GitHub's release hosts. allowLocal is
// for tests and the local release harness.
func AllowedDownloadHost(raw string, allowLocal bool) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if allowLocal && (host == "127.0.0.1" || host == "localhost") {
		return parsed.Scheme == "https" || parsed.Scheme == "http"
	}
	if parsed.Scheme != "https" {
		return false
	}
	return host == "github.com" || strings.HasSuffix(host, ".github.com") ||
		host == "githubusercontent.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

// Download fetches the installer into tempDir, reports progress and verifies
// the digest before renaming the finished file. A cancelled context removes
// the partial file.
func Download(ctx context.Context, client *http.Client, rawURL, digest, tempDir string, allowLocal bool, progress func(Progress)) (string, error) {
	if !AllowedDownloadHost(rawURL, allowLocal) {
		return "", fmt.Errorf("下载地址不在白名单内")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	name := path.Base(parsed.Path)
	if name == "" || name == "." || name == "/" {
		name = "transactions-update.exe"
	}
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(tempDir, name)
	partial := target + ".part"

	if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
		if matches, err := fileDigest(target); err == nil && DigestMatches(digest, matches) {
			if progress != nil {
				progress(Progress{Percent: 100, Speed: ""})
			}
			return target, nil
		}
		_ = os.Remove(target)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "Transactions-App")
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("下载失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败：HTTP %d", response.StatusCode)
	}

	file, err := os.Create(partial)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	total := response.ContentLength
	var written int64
	started := time.Now()
	buffer := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			file.Close()
			_ = os.Remove(partial)
			return "", err
		}
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			if _, err := file.Write(buffer[:count]); err != nil {
				file.Close()
				_ = os.Remove(partial)
				return "", err
			}
			hasher.Write(buffer[:count])
			written += int64(count)
			if progress != nil {
				percent := 0
				if total > 0 {
					percent = int(written * 100 / total)
					if percent > 100 {
						percent = 100
					}
				}
				speed := ""
				if elapsed := time.Since(started).Seconds(); elapsed > 0 {
					speed = FormatSpeed(float64(written) / elapsed)
				}
				progress(Progress{Percent: percent, Speed: speed})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			file.Close()
			_ = os.Remove(partial)
			return "", fmt.Errorf("下载失败：%w", readErr)
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(partial)
		return "", err
	}
	if !DigestMatches(digest, hex.EncodeToString(hasher.Sum(nil))) {
		_ = os.Remove(partial)
		return "", fmt.Errorf("下载文件校验失败（SHA256 不匹配）")
	}
	if err := os.Rename(partial, target); err != nil {
		_ = os.Remove(partial)
		return "", err
	}
	if progress != nil {
		progress(Progress{Percent: 100})
	}
	return target, nil
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// FormatSpeed is the display string shown beside the progress bar.
func FormatSpeed(bytesPerSecond float64) string {
	const kib, mib = 1024, 1024 * 1024
	switch {
	case bytesPerSecond >= mib:
		return fmt.Sprintf("%.1f MB/s", bytesPerSecond/mib)
	case bytesPerSecond >= kib:
		return fmt.Sprintf("%.1f KB/s", bytesPerSecond/kib)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSecond)
	}
}

// Launch starts the installer in its own process and releases it so the
// application can exit. The caller decides when to quit.
func Launch(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("安装文件不存在")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("安装文件不存在")
	}
	command := exec.Command(path)
	command.Stdout = nil
	command.Stderr = nil
	command.Stdin = nil
	setDetached(command)
	if err := command.Start(); err != nil {
		return fmt.Errorf("启动安装程序失败：%w", err)
	}
	return command.Process.Release()
}
