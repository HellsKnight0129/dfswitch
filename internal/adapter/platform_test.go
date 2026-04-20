package adapter

import "testing"

// GatewayURL now returns the bare host (no /v1) for every platform except
// antigravity, which prepends /antigravity. Adapters are responsible for
// appending /v1 or /v1beta when their client expects it — Claude Code and
// Gemini CLI append version paths themselves, while OpenAI-compat clients
// want the full /v1 URL.
func TestGatewayURL(t *testing.T) {
	const server = "https://df.dawnloadai.com:8443"
	cases := []struct {
		platform string
		want     string
	}{
		{PlatformAnthropic, server},
		{PlatformOpenAI, server},
		{PlatformMiniMax, server},
		{PlatformCustom, server},
		{PlatformGemini, server},
		{PlatformAntigravity, server + "/antigravity"},
		{"", server},
		{"made-up", server},
	}
	for _, tc := range cases {
		if got := GatewayURL(server, tc.platform); got != tc.want {
			t.Errorf("GatewayURL(%q) = %q, want %q", tc.platform, got, tc.want)
		}
	}
}

func TestGatewayURL_TrimsTrailingSlash(t *testing.T) {
	if got := GatewayURL("https://x.y/", PlatformAnthropic); got != "https://x.y" {
		t.Errorf("want trim, got %q", got)
	}
}

func TestIsCompatible(t *testing.T) {
	sup := []string{PlatformAnthropic, PlatformMiniMax}
	if !IsCompatible(sup, PlatformAnthropic) {
		t.Error("anthropic should be compatible")
	}
	if IsCompatible(sup, PlatformGemini) {
		t.Error("gemini should not be compatible")
	}
	if IsCompatible(nil, PlatformAnthropic) {
		t.Error("nil supported should reject all")
	}
}

func TestAllAdaptersSupportedPlatforms(t *testing.T) {
	for _, a := range AllAdapters() {
		sp := a.SupportedPlatforms()
		if len(sp) == 0 {
			t.Errorf("adapter %s declares no supported platforms", a.ID())
		}
	}
}

// Regression guards for the env parser: Claude/Gemini lose user comments and
// manual edits if we round-trip through a simplistic parser.
func TestMergeEnv_PreservesCommentsAndOrder(t *testing.T) {
	existing := "# user comment\nexport FOO=bar\nGEMINI_API_KEY=old\n\n# trailing note\n"
	out := mergeEnv(existing, map[string]string{"GEMINI_API_KEY": "new"})
	wantSubstrings := []string{"# user comment", "export FOO=bar", "GEMINI_API_KEY=new", "# trailing note"}
	for _, s := range wantSubstrings {
		if !contains(out, s) {
			t.Errorf("mergeEnv dropped %q\n---\n%s", s, out)
		}
	}
	if contains(out, "GEMINI_API_KEY=old") {
		t.Errorf("mergeEnv did not update key\n%s", out)
	}
}

func TestParseEnv_HandlesQuotesAndExport(t *testing.T) {
	content := `export QUOTED="quoted value"
PLAIN=plain
SINGLE='single q'
# comment
BLANK=
`
	m := parseEnv(content)
	if m["QUOTED"] != "quoted value" {
		t.Errorf("double quote strip failed: %q", m["QUOTED"])
	}
	if m["SINGLE"] != "single q" {
		t.Errorf("single quote strip failed: %q", m["SINGLE"])
	}
	if m["PLAIN"] != "plain" {
		t.Errorf("plain failed: %q", m["PLAIN"])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
