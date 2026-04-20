package adapter

import (
	"fmt"
	"strings"
)

// Platform constants mirror backend/internal/domain/constants.go.
const (
	PlatformAnthropic   = "anthropic"
	PlatformOpenAI      = "openai"
	PlatformGemini      = "gemini"
	PlatformAntigravity = "antigravity"
	PlatformMiniMax     = "minimax"
	PlatformCustom      = "custom"
)

// GatewayURL returns the bare base URL (no version suffix) for a sub2api
// group platform. Each adapter is responsible for appending whatever path
// segment its target tool expects — e.g. Claude Code's ANTHROPIC_BASE_URL
// expects the naked host (Claude Code appends /v1/messages itself), while
// an OpenAI-compatible client expects .../v1.
//
// Antigravity is the one exception: its gateway is mounted at a subpath
// (/antigravity) rather than at the root, so we include that subpath here.
func GatewayURL(serverURL, platform string) string {
	base := strings.TrimRight(serverURL, "/")
	if platform == PlatformAntigravity {
		return base + "/antigravity"
	}
	return base
}

// OpenAIBaseURL returns an OpenAI-compatible base URL ending in /v1.
func OpenAIBaseURL(serverURL string) string {
	return strings.TrimRight(serverURL, "/") + "/v1"
}

// IsCompatible reports whether a tool supports the given key platform.
func IsCompatible(supported []string, platform string) bool {
	for _, p := range supported {
		if p == platform {
			return true
		}
	}
	return false
}

// IncompatibleError is returned by Apply-layer validators when a platform
// doesn't match the adapter's SupportedPlatforms.
type IncompatibleError struct {
	ToolID    string
	Platform  string
	Supported []string
}

func (e *IncompatibleError) Error() string {
	return fmt.Sprintf("tool %s does not support platform %s (supports: %s)",
		e.ToolID, e.Platform, strings.Join(e.Supported, ","))
}
