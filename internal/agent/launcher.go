package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pvotal-tech/crossagent/internal/prompt"
	"github.com/pvotal-tech/crossagent/internal/state"
)

// ANSI formatting constants for error messages (matching bash ${BOLD} and ${NC}).
const (
	ansiBold  = "\033[1m"
	ansiReset = "\033[0m"
)

// PhaseCmdResult holds the launch parameters for a phase command.
// Matches the JSON schema consumed by the Web UI.
type PhaseCmdResult struct {
	Agent       PhaseCmdAgent `json:"agent"`
	Command     string        `json:"command"`
	Args        []string      `json:"args"`
	Cwd         string        `json:"cwd"`
	Prompt      string        `json:"prompt"`
	PromptFile  string        `json:"prompt_file"`
	OutputFile  *string       `json:"output_file"` // null when absent
	Phase       int           `json:"phase"`
	PhaseLabel  string        `json:"phase_label"`
	Workflow    string        `json:"workflow"`
	WorkflowDir string        `json:"workflow_dir"`
}

// PhaseCmdAgent is the nested agent object in phase-cmd JSON.
type PhaseCmdAgent struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Adapter     string `json:"adapter"`
}

// BuildLaunchArgs constructs --add-dir flags matching bash _build_launch_args().
// Returns the slice of arguments (e.g., --add-dir <wfDir> --add-dir <globalMemDir> ...).
func BuildLaunchArgs(wfDir string, addDirs []string, projectMemDir string) []string {
	args := []string{"--add-dir", wfDir, "--add-dir", state.GlobalMemoryDir()}

	if projectMemDir != "" {
		if info, err := os.Stat(projectMemDir); err == nil && info.IsDir() {
			args = append(args, "--add-dir", projectMemDir)
		}
	}

	for _, d := range addDirs {
		if d != "" {
			args = append(args, "--add-dir", d)
		}
	}

	return args
}

// GenSandboxSettings generates .sandbox-settings.json matching bash _gen_sandbox_settings().
// Returns the path to the written settings file.
func GenSandboxSettings(wfDir, repo string, addDirs []string, projectMemDir string) (string, error) {
	settingsFile := filepath.Join(wfDir, "prompts", ".sandbox-settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0755); err != nil {
		return "", err
	}

	// Build allowWrite paths with // prefix convention (matching bash /${path} where path starts with /)
	allowWrite := []string{
		"/" + wfDir,
		"/" + repo,
		"/" + state.GlobalMemoryDir(),
	}

	if projectMemDir != "" {
		if info, err := os.Stat(projectMemDir); err == nil && info.IsDir() {
			allowWrite = append(allowWrite, "/"+projectMemDir)
		}
	}

	for _, d := range addDirs {
		if d != "" {
			allowWrite = append(allowWrite, "/"+d)
		}
	}

	settings := map[string]interface{}{
		"sandbox": map[string]interface{}{
			"enabled": true,
			"filesystem": map[string]interface{}{
				"allowWrite": allowWrite,
			},
		},
		"permissions": map[string]interface{}{
			"allow": []string{
				"Bash(*)",
				"Read(*)",
				"Edit(*)",
				"Write(*)",
				"Glob(*)",
				"Grep(*)",
			},
		},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(settingsFile, append(data, '\n'), 0644); err != nil {
		return "", err
	}

	return settingsFile, nil
}

// RequireAgentCommand checks that the agent's command exists on PATH or is an executable.
// Matches bash require_agent_command().
func RequireAgentCommand(agent *Agent) error {
	if agent.Command == "" {
		return fmt.Errorf("agent '%s' has no command configured", agent.Name)
	}

	if strings.Contains(agent.Command, "/") {
		// Absolute or relative path — check if executable
		info, err := os.Stat(agent.Command)
		if err != nil {
			return fmt.Errorf("agent '%s' command not found: %s", agent.Name, agent.Command)
		}
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("agent '%s' command is not executable: %s", agent.Name, agent.Command)
		}
		return nil
	}

	// PATH-based command
	if _, err := exec.LookPath(agent.Command); err != nil {
		return fmt.Errorf("agent '%s' command not found on PATH: %s", agent.Name, agent.Command)
	}
	return nil
}

