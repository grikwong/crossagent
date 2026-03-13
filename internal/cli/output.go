package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// ── Status ──────────────────────────────────────────────────────────────────

// StatusJSON matches `status --json` output from the bash CLI.
type StatusJSON struct {
	Name        string                  `json:"name"`
	Phase       string                  `json:"phase"`
	PhaseLabel  string                  `json:"phase_label"`
	Complete    bool                    `json:"complete"`
	Project     string                  `json:"project"`
	Repo        string                  `json:"repo"`
	AddDirs     []string                `json:"add_dirs"`
	Repos       ReposJSON               `json:"repos"`
	Description string                  `json:"description"`
	Created     string                  `json:"created"`
	WorkflowDir string                  `json:"workflow_dir"`
	Agents      map[string]AgentRefJSON `json:"agents"`
	RetryCount  int                     `json:"retry_count"`
	MaxRetries  int                     `json:"max_retries"`
	Artifacts   map[string]ArtifactJSON `json:"artifacts"`
}

// AgentRefJSON is the agent reference in status JSON.
type AgentRefJSON struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// ArtifactJSON represents an artifact in status JSON.
// When exists=false, lines is omitted (matches bash _json_artifact).
type ArtifactJSON struct {
	Exists bool   `json:"exists"`
	Path   string `json:"path"`
	Lines  int    `json:"lines,omitempty"`
}

// ── List ────────────────────────────────────────────────────────────────────

// ListJSON matches `list --json` output from the bash CLI.
type ListJSON struct {
	Workflows []ListWorkflowJSON `json:"workflows"`
	Projects  []ListProjectJSON  `json:"projects"`
	Active    string             `json:"active"`
}

// ListWorkflowJSON is a workflow entry in list JSON.
type ListWorkflowJSON struct {
	Name       string            `json:"name"`
	Phase      string            `json:"phase"`
	PhaseLabel string            `json:"phase_label"`
	Active     bool              `json:"active"`
	Project    string            `json:"project"`
	Agents     map[string]string `json:"agents"`
}

// ListProjectJSON is a project entry in list JSON.
type ListProjectJSON struct {
	Name          string `json:"name"`
	WorkflowCount int    `json:"workflow_count"`
}

// ── Agents ──────────────────────────────────────────────────────────────────

// AgentsListJSON matches `agents list --json`.
type AgentsListJSON struct {
	Agents []AgentJSON `json:"agents"`
}

// AgentJSON is a single agent in the agents list.
type AgentJSON struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Adapter     string `json:"adapter"`
	Command     string `json:"command"`
	Builtin     bool   `json:"builtin"`
}

// AgentsShowJSON matches `agents show --json`.
type AgentsShowJSON struct {
	Workflow string            `json:"workflow"`
	Agents   map[string]string `json:"agents"`
}

// ── Repos ───────────────────────────────────────────────────────────────────

// ReposJSON matches `repos list --json`.
type ReposJSON struct {
	Primary    string   `json:"primary"`
	Additional []string `json:"additional"`
}

// ── Projects ────────────────────────────────────────────────────────────────

// ProjectsListJSON matches `projects list --json`.
type ProjectsListJSON struct {
	Projects []ListProjectJSON `json:"projects"`
}

// ProjectShowJSON matches `projects show --json`.
type ProjectShowJSON struct {
	Name          string                `json:"name"`
	WorkflowCount int                   `json:"workflow_count"`
	Workflows     []ProjectWorkflowJSON `json:"workflows"`
	MemoryDir     string                `json:"memory_dir"`
}

// ProjectWorkflowJSON is a workflow entry in project show JSON.
type ProjectWorkflowJSON struct {
	Name       string `json:"name"`
	Phase      string `json:"phase"`
	PhaseLabel string `json:"phase_label"`
}

// ProjectSuggestJSON matches `projects suggest --json`.
type ProjectSuggestJSON struct {
	SuggestedProject *string `json:"suggested_project"` // null when no match
	Score            int     `json:"score"`
	MatchedTerms     string  `json:"matched_terms"`
}

// ── Revert ──────────────────────────────────────────────────────────────────

// RevertJSON matches `revert --json`.
type RevertJSON struct {
	Action        string `json:"action"`
	TargetPhase   int    `json:"target_phase,omitempty"`
	TargetLabel   string `json:"target_label,omitempty"`
	Attempt       int    `json:"attempt,omitempty"`
	RetryCount    int    `json:"retry_count"`
	MaxRetries    int    `json:"max_retries"`
	RevertContext string `json:"revert_context,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// ── Supervise ───────────────────────────────────────────────────────────────

// SuperviseJSON matches `supervise --json`.
// The shape varies by verdict type, matching bash behavior.
type SuperviseJSON struct {
	Action         string `json:"action"`
	Verdict        string `json:"verdict,omitempty"`
	Status         string `json:"status,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
	ReviewVerdict  string `json:"review_verdict,omitempty"`
	Reason         string `json:"reason,omitempty"`
	RetryCount     int    `json:"retry_count,omitempty"`
	MaxRetries     int    `json:"max_retries,omitempty"`
}

// ── Memory ──────────────────────────────────────────────────────────────────

// MemoryShowJSON matches `memory show --json`.
type MemoryShowJSON struct {
	Type    string                    `json:"type"`
	Name    string                    `json:"name,omitempty"`
	Path    string                    `json:"path,omitempty"`
	Content *string                   `json:"content,omitempty"`
	Files   map[string]MemoryFileJSON `json:"files,omitempty"`
}

// MemoryFileJSON is a single file in memory show JSON.
type MemoryFileJSON struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// MemoryListJSON matches `memory list --json`.
type MemoryListJSON struct {
	Type  string            `json:"type"`
	Name  string            `json:"name,omitempty"`
	Files []MemoryListEntry `json:"files"`
}

// MemoryListEntry is a file entry in memory list JSON.
type MemoryListEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Lines int    `json:"lines"`
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// PrintJSON marshals v with 2-space indentation and prints to stdout with a trailing newline.
func PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	_, err = fmt.Fprintf(os.Stdout, "%s\n", data)
	return err
}

// PrintJSONCompact marshals v without indentation and prints to stdout with a trailing newline.
// Used for endpoints where bash outputs compact (single-line) JSON.
func PrintJSONCompact(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	_, err = fmt.Fprintf(os.Stdout, "%s\n", data)
	return err
}
