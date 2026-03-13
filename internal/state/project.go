package state

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// NameRE is the validation regex for workflow and project names.
var NameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ValidateName validates a workflow or project name.
func ValidateName(name string) error {
	if !NameRE.MatchString(name) {
		return fmt.Errorf("invalid name: %s. Must match: %s", name, NameRE.String())
	}
	return nil
}

// ProjectDir returns the directory path for a project.
func ProjectDir(name string) string {
	return filepath.Join(ProjectsDir(), name)
}

// ProjectExists checks if a project directory exists.
func ProjectExists(name string) bool {
	info, err := os.Stat(ProjectDir(name))
	return err == nil && info.IsDir()
}

// ProjectInfo holds summary information about a project.
type ProjectInfo struct {
	Name          string
	WorkflowCount int
	MemoryDir     string
}

// ListProjects returns all projects with their workflow counts.
func ListProjects() ([]ProjectInfo, error) {
	entries, err := os.ReadDir(ProjectsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Collect names first, then sort to match bash glob ordering
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	GlobSort(names)

	var projects []ProjectInfo
	for _, name := range names {
		count, _ := countProjectWorkflows(name)
		projects = append(projects, ProjectInfo{
			Name:          name,
			WorkflowCount: count,
			MemoryDir:     ProjectMemoryDir(name),
		})
	}
	return projects, nil
}

func countProjectWorkflows(project string) (int, error) {
	workflows, err := ListWorkflows()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, wf := range workflows {
		proj, _ := GetConf(WorkflowDir(wf), "project")
		if proj == project {
			count++
		}
	}
	return count, nil
}

// CreateProject creates a new project with its memory directory.
func CreateProject(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if ProjectExists(name) {
		return fmt.Errorf("project already exists: %s", name)
	}

	memDir := ProjectMemoryDir(name)
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return err
	}
	return InitProjectMemory(name)
}

// DeleteProject deletes a project and moves its workflows to "default".
// Cannot delete the "default" project.
func DeleteProject(name string) error {
	if name == "default" {
		return fmt.Errorf("cannot delete the default project")
	}
	if !ProjectExists(name) {
		return fmt.Errorf("project not found: %s", name)
	}

	// Move workflows to default
	workflows, err := ListWorkflows()
	if err != nil {
		return err
	}
	for _, wf := range workflows {
		wfDir := WorkflowDir(wf)
		proj, _ := GetConf(wfDir, "project")
		if proj == name {
			if err := SetConf(wfDir, "project", "default"); err != nil {
				return fmt.Errorf("failed to reassign workflow %s: %w", wf, err)
			}
		}
	}

	return os.RemoveAll(ProjectDir(name))
}

// RenameProject renames a project and updates all its workflow configs.
func RenameProject(oldName, newName string) error {
	if err := ValidateName(newName); err != nil {
		return err
	}
	if !ProjectExists(oldName) {
		return fmt.Errorf("project not found: %s", oldName)
	}
	if ProjectExists(newName) {
		return fmt.Errorf("project already exists: %s", newName)
	}
	if oldName == "default" {
		return fmt.Errorf("cannot rename the default project")
	}

	// Rename directory
	if err := os.Rename(ProjectDir(oldName), ProjectDir(newName)); err != nil {
		return err
	}

	// Update workflow configs
	workflows, err := ListWorkflows()
	if err != nil {
		return err
	}
	for _, wf := range workflows {
		wfDir := WorkflowDir(wf)
		proj, _ := GetConf(wfDir, "project")
		if proj == oldName {
			if err := SetConf(wfDir, "project", newName); err != nil {
				return fmt.Errorf("failed to update workflow %s during rename: %w", wf, err)
			}
		}
	}
	return nil
}

// Suggestion holds the result of project suggestion scoring.
type Suggestion struct {
	Project      string
	Score        int
	MatchedTerms string
}

