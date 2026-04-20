package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Release source is hard-coded; flip GITHUB_REPO env to override during dev.
const defaultGitHubRepo = "sub2api/dfswitch"

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

func releaseRepo() string {
	if v := os.Getenv("DFSWITCH_RELEASE_REPO"); v != "" {
		return v
	}
	return defaultGitHubRepo
}

func checkGitHubRelease(ctx context.Context) (*UpdateInfo, error) {
	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(Version, "v")
	info := &UpdateInfo{
		CurrentVersion: Version,
		LatestVersion:  rel.TagName,
		ReleaseNotes:   rel.Body,
		Available:      current != "dev" && latest != "" && latest != current && compareSemver(latest, current) > 0,
	}
	if asset := pickAsset(rel); asset != nil {
		info.DownloadURL = asset.BrowserDownloadURL
	}
	// dev builds: surface the latest tag but don't mark "available" — avoids
	// forcing a re-download of an unstamped local build.
	return info, nil
}

func applyGitHubUpdate(ctx context.Context) error {
	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		return err
	}
	asset := pickAsset(rel)
	if asset == nil {
		return fmt.Errorf("未找到适合 %s/%s 的发布资产", runtime.GOOS, runtime.GOARCH)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("定位当前可执行文件失败: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("解析可执行文件路径失败: %w", err)
	}

	tmp, err := downloadToTemp(ctx, asset.BrowserDownloadURL, filepath.Dir(exe))
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	if err := os.Chmod(tmp, 0o755); err != nil {
		return fmt.Errorf("设置可执行权限失败: %w", err)
	}

	// os.Rename across same dir is atomic; on Windows it fails if exe is
	// running, so fall back to copy-over (rename old away first).
	if err := os.Rename(tmp, exe); err != nil {
		backup := exe + ".old"
		_ = os.Remove(backup)
		if err2 := os.Rename(exe, backup); err2 != nil {
			return fmt.Errorf("替换可执行文件失败: %v / %v", err, err2)
		}
		if err2 := os.Rename(tmp, exe); err2 != nil {
			_ = os.Rename(backup, exe)
			return fmt.Errorf("写入新版本失败: %w", err2)
		}
	}
	return nil
}

func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", releaseRepo())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("联系 GitHub 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回 %s", resp.Status)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("解析发布信息失败: %w", err)
	}
	if rel.TagName == "" {
		return nil, errors.New("GitHub 返回的发布信息缺少 tag")
	}
	return &rel, nil
}

// pickAsset chooses the asset matching GOOS/GOARCH. Naming convention:
// dfswitch-<goos>-<goarch>[.exe]. Falls back to any asset whose name contains
// both goos and goarch tokens.
func pickAsset(rel *githubRelease) *struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
} {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	suffix := ""
	if goos == "windows" {
		suffix = ".exe"
	}
	preferred := fmt.Sprintf("dfswitch-%s-%s%s", goos, goarch, suffix)
	for i := range rel.Assets {
		if rel.Assets[i].Name == preferred {
			return &rel.Assets[i]
		}
	}
	for i := range rel.Assets {
		name := strings.ToLower(rel.Assets[i].Name)
		if strings.Contains(name, goos) && strings.Contains(name, goarch) {
			return &rel.Assets[i]
		}
	}
	return nil
}

func downloadToTemp(ctx context.Context, url, dir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败: %s", resp.Status)
	}

	f, err := os.CreateTemp(dir, "dfswitch-update-*")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	path := f.Name()
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(path)
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// compareSemver returns -1/0/1 comparing dotted numeric segments. Non-numeric
// segments (e.g. "1.2.3-rc1") compare lexicographically within their segment.
func compareSemver(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var ax, bx string
		if i < len(as) {
			ax = as[i]
		}
		if i < len(bs) {
			bx = bs[i]
		}
		if ax == bx {
			continue
		}
		ai, aok := parseLeadingInt(ax)
		bi, bok := parseLeadingInt(bx)
		if aok && bok && ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
		if ax < bx {
			return -1
		}
		return 1
	}
	return 0
}

func parseLeadingInt(s string) (int, bool) {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			if i == 0 {
				return 0, false
			}
			break
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}
