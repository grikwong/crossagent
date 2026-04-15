package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grikwong/crossagent/internal/prompt"
	"github.com/grikwong/crossagent/internal/state"
)

// ANSI formatting constants for error messages.
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

// BuildLaunchArgs returns the repeated "--add-dir <dir>" pairs expected
// by claude/codex's native workspace-dir flag syntax. Retained as a
// package-level helper because external callers and tests rely on it;
// new adapter code should consume LaunchContext.AllDirs directly.
func BuildLaunchArgs(wfDir string, addDirs []string, projectMemDir string) []string {
	ctx := NewLaunchContext("", wfDir, "", addDirs, projectMemDir)
	args := make([]string, 0, 2*len(ctx.AllDirs))
	for _, d := range ctx.AllDirs {
		args = append(args, "--add-dir", d)
	}
	return args
}

// GenSandboxSettings is retained for backward compatibility. New code
// should go through claudeAdapter.Plan, which writes the same file.
func GenSandboxSettings(wfDir, repo string, addDirs []string, projectMemDir string) (string, error) {
	ctx := NewLaunchContext(repo, wfDir, "", addDirs, projectMemDir)
	return writeClaudeSandboxSettings(ctx)
}

// extractAddDirs returns the directory values from a launchArgs slice
// of alternating "--add-dir" flags and paths. Used by the legacy
// spawn-args builders that take launchArgs rather than LaunchContext.
func extractAddDirs(launchArgs []string) []string {
	var dirs []string
	for i := 0; i+1 < len(launchArgs); i += 2 {
		if launchArgs[i] == "--add-dir" {
			dirs = append(dirs, launchArgs[i+1])
		}
	}
	return dirs
}

// RequireAgentCommand checks that the agent's command exists on PATH
// or is an executable file.
func RequireAgentCommand(agent *Agent) error {
	if agent.Command == "" {
		return fmt.Errorf("agent '%s' has no command configured", agent.Name)
	}
	if strings.Contains(agent.Command, "/") {
		info, err := os.Stat(agent.Command)
		if err != nil {
			return fmt.Errorf("agent '%s' command not found: %s", agent.Name, agent.Command)
		}
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("agent '%s' command is not executable: %s", agent.Name, agent.Command)
		}
		return nil
	}
	if _, err := exec.LookPath(agent.Command); err != nil {
		return fmt.Errorf("agent '%s' command not found on PATH: %s", agent.Name, agent.Command)
	}
	return nil
}

// LaunchAgent launches the given agent synchronously via its adapter.
// All adapter-specific launch shaping (argv, sandbox side effects,
// prompt construction) lives in the Adapter implementation — this
// function only knows how to call Plan() and exec the result.
func LaunchAgent(agent *Agent, repo, promptFile, wfDir string, addDirs []string, projectMemDir string) error {
	if err := RequireAgentCommand(agent); err != nil {
		return err
	}

	ad, ok := AdapterFor(agent.Adapter)
	if !ok {
		return fmt.Errorf("unsupported agent adapter '%s' for agent '%s'", agent.Adapter, agent.Name)
	}

	ctx := NewLaunchContext(repo, wfDir, promptFile, addDirs, projectMemDir)
	plan, err := ad.Plan(ctx)
	if err != nil {
		return err
	}

	cmd := exec.Command(agent.Command, plan.Args...)
	if plan.WorkDir != "" {
		cmd.Dir = plan.WorkDir
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// BuildPhaseCmd builds the launch parameters without executing, for
// phase-cmd --json. The adapter-specific shaping is delegated to the
// registered Adapter; this function only assembles the workflow-level
// context (phase precondition checks, prompt file generation, config).
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

	if phaseKey == "implement" && implPhase < 1 {
		return nil, fmt.Errorf("implementation phase must be a positive integer")
	}

	targetPhase, outputFile, err := resolveTargetPhase(wfDir, phaseKey, pn, force)
	if err != nil {
		return nil, err
	}

	ag, err := GetPhaseAgent(wfDir, fmt.Sprintf("%d", targetPhase))
	if err != nil {
		return nil, err
	}

	ad, ok := AdapterFor(ag.Adapter)
	if !ok {
		return nil, fmt.Errorf("unsupported agent adapter '%s' for agent '%s'", ag.Adapter, ag.Name)
	}

	promptFile, err := generatePhasePrompt(wfDir, phaseKey, cfg, implPhase)
	if err != nil {
		return nil, fmt.Errorf("failed to generate %s prompt: %w", phaseKey, err)
	}

	projectMemDir := ""
	if cfg.Project != "" {
		pmDir := state.ProjectMemoryDir(cfg.Project)
		if info, err := os.Stat(pmDir); err == nil && info.IsDir() {
			projectMemDir = pmDir
		}
	}

	ctx := NewLaunchContext(cfg.Repo, wfDir, promptFile, cfg.AddDirs, projectMemDir)
	plan, err := ad.Plan(ctx)
	if err != nil {
		return nil, err
	}

	phaseID := fmt.Sprintf("%d", targetPhase)
	return &PhaseCmdResult{
		Agent: PhaseCmdAgent{
			Name:        ag.Name,
			DisplayName: ag.DisplayName,
			Adapter:     ag.Adapter,
		},
		Command:     ag.Command,
		Args:        plan.Args,
		Cwd:         cfg.Repo,
		Prompt:      plan.Prompt,
		PromptFile:  promptFile,
		OutputFile:  &outputFile,
		Phase:       targetPhase,
		PhaseLabel:  state.PhaseLabel(phaseID),
		Workflow:    wfName,
		WorkflowDir: wfDir,
	}, nil
}

// resolveTargetPhase validates phase preconditions and returns the
// target phase number + output file path. Extracted from BuildPhaseCmd
// to keep the main flow linear.
func resolveTargetPhase(wfDir, phaseKey string, pn int, force bool) (int, string, error) {
	switch phaseKey {
	case "plan":
		if pn > 1 && !force {
			return 0, "", fmt.Errorf("plan phase already completed, use --force to re-run")
		}
		return 1, filepath.Join(wfDir, "plan.md"), nil
	case "review":
		if pn < 2 {
			return 0, "", fmt.Errorf("complete Phase 1 first, run: %scrossagent plan%s", ansiBold, ansiReset)
		}
		if pn > 2 && !force {
			return 0, "", fmt.Errorf("review phase already completed, use --force to re-run")
		}
		if _, err := os.Stat(filepath.Join(wfDir, "plan.md")); os.IsNotExist(err) {
			return 0, "", fmt.Errorf("plan file missing: %s/plan.md", wfDir)
		}
		return 2, filepath.Join(wfDir, "review.md"), nil
	case "implement":
		if pn < 3 {
			return 0, "", fmt.Errorf("complete Phase 2 first, run: %scrossagent review%s", ansiBold, ansiReset)
		}
		if pn > 4 && !force {
			return 0, "", fmt.Errorf("workflow is complete, use --force to re-run implementation")
		}
		return 3, filepath.Join(wfDir, "implement.md"), nil
	case "verify":
		if pn < 4 {
			return 0, "", fmt.Errorf("complete Phase 3 first, run: %scrossagent implement%s", ansiBold, ansiReset)
		}
		if pn > 4 && !force {
			return 0, "", fmt.Errorf("workflow is complete, use --force to re-run verification")
		}
		return 4, filepath.Join(wfDir, "verify.md"), nil
	}
	return 0, "", fmt.Errorf("unknown phase: %s", phaseKey)
}

// generatePhasePrompt calls the appropriate prompt generation function for the given phase.
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