// suggestKeywords are the tech/domain keywords used for project scoring.
var suggestKeywords = []string{
	"grpc", "proto", "vendor", "admin", "portal", "customer",
	"event.sourc", "cqrs", "postgresql", "kurrentdb", "sendgrid",
	"dart", "flutter",
}

// SuggestProject scores all non-default projects against a description
// and returns the best match if the score meets the threshold (>=30).
func SuggestProject(description string) (*Suggestion, error) {
	if description == "" {
		return nil, fmt.Errorf("no description provided")
	}

	descLower := strings.ToLower(description)

	entries, err := os.ReadDir(ProjectsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var bestProject string
	var bestScore int
	var bestTerms string

	for _, e := range entries {
		if !e.IsDir() || e.Name() == "default" {
			continue
		}
		pname := e.Name()
		memDir := ProjectMemoryDir(pname)
		if _, err := os.Stat(memDir); os.IsNotExist(err) {
			continue
		}

		// Collect corpus from all .md files in project memory
		corpus := collectCorpus(memDir)
		if corpus == "" {
			continue
		}
		corpusLower := strings.ToLower(corpus)

		score := 0
		var matched []string

		// Check repo path fragments
		repoPaths := extractRepoPaths(corpus)
		for _, rp := range repoPaths {
			rpBase := filepath.Base(rp)
			if strings.Contains(descLower, strings.ToLower(rpBase)) {
				score += 15
				matched = append(matched, rpBase)
			}
		}

		// Check tech/domain keywords
		for _, kw := range suggestKeywords {
			if strings.Contains(corpusLower, kw) && strings.Contains(descLower, kw) {
				score += 10
				matched = append(matched, kw)
			}
		}

		// Check project name
		if strings.Contains(descLower, strings.ToLower(pname)) {
			score += 20
			matched = append(matched, pname)
		}

		if score > bestScore {
			bestScore = score
			bestProject = pname
			bestTerms = strings.Join(matched, ", ")
		}
	}

	// Threshold: only suggest if score >= 30
	if bestScore >= 30 && bestProject != "" {
		return &Suggestion{
			Project:      bestProject,
			Score:        bestScore,
			MatchedTerms: bestTerms,
		}, nil
	}
	return nil, nil
}

// collectCorpus reads all .md files in a directory tree and concatenates them.
func collectCorpus(dir string) string {
	var parts []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		parts = append(parts, string(data))
		return nil
	})
	return strings.Join(parts, " ")
}

// extractRepoPaths finds /path/name patterns in text.
var repoPathRE = regexp.MustCompile(`/[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+`)

func extractRepoPaths(text string) []string {
	matches := repoPathRE.FindAllString(text, -1)
	// Deduplicate
	seen := map[string]bool{}
	var unique []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	return unique
}

// GetWorkflowProject reads the project name from a workflow's config.
func GetWorkflowProject(wfDir string) string {
	proj, _ := GetConf(wfDir, "project")
	if proj == "" {
		return "default"
	}
	return proj
}

// ListProjectWorkflows returns workflow names belonging to a project.
func ListProjectWorkflows(project string) ([]string, error) {
	workflows, err := ListWorkflows()
	if err != nil {
		return nil, err
	}
	var result []string
	for _, wf := range workflows {
		proj, _ := GetConf(WorkflowDir(wf), "project")
		if proj == project {
			result = append(result, wf)
		}
	}
	return result, nil
}

// MoveWorkflow moves a workflow to a different project.
func MoveWorkflow(workflowName, targetProject string) error {
	if !WorkflowExists(workflowName) {
		return fmt.Errorf("workflow not found: %s", workflowName)
	}
	if !ProjectExists(targetProject) {
		return fmt.Errorf("project not found: %s", targetProject)
	}

	wfDir := WorkflowDir(workflowName)
	currentProj := GetWorkflowProject(wfDir)
	if currentProj == targetProject {
		return fmt.Errorf("workflow '%s' is already in project '%s'", workflowName, targetProject)
	}

	return SetConf(wfDir, "project", targetProject)
}
