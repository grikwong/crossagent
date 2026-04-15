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
// Note: gemini's macOS seatbelt profile typically grants writes only
// to the working directory. Phase outputs the agent was asked to emit
// into the workflow dir may end up in the repo root instead; the
// state.RecoverWorkflowOutputs helper relocates them after the fact.
type geminiAdapter struct{}

func (geminiAdapter) Name() string           { return "gemini" }
func (geminiAdapter) DisplayName() string    { return "Google Gemini" }
func (geminiAdapter) DefaultCommand() string { return "gemini" }

func (geminiAdapter) Plan(ctx *LaunchContext) (*LaunchPlan, error) {
	prompt := "Read and follow the instructions at " + ctx.PromptFile

	args := []string{"--yolo", "--sandbox"}
	if len(ctx.AllDirs) > 0 {
		for _, d := range ctx.AllDirs {
			if strings.Contains(d, ",") {
				return nil, fmt.Errorf("gemini adapter: directory path cannot contain commas: %q", d)
			}
		}
		args = append(args, "--include-directories", strings.Join(ctx.AllDirs, ","))
	}
	args = append(args, "-p", prompt)

	return &LaunchPlan{
		Args:    args,
		Prompt:  prompt,
		WorkDir: ctx.Repo,
	}, nil
}

func init() { RegisterAdapter(geminiAdapter{}) }
