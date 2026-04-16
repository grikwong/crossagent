package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
func (claudeAdapter) Family() string         { return "anthropic" }

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
	formatPath := func(p string) string {
		p = filepath.ToSlash(p)
		if !strings.HasPrefix(p, "/") {
			return "/" + p
		}
		return p
	}

	var allowWrite []string
	if ctx.PhaseKey == "implement" {
		// Fail closed: implement writes are pinned to the reviewed file
		// list (union of plan.md + review.md affected-files sections).
		// Empty means no repo writes are authorized — we deliberately do
		// NOT fall back to the repo root, because that would restore
		// blanket repo-wide writes and defeat the sandbox.
		// Defensive assertion: judge.ExtractAffectedFiles is the
		// canonicalization boundary. Re-check so we fail closed if a
		// future caller populates AffectedFiles from a different source.
		for _, f := range ctx.AffectedFiles {
			if _, ok := assertUnderRepo(ctx.Repo, f); !ok {
				continue
			}
			allowWrite = append(allowWrite, formatPath(f))
		}
		// Always allow writing the implement.md report in the workflow dir.
		allowWrite = append(allowWrite, formatPath(filepath.Join(ctx.WorkflowDir, "implement.md")))
	} else {
		// Default: allow writing to repo and all workspace dirs
		allowWrite = []string{formatPath(ctx.Repo)}
	}

	for _, d := range ctx.AllDirs {
		allowWrite = append(allowWrite, formatPath(d))
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
