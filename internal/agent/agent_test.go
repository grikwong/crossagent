package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestHome creates a temporary CROSSAGENT_HOME and returns a cleanup function.
func setupTestHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CROSSAGENT_HOME", dir)

	// Create required directories.
	os.MkdirAll(filepath.Join(dir, "agents"), 0755)
	os.MkdirAll(filepath.Join(dir, "workflows"), 0755)
	os.MkdirAll(filepath.Join(dir, "projects", "default", "memory"), 0755)

	return dir
}

func TestGetAgentBuiltins(t *testing.T) {
	setupTestHome(t)

	tests := []struct {
		name        string
		adapter     string
		command     string
		displayName string
	}{
		{"claude", "claude", "claude", "Claude Code"},
		{"codex", "codex", "codex", "OpenAI Codex"},
	}

	for _, tt := range tests {
		a, err := GetAgent(tt.name)
		if err != nil {
			t.Fatalf("GetAgent(%q): %v", tt.name, err)
		}
		if a.Adapter != tt.adapter {
			t.Errorf("GetAgent(%q).Adapter = %q, want %q", tt.name, a.Adapter, tt.adapter)
		}
		if a.Command != tt.command {
			t.Errorf("GetAgent(%q).Command = %q, want %q", tt.name, a.Command, tt.command)
		}
		if a.DisplayName != tt.displayName {
			t.Errorf("GetAgent(%q).DisplayName = %q, want %q", tt.name, a.DisplayName, tt.displayName)
		}
		if !a.Builtin {
			t.Errorf("GetAgent(%q).Builtin should be true", tt.name)
		}
	}
}

func TestGetAgentCustom(t *testing.T) {
	home := setupTestHome(t)

	// Create a custom agent file.
	agentContent := "adapter=claude\ncommand=my-claude\ndisplay_name=My Claude\n"
	if err := os.WriteFile(filepath.Join(home, "agents", "my-agent"), []byte(agentContent), 0644); err != nil {
		t.Fatal(err)
	}

	a, err := GetAgent("my-agent")
	if err != nil {
		t.Fatalf("GetAgent(my-agent): %v", err)
	}
	if a.Name != "my-agent" {
		t.Errorf("Name = %q, want %q", a.Name, "my-agent")
	}
	if a.Adapter != "claude" {
		t.Errorf("Adapter = %q, want %q", a.Adapter, "claude")
	}
	if a.Command != "my-claude" {
		t.Errorf("Command = %q, want %q", a.Command, "my-claude")
	}
	if a.DisplayName != "My Claude" {
		t.Errorf("DisplayName = %q, want %q", a.DisplayName, "My Claude")
	}
	if a.Builtin {
		t.Error("custom agent should not be Builtin")
	}
}

func TestGetAgentNotFound(t *testing.T) {
	setupTestHome(t)

	_, err := GetAgent("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestListAgents(t *testing.T) {
	home := setupTestHome(t)

	// Add two custom agents.
	os.WriteFile(filepath.Join(home, "agents", "zebra"), []byte("adapter=codex\ncommand=zeb\ndisplay_name=Zebra\n"), 0644)
	os.WriteFile(filepath.Join(home, "agents", "alpha"), []byte("adapter=claude\ncommand=alp\ndisplay_name=Alpha\n"), 0644)

	agents, err := ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}

	// Builtins first (in adapter registration order), then custom agents
	// sorted alphabetically. Derived from the registry so adding an
	// adapter doesn't require a hand-updated expected count.
	builtins := AdapterNames()
	expected := append(append([]string(nil), builtins...), "alpha", "zebra")
	if len(agents) != len(expected) {
		t.Fatalf("expected %d agents, got %d", len(expected), len(agents))
	}
	for i, name := range expected {
		if agents[i].Name != name {
			t.Errorf("agents[%d].Name = %q, want %q", i, agents[i].Name, name)
		}
	}
}

func TestListAgentsNoCustom(t *testing.T) {
	setupTestHome(t)

	agents, err := ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	// Expected count is derived from the registry (one builtin per
	// registered adapter) so this test doesn't need a manual bump when
	// a new adapter is added.
	if want := len(AdapterNames()); len(agents) != want {
		t.Fatalf("expected %d builtins, got %d", want, len(agents))
	}
}

