package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// ── Ordered types for deterministic JSON key ordering ────────────────────────
// These ensure keys appear in the same order as the bash CLI:
// plan, review, implement, verify (+ memory for artifacts).

// OrderedAgents holds per-phase agent references in fixed order.
type OrderedAgents struct {
	Plan      AgentRefJSON `json:"plan"`
	Review    AgentRefJSON `json:"review"`
	Implement AgentRefJSON `json:"implement"`
	Verify    AgentRefJSON `json:"verify"`
}

// OrderedArtifacts holds per-phase artifacts in fixed order.
type OrderedArtifacts struct {
	Plan      ArtifactJSON `json:"plan"`
	Review    ArtifactJSON `json:"review"`
	Implement ArtifactJSON `json:"implement"`
	Verify    ArtifactJSON `json:"verify"`
	Memory    ArtifactJSON `json:"memory"`
}

// OrderedAgentNames holds phase-to-agent-name in fixed order.
type OrderedAgentNames struct {
	Plan      string `json:"plan"`
	Review    string `json:"review"`
	Implement string `json:"implement"`
	Verify    string `json:"verify"`
}

// OrderedFileMap preserves insertion order when marshaling string-keyed maps.
type OrderedFileMap struct {
	keys   []string
	values map[string]MemoryFileJSON
}

// NewOrderedFileMap creates an empty ordered file map.
func NewOrderedFileMap() *OrderedFileMap {
	return &OrderedFileMap{values: make(map[string]MemoryFileJSON)}
}

// Set adds or updates a key-value pair, preserving insertion order.
func (m *OrderedFileMap) Set(key string, val MemoryFileJSON) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = val
}

