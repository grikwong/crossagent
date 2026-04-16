package agent

import (
	"fmt"
	"strings"
)

// geminiAdapter implements Adapter for the Google Gemini CLI.
//
// Launch shape:
//
//	gemini --yolo --sandbox --include-directories <comma-list>
//	       -p "Read and follow the instructions at <promptFile>"
//
// Gemini's flag shape differs from claude/codex:
//
//   - --yolo auto-approves tool calls.
//   - --sandbox runs the agent under an OS-level sandbox (sandbox-exec
//     on macOS, Docker/Podman on Linux) confining filesystem writes.
//   - --include-directories takes a single comma-separated list in
//     place of repeated --add-dir flags.
//   - -p passes the bootstrap prompt.
//
// Note: gemini's macOS seatbelt profile (bundle/sandbox-macos-*.sb)
// grants writes to TARGET_DIR, TMP_DIR, CACHE_DIR, ~/.gemini, ~/.npm,
// ~/.cache, and up to five INCLUDE_DIR_N slots populated from
// --include-directories. Because ctx.AllDirs puts the workflow dir
// first, a correctly-resolved workflow path should always be
// writable under the sandbox.
//
// In practice we still see rare fallbacks — usually from path-
// resolution mismatches (symlinks, /private/ prefix on macOS) or
// older gemini builds where INCLUDE_DIR substitution was incomplete
// — where the agent lands a probe or partial artifact in the repo
// root (its CWD). state.RecoverWorkflowOutputs relocates substantive
// artifacts back into the workflow dir, and quarantines short/
// headerless probe files with a `.sandbox-probe` suffix so they do
// not stall retry loops by being promoted to canonical plan/review
// artifacts.
type geminiAdapter struct{}

func (geminiAdapter) Name() string           { return "gemini" }
func (geminiAdapter) DisplayName() string    { return "Google Gemini" }
func (geminiAdapter) DefaultCommand() string { return "gemini" }
func (geminiAdapter) Family() string         { return "google" }

func (geminiAdapter) Plan(ctx *LaunchContext) (*LaunchPlan, error) {
	// Prepend a system instruction to the prompt to ensure autonomy in automated flows.
	prompt := "Do not ask for confirmation or agreement; proceed directly to the task and write the required output file. Read and follow the instructions at " + ctx.PromptFile

	// --approval-mode=yolo (modern) auto-approves tool calls.
	// --sandbox runs the agent under an OS-level sandbox.
	// --allowed-tools "*" explicitly grants permissions for all tools.
	args := []string{"--approval-mode", "yolo", "--sandbox", "--allowed-tools", "*"}
	if len(ctx.AllDirs) > 0 {
		dirs := ctx.AllDirs
		if len(dirs) > 5 {
			// Gemini's macOS seatbelt profile only supports up to 5 slots.
			// Truncate to ensure the most important ones (wfDir, Home) win.
			dirs = dirs[:5]
		}
		for _, d := range dirs {
			if strings.Contains(d, ",") {
				return nil, fmt.Errorf("gemini adapter: directory path cannot contain commas: %q", d)
			}
		}
		args = append(args, "--include-directories", strings.Join(dirs, ","))
	}
	args = append(args, "-p", prompt)

	return &LaunchPlan{
		Args:    args,
		Prompt:  prompt,
		WorkDir: ctx.Repo,
	}, nil
}

func init() { RegisterAdapter(geminiAdapter{}) }
