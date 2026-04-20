package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Tool represents a supported AI client tool
type Tool struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Installed          bool     `json:"installed"`
	ConfigPath         string   `json:"config_path"`
	SupportedPlatforms []string `json:"supported_platforms"`
}

// ApplyRequest contains the key info to write into a tool's config
type ApplyRequest struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	// Platform is the sub2api group.platform value (anthropic/openai/gemini/...).
	// Adapters may use this to choose a protocol variant — e.g. OpenCode
	// needs @ai-sdk/anthropic for anthropic-protocol keys vs
	// @ai-sdk/openai-compatible for openai keys.
	Platform string `json:"platform"`
}

// Adapter defines the interface for each tool's config operations
type Adapter interface {
	ID() string
	Name() string
	Description() string
	Detect() bool                 // Check if the tool is installed locally
	ConfigFilePath() string       // Return the config file path
	Apply(req ApplyRequest) error // Write API key + base_url into config
	Read() (*ApplyRequest, error) // Read current config (if any)
	SupportedPlatforms() []string // sub2api group.platform values this tool can consume
}

// AllAdapters returns all registered tool adapters.
//
// Intentionally excluded until we have verified schemas:
//   - Cursor: AI API Key is persisted inside Cursor's internal SQLite
//     (state.vscdb), not settings.json. Writing settings.json silently
//     has no effect on the chat panel.
//   - CherryStudio: providers are persisted in IndexedDB/better-sqlite3
//     under the app's userData dir, not in config.json. Writing config.json
//     does not change the in-app provider list.
func AllAdapters() []Adapter {
	return []Adapter{
		&ClaudeAdapter{},
		&GeminiAdapter{},
		&OpenClawAdapter{},
		&OpenCodeAdapter{},
		&ChatBoxAdapter{},
	}
}

// helper: get user home directory
func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// helper: get platform-specific app data directory
func appDataDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Application Support")
	case "windows":
		return os.Getenv("APPDATA")
	default:
		return filepath.Join(homeDir(), ".config")
	}
}

// helper: check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ensureV1 appends /v1 to base if it doesn't already have it.
// Used by OpenAI-compatible clients that expect the full /v1 URL.
func ensureV1(base string) string {
	b := strings.TrimRight(base, "/")
	if strings.HasSuffix(b, "/v1") || strings.HasSuffix(b, "/v1beta") {
		return b
	}
	return b + "/v1"
}

// stripV1 removes a trailing /v1 if present. Used by clients that
// append the version segment themselves (e.g. Claude Code).
func stripV1(base string) string {
	b := strings.TrimRight(base, "/")
	return strings.TrimSuffix(b, "/v1")
}

// stripV1Beta removes a trailing /v1beta if present.
func stripV1Beta(base string) string {
	b := strings.TrimRight(base, "/")
	return strings.TrimSuffix(b, "/v1beta")
}

// readJSONObject reads path and unmarshals into a map. Returns (nil, nil)
// when the file does not exist. If the file exists but cannot be parsed
// (e.g. contains // comments — JSONC — or is corrupt), it returns an
// explicit error so callers refuse to write and risk wiping the user's
// existing configuration.
func readJSONObject(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("%s 解析失败（可能含 // 注释或格式损坏），dfswitch 拒绝覆盖；请手动修正或删除该文件后重试：%w", path, err)
	}
	return out, nil
}

// atomicWriteWithBackup writes data to path atomically: tmp file + fsync + rename.
// Before replacing, the existing file is copied to path + ".dfswitch.bak" so a
// user can recover the previous config. If the rename fails, the target file
// is unchanged.
func atomicWriteWithBackup(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if fileExists(path) {
		backupPath := path + ".dfswitch.bak"
		// Only create the backup once — preserving the user's ORIGINAL
		// (pre-dfswitch) config. Otherwise every Apply would rewrite the
		// backup with the previous dfswitch-written content, leaking old
		// API keys and losing the only copy of the user's original file.
		if !fileExists(backupPath) {
			if existing, err := os.ReadFile(path); err == nil {
				_ = os.WriteFile(backupPath, existing, 0600)
			}
		}
	}

	tmp := path + ".dfswitch.tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// fsync the directory so the rename survives power loss.
	// Best-effort: Windows returns EACCES opening a dir; ignore in that case.
	if dirF, err := os.Open(dir); err == nil {
		_ = dirF.Sync()
		_ = dirF.Close()
	}
	return nil
}
