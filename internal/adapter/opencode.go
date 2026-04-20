package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type OpenCodeAdapter struct{}

func (a *OpenCodeAdapter) ID() string          { return "opencode" }
func (a *OpenCodeAdapter) Name() string        { return "OpenCode" }
func (a *OpenCodeAdapter) Description() string { return "~/.config/opencode/opencode.json" }

func (a *OpenCodeAdapter) SupportedPlatforms() []string {
	// Only OpenAI-compatible: we write @ai-sdk/openai-compatible. Anthropic
	// and MiniMax keys speak the Anthropic /v1/messages protocol and would
	// 404 through OpenAI chat.completions. Add those back only when we
	// route them to @ai-sdk/anthropic with a verified schema.
	return []string{PlatformOpenAI, PlatformCustom}
}

func (a *OpenCodeAdapter) configDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "opencode")
	}
	return filepath.Join(homeDir(), ".config", "opencode")
}

func (a *OpenCodeAdapter) ConfigFilePath() string {
	return filepath.Join(a.configDir(), "opencode.json")
}

func (a *OpenCodeAdapter) Detect() bool {
	return fileExists(a.configDir())
}

func (a *OpenCodeAdapter) Apply(req ApplyRequest) error {
	path := a.ConfigFilePath()
	config, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	providers, _ := config["provider"].(map[string]interface{})
	if providers == nil {
		providers = make(map[string]interface{})
	}
	// OpenCode uses ai-sdk's openai-compatible provider, which appends
	// /chat/completions to the baseURL — so the baseURL must already
	// include /v1.
	baseURL := req.BaseURL
	if baseURL != "" {
		baseURL = ensureV1(baseURL)
	}

	// Preserve any user-added fields (models, custom headers, etc.) by
	// merging into the existing entry rather than replacing wholesale.
	existing, _ := providers["dfswitch"].(map[string]interface{})
	if existing == nil {
		existing = map[string]interface{}{
			"npm":  "@ai-sdk/openai-compatible",
			"name": "DFSwitch",
		}
	}
	opts, _ := existing["options"].(map[string]interface{})
	if opts == nil {
		opts = make(map[string]interface{})
	}
	opts["apiKey"] = req.APIKey
	if baseURL != "" {
		opts["baseURL"] = baseURL
	}
	existing["options"] = opts
	providers["dfswitch"] = existing
	config["provider"] = providers

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteWithBackup(path, data)
}

func (a *OpenCodeAdapter) Read() (*ApplyRequest, error) {
	path := a.ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	providers, _ := config["provider"].(map[string]interface{})
	if providers == nil {
		return &ApplyRequest{}, nil
	}
	p, _ := providers["dfswitch"].(map[string]interface{})
	if p == nil {
		return &ApplyRequest{}, nil
	}
	opts, _ := p["options"].(map[string]interface{})
	if opts == nil {
		return &ApplyRequest{}, nil
	}
	req := &ApplyRequest{}
	if v, ok := opts["apiKey"].(string); ok {
		req.APIKey = v
	}
	if v, ok := opts["baseURL"].(string); ok {
		req.BaseURL = v
	}
	return req, nil
}
