package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Home returns the crossagent home directory.
// Respects $CROSSAGENT_HOME if set, otherwise defaults to ~/.crossagent.
func Home() string {
	if h := os.Getenv("CROSSAGENT_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".crossagent")
	}
	return filepath.Join(home, ".crossagent")
}

// WorkflowsDir returns the workflows directory path.
func WorkflowsDir() string {
	return filepath.Join(Home(), "workflows")
}

// WorkflowDir returns the directory path for a specific workflow.
func WorkflowDir(name string) string {
	return filepath.Join(WorkflowsDir(), name)
}

// AgentsDir returns the agents directory path.
func AgentsDir() string {
	return filepath.Join(Home(), "agents")
}

// ProjectsDir returns the projects directory path.
func ProjectsDir() string {
	return filepath.Join(Home(), "projects")
}

// GetPhase reads the current phase from the workflow directory.
// Returns "0" if the phase file doesn't exist (matches bash behavior).
func GetPhase(wfDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(wfDir, "phase"))
	if err != nil {
		if os.IsNotExist(err) {
			return "0", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SetPhase writes the phase to the workflow directory.
func SetPhase(wfDir, phase string) error {
	return os.WriteFile(filepath.Join(wfDir, "phase"), []byte(phase+"\n"), 0644)
}

// GetDescription reads the workflow description.
// Returns empty string if not found (matches bash behavior).
func GetDescription(wfDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(wfDir, "description"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// GetCurrent reads the active workflow name from ~/.crossagent/current.
// Returns empty string if no active workflow.
func GetCurrent() (string, error) {
	data, err := os.ReadFile(filepath.Join(Home(), "current"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SetCurrent sets the active workflow name.
func SetCurrent(name string) error {
	return os.WriteFile(filepath.Join(Home(), "current"), []byte(name+"\n"), 0644)
}

// ListWorkflows returns a list of workflow names sorted to match bash glob ordering.
func ListWorkflows() ([]string, error) {
	entries, err := os.ReadDir(WorkflowsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	GlobSort(names)
	return names, nil
}

// GlobSort sorts a string slice to match macOS bash glob ordering.
// In macOS glob collation, non-alphanumeric characters (hyphens, underscores, dots)
// sort before end-of-string, unlike Go's byte comparison where end-of-string sorts
// before any character. This matters for names like "foo-bar" vs "foo".
func GlobSort(names []string) {
	sort.Slice(names, func(i, j int) bool {
		return globCompare(names[i], names[j]) < 0
	})
}

func globCompare(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	if len(a) == len(b) {
		return 0
	}
	// One string is a prefix of the other.
	// Match macOS glob collation: non-alphanumeric chars sort before end-of-string.
	if len(a) > len(b) {
		if !isAlphaNum(a[n]) {
			return -1
		}
		return 1
	}
	if !isAlphaNum(b[n]) {
		return 1
	}
	return -1
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// WorkflowExists checks if a workflow directory exists.
func WorkflowExists(name string) bool {
	info, err := os.Stat(WorkflowDir(name))
	return err == nil && info.IsDir()
}

// EnsureDirs creates the required directory structure under ~/.crossagent.
// Also handles legacy migration (features dir) and backfills missing project keys.
func EnsureDirs() error {
	home := Home()
	dirs := []string{
		home,
		WorkflowsDir(),
		AgentsDir(),
		filepath.Join(home, "memory"),
		ProjectsDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Initialize global memory
	if err := InitGlobalMemory(); err != nil {
		return err
	}

	// Ensure default project exists
	defaultDir := filepath.Join(ProjectsDir(), "default")
	if _, err := os.Stat(defaultDir); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(defaultDir, "memory"), 0755); err != nil {
			return err
		}
		if err := InitProjectMemory("default"); err != nil {
			return err
		}
	}

	// Legacy migration: move memory/features -> projects/default/memory/features
	memoryDir := GlobalMemoryDir()
	featuresSource := filepath.Join(memoryDir, "features")
	featuresDest := filepath.Join(defaultDir, "memory", "features")

	srcInfo, srcErr := os.Stat(featuresSource)
	_, dstErr := os.Stat(featuresDest)

	if srcErr == nil && srcInfo.IsDir() && os.IsNotExist(dstErr) {
		// Source exists, dest does not — migrate
		if err := os.Rename(featuresSource, featuresDest); err != nil {
			return fmt.Errorf("failed to migrate features directory: %w", err)
		}
		migrationNote := fmt.Sprintf("Migrated on %s\nSource: %s/\nDestination: %s/\n",
			timeNow(), featuresSource, featuresDest)
		os.WriteFile(filepath.Join(memoryDir, "features-migrated.txt"), []byte(migrationNote), 0644)
	} else if srcErr == nil && srcInfo.IsDir() && dstErr == nil {
		// Both exist — warn (matches bash behavior)
		fmt.Fprintf(os.Stderr, "  \033[1;33m!\033[0m Both %s/ and %s/ exist.\n", featuresSource, featuresDest)
		fmt.Fprintf(os.Stderr, "  \033[1;33m!\033[0m Manual merge required. Remove or merge the source directory.\n")
	}

	// Backfill existing workflows without a project key
	workflows, err := ListWorkflows()
	if err != nil {
		return err
	}
	for _, wf := range workflows {
		wfDir := WorkflowDir(wf)
		proj, err := GetConf(wfDir, "project")
		if err != nil {
			continue
		}
		if proj == "" {
			if err := SetConf(wfDir, "project", "default"); err != nil {
				return fmt.Errorf("failed to backfill project for workflow %s: %w", wf, err)
			}
		}
	}

	return nil
}

// PhaseLabel returns the human-readable label for a phase number.
func PhaseLabel(phase string) string {
	switch phase {
	case "1":
		return "plan"
	case "2":
		return "review"
	case "3":
		return "implement"
	case "4":
		return "verify"
	case "done":
		return "done"
	default:
		return phase
	}
}

// PhaseLabelUpper returns the uppercase phase label.
func PhaseLabelUpper(phase string) string {
	switch phase {
	case "1":
		return "PLAN"
	case "2":
		return "REVIEW"
	case "3":
		return "IMPLEMENT"
	case "4":
		return "VERIFY"
	case "done":
		return "DONE"
	default:
		return phase
	}
}

// PhaseOutputFile returns the output file name for a phase.
func PhaseOutputFile(phase string) string {
	switch phase {
	case "1":
		return "plan.md"
	case "2":
		return "review.md"
	case "3":
		return "implement.md"
	case "4":
		return "verify.md"
	default:
		return ""
	}
}

// PhaseNum converts a phase string to an integer for comparison.
// "done" is treated as 5.
func PhaseNum(phase string) int {
	switch phase {
	case "done":
		return 5
	default:
		return atoi(phase)
	}
}

// PhaseKey normalizes a phase identifier to its key name.
func PhaseKey(phase string) (string, error) {
	switch phase {
	case "1", "plan":
		return "plan", nil
	case "2", "review":
		return "review", nil
	case "3", "implement", "impl":
		return "implement", nil
	case "4", "verify":
		return "verify", nil
	default:
		return "", fmt.Errorf("unknown phase: %s", phase)
	}
}

// PhaseID normalizes a phase identifier to its numeric string.
func PhaseID(phase string) (string, error) {
	switch phase {
	case "1", "plan":
		return "1", nil
	case "2", "review":
		return "2", nil
	case "3", "implement", "impl":
		return "3", nil
	case "4", "verify":
		return "4", nil
	default:
		return "", fmt.Errorf("unknown phase: %s", phase)
	}
}

// RequireWorkflow returns the current workflow name and directory, or an error if none is active.
func RequireWorkflow() (string, string, error) {
	name, err := GetCurrent()
	if err != nil {
		return "", "", err
	}
	if name == "" {
		return "", "", fmt.Errorf("no active workflow. Run: crossagent new <name>")
	}
	dir := WorkflowDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", "", fmt.Errorf("workflow dir missing: %s", dir)
	}
	return name, dir, nil
}

// CreateWorkflow creates a new workflow with the given parameters.
// It handles directory creation, config writing, description, phase init,
// setting current, and memory initialization.
// All paths (repo, addDirs) must be pre-validated absolute paths.
func CreateWorkflow(name, repo, project, description string, addDirs []string) error {
	if project == "" {
		project = "default"
	}
	if !ProjectExists(project) {
		return fmt.Errorf("project '%s' not found", project)
	}
	if WorkflowExists(name) {
		return fmt.Errorf("workflow '%s' already exists", name)
	}

	d := WorkflowDir(name)
	if err := os.MkdirAll(filepath.Join(d, "prompts"), 0755); err != nil {
		return fmt.Errorf("failed to create workflow directory: %w", err)
	}

	// Write config
	addDirsCSV := strings.Join(addDirs, ",")
	pairs := [][2]string{
		{"repo", repo},
		{"add_dirs", addDirsCSV},
		{"created", timeNow()},
		{"project", project},
		{"max_retries", "10"},
	}
	for _, p := range pairs {
		if err := SetConf(d, p[0], p[1]); err != nil {
			os.RemoveAll(d)
			return fmt.Errorf("failed to write config key %s: %w", p[0], err)
		}
	}

	if err := os.WriteFile(filepath.Join(d, "description"), []byte(description+"\n"), 0644); err != nil {
		os.RemoveAll(d)
		return fmt.Errorf("failed to write description: %w", err)
	}
	if err := SetPhase(d, "1"); err != nil {
		os.RemoveAll(d)
		return fmt.Errorf("failed to set phase: %w", err)
	}
	if err := SetCurrent(name); err != nil {
		os.RemoveAll(d)
		return fmt.Errorf("failed to set current: %w", err)
	}
	if err := InitWorkflowMemory(d, name, description, repo); err != nil {
		os.RemoveAll(d)
		return fmt.Errorf("failed to init memory: %w", err)
	}

	return nil
}

// FollowupWorkflow archives completed artifacts into rounds/N/ and resets
// the workflow to phase 1 for a new round of work. Returns the new round number.
func FollowupWorkflow(wfDir, newDescription string) (int, error) {
	phase, err := GetPhase(wfDir)
	if err != nil {
		return 0, err
	}
	if phase != "done" {
		return 0, fmt.Errorf("workflow must be completed (phase=done) to follow up, current phase: %s", phase)
	}

	cfg, err := ReadConfig(wfDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read config: %w", err)
	}

	roundNum := cfg.FollowupRound + 1
	roundDir := filepath.Join(wfDir, "rounds", fmt.Sprintf("%d", roundNum))

	if _, err := os.Stat(roundDir); err == nil {
		return 0, fmt.Errorf("round directory already exists: %s", roundDir)
	}

	if err := os.MkdirAll(roundDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create round directory: %w", err)
	}

	// Move phase artifacts
	for _, name := range []string{"plan.md", "review.md", "implement.md", "verify.md"} {
		src := filepath.Join(wfDir, name)
		if fileExists(src) {
			if err := os.Rename(src, filepath.Join(roundDir, name)); err != nil {
				return 0, fmt.Errorf("failed to archive %s: %w", name, err)
			}
		}
	}

	// Move attempt archives (*.attempt-*.md)
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read workflow directory: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.Contains(e.Name(), ".attempt-") && strings.HasSuffix(e.Name(), ".md") {
			src := filepath.Join(wfDir, e.Name())
			if err := os.Rename(src, filepath.Join(roundDir, e.Name())); err != nil {
				return 0, fmt.Errorf("failed to archive %s: %w", e.Name(), err)
			}
		}
	}

	// Move chat-history contents
	chatDir := filepath.Join(wfDir, "chat-history")
	if info, err := os.Stat(chatDir); err == nil && info.IsDir() {
		roundChatDir := filepath.Join(roundDir, "chat-history")
		if err := os.MkdirAll(roundChatDir, 0755); err != nil {
			return 0, fmt.Errorf("failed to create round chat-history dir: %w", err)
		}
		chatEntries, err := os.ReadDir(chatDir)
		if err != nil {
			return 0, fmt.Errorf("failed to read chat-history: %w", err)
		}
		for _, e := range chatEntries {
			if !e.IsDir() {
				src := filepath.Join(chatDir, e.Name())
				if err := os.Rename(src, filepath.Join(roundChatDir, e.Name())); err != nil {
					return 0, fmt.Errorf("failed to archive chat %s: %w", e.Name(), err)
				}
			}
		}
	}

	// Move prompts contents (except .sandbox-settings.json)
	promptsDir := filepath.Join(wfDir, "prompts")
	if info, err := os.Stat(promptsDir); err == nil && info.IsDir() {
		roundPromptsDir := filepath.Join(roundDir, "prompts")
		if err := os.MkdirAll(roundPromptsDir, 0755); err != nil {
			return 0, fmt.Errorf("failed to create round prompts dir: %w", err)
		}
		promptEntries, err := os.ReadDir(promptsDir)
		if err != nil {
			return 0, fmt.Errorf("failed to read prompts dir: %w", err)
		}
		for _, e := range promptEntries {
			if !e.IsDir() && e.Name() != ".sandbox-settings.json" {
				src := filepath.Join(promptsDir, e.Name())
				if err := os.Rename(src, filepath.Join(roundPromptsDir, e.Name())); err != nil {
					return 0, fmt.Errorf("failed to archive prompt %s: %w", e.Name(), err)
				}
			}
		}
	}

	// Generate followup context from archived artifacts using priority fallback:
	// verify → review → implement → plan (addresses review issue #2)
	followupContext := generateFollowupContext(roundDir, roundNum)
	followupContextPath := filepath.Join(promptsDir, "followup-context.md")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to ensure prompts dir: %w", err)
	}
	if err := atomicWrite(followupContextPath, []byte(followupContext)); err != nil {
		return 0, fmt.Errorf("failed to write followup context: %w", err)
	}

	// Reset phase to 1
	if err := SetPhase(wfDir, "1"); err != nil {
		return 0, fmt.Errorf("failed to reset phase: %w", err)
	}

	// Update config
	cfg.FollowupRound = roundNum
	cfg.RetryCount = 0
	if err := WriteConfig(wfDir, cfg); err != nil {
		return 0, fmt.Errorf("failed to update config: %w", err)
	}

	// Update description if provided
	if newDescription != "" {
		descPath := filepath.Join(wfDir, "description")
		if err := atomicWrite(descPath, []byte(newDescription+"\n")); err != nil {
			return 0, fmt.Errorf("failed to update description: %w", err)
		}
	}

	// Append followup session note to memory.md
	memPath := filepath.Join(wfDir, "memory.md")
	if fileExists(memPath) {
		note := fmt.Sprintf("\n### %s\n- Follow-up round %d started\n", dateNow(), roundNum)
		f, err := os.OpenFile(memPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString(note)
			f.Close()
		}
	}

	return roundNum, nil
}

// generateFollowupContext builds the followup context from archived artifacts.
// Uses priority-ordered fallback: verify → review → implement → plan.
func generateFollowupContext(roundDir string, roundNum int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Previous Round %d Context\n\n", roundNum))

	type artifactEntry struct {
		file  string
		label string
	}
	// Priority order: verify, review, implement, plan
	artifacts := []artifactEntry{
		{"verify.md", "Verification Report"},
		{"review.md", "Review Feedback"},
		{"implement.md", "Implementation Summary"},
		{"plan.md", "Implementation Plan"},
	}

	found := 0
	for _, a := range artifacts {
		path := filepath.Join(roundDir, a.file)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		// Truncate large artifacts to 5000 chars
		if len(content) > 5000 {
			content = content[:5000] + "\n\n... (truncated)\n"
		}
		sb.WriteString(fmt.Sprintf("## %s (%s)\n\n", a.label, a.file))
		sb.WriteString(content)
		sb.WriteString("\n\n")
		found++
	}

	if found == 0 {
		sb.WriteString("No artifacts found from the previous round.\n")
	}

	return sb.String()
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// timeNow returns the current date as YYYY-MM-DD HH:MM.
// Extracted as a variable for testing.
var timeNow = func() string {
	return currentTime().Format("2006-01-02 15:04")
}

// dateNow returns the current date as YYYY-MM-DD.
var dateNow = func() string {
	return currentTime().Format("2006-01-02")
}
