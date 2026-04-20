package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ClaudeAdapter struct{}

func (a *ClaudeAdapter) ID() string          { return "claude" }
func (a *ClaudeAdapter) Name() string        { return "Claude Code" }
func (a *ClaudeAdapter) Description() string { return "~/.claude/settings.json" }

func (a *ClaudeAdapter) SupportedPlatforms() []string {
	return []string{PlatformAnthropic, PlatformMiniMax, PlatformCustom}
}

func (a *ClaudeAdapter) configDir() string {
	return filepath.Join(homeDir(), ".claude")
}

func (a *ClaudeAdapter) ConfigFilePath() string {
	return filepath.Join(a.configDir(), "settings.json")
}

func (a *ClaudeAdapter) Detect() bool {
	return fileExists(a.configDir())
}

func (a *ClaudeAdapter) Apply(req ApplyRequest) error {
	path := a.ConfigFilePath()
	config, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	// Claude Code reads env.ANTHROPIC_AUTH_TOKEN and env.ANTHROPIC_BASE_URL.
	// ANTHROPIC_BASE_URL must be the bare host — Claude Code appends
	// /v1/messages itself, so any trailing /v1 leads to /v1/v1/messages 404s.
	env, _ := config["env"].(map[string]interface{})
	if env == nil {
		env = make(map[string]interface{})
	}
	env["ANTHROPIC_AUTH_TOKEN"] = req.APIKey
	env["ANTHROPIC_API_KEY"] = req.APIKey
	if req.BaseURL != "" {
		env["ANTHROPIC_BASE_URL"] = stripV1(req.BaseURL)
	}
	config["env"] = env

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteWithBackup(path, data)
}

// Warnings returns any post-apply conditions that may prevent the
// written config from taking effect. Returned values are user-facing
// Chinese strings the frontend can display verbatim.
func (a *ClaudeAdapter) Warnings() []string {
	var out []string
	credPath := filepath.Join(a.configDir(), ".credentials.json")
	if fileExists(credPath) {
		out = append(out, "检测到 ~/.claude/.credentials.json（OAuth 凭据），Claude Code 会优先使用它，dfswitch 写入的 API Key 可能不生效。可用 `claude /logout` 清除。")
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("ANTHROPIC_AUTH_TOKEN") != "" || os.Getenv("ANTHROPIC_BASE_URL") != "" {
		out = append(out, "检测到 shell 环境变量 ANTHROPIC_* 已设置，shell env 会覆盖 settings.json 中的 env 块；请从 shell 启动脚本中移除相关变量。")
	}
	return out
}

func (a *ClaudeAdapter) Read() (*ApplyRequest, error) {
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
	env, _ := config["env"].(map[string]interface{})
	if env != nil {
		if v, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); ok {
			req.APIKey = v
		}
		if v, ok := env["ANTHROPIC_BASE_URL"].(string); ok {
			req.BaseURL = v
		}
	}
	return req, nil
}
