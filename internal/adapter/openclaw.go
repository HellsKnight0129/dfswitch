package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type OpenClawAdapter struct{}

func (a *OpenClawAdapter) ID() string          { return "openclaw" }
func (a *OpenClawAdapter) Name() string        { return "OpenClaw" }
func (a *OpenClawAdapter) Description() string { return "~/.openclaw/openclaw.json" }

func (a *OpenClawAdapter) SupportedPlatforms() []string {
	// Experimental — schema not officially aligned. Restrict to
	// openai-compatible to avoid silently writing wrong protocol.
	return []string{PlatformOpenAI, PlatformCustom}
}

func (a *OpenClawAdapter) configDir() string {
	return filepath.Join(homeDir(), ".openclaw")
}

func (a *OpenClawAdapter) ConfigFilePath() string {
	return filepath.Join(a.configDir(), "openclaw.json")
}

func (a *OpenClawAdapter) Detect() bool {
	return fileExists(a.configDir())
}

func (a *OpenClawAdapter) Apply(req ApplyRequest) error {
	path := a.ConfigFilePath()
	config, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	// Ensure models.providers structure
	models, _ := config["models"].(map[string]interface{})
	if models == nil {
		models = map[string]interface{}{"mode": "merge", "providers": map[string]interface{}{}}
	}
	providers, _ := models["providers"].(map[string]interface{})
	if providers == nil {
		providers = make(map[string]interface{})
	}

	baseURL := req.BaseURL
	if baseURL != "" {
		baseURL = ensureV1(baseURL)
	}

	// Merge into existing dfswitch provider to preserve user-added fields
	// like custom models lists.
	provider, _ := providers["dfswitch"].(map[string]interface{})
	if provider == nil {
		provider = make(map[string]interface{})
	}
	provider["apiKey"] = req.APIKey
	if baseURL != "" {
		provider["baseUrl"] = baseURL
	}
	providers["dfswitch"] = provider
	models["providers"] = providers
	config["models"] = models

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteWithBackup(path, data)
}

func (a *OpenClawAdapter) Read() (*ApplyRequest, error) {
	path := a.ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	models, _ := config["models"].(map[string]interface{})
	if models == nil {
		return &ApplyRequest{}, nil
	}
	providers, _ := models["providers"].(map[string]interface{})
	if providers == nil {
		return &ApplyRequest{}, nil
	}
	p, _ := providers["dfswitch"].(map[string]interface{})
	if p == nil {
		return &ApplyRequest{}, nil
	}

	req := &ApplyRequest{}
	if v, ok := p["apiKey"].(string); ok {
		req.APIKey = v
	}
	if v, ok := p["baseUrl"].(string); ok {
		req.BaseURL = v
	}
	return req, nil
}
