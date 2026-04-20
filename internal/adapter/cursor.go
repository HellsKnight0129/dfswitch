package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type CursorAdapter struct{}

func (a *CursorAdapter) ID() string          { return "cursor" }
func (a *CursorAdapter) Name() string        { return "Cursor" }
func (a *CursorAdapter) Description() string { return "Cursor/User/settings.json" }

func (a *CursorAdapter) SupportedPlatforms() []string {
	return []string{PlatformOpenAI}
}

func (a *CursorAdapter) configDir() string {
	return filepath.Join(appDataDir(), "Cursor", "User")
}

func (a *CursorAdapter) ConfigFilePath() string {
	return filepath.Join(a.configDir(), "settings.json")
}

func (a *CursorAdapter) Detect() bool {
	return fileExists(filepath.Join(appDataDir(), "Cursor"))
}

func (a *CursorAdapter) Apply(req ApplyRequest) error {
	path := a.ConfigFilePath()
	config := make(map[string]interface{})

	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &config)
	}

	config["openai.apiKey"] = req.APIKey
	if req.BaseURL != "" {
		config["openai.baseUrl"] = req.BaseURL
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteWithBackup(path, data)
}

func (a *CursorAdapter) Read() (*ApplyRequest, error) {
	path := a.ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	req := &ApplyRequest{}
	if v, ok := config["openai.apiKey"].(string); ok {
		req.APIKey = v
	}
	if v, ok := config["openai.baseUrl"].(string); ok {
		req.BaseURL = v
	}
	return req, nil
}
