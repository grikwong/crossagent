package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// claudeAdapter implements Adapter for the Claude Code CLI.
//
// Launch shape:
//
//	claude --permission-mode auto --settings <sandbox-json>
//	       --add-dir <workflow> --add-dir <global-memory> ...
//	       -- "Read and follow the instructions at <promptFile>"
//
// The sandbox is enforced via a generated JSON settings file whose
// allowWrite list is pinned to {workflowDir, repo, memory dirs,
// add_dirs}. --permission-mode auto auto-approves tool calls.
type claudeAdapter struct{}

func (claudeAdapter) Name() string           { return "claude" }
func (claudeAdapter) DisplayName() string    { return "Claude Code" }
func (claudeAdapter) DefaultCommand() string { return "claude" }

func (claudeAdapter) Plan(ctx *LaunchContext) (*LaunchPlan, error) {
	settingsFile, err := writeClaudeSandboxSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("claude: generate sandbox settings: %w", err)
	}

	prompt := "Read and follow the instructions at " + ctx.PromptFile

	args := []string{"--permission-mode", "auto", "--settings", settingsFile}
	for _, d := range ctx.AllDirs {
		args = append(args, "--add-dir", d)
	}
	args = append(args, "--", prompt)

	return &LaunchPlan{
		Args:    args,
		Prompt:  prompt,
		WorkDir: ctx.Repo,
	}, nil
}

// writeClaudeSandboxSettings writes claude's sandbox settings JSON into
// the workflow's prompts/ dir and returns the path. The allowWrite list
// is pinned to the repo plus every directory in ctx.AllDirs, so writes
// outside this explicit set are refused by claude's sandbox.
func writeClaudeSandboxSettings(ctx *LaunchContext) (string, error) {
	settingsFile := filepath.Join(ctx.WorkflowDir, "prompts", ".sandbox-settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0755); err != nil {
		return "", err
	}

	// Claude's sandbox uses a "/<path>" convention where the leading
	// slash is part of the token rather than a filesystem root.
	allowWrite := []string{"/" + ctx.Repo}
	for _, d := range ctx.AllDirs {
		allowWrite = append(allowWrite, "/"+d)
	}

	settings := map[string]any{
		"sandbox": map[string]any{
			"enabled": true,
			"filesystem": map[string]any{
				"allowWrite": allowWrite,
			},
		},
		"permissions": map[string]any{
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

func init() { RegisterAdapter(claudeAdapter{}) }