// LaunchAgent dispatches to the Claude or Codex adapter, matching bash launch_agent().
// Returns the error from the process (does NOT swallow it with || true like bash).
func LaunchAgent(agent *Agent, repo, promptFile, wfDir string, addDirs []string, projectMemDir string) error {
	if err := RequireAgentCommand(agent); err != nil {
		return err
	}

	launchArgs := BuildLaunchArgs(wfDir, addDirs, projectMemDir)

	switch agent.Adapter {
	case "claude":
		settingsFile, err := GenSandboxSettings(wfDir, repo, addDirs, projectMemDir)
		if err != nil {
			return fmt.Errorf("failed to generate sandbox settings: %w", err)
		}

		prompt := "Read and follow the instructions at " + promptFile

		args := []string{"--permission-mode", "auto", "--settings", settingsFile}
		args = append(args, launchArgs...)
		args = append(args, "--", prompt)

		cmd := exec.Command(agent.Command, args...)
		cmd.Dir = repo
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "codex":
		// Codex needs full prompt content inline
		prompt, err := buildCodexPrompt(wfDir, promptFile)
		if err != nil {
			return err
		}

		args := []string{"--full-auto", "-C", repo}
		args = append(args, launchArgs...)
		args = append(args, "--", prompt)

		cmd := exec.Command(agent.Command, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		return fmt.Errorf("unsupported agent adapter '%s' for agent '%s'", agent.Adapter, agent.Name)
	}
}

// buildCodexPrompt reads the general and phase prompt files, concatenating them for Codex.
func buildCodexPrompt(wfDir, promptFile string) (string, error) {
	generalFile := filepath.Join(wfDir, "prompts", "general.md")
	promptData, err := os.ReadFile(promptFile)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file: %w", err)
	}

	generalData, err := os.ReadFile(generalFile)
	if err != nil {
		if os.IsNotExist(err) {
			return string(promptData), nil
		}
		return "", fmt.Errorf("failed to read general instructions: %w", err)
	}

	return strings.TrimRight(string(generalData), "\n") + "\n\n---\n\n" + strings.TrimRight(string(promptData), "\n"), nil
}

