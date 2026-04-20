package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ChatBoxAdapter struct{}

func (a *ChatBoxAdapter) ID() string          { return "chatbox" }
func (a *ChatBoxAdapter) Name() string        { return "ChatBox" }
func (a *ChatBoxAdapter) Description() string { return "xyz.chatboxapp.app/settings.json" }

func (a *ChatBoxAdapter) SupportedPlatforms() []string {
	return []string{PlatformOpenAI}
}

func (a *ChatBoxAdapter) configDir() string {
	return filepath.Join(appDataDir(), "xyz.chatboxapp.app")
}

func (a *ChatBoxAdapter) ConfigFilePath() string {
	return filepath.Join(a.configDir(), "settings.json")
}

func (a *ChatBoxAdapter) Detect() bool {
	return fileExists(a.configDir())
}

func (a *ChatBoxAdapter) Apply(req ApplyRequest) error {
	path := a.ConfigFilePath()
	config, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	config["openaiApiKey"] = req.APIKey
	if req.BaseURL != "" {
		// ChatBox's apiHost historically expects the full /v1 URL.
		config["apiHost"] = ensureV1(req.BaseURL)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteWithBackup(path, data)
}

func (a *ChatBoxAdapter) Read() (*ApplyRequest, error) {
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
	if v, ok := config["openaiApiKey"].(string); ok {
		req.APIKey = v
	}
	if v, ok := config["apiHost"].(string); ok {
		req.BaseURL = v
	}
	return req, nil
}
