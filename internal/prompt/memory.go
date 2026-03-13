package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/grikwong/crossagent/internal/state"
)

// tableRowRE matches markdown table rows that are not separator rows.
// Used to check for substantive global context content.
var tableRowRE = regexp.MustCompile(`^\|[^-]`)

// BuildMemoryContext reads all three memory tiers and returns formatted markdown.
// Matches bash gen_memory_context().
func BuildMemoryContext(wfDir string) (string, error) {
	var ctx strings.Builder

	// 1. Workflow memory
	wfMemFile := filepath.Join(wfDir, "memory.md")
	if data, err := os.ReadFile(wfMemFile); err == nil {
		ctx.WriteString("## Workflow Memory\n\n")
		ctx.WriteString("The following is accumulated context from this workflow. Use it to inform\n")
		ctx.WriteString("your work and update it with new learnings when done.\n\n")
		ctx.WriteString("<workflow-memory>\n")
		ctx.Write(data)
		ctx.WriteString("</workflow-memory>\n\n")
	}

	// 2. Project memory
	project, err := state.GetConf(wfDir, "project")
	if err != nil {
		return "", err
	}
	if project != "" {
		projMemDir := state.ProjectMemoryDir(project)
		if info, err := os.Stat(projMemDir); err == nil && info.IsDir() {
			files, err := state.ListProjectMemoryFiles(project)
			if err != nil {
				return "", err
			}

			var projCtx strings.Builder
			hasProjContent := false

			for _, pfile := range files {
				content, err := state.ReadMemoryContent(pfile)
				if err != nil {
					return "", err
				}
				if content != "" {
					hasProjContent = true
					projCtx.WriteString("### " + filepath.Base(pfile) + "\n")
					projCtx.WriteString(content)
					projCtx.WriteString("\n")
				}
			}

			if hasProjContent {
				ctx.WriteString(fmt.Sprintf("## Project Memory (%s)\n\n", project))
				ctx.WriteString(fmt.Sprintf("The following is accumulated knowledge specific to the \"%s\" project.\n", project))
				ctx.WriteString("Reference it for project-specific patterns, conventions, and domain knowledge.\n\n")
				ctx.WriteString("<project-memory>\n")
				ctx.WriteString(projCtx.String())
				ctx.WriteString("</project-memory>\n\n")
			}
		}
	}

	// 3. Global context
	globalCtxFile := filepath.Join(state.GlobalMemoryDir(), "global-context.md")
	if data, err := os.ReadFile(globalCtxFile); err == nil {
		// Check if it has substantive entries (>1 table row that isn't a separator)
		lines := strings.Split(string(data), "\n")
		tableRows := 0
		for _, line := range lines {
			if tableRowRE.MatchString(line) {
				tableRows++
			}
		}
		// Default template has exactly 1 header row; >1 means real entries
		if tableRows > 1 {
			ctx.WriteString("## Global Context\n\n")
			ctx.WriteString("The following is accumulated knowledge from previous workflows on this\n")
			ctx.WriteString("and other projects. Reference it for patterns and conventions.\n\n")
			ctx.WriteString("<global-context>\n")
			ctx.Write(data)
			ctx.WriteString("</global-context>\n\n")
		}
	}

	return ctx.String(), nil
}

// BuildMemoryUpdateInstructions returns instructions for agents to update memory.
// Matches bash gen_memory_update_instructions().
func BuildMemoryUpdateInstructions(wfDir, phaseName string) (string, error) {
	memDir := state.GlobalMemoryDir()

	project, err := state.GetConf(wfDir, "project")
	if err != nil {
		return "", err
	}
	projMemDir := ""
	if project != "" {
		projMemDir = state.ProjectMemoryDir(project)
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n## Memory Update Instructions\n\n"))
	b.WriteString(fmt.Sprintf("After completing your %s work, update the memory files:\n\n", phaseName))

	b.WriteString("### Workflow Memory (REQUIRED)\n")
	b.WriteString(fmt.Sprintf("Update `%s/memory.md` with:\n", wfDir))
	b.WriteString("- Decisions made during this phase (add to Decisions Log table)\n")
	b.WriteString("- Key findings or discoveries (add to Key Findings)\n")
	b.WriteString("- Session notes summarizing what was done\n")

	if projMemDir != "" {
		b.WriteString("\n### Project Memory (IF project-relevant patterns discovered)\n")
		b.WriteString(fmt.Sprintf("Update files under `%s/` with:\n", projMemDir))
		b.WriteString(fmt.Sprintf("- Patterns specific to this project's codebase -> `%s/project-context.md`\n", projMemDir))
		b.WriteString(fmt.Sprintf("- Conventions that apply across this project's workflows -> `%s/project-context.md`\n", projMemDir))
		b.WriteString(fmt.Sprintf("- Domain knowledge relevant to this project -> `%s/project-context.md`\n", projMemDir))
		b.WriteString(fmt.Sprintf("- Project-specific retrospective insights -> `%s/lessons-learned.md`\n", projMemDir))
		b.WriteString(fmt.Sprintf("- Feature-level context (if applicable) -> `%s/features/<feature-name>.md`\n", projMemDir))
		b.WriteString(fmt.Sprintf("\nDo NOT duplicate information that belongs in global memory.\n"))
		b.WriteString(fmt.Sprintf("Project memory is for patterns/knowledge specific to the \"%s\" project.\n", project))
	}

	b.WriteString("\n### Global Context (IF new cross-project patterns discovered)\n")
	b.WriteString("If you discovered reusable patterns, conventions, or pitfalls that would\n")
	b.WriteString("benefit future workflows across projects, append them to:\n")
	b.WriteString(fmt.Sprintf("`%s/global-context.md`\n\n", memDir))
	b.WriteString("Only add genuinely new, reusable knowledge — not workflow-specific or\n")
	b.WriteString("project-specific details.\n")
	b.WriteString("Add a Changelog entry with today's date and the source workflow name.\n\n")

	b.WriteString("### Lessons Learned (IF issues encountered)\n")
	b.WriteString("If you encountered process issues, rework, or discovered improvements to\n")
	b.WriteString("how Crossagent workflows should be run, append to:\n")
	b.WriteString(fmt.Sprintf("`%s/lessons-learned.md`\n", memDir))

	return b.String(), nil
}