// BuildPhaseCmd builds the launch parameters without executing, for phase-cmd --json.
// Matches bash cmd_phase_cmd() including precondition checks and prompt generation.
func BuildPhaseCmd(wfDir, wfName, phase string, force bool, implPhase int) (*PhaseCmdResult, error) {
	currentPhase, err := state.GetPhase(wfDir)
	if err != nil {
		return nil, err
	}
	pn := state.PhaseNum(currentPhase)

	cfg, err := state.ReadConfig(wfDir)
	if err != nil {
		return nil, err
	}

	phaseKey, err := state.PhaseKey(phase)
	if err != nil {
		return nil, err
	}

	// Validate implementation sub-phase is a positive integer (matching bash guard)
	if phaseKey == "implement" && implPhase < 1 {
		return nil, fmt.Errorf("implementation phase must be a positive integer")
	}

	var targetPhase int
	var outputFile string

	switch phaseKey {
	case "plan":
		targetPhase = 1
		if pn > 1 && !force {
			return nil, fmt.Errorf("Plan phase already completed. Use --force to re-run.")
		}
		outputFile = filepath.Join(wfDir, "plan.md")

	case "review":
		targetPhase = 2
		if pn < 2 {
			return nil, fmt.Errorf("Complete Phase 1 first. Run: %scrossagent plan%s", ansiBold, ansiReset)
		}
		if pn > 2 && !force {
			return nil, fmt.Errorf("Review phase already completed. Use --force to re-run.")
		}
		if _, err := os.Stat(filepath.Join(wfDir, "plan.md")); os.IsNotExist(err) {
			return nil, fmt.Errorf("Plan file missing: %s/plan.md", wfDir)
		}
		outputFile = filepath.Join(wfDir, "review.md")

	case "implement":
		targetPhase = 3
		if pn < 3 {
			return nil, fmt.Errorf("Complete Phase 2 first. Run: %scrossagent review%s", ansiBold, ansiReset)
		}
		if pn > 4 && !force {
			return nil, fmt.Errorf("Workflow is complete. Use --force to re-run implementation.")
		}
		outputFile = filepath.Join(wfDir, "implement.md")

	case "verify":
		targetPhase = 4
		if pn < 4 {
			return nil, fmt.Errorf("Complete Phase 3 first. Run: %scrossagent implement%s", ansiBold, ansiReset)
		}
		if pn > 4 && !force {
			return nil, fmt.Errorf("Workflow is complete. Use --force to re-run verification.")
		}
		outputFile = filepath.Join(wfDir, "verify.md")
	}

	// Resolve agent
	agent, err := GetPhaseAgent(wfDir, fmt.Sprintf("%d", targetPhase))
	if err != nil {
		return nil, err
	}

	// Generate prompt file (matching bash: gen_*_prompt is called before building JSON).
	// This ensures prompt files are always fresh when phase-cmd --json is called.
	promptFile, err := generatePhasePrompt(wfDir, phaseKey, cfg, implPhase)
	if err != nil {
		return nil, fmt.Errorf("failed to generate %s prompt: %w", phaseKey, err)
	}

	// Resolve project memory directory
	projectMemDir := ""
	if cfg.Project != "" {
		pmDir := state.ProjectMemoryDir(cfg.Project)
		if info, err := os.Stat(pmDir); err == nil && info.IsDir() {
			projectMemDir = pmDir
		}
	}

	// Build prompt text (adapter-dependent).
	// Now that we've generated the prompt, we can always read it.
	var promptText string
	if agent.Adapter == "codex" {
		generalFile := filepath.Join(wfDir, "prompts", "general.md")
		if generalData, err := os.ReadFile(generalFile); err == nil {
			if promptData, err := os.ReadFile(promptFile); err == nil {
				promptText = strings.TrimRight(string(generalData), "\n") + "\n\n---\n\n" + strings.TrimRight(string(promptData), "\n")
			} else {
				promptText = "Read and follow the instructions at " + promptFile
			}
		} else {
			promptText = "Read and follow the instructions at " + promptFile
		}
	} else {
		promptText = "Read and follow the instructions at " + promptFile
	}

	// Build spawn args (adapter-specific flags + launch args)
	launchArgs := BuildLaunchArgs(wfDir, cfg.AddDirs, projectMemDir)
	var spawnArgs []string

	if agent.Adapter == "claude" {
		settingsFile, err := GenSandboxSettings(wfDir, cfg.Repo, cfg.AddDirs, projectMemDir)
		if err != nil {
			return nil, fmt.Errorf("failed to generate sandbox settings: %w", err)
		}
		spawnArgs = append(spawnArgs, "--permission-mode", "auto", "--settings", settingsFile)
	} else if agent.Adapter == "codex" {
		spawnArgs = append(spawnArgs, "--full-auto", "-C", cfg.Repo)
	}
	spawnArgs = append(spawnArgs, launchArgs...)
	spawnArgs = append(spawnArgs, "--", promptText)

	phaseID := fmt.Sprintf("%d", targetPhase)

	result := &PhaseCmdResult{
		Agent: PhaseCmdAgent{
			Name:        agent.Name,
			DisplayName: agent.DisplayName,
			Adapter:     agent.Adapter,
		},
		Command:     agent.Command,
		Args:        spawnArgs,
		Cwd:         cfg.Repo,
		Prompt:      promptText,
		PromptFile:  promptFile,
		OutputFile:  &outputFile,
		Phase:       targetPhase,
		PhaseLabel:  state.PhaseLabel(phaseID),
		Workflow:    wfName,
		WorkflowDir: wfDir,
	}

	return result, nil
}

// generatePhasePrompt calls the appropriate prompt generation function for the given phase.
// This mirrors bash behavior where cmd_phase_cmd() always generates prompts before building JSON.
func generatePhasePrompt(wfDir, phaseKey string, cfg *state.Config, implPhase int) (string, error) {
	switch phaseKey {
	case "plan":
		return prompt.GeneratePlanPrompt(wfDir, cfg)
	case "review":
		return prompt.GenerateReviewPrompt(wfDir, cfg)
	case "implement":
		return prompt.GenerateImplementPrompt(wfDir, cfg, implPhase)
	case "verify":
		return prompt.GenerateVerifyPrompt(wfDir, cfg)
	default:
		return "", fmt.Errorf("unknown phase: %s", phaseKey)
	}
}