func TestAddAgent(t *testing.T) {
	home := setupTestHome(t)

	err := AddAgent("my-tool", "claude", "my-cmd", "My Tool")
	if err != nil {
		t.Fatalf("AddAgent: %v", err)
	}

	// Verify the file was created with correct content.
	data, err := os.ReadFile(filepath.Join(home, "agents", "my-tool"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if content != "adapter=claude\ncommand=my-cmd\ndisplay_name=My Tool\n" {
		t.Errorf("unexpected agent file content: %q", content)
	}

	// Verify we can read it back.
	a, err := GetAgent("my-tool")
	if err != nil {
		t.Fatalf("GetAgent after AddAgent: %v", err)
	}
	if a.Adapter != "claude" || a.Command != "my-cmd" || a.DisplayName != "My Tool" {
		t.Errorf("agent properties don't match: %+v", a)
	}
}

func TestAddAgentDefaults(t *testing.T) {
	setupTestHome(t)

	// Empty command defaults to adapter, empty display defaults to name.
	err := AddAgent("test-agent", "codex", "", "")
	if err != nil {
		t.Fatalf("AddAgent: %v", err)
	}

	a, err := GetAgent("test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if a.Command != "codex" {
		t.Errorf("Command should default to adapter, got %q", a.Command)
	}
	if a.DisplayName != "test-agent" {
		t.Errorf("DisplayName should default to name, got %q", a.DisplayName)
	}
}

func TestAddAgentRejectsBuiltin(t *testing.T) {
	setupTestHome(t)

	err := AddAgent("claude", "claude", "claude", "Claude")
	if err == nil {
		t.Fatal("expected error when overwriting builtin")
	}
}

func TestAddAgentRejectsInvalidAdapter(t *testing.T) {
	setupTestHome(t)

	err := AddAgent("test", "invalid", "cmd", "Test")
	if err == nil {
		t.Fatal("expected error for invalid adapter")
	}
}

func TestAddAgentRejectsDuplicate(t *testing.T) {
	setupTestHome(t)

	if err := AddAgent("dup", "claude", "cmd", "Dup"); err != nil {
		t.Fatal(err)
	}
	err := AddAgent("dup", "claude", "cmd", "Dup")
	if err == nil {
		t.Fatal("expected error for duplicate agent")
	}
}

func TestAddAgentRejectsInvalidName(t *testing.T) {
	setupTestHome(t)

	err := AddAgent("invalid name!", "claude", "cmd", "Test")
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}

func TestRemoveAgent(t *testing.T) {
	home := setupTestHome(t)

	// Create a custom agent.
	os.WriteFile(filepath.Join(home, "agents", "removable"), []byte("adapter=claude\ncommand=x\ndisplay_name=X\n"), 0644)

	err := RemoveAgent("removable")
	if err != nil {
		t.Fatalf("RemoveAgent: %v", err)
	}

	// Verify it's gone.
	_, err = GetAgent("removable")
	if err == nil {
		t.Fatal("expected agent to be removed")
	}
}

func TestRemoveAgentRejectsBuiltin(t *testing.T) {
	setupTestHome(t)

	err := RemoveAgent("claude")
	if err == nil {
		t.Fatal("expected error when removing builtin")
	}
}

func TestRemoveAgentNotFound(t *testing.T) {
	setupTestHome(t)

	err := RemoveAgent("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestDefaultPhaseAgent(t *testing.T) {
	tests := []struct {
		phase string
		want  string
	}{
		{"plan", "claude"},
		{"1", "claude"},
		{"review", "codex"},
		{"2", "codex"},
		{"implement", "claude"},
		{"3", "claude"},
		{"verify", "codex"},
		{"4", "codex"},
	}

	for _, tt := range tests {
		got := DefaultPhaseAgent(tt.phase)
		if got != tt.want {
			t.Errorf("DefaultPhaseAgent(%q) = %q, want %q", tt.phase, got, tt.want)
		}
	}
}

func TestPhaseAgentConfKey(t *testing.T) {
	tests := []struct {
		phase string
		want  string
		err   bool
	}{
		{"plan", "plan_agent", false},
		{"1", "plan_agent", false},
		{"review", "review_agent", false},
		{"2", "review_agent", false},
		{"implement", "implement_agent", false},
		{"3", "implement_agent", false},
		{"verify", "verify_agent", false},
		{"4", "verify_agent", false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		got, err := PhaseAgentConfKey(tt.phase)
		if tt.err {
			if err == nil {
				t.Errorf("PhaseAgentConfKey(%q): expected error", tt.phase)
			}
			continue
		}
		if err != nil {
			t.Errorf("PhaseAgentConfKey(%q): %v", tt.phase, err)
			continue
		}
		if got != tt.want {
			t.Errorf("PhaseAgentConfKey(%q) = %q, want %q", tt.phase, got, tt.want)
		}
	}
}

func TestGetPhaseAgentDefault(t *testing.T) {
	setupTestHome(t)

	// Create a workflow dir with no agent assignments.
	wfDir := t.TempDir()

	a, err := GetPhaseAgent(wfDir, "plan")
	if err != nil {
		t.Fatalf("GetPhaseAgent: %v", err)
	}
	if a.Name != "claude" {
		t.Errorf("expected default plan agent 'claude', got %q", a.Name)
	}

	a, err = GetPhaseAgent(wfDir, "review")
	if err != nil {
		t.Fatalf("GetPhaseAgent: %v", err)
	}
	if a.Name != "codex" {
		t.Errorf("expected default review agent 'codex', got %q", a.Name)
	}
}

func TestGetPhaseAgentConfigured(t *testing.T) {
	home := setupTestHome(t)

	// Create a custom agent.
	os.WriteFile(filepath.Join(home, "agents", "custom"), []byte("adapter=codex\ncommand=custom-cmd\ndisplay_name=Custom\n"), 0644)

	// Create a workflow dir with a custom assignment.
	wfDir := t.TempDir()
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("plan_agent=custom\n"), 0644)

	a, err := GetPhaseAgent(wfDir, "plan")
	if err != nil {
		t.Fatalf("GetPhaseAgent: %v", err)
	}
	if a.Name != "custom" {
		t.Errorf("expected configured agent 'custom', got %q", a.Name)
	}
}

func TestSetPhaseAgent(t *testing.T) {
	setupTestHome(t)

	wfDir := t.TempDir()

	// Set plan agent to codex.
	if err := SetPhaseAgent(wfDir, "plan", "codex"); err != nil {
		t.Fatalf("SetPhaseAgent: %v", err)
	}

	a, err := GetPhaseAgent(wfDir, "plan")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "codex" {
		t.Errorf("expected 'codex', got %q", a.Name)
	}
}

func TestSetPhaseAgentRejectsUnknown(t *testing.T) {
	setupTestHome(t)

	wfDir := t.TempDir()

	err := SetPhaseAgent(wfDir, "plan", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestResetPhaseAgent(t *testing.T) {
	setupTestHome(t)

	wfDir := t.TempDir()

	// Set to non-default, then reset.
	SetPhaseAgent(wfDir, "plan", "codex")
	if err := ResetPhaseAgent(wfDir, "plan"); err != nil {
		t.Fatalf("ResetPhaseAgent: %v", err)
	}

	a, err := GetPhaseAgent(wfDir, "plan")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "claude" {
		t.Errorf("expected default 'claude' after reset, got %q", a.Name)
	}
}

func TestSetPhaseAgentRoundTrip(t *testing.T) {
	setupTestHome(t)

	wfDir := t.TempDir()

	phases := []string{"plan", "review", "implement", "verify"}
	agents := []string{"codex", "claude", "codex", "claude"}

	for i, phase := range phases {
		if err := SetPhaseAgent(wfDir, phase, agents[i]); err != nil {
			t.Fatalf("SetPhaseAgent(%q, %q): %v", phase, agents[i], err)
		}
	}

	for i, phase := range phases {
		a, err := GetPhaseAgent(wfDir, phase)
		if err != nil {
			t.Fatalf("GetPhaseAgent(%q): %v", phase, err)
		}
		if a.Name != agents[i] {
			t.Errorf("GetPhaseAgent(%q) = %q, want %q", phase, a.Name, agents[i])
		}
	}
}
