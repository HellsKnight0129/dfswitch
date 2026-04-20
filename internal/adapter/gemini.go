package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type GeminiAdapter struct{}

func (a *GeminiAdapter) ID() string          { return "gemini" }
func (a *GeminiAdapter) Name() string        { return "Gemini CLI" }
func (a *GeminiAdapter) Description() string { return "~/.gemini/.env" }

func (a *GeminiAdapter) SupportedPlatforms() []string {
	return []string{PlatformGemini}
}

func (a *GeminiAdapter) configDir() string {
	return filepath.Join(homeDir(), ".gemini")
}

func (a *GeminiAdapter) ConfigFilePath() string {
	return filepath.Join(a.configDir(), ".env")
}

func (a *GeminiAdapter) Detect() bool {
	return fileExists(a.configDir())
}

func (a *GeminiAdapter) Apply(req ApplyRequest) error {
	// Write .env — preserve user's comments, blank lines, and unrelated vars.
	envPath := a.ConfigFilePath()
	var existing string
	if data, err := os.ReadFile(envPath); err == nil {
		existing = string(data)
	}

	updates := map[string]string{"GEMINI_API_KEY": req.APIKey}
	if req.BaseURL != "" {
		// Gemini CLI appends /v1beta/... itself; pass only the host here.
		updates["GOOGLE_GEMINI_BASE_URL"] = stripV1Beta(req.BaseURL)
	}

	content := mergeEnv(existing, updates)
	if err := atomicWriteWithBackup(envPath, []byte(content)); err != nil {
		return err
	}

	// Also write settings.json to set auth type
	settingsPath := filepath.Join(a.configDir(), "settings.json")
	settings, err := readJSONObject(settingsPath)
	if err != nil {
		return err
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	// Set security.auth.selectedType = "gemini-api-key"
	security, _ := settings["security"].(map[string]interface{})
	if security == nil {
		security = make(map[string]interface{})
	}
	auth, _ := security["auth"].(map[string]interface{})
	if auth == nil {
		auth = make(map[string]interface{})
	}
	auth["selectedType"] = "gemini-api-key"
	security["auth"] = auth
	settings["security"] = security

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteWithBackup(settingsPath, out)
}

func (a *GeminiAdapter) Read() (*ApplyRequest, error) {
	path := a.ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := parseEnv(string(data))
	return &ApplyRequest{
		APIKey:  env["GEMINI_API_KEY"],
		BaseURL: env["GOOGLE_GEMINI_BASE_URL"],
	}, nil
}

// parseEnv extracts KEY=VALUE pairs from .env content.
// Handles `export` prefix and single/double quoted values.
func parseEnv(content string) map[string]string {
	m := make(map[string]string)
	for _, raw := range strings.Split(content, "\n") {
		if k, v, ok := parseEnvLine(raw); ok {
			m[k] = v
		}
	}
	return m
}

// parseEnvLine returns (key, unquoted_value, true) if the line is a KEY=VALUE
// assignment. Comment lines and blanks return false. `export FOO=bar` is
// accepted. Quotes around the value are stripped.
func parseEnvLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "export ")
	trimmed = strings.TrimSpace(trimmed)
	k, v, ok := strings.Cut(trimmed, "=")
	if !ok {
		return "", "", false
	}
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
	}
	return k, v, true
}

// mergeEnv updates only the keys in `updates`, preserving every other line
// (comments, blanks, unrelated vars, `export` prefixes). New keys are
// appended in sorted order.
func mergeEnv(existing string, updates map[string]string) string {
	handled := make(map[string]bool, len(updates))
	var lines []string
	if existing != "" {
		lines = strings.Split(strings.TrimRight(existing, "\n"), "\n")
	}
	out := make([]string, 0, len(lines)+len(updates))
	for _, line := range lines {
		k, _, ok := parseEnvLine(line)
		if ok {
			if v, hit := updates[k]; hit {
				out = append(out, fmt.Sprintf("%s=%s", k, v))
				handled[k] = true
				continue
			}
		}
		out = append(out, line)
	}
	newKeys := make([]string, 0, len(updates))
	for k := range updates {
		if !handled[k] {
			newKeys = append(newKeys, k)
		}
	}
	sort.Strings(newKeys)
	for _, k := range newKeys {
		out = append(out, fmt.Sprintf("%s=%s", k, updates[k]))
	}
	return strings.Join(out, "\n") + "\n"
}