// MarshalJSON emits the map in insertion order without HTML escaping.
func (m *OrderedFileMap) MarshalJSON() ([]byte, error) {
	if m == nil || len(m.keys) == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyBytes, err := marshalNoEscape(key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		valBytes, err := marshalNoEscape(m.values[key])
		if err != nil {
			return nil, err
		}
		buf.Write(valBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// ── Status ──────────────────────────────────────────────────────────────────

// StatusJSON matches `status --json` output from the bash CLI.
type StatusJSON struct {
	Name        string           `json:"name"`
	Phase       string           `json:"phase"`
	PhaseLabel  string           `json:"phase_label"`
	Complete    bool             `json:"complete"`
	Project     string           `json:"project"`
	Repo        string           `json:"repo"`
	AddDirs     []string         `json:"add_dirs"`
	Repos       ReposJSON        `json:"repos"`
	Description string           `json:"description"`
	Created     string           `json:"created"`
	WorkflowDir string           `json:"workflow_dir"`
	Agents      OrderedAgents    `json:"agents"`
	RetryCount  int              `json:"retry_count"`
	MaxRetries  int              `json:"max_retries"`
	Artifacts   OrderedArtifacts `json:"artifacts"`
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
	Agents     OrderedAgentNames `json:"agents"`
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
	Agents   OrderedAgentNames `json:"agents"`
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
// JSON shape varies by type; MarshalJSON handles the differences.
type MemoryShowJSON struct {
	Type    string
	Name    string
	Path    string
	Content *string
	Files   *OrderedFileMap
}

// MarshalJSON emits the correct JSON shape per memory type:
//   - workflow: {"type","name","path","content"} (content is null when file missing)
//   - global:   {"type","files"}
//   - project:  {"type","name","files"}
func (m MemoryShowJSON) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case "workflow":
		type wfMem struct {
			Type    string  `json:"type"`
			Name    string  `json:"name"`
			Path    string  `json:"path"`
			Content *string `json:"content"`
		}
		return marshalNoEscape(wfMem{m.Type, m.Name, m.Path, m.Content})
	case "global":
		type glMem struct {
			Type  string          `json:"type"`
			Files *OrderedFileMap `json:"files"`
		}
		return marshalNoEscape(glMem{m.Type, m.Files})
	case "project":
		type pjMem struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Files *OrderedFileMap `json:"files"`
		}
		return marshalNoEscape(pjMem{m.Type, m.Name, m.Files})
	default:
		type raw struct {
			Type    string          `json:"type"`
			Name    string          `json:"name,omitempty"`
			Path    string          `json:"path,omitempty"`
			Content *string         `json:"content"`
			Files   *OrderedFileMap `json:"files,omitempty"`
		}
		return marshalNoEscape(raw{m.Type, m.Name, m.Path, m.Content, m.Files})
	}
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

// marshalNoEscape marshals v to JSON without HTML escaping of &, <, >.
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	b := buf.Bytes()
	// json.Encoder.Encode appends a newline; strip it
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b, nil
}

// MarshalCompact marshals v to compact JSON without HTML escaping.
// Exported for use by callers that need compact sub-values in hybrid formatting.
func MarshalCompact(v any) ([]byte, error) {
	return marshalNoEscape(v)
}

// PrintJSON marshals v with 2-space indentation and prints to stdout with a trailing newline.
// HTML characters (&, <, >) are not escaped, matching bash CLI behavior.
func PrintJSON(v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	_, err := os.Stdout.Write(buf.Bytes())
	return err
}

// PrintJSONCompact marshals v without indentation and prints to stdout with a trailing newline.
// HTML characters (&, <, >) are not escaped, matching bash CLI behavior.
func PrintJSONCompact(v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	_, err := os.Stdout.Write(buf.Bytes())
	return err
}

// PrintStatusJSON writes status JSON in the exact hybrid format matching bash CLI:
// top-level keys indented 2 spaces, nested objects/arrays compact on the same line,
// agents/artifacts sub-keys indented 4 spaces with compact values.
func PrintStatusJSON(s StatusJSON) error {
	mc := func(v any) string {
		b, _ := marshalNoEscape(v)
		return string(b)
	}
	js := func(s string) string {
		b, _ := marshalNoEscape(s)
		return string(b)
	}

	var buf bytes.Buffer
	buf.WriteString("{\n")
	fmt.Fprintf(&buf, "  \"name\": %s,\n", js(s.Name))
	fmt.Fprintf(&buf, "  \"phase\": %s,\n", js(s.Phase))
	fmt.Fprintf(&buf, "  \"phase_label\": %s,\n", js(s.PhaseLabel))
	fmt.Fprintf(&buf, "  \"complete\": %t,\n", s.Complete)
	fmt.Fprintf(&buf, "  \"project\": %s,\n", js(s.Project))
	fmt.Fprintf(&buf, "  \"repo\": %s,\n", js(s.Repo))
	fmt.Fprintf(&buf, "  \"add_dirs\": %s,\n", mc(s.AddDirs))
	fmt.Fprintf(&buf, "  \"repos\": %s,\n", mc(s.Repos))
	fmt.Fprintf(&buf, "  \"description\": %s,\n", js(s.Description))
	fmt.Fprintf(&buf, "  \"created\": %s,\n", js(s.Created))
	fmt.Fprintf(&buf, "  \"workflow_dir\": %s,\n", js(s.WorkflowDir))
	buf.WriteString("  \"agents\": {\n")
	fmt.Fprintf(&buf, "    \"plan\": %s,\n", mc(s.Agents.Plan))
	fmt.Fprintf(&buf, "    \"review\": %s,\n", mc(s.Agents.Review))
	fmt.Fprintf(&buf, "    \"implement\": %s,\n", mc(s.Agents.Implement))
	fmt.Fprintf(&buf, "    \"verify\": %s\n", mc(s.Agents.Verify))
	buf.WriteString("  },\n")
	fmt.Fprintf(&buf, "  \"retry_count\": %d,\n", s.RetryCount)
	fmt.Fprintf(&buf, "  \"max_retries\": %d,\n", s.MaxRetries)
	buf.WriteString("  \"artifacts\": {\n")
	fmt.Fprintf(&buf, "    \"plan\": %s,\n", mc(s.Artifacts.Plan))
	fmt.Fprintf(&buf, "    \"review\": %s,\n", mc(s.Artifacts.Review))
	fmt.Fprintf(&buf, "    \"implement\": %s,\n", mc(s.Artifacts.Implement))
	fmt.Fprintf(&buf, "    \"verify\": %s,\n", mc(s.Artifacts.Verify))
	fmt.Fprintf(&buf, "    \"memory\": %s\n", mc(s.Artifacts.Memory))
	buf.WriteString("  }\n")
	buf.WriteString("}\n")

	_, err := os.Stdout.Write(buf.Bytes())
	return err
}
