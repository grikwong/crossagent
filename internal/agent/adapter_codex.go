package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// codexAdapter implements Adapter for the OpenAI Codex CLI.
//
// Launch shape:
//
//	codex --full-auto -C <repo>
//	      -c projects."<repo>".trust_level="trusted"
//	      -c sandbox_mode="workspace-write"
//	      -c sandbox_workspace_write.writable_roots=[...]
//	      --add-dir <dir> --add-dir <dir> ...
//	      -- <general+phase-prompt>
//
// The trust override avoids the "Do you trust this folder?" prompt.
// The sandbox_mode + writable_roots overrides pin codex to
// workspace-write with every directory crossagent explicitly handed
// out listed as writable — writes outside that set are refused.
//
// Codex requires the full prompt content inlined, so this adapter
// concatenates general.md + the phase prompt rather than passing a
// "Read and follow" bootstrap string.
type codexAdapter struct{}

func (codexAdapter) Name() string           { return "codex" }
func (codexAdapter) DisplayName() string    { return "OpenAI Codex" }
func (codexAdapter) DefaultCommand() string { return "codex" }
func (codexAdapter) Family() string         { return "openai" }

func (codexAdapter) Plan(ctx *LaunchContext) (*LaunchPlan, error) {
	prompt, err := buildCodexPrompt(ctx.WorkflowDir, ctx.PromptFile)
	if err != nil {
		return nil, err
	}

	args := []string{"--full-auto", "-C", ctx.Repo}
	args = append(args, codexTrustArgs(ctx.Repo)...)
	args = append(args, codexSandboxArgs(ctx)...)
	for _, d := range ctx.AllDirs {
		args = append(args, "--add-dir", d)
	}
	args = append(args, "--", prompt)

	return &LaunchPlan{
		Args:   args,
		Prompt: prompt,
		// Codex takes the working directory via -C, so we don't need
		// to set cmd.Dir — leaving WorkDir empty means "caller's CWD".
		WorkDir: "",
	}, nil
}

// codexTrustArgs returns the -c override that pre-trusts the codex
// working directory. The repo path is TOML-escaped via %q; ValidatePath
// rejects repo paths containing characters (quotes, backslashes) that
// would break the escape.
func codexTrustArgs(repo string) []string {
	return []string{
		"-c",
		fmt.Sprintf(`projects.%q.trust_level="trusted"`, repo),
	}
}

// codexSandboxArgs pins codex to workspace-write mode and lists every
// directory/file crossagent wants writable. In implement phase, this is
// restricted to affected files plus the implement report.
func codexSandboxArgs(ctx *LaunchContext) []string {
	var writable []string
	if ctx.PhaseKey == "implement" {
		// Fail closed: implement writes are pinned to the reviewed file
		// list (union of plan.md + review.md affected-files sections).
		// Empty means no repo writes are authorized — we deliberately do
		// NOT fall back to the repo root, because that would restore
		// blanket repo-wide writes and defeat the sandbox.
		// Defensive assertion: AffectedFiles are canonicalized absolute
		// paths confined to the repo by judge.ExtractAffectedFiles.
		// Re-check here so we fail closed if a future caller ever skips
		// that boundary.
		for _, f := range ctx.AffectedFiles {
			if _, ok := assertUnderRepo(ctx.Repo, f); !ok {
				continue
			}
			writable = append(writable, f)
		}
		// Always allow writing the implement report in workflow dir.
		writable = append(writable, filepath.Join(ctx.WorkflowDir, "implement.md"))
	} else {
		// Default: allow writing to repo
		writable = append(writable, ctx.Repo)
	}

	// Always allow writing to AllDirs (workflow, memory, etc.)
	writable = append(writable, ctx.AllDirs...)

	quoted := make([]string, 0, len(writable))
	for _, d := range writable {
		quoted = append(quoted, fmt.Sprintf("%q", d))
	}
	args := []string{"-c", `sandbox_mode="workspace-write"`}
	if len(quoted) > 0 {
		args = append(args,
			"-c",
			fmt.Sprintf("sandbox_workspace_write.writable_roots=[%s]", strings.Join(quoted, ",")),
		)
	}
	return args
}

// buildCodexPrompt reads general.md + the phase prompt and concatenates
// them with a separator. Falls back to just the phase prompt if
// general.md is absent.
func buildCodexPrompt(wfDir, promptFile string) (string, error) {
	generalFile := filepath.Join(wfDir, "prompts", "general.md")
	promptData, err := os.ReadFile(promptFile)
	if err != nil {
		return "", fmt.Errorf("read prompt file: %w", err)
	}
	generalData, err := os.ReadFile(generalFile)
	if err != nil {
		if os.IsNotExist(err) {
			return string(promptData), nil
		}
		return "", fmt.Errorf("read general instructions: %w", err)
	}
	return strings.TrimRight(string(generalData), "\n") + "\n\n---\n\n" +
		strings.TrimRight(string(promptData), "\n"), nil
}

// buildCodexSpawnArgs is preserved as a package-level helper because
// existing tests (TestBuildCodexSpawnArgsOrdering) exercise the argv
// shape directly. It delegates to codexAdapter.Plan under the hood via
// a synthetic context so there's a single source of truth.
func buildCodexSpawnArgs(repo string, launchArgs []string, promptText string) []string {
	args := []string{"--full-auto", "-C", repo}
	args = append(args, codexTrustArgs(repo)...)
	dirs := extractAddDirs(launchArgs)
	ctx := &LaunchContext{
		Repo:    repo,
		AllDirs: dirs,
	}
	args = append(args, codexSandboxArgs(ctx)...)
	args = append(args, launchArgs...)
	args = append(args, "--", promptText)
	return args
}

func init() { RegisterAdapter(codexAdapter{}) }
