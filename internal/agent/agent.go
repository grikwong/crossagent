package agent

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grikwong/crossagent/internal/state"
)

// Agent represents a registered agent (builtin or custom).
type Agent struct {
	Name        string
	Adapter     string // "claude", "codex", or "gemini"
	Command     string // CLI command to invoke
	DisplayName string
	Builtin     bool
}

// validAdapters are the supported adapter types.
var validAdapters = map[string]bool{
	"claude": true,
	"codex":  true,
	"gemini": true,
}

// builtinAgents are the hardcoded builtin agents.
var builtinAgents = map[string]*Agent{
	"claude": {
		Name:        "claude",
		Adapter:     "claude",
		Command:     "claude",
		DisplayName: "Claude Code",
		Builtin:     true,
	},
	"codex": {
		Name:        "codex",
		Adapter:     "codex",
		Command:     "codex",
		DisplayName: "OpenAI Codex",
		Builtin:     true,
	},
	"gemini": {
		Name:        "gemini",
		Adapter:     "gemini",
		Command:     "gemini",
		DisplayName: "Google Gemini",
		Builtin:     true,
	},
}

// GetAgent returns the agent with the given name.
// Checks builtins first, then custom agents from ~/.crossagent/agents/<name>.
func GetAgent(name string) (*Agent, error) {
	if a, ok := builtinAgents[name]; ok {
		// Return a copy to prevent mutation of the global.
		copy := *a
		return &copy, nil
	}

	agentFile := filepath.Join(state.AgentsDir(), name)
	f, err := os.Open(agentFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("agent not found: %s", name)
		}
		return nil, err
	}
	defer f.Close()

	agent := &Agent{Name: name}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := line[idx+1:]
		switch key {
		case "adapter":
			agent.Adapter = val
		case "command":
			agent.Command = val
		case "display_name":
			agent.DisplayName = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return agent, nil
}

// ListAgents returns all agents (builtins first, then custom), sorted by name.
func ListAgents() ([]Agent, error) {
	// Start with builtins in fixed order.
	agents := []Agent{
		*builtinAgents["claude"],
		*builtinAgents["codex"],
		*builtinAgents["gemini"],
	}

	// Read custom agents from agents directory.
	agentsDir := state.AgentsDir()
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return agents, nil
		}
		return nil, err
	}

	var custom []Agent
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		a, err := GetAgent(e.Name())
		if err != nil {
			continue
		}
		custom = append(custom, *a)
	}

	sort.Slice(custom, func(i, j int) bool {
		return custom[i].Name < custom[j].Name
	})

	agents = append(agents, custom...)
	return agents, nil
}

// AddAgent creates a new custom agent.
func AddAgent(name, adapter, command, displayName string) error {
	if err := state.ValidateName(name); err != nil {
		return err
	}
	if _, ok := builtinAgents[name]; ok {
		return fmt.Errorf("cannot overwrite builtin agent '%s'", name)
	}
	if !validAdapters[adapter] {
		return fmt.Errorf("agent adapter must be one of: claude, codex, gemini")
	}

	agentFile := filepath.Join(state.AgentsDir(), name)
	if _, err := os.Stat(agentFile); err == nil {
		return fmt.Errorf("agent already exists: %s", name)
	}

	// Default command to adapter name if not specified.
	if command == "" {
		command = adapter
	}
	// Default display name to agent name if not specified.
	if displayName == "" {
		displayName = name
	}

	content := fmt.Sprintf("adapter=%s\ncommand=%s\ndisplay_name=%s\n", adapter, command, displayName)
	return os.WriteFile(agentFile, []byte(content), 0644)
}

// RemoveAgent deletes a custom agent. Refuses to remove builtins.
func RemoveAgent(name string) error {
	if _, ok := builtinAgents[name]; ok {
		return fmt.Errorf("cannot remove builtin agent '%s'", name)
	}

	agentFile := filepath.Join(state.AgentsDir(), name)
	if _, err := os.Stat(agentFile); os.IsNotExist(err) {
		return fmt.Errorf("agent not found: %s", name)
	}

	return os.Remove(agentFile)
}

// DefaultPhaseAgent returns the default agent name for a phase.
// Matches bash: plan/implement→claude, review/verify→codex.
func DefaultPhaseAgent(phase string) string {
	key, err := state.PhaseKey(phase)
	if err != nil {
		return "claude"
	}
	switch key {
	case "plan", "implement":
		return "claude"
	case "review", "verify":
		return "codex"
	default:
		return "claude"
	}
}

// PhaseAgentConfKey returns the config key for a phase's agent assignment.
// E.g., "plan" → "plan_agent", "review" → "review_agent".
func PhaseAgentConfKey(phase string) (string, error) {
	key, err := state.PhaseKey(phase)
	if err != nil {
		return "", err
	}
	return key + "_agent", nil
}

// GetPhaseAgent returns the agent assigned to a phase for a workflow.
// Falls back to DefaultPhaseAgent if no assignment is configured.
func GetPhaseAgent(wfDir, phase string) (*Agent, error) {
	confKey, err := PhaseAgentConfKey(phase)
	if err != nil {
		return nil, err
	}

	value, err := state.GetConf(wfDir, confKey)
	if err != nil {
		return nil, err
	}

	agentName := value
	if agentName == "" {
		agentName = DefaultPhaseAgent(phase)
	}

	return GetAgent(agentName)
}

// SetPhaseAgent assigns an agent to a phase for a workflow.
func SetPhaseAgent(wfDir, phase, agentName string) error {
	// Validate the agent exists.
	if _, err := GetAgent(agentName); err != nil {
		return err
	}

	confKey, err := PhaseAgentConfKey(phase)
	if err != nil {
		return err
	}

	return state.SetConf(wfDir, confKey, agentName)
}

// ResetPhaseAgent resets a phase's agent to the default.
func ResetPhaseAgent(wfDir, phase string) error {
	return SetPhaseAgent(wfDir, phase, DefaultPhaseAgent(phase))
}
