package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// AppliedEntry records what was written to a tool's config, so the sync service
// can replay the same key/baseURL pair on each tick.
type AppliedEntry struct {
	KeyValue string `json:"key_value"`
	BaseURL  string `json:"base_url"`
}

type Config struct {
	mu sync.RWMutex

	ServerURL    string                  `json:"server_url"`
	AccessToken  string                  `json:"access_token"`
	RefreshToken string                  `json:"refresh_token"`
	SyncEnabled  bool                    `json:"sync_enabled"`
	SyncInterval int                     `json:"sync_interval_minutes"`
	SelectedKey  string                  `json:"selected_key"`
	AppliedTools map[string]AppliedEntry `json:"applied_tools"`
}

func NewDefault() *Config {
	return &Config{
		ServerURL:    "https://df.dawnloadai.com:8443",
		SyncInterval: 30,
		AppliedTools: make(map[string]AppliedEntry),
	}
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dfswitch")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return NewDefault(), nil
		}
		return nil, err
	}

	// Try new format first.
	c := &Config{}
	if err := json.Unmarshal(data, c); err != nil {
		// Legacy format: AppliedTools was map[string]string. Migrate it.
		var legacy struct {
			ServerURL    string            `json:"server_url"`
			AccessToken  string            `json:"access_token"`
			RefreshToken string            `json:"refresh_token"`
			SyncEnabled  bool              `json:"sync_enabled"`
			SyncInterval int               `json:"sync_interval_minutes"`
			SelectedKey  string            `json:"selected_key"`
			AppliedTools map[string]string `json:"applied_tools"`
		}
		if err2 := json.Unmarshal(data, &legacy); err2 != nil {
			return nil, err
		}
		c.ServerURL = legacy.ServerURL
		c.AccessToken = legacy.AccessToken
		c.RefreshToken = legacy.RefreshToken
		c.SyncEnabled = legacy.SyncEnabled
		c.SyncInterval = legacy.SyncInterval
		c.SelectedKey = legacy.SelectedKey
		c.AppliedTools = make(map[string]AppliedEntry, len(legacy.AppliedTools))
		for toolID, keyValue := range legacy.AppliedTools {
			c.AppliedTools[toolID] = AppliedEntry{KeyValue: keyValue}
		}
	}

	if c.AppliedTools == nil {
		c.AppliedTools = make(map[string]AppliedEntry)
	}
	if c.SyncInterval == 0 {
		c.SyncInterval = 30
	}
	return c, nil
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

func (c *Config) SetTokens(access, refresh string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AccessToken = access
	c.RefreshToken = refresh
}

func (c *Config) GetAccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AccessToken
}

func (c *Config) GetRefreshToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.RefreshToken
}

func (c *Config) GetServerURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ServerURL
}

func (c *Config) SetServerURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ServerURL = url
}

func (c *Config) SetAppliedTool(toolID, keyValue, baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AppliedTools[toolID] = AppliedEntry{KeyValue: keyValue, BaseURL: baseURL}
}

func (c *Config) GetAppliedTools() map[string]AppliedEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]AppliedEntry, len(c.AppliedTools))
	for k, v := range c.AppliedTools {
		out[k] = v
	}
	return out
}
