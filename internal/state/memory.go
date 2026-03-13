package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GlobalMemoryDir returns the global memory directory path.
func GlobalMemoryDir() string {
	return filepath.Join(Home(), "memory")
}

// InitGlobalMemory initializes the global memory files if they don't exist.
func InitGlobalMemory() error {
	memDir := GlobalMemoryDir()
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return err
	}

	globalCtx := filepath.Join(memDir, "global-context.md")
	if _, err := os.Stat(globalCtx); os.IsNotExist(err) {
		content := `# Crossagent Global Context

> Cross-workflow patterns, conventions, and accumulated knowledge.
> Updated by agents after each workflow phase.

---

## Codebase Patterns

(Patterns discovered across workflows will be added here)

## Conventions

(Coding conventions, naming patterns, etc.)

## Changelog

| Date | Change | Source Workflow |
|------|--------|----------------|
`
		if err := os.WriteFile(globalCtx, []byte(content), 0644); err != nil {
			return err
		}
	}

	lessonsFile := filepath.Join(memDir, "lessons-learned.md")
	if _, err := os.Stat(lessonsFile); os.IsNotExist(err) {
		content := `# Crossagent Lessons Learned

> Retrospective insights from completed workflows.
> Each entry documents what worked, what didn't, and process improvements.

---

(Entries will be added after workflow completion)
`
		if err := os.WriteFile(lessonsFile, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// InitProjectMemory initializes project memory files if they don't exist.
func InitProjectMemory(project string) error {
	memDir := ProjectMemoryDir(project)
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return err
	}

	ctxFile := filepath.Join(memDir, "project-context.md")
	if _, err := os.Stat(ctxFile); os.IsNotExist(err) {
		content := fmt.Sprintf(`# Project Context: %s

> Project-specific patterns, conventions, and domain knowledge.
> Updated by agents after each workflow phase.

---

## Codebase Patterns

(Patterns discovered in this project will be added here)

## Conventions

(Project-specific coding conventions, naming patterns, etc.)

## Domain Knowledge

(Domain-specific knowledge relevant to this project)
`, project)
		if err := os.WriteFile(ctxFile, []byte(content), 0644); err != nil {
			return err
		}
	}

	lessonsFile := filepath.Join(memDir, "lessons-learned.md")
	if _, err := os.Stat(lessonsFile); os.IsNotExist(err) {
		content := fmt.Sprintf(`# Project Lessons Learned: %s

> Retrospective insights from workflows in this project.

---

(Entries will be added after workflow completion)
`, project)
		if err := os.WriteFile(lessonsFile, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// InitWorkflowMemory initializes the workflow memory file if it doesn't exist.
func InitWorkflowMemory(wfDir, name, desc, repo string) error {
	memFile := filepath.Join(wfDir, "memory.md")
	if _, err := os.Stat(memFile); err == nil {
		return nil // already exists
	}

	date := dateNow()
	content := fmt.Sprintf(`# Workflow Memory: %s

## Metadata
| Field | Value |
|-------|-------|
| Workflow | %s |
| Repository | %s |
| Created | %s |
| Last Updated | %s |

## Task Summary
%s

## Decisions Log
| Decision | Rationale | Phase | Date |
|----------|-----------|-------|------|

## Key Findings
(Important discoveries made during the workflow)

## Session Notes
### %s
- Workflow created
`, name, name, repo, date, date, desc, date)

	return os.WriteFile(memFile, []byte(content), 0644)
}

// ReadMemoryContent reads a memory file and returns its content.
// Returns empty string if the file doesn't exist or has insufficient content
// (fewer than 5 non-empty lines, matching bash behavior of checking for substantive content).
func ReadMemoryContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	content := string(data)

	// Check if content is substantive (>5 non-empty lines)
	lines := strings.Split(content, "\n")
	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	if nonEmpty <= 5 {
		return "", nil
	}

	return content, nil
}

// ListProjectMemoryFiles returns all .md files in a project's memory directory,
// including files in the features/ subdirectory.
func ListProjectMemoryFiles(project string) ([]string, error) {
	memDir := ProjectMemoryDir(project)
	if _, err := os.Stat(memDir); os.IsNotExist(err) {
		return nil, nil
	}

	var files []string
	err := filepath.Walk(memDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// ProjectMemoryDir returns the memory directory path for a project.
func ProjectMemoryDir(name string) string {
	return filepath.Join(ProjectsDir(), name, "memory")
}
