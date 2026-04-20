package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type CherryStudioAdapter struct{}

func (a *CherryStudioAdapter) ID() string          { return "cherry_studio" }
func (a *CherryStudioAdapter) Name() string        { return "Cherry Studio" }
func (a *CherryStudioAdapter) Description() string { return "cherry-studio config" }

func (a *CherryStudioAdapter) SupportedPlatforms() []string {
	return []string{PlatformOpenAI, PlatformAnthropic, PlatformMiniMax, PlatformCustom}
}

func (a *CherryStudioAdapter) configDir() string {
	return filepath.Join(appDataDir(), "cherry-studio")
}

func (a *CherryStudioAdapter) ConfigFilePath() string {
	return filepath.Join(a.configDir(), "config.json")
}

func (a *CherryStudioAdapter) Detect() bool {
	return fileExists(a.configDir())
}

func (a *CherryStudioAdapter) Apply(req ApplyRequest) error {
	path := a.ConfigFilePath()
	config, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	providers, _ := config["providers"].([]interface{})

	baseURL := req.BaseURL
	if baseURL != "" {
		baseURL = ensureV1(baseURL)
	}

	found := false
	for i, p := range providers {
		pm, _ := p.(map[string]interface{})
		if pm != nil && pm["id"] == "dfswitch" {
			pm["apiKey"] = req.APIKey
			if baseURL != "" {
				pm["apiHost"] = baseURL
			}
			// Preserve the user's enabled choice rather than forcing true.
			if _, ok := pm["enabled"]; !ok {
				pm["enabled"] = true
			}
			providers[i] = pm
			found = true
			break
		}
	}
	if !found {
		providers = append(providers, map[string]interface{}{
			"id":      "dfswitch",
			"name":    "DFSwitch",
			"apiKey":  req.APIKey,
			"apiHost": baseURL,
			"enabled": true,
		})
	}
	config["providers"] = providers

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteWithBackup(path, data)
}

func (a *CherryStudioAdapter) Read() (*ApplyRequest, error) {
	path := a.ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	providers, _ := config["providers"].([]interface{})
	for _, p := range providers {
		pm, _ := p.(map[string]interface{})
		if pm != nil && pm["id"] == "dfswitch" {
			req := &ApplyRequest{}
			if v, ok := pm["apiKey"].(string); ok {
				req.APIKey = v
			}
			if v, ok := pm["apiHost"].(string); ok {
				req.BaseURL = v
			}
			return req, nil
		}
	}
	return &ApplyRequest{}, nil
}
