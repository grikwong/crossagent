package agent

import "testing"

func TestRecommendAgent_EmptyList(t *testing.T) {
	if got := RecommendAgent("plan", nil); got != "" {
		t.Errorf("expected empty string for empty list, got %q", got)
	}
}

func TestRecommendAgent_Plan(t *testing.T) {
	agents := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
		{Name: "gemini-pro", DisplayName: "Gemini Pro", Adapter: "claude"},
	}
	if got := RecommendAgent("plan", agents); got != "gemini-pro" {
		t.Errorf("plan: want gemini-pro, got %q", got)
	}

	// Without gemini, falls back to claude.
	agents2 := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
	}
	if got := RecommendAgent("plan", agents2); got != "claude" {
		t.Errorf("plan fallback: want claude, got %q", got)
	}

	// Without gemini or claude, falls back to codex.
	agents3 := []Agent{
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
	}
	if got := RecommendAgent("plan", agents3); got != "codex" {
		t.Errorf("plan fallback codex: want codex, got %q", got)
	}

	// None of the preferred substrings: first alphabetical.
	agents4 := []Agent{
		{Name: "zeta", DisplayName: "Z", Adapter: "claude"},
		{Name: "alpha", DisplayName: "A", Adapter: "claude"},
	}
	if got := RecommendAgent("plan", agents4); got != "alpha" {
		t.Errorf("plan no-match: want alpha (first sorted), got %q", got)
	}
}

func TestRecommendAgent_Review(t *testing.T) {
	agents := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
		{Name: "gpt-5", DisplayName: "GPT-5", Adapter: "codex"},
	}
	// codex/gpt tier — codex comes first alphabetically.
	if got := RecommendAgent("review", agents); got != "codex" {
		t.Errorf("review: want codex, got %q", got)
	}

	// gpt-only satisfies the first tier.
	agents2 := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
		{Name: "gpt-5", DisplayName: "GPT-5", Adapter: "codex"},
	}
	if got := RecommendAgent("review", agents2); got != "gpt-5" {
		t.Errorf("review gpt: want gpt-5, got %q", got)
	}

	// Fall back to claude when no codex/gpt present.
	agents3 := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
	}
	if got := RecommendAgent("review", agents3); got != "claude" {
		t.Errorf("review fallback: want claude, got %q", got)
	}
}

func TestRecommendAgent_Implement(t *testing.T) {
	agents := []Agent{
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
		{Name: "gemini-pro", DisplayName: "Gemini Pro", Adapter: "claude"},
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
	}
	if got := RecommendAgent("implement", agents); got != "claude" {
		t.Errorf("implement: want claude, got %q", got)
	}

	// Without claude, falls back to codex.
	agents2 := []Agent{
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
	}
	if got := RecommendAgent("implement", agents2); got != "codex" {
		t.Errorf("implement fallback: want codex, got %q", got)
	}
}

func TestRecommendAgent_Verify(t *testing.T) {
	// Verify should prefer gemini, then codex, then claude — preserving
	// the existing default mapping (codex over claude) when no gemini.
	agents := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
	}
	if got := RecommendAgent("verify", agents); got != "codex" {
		t.Errorf("verify builtins: want codex (preserves default), got %q", got)
	}

	agents2 := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
		{Name: "gemini-pro", DisplayName: "Gemini Pro", Adapter: "claude"},
	}
	if got := RecommendAgent("verify", agents2); got != "gemini-pro" {
		t.Errorf("verify with gemini: want gemini-pro, got %q", got)
	}

	// Claude-only fallback.
	agents3 := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
	}
	if got := RecommendAgent("verify", agents3); got != "claude" {
		t.Errorf("verify claude-only: want claude, got %q", got)
	}
}

func TestRecommendAgent_UnknownPhase(t *testing.T) {
	agents := []Agent{
		{Name: "codex", DisplayName: "OpenAI Codex"},
		{Name: "claude", DisplayName: "Claude Code"},
	}
	// Sorted alphabetically, claude is first.
	if got := RecommendAgent("bogus", agents); got != "claude" {
		t.Errorf("unknown phase: want first sorted (claude), got %q", got)
	}
}

func TestRecommendAgent_DisplayNameMatches(t *testing.T) {
	// Agent name doesn't contain the keyword but DisplayName does.
	agents := []Agent{
		{Name: "claude", DisplayName: "Claude Code", Adapter: "claude"},
		{Name: "codex", DisplayName: "OpenAI Codex", Adapter: "codex"},
		{Name: "google-model", DisplayName: "Gemini 2.0", Adapter: "claude"},
	}
	if got := RecommendAgent("plan", agents); got != "google-model" {
		t.Errorf("plan via displayname: want google-model, got %q", got)
	}
}

func TestRecommendAgent_TieBreakAlphabetical(t *testing.T) {
	// Multiple gemini matches — should pick first alphabetically.
	agents := []Agent{
		{Name: "gemini-pro", DisplayName: "Gemini Pro", Adapter: "claude"},
		{Name: "gemini-flash", DisplayName: "Gemini Flash", Adapter: "claude"},
	}
	if got := RecommendAgent("plan", agents); got != "gemini-flash" {
		t.Errorf("tie break: want gemini-flash (alphabetical), got %q", got)
	}
}
