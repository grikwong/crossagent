package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestStatusJSON_FieldNames(t *testing.T) {
	s := StatusJSON{
		Name:        "test-wf",
		Phase:       "2",
		PhaseLabel:  "review",
		Complete:    false,
		Project:     "default",
		Repo:        "/tmp/repo",
		AddDirs:     []string{},
		Repos:       ReposJSON{Primary: "/tmp/repo", Additional: []string{}},
		Description: "test description",
		Created:     "2026-03-13 10:00",
		WorkflowDir: "/home/.crossagent/workflows/test-wf",
		Agents: OrderedAgents{
			Plan:      AgentRefJSON{Name: "claude", DisplayName: "Claude Code"},
			Review:    AgentRefJSON{Name: "codex", DisplayName: "OpenAI Codex"},
			Implement: AgentRefJSON{Name: "claude", DisplayName: "Claude Code"},
			Verify:    AgentRefJSON{Name: "codex", DisplayName: "OpenAI Codex"},
		},
		RetryCount: 0,
		MaxRetries: 3,
		Artifacts: OrderedArtifacts{
			Plan:      ArtifactJSON{Exists: true, Path: "/home/.crossagent/workflows/test-wf/plan.md", Lines: 42},
			Review:    ArtifactJSON{Exists: false, Path: "/home/.crossagent/workflows/test-wf/review.md"},
			Implement: ArtifactJSON{Exists: false, Path: "/home/.crossagent/workflows/test-wf/implement.md"},
			Verify:    ArtifactJSON{Exists: false, Path: "/home/.crossagent/workflows/test-wf/verify.md"},
			Memory:    ArtifactJSON{Exists: true, Path: "/home/.crossagent/workflows/test-wf/memory.md", Lines: 20},
		},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	requiredKeys := []string{
		`"name"`, `"phase"`, `"phase_label"`, `"complete"`,
		`"project"`, `"repo"`, `"add_dirs"`, `"repos"`,
		`"description"`, `"created"`, `"workflow_dir"`,
		`"agents"`, `"retry_count"`, `"max_retries"`, `"artifacts"`,
		`"chat_history"`,
	}
	for _, key := range requiredKeys {
		if !strings.Contains(out, key) {
			t.Errorf("StatusJSON missing key %s in output: %s", key, out)
		}
	}
}

func TestStatusJSON_AgentKeyOrder(t *testing.T) {
	s := StatusJSON{
		AddDirs: []string{},
		Repos:   ReposJSON{Additional: []string{}},
		Agents: OrderedAgents{
			Plan:      AgentRefJSON{Name: "claude", DisplayName: "Claude Code"},
			Review:    AgentRefJSON{Name: "codex", DisplayName: "OpenAI Codex"},
			Implement: AgentRefJSON{Name: "claude", DisplayName: "Claude Code"},
			Verify:    AgentRefJSON{Name: "codex", DisplayName: "OpenAI Codex"},
		},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	// Verify keys appear in bash order: plan, review, implement, verify
	planIdx := strings.Index(out, `"plan"`)
	reviewIdx := strings.Index(out, `"review"`)
	implementIdx := strings.Index(out, `"implement"`)
	verifyIdx := strings.Index(out, `"verify"`)

	if planIdx >= reviewIdx || reviewIdx >= implementIdx || implementIdx >= verifyIdx {
		t.Errorf("agent keys not in expected order (plan < review < implement < verify) in: %s", out)
	}
}

func TestStatusJSON_ArtifactKeyOrder(t *testing.T) {
	s := StatusJSON{
		AddDirs: []string{},
		Repos:   ReposJSON{Additional: []string{}},
		Artifacts: OrderedArtifacts{
			Plan:      ArtifactJSON{Exists: true, Path: "/tmp/plan.md", Lines: 10},
			Review:    ArtifactJSON{Exists: false, Path: "/tmp/review.md"},
			Implement: ArtifactJSON{Exists: false, Path: "/tmp/implement.md"},
			Verify:    ArtifactJSON{Exists: false, Path: "/tmp/verify.md"},
			Memory:    ArtifactJSON{Exists: false, Path: "/tmp/memory.md"},
		},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	// Find positions within artifacts section
	artStart := strings.Index(out, `"artifacts"`)
	artSection := out[artStart:]
	planIdx := strings.Index(artSection, `"plan"`)
	reviewIdx := strings.Index(artSection, `"review"`)
	implementIdx := strings.Index(artSection, `"implement"`)
	verifyIdx := strings.Index(artSection, `"verify"`)
	memoryIdx := strings.Index(artSection, `"memory"`)

	if planIdx >= reviewIdx || reviewIdx >= implementIdx || implementIdx >= verifyIdx || verifyIdx >= memoryIdx {
		t.Errorf("artifact keys not in expected order (plan < review < implement < verify < memory) in: %s", artSection)
	}
}

func TestArtifactJSON_OmitEmpty(t *testing.T) {
	// When exists=false, lines should be omitted (0 value + omitempty)
	a := ArtifactJSON{Exists: false, Path: "/tmp/plan.md"}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"lines"`) {
		t.Errorf("ArtifactJSON with exists=false should omit lines, got: %s", data)
	}

	// When exists=true and lines>0, lines should be present
	a2 := ArtifactJSON{Exists: true, Path: "/tmp/plan.md", Lines: 42}
	data2, err := json.Marshal(a2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data2), `"lines":42`) {
		t.Errorf("ArtifactJSON with lines=42 should include lines, got: %s", data2)
	}
}

func TestListJSON_FieldNames(t *testing.T) {
	l := ListJSON{
		Workflows: []ListWorkflowJSON{
			{
				Name:       "wf1",
				Phase:      "1",
				PhaseLabel: "plan",
				Active:     true,
				Project:    "default",
				Agents: OrderedAgentNames{
					Plan: "claude", Review: "codex", Implement: "claude", Verify: "codex",
				},
			},
		},
		Projects: []ListProjectJSON{
			{Name: "default", WorkflowCount: 1},
		},
		Active: "wf1",
	}

	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	for _, key := range []string{`"workflows"`, `"projects"`, `"active"`, `"phase_label"`, `"workflow_count"`} {
		if !strings.Contains(out, key) {
			t.Errorf("ListJSON missing key %s in output: %s", key, out)
		}
	}
}

func TestListJSON_AgentKeyOrder(t *testing.T) {
	l := ListJSON{
		Workflows: []ListWorkflowJSON{
			{
				Name: "wf1",
				Agents: OrderedAgentNames{
					Plan: "claude", Review: "codex", Implement: "claude", Verify: "codex",
				},
			},
		},
		Projects: make([]ListProjectJSON, 0),
	}

	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	planIdx := strings.Index(out, `"plan"`)
	reviewIdx := strings.Index(out, `"review"`)
	implementIdx := strings.Index(out, `"implement"`)
	verifyIdx := strings.Index(out, `"verify"`)

	if planIdx >= reviewIdx || reviewIdx >= implementIdx || implementIdx >= verifyIdx {
		t.Errorf("list agent keys not in expected order: %s", out)
	}
}

func TestAgentsListJSON_FieldNames(t *testing.T) {
	a := AgentsListJSON{
		Agents: []AgentJSON{
			{Name: "claude", DisplayName: "Claude Code", Adapter: "claude", Command: "claude", Builtin: true},
		},
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	for _, key := range []string{`"display_name"`, `"adapter"`, `"command"`, `"builtin"`} {
		if !strings.Contains(out, key) {
			t.Errorf("AgentsListJSON missing key %s in output: %s", key, out)
		}
	}
}

func TestAgentsShowJSON_KeyOrder(t *testing.T) {
	a := AgentsShowJSON{
		Workflow: "test-wf",
		Agents: OrderedAgentNames{
			Plan: "claude", Review: "codex", Implement: "claude", Verify: "codex",
		},
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	planIdx := strings.Index(out, `"plan"`)
	reviewIdx := strings.Index(out, `"review"`)
	implementIdx := strings.Index(out, `"implement"`)
	verifyIdx := strings.Index(out, `"verify"`)

	if planIdx >= reviewIdx || reviewIdx >= implementIdx || implementIdx >= verifyIdx {
		t.Errorf("agents show keys not in expected order: %s", out)
	}
}

func TestProjectSuggestJSON_NullProject(t *testing.T) {
	// When no match, suggested_project should be null
	s := ProjectSuggestJSON{SuggestedProject: nil, Score: 0, MatchedTerms: ""}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"suggested_project":null`) {
		t.Errorf("expected null suggested_project, got: %s", data)
	}

	// When match, suggested_project should be a string
	proj := "my-project"
	s2 := ProjectSuggestJSON{SuggestedProject: &proj, Score: 35, MatchedTerms: "my-project"}
	data2, err := json.Marshal(s2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data2), `"suggested_project":"my-project"`) {
		t.Errorf("expected string suggested_project, got: %s", data2)
	}
}

func TestRevertJSON_OmitEmpty(t *testing.T) {
	// needs_human action: only action, reason, retry_count, max_retries
	r := RevertJSON{
		Action:     "needs_human",
		Reason:     "Retry limit reached (3/3)",
		RetryCount: 3,
		MaxRetries: 3,
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	// Should NOT have target_phase, target_label, attempt, revert_context (all zero/empty with omitempty)
	if strings.Contains(out, `"target_phase"`) {
		t.Errorf("needs_human should not have target_phase, got: %s", out)
	}
}

func TestMemoryShowJSON_WorkflowWithContent(t *testing.T) {
	content := "# Memory"
	m := MemoryShowJSON{
		Type:    "workflow",
		Name:    "test-wf",
		Path:    "/tmp/memory.md",
		Content: &content,
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, `"content":"# Memory"`) {
		t.Errorf("expected content string, got: %s", out)
	}
}

func TestMemoryShowJSON_WorkflowContentNull(t *testing.T) {
	// When file doesn't exist, content should be null (not omitted).
	// Matches bash: printf '{"type":"workflow","name":"%s","path":"%s","content":null}'
	m := MemoryShowJSON{
		Type: "workflow",
		Name: "test-wf",
		Path: "/tmp/memory.md",
		// Content is nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, `"content":null`) {
		t.Errorf("workflow memory with no file should have content:null, got: %s", out)
	}
}

func TestMemoryShowJSON_GlobalShape(t *testing.T) {
	// Global memory should have type + files only (no name, path, content)
	files := NewOrderedFileMap()
	files.Set("global-context.md", MemoryFileJSON{Path: "/tmp/gc.md", Content: "gc"})
	files.Set("lessons-learned.md", MemoryFileJSON{Path: "/tmp/ll.md", Content: "ll"})
	m := MemoryShowJSON{Type: "global", Files: files}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if strings.Contains(out, `"name"`) {
		t.Errorf("global memory should not have name field, got: %s", out)
	}
	if strings.Contains(out, `"content":null`) {
		t.Errorf("global memory should not have content field, got: %s", out)
	}
	if !strings.Contains(out, `"files"`) {
		t.Errorf("global memory should have files field, got: %s", out)
	}
}

func TestMemoryShowJSON_GlobalFileOrder(t *testing.T) {
	// Verify insertion order is preserved (global-context.md before lessons-learned.md)
	files := NewOrderedFileMap()
	files.Set("global-context.md", MemoryFileJSON{Path: "/tmp/gc.md", Content: "gc"})
	files.Set("lessons-learned.md", MemoryFileJSON{Path: "/tmp/ll.md", Content: "ll"})
	m := MemoryShowJSON{Type: "global", Files: files}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	gcIdx := strings.Index(out, `"global-context.md"`)
	llIdx := strings.Index(out, `"lessons-learned.md"`)
	if gcIdx >= llIdx {
		t.Errorf("global memory files not in insertion order (global-context.md should come before lessons-learned.md): %s", out)
	}
}

func TestMemoryShowJSON_ProjectShape(t *testing.T) {
	// Project memory should have type + name + files (no top-level path or content)
	files := NewOrderedFileMap()
	files.Set("context.md", MemoryFileJSON{Path: "/tmp/ctx.md", Content: "ctx"})
	m := MemoryShowJSON{Type: "project", Name: "my-proj", Files: files}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	// Unmarshal into a generic map to check top-level keys
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(parsed["name"]), `my-proj`) {
		t.Errorf("project memory should have name, got keys: %v", parsed)
	}
	if _, ok := parsed["path"]; ok {
		t.Errorf("project memory should not have top-level path field")
	}
	if _, ok := parsed["content"]; ok {
		t.Errorf("project memory should not have top-level content field")
	}
}

func TestPrintJSON_Format(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	v := map[string]string{"key": "value"}
	err := PrintJSON(v)
	if err != nil {
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should have indentation
	if !strings.Contains(output, "  ") {
		t.Errorf("PrintJSON should produce indented output, got: %s", output)
	}

	// Should end with newline
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("PrintJSON should end with newline, got: %q", output)
	}

	// Should be valid JSON
	var parsed map[string]string
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("PrintJSON output should be valid JSON: %v", err)
	}
}

func TestPrintJSON_NoHTMLEscape(t *testing.T) {
	// Verify that &, <, > are not escaped to \u0026, \u003c, \u003e
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	v := map[string]string{"cmd": "foo && bar > baz"}
	err := PrintJSON(v)
	if err != nil {
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if strings.Contains(output, `\u0026`) {
		t.Errorf("PrintJSON should not HTML-escape &, got: %s", output)
	}
	if strings.Contains(output, `\u003e`) {
		t.Errorf("PrintJSON should not HTML-escape >, got: %s", output)
	}
	if !strings.Contains(output, `&&`) {
		t.Errorf("PrintJSON should preserve && as-is, got: %s", output)
	}
}

func TestPrintJSONCompact_Format(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	v := map[string]string{"key": "value"}
	err := PrintJSONCompact(v)
	if err != nil {
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should be single line (no newlines except trailing)
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("PrintJSONCompact should produce single line, got %d lines: %s", len(lines), output)
	}

	if !strings.HasSuffix(output, "\n") {
		t.Errorf("PrintJSONCompact should end with newline, got: %q", output)
	}
}

func TestPrintJSONCompact_NoHTMLEscape(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	v := map[string]string{"cmd": "a < b && c > d"}
	err := PrintJSONCompact(v)
	if err != nil {
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if strings.Contains(output, `\u0026`) || strings.Contains(output, `\u003c`) || strings.Contains(output, `\u003e`) {
		t.Errorf("PrintJSONCompact should not HTML-escape, got: %s", output)
	}
}

func TestEmptySlicesNotNull(t *testing.T) {
	// Verify that initialized empty slices marshal as [] not null
	l := ListJSON{
		Workflows: make([]ListWorkflowJSON, 0),
		Projects:  make([]ListProjectJSON, 0),
		Active:    "",
	}
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if strings.Contains(out, "null") {
		t.Errorf("empty slices should marshal as [], not null. Got: %s", out)
	}
}

func TestNilSlicesAreNull(t *testing.T) {
	// Verify that nil slices marshal as null — callers must use make([]T, 0) to get []
	l := ListJSON{Active: ""}
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, `"workflows":null`) {
		t.Errorf("nil slice should marshal as null. Got: %s", out)
	}
}

func TestReposJSON_EmptyAdditional(t *testing.T) {
	r := ReposJSON{Primary: "/tmp/repo", Additional: make([]string, 0)}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"additional":[]`) {
		t.Errorf("empty additional should be [], got: %s", data)
	}
}

func TestSuperviseJSON_FieldVariants(t *testing.T) {
	// done action from verify
	s := SuperviseJSON{
		Action:         "done",
		Verdict:        "pass",
		Status:         "Ship It",
		Recommendation: "No issues found",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, key := range []string{`"action"`, `"verdict"`, `"status"`, `"recommendation"`} {
		if !strings.Contains(out, key) {
			t.Errorf("SuperviseJSON done missing key %s: %s", key, out)
		}
	}

	// pass action from review
	s2 := SuperviseJSON{
		Action:        "pass",
		Verdict:       "approve",
		ReviewVerdict: "APPROVE WITH CHANGES",
	}
	data2, err := json.Marshal(s2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data2), `"review_verdict"`) {
		t.Errorf("SuperviseJSON pass from review missing review_verdict: %s", data2)
	}
}

func TestOrderedFileMap_MarshalOrder(t *testing.T) {
	m := NewOrderedFileMap()
	m.Set("b.md", MemoryFileJSON{Path: "/b.md", Content: "b"})
	m.Set("a.md", MemoryFileJSON{Path: "/a.md", Content: "a"})

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	bIdx := strings.Index(out, `"b.md"`)
	aIdx := strings.Index(out, `"a.md"`)
	if bIdx >= aIdx {
		t.Errorf("OrderedFileMap should preserve insertion order (b before a), got: %s", out)
	}
}

func TestOrderedFileMap_NoHTMLEscape(t *testing.T) {
	m := NewOrderedFileMap()
	m.Set("test.md", MemoryFileJSON{Path: "/test.md", Content: "a && b > c"})

	// Use marshalNoEscape (same as PrintJSON/PrintJSONCompact path) since
	// json.Marshal always re-escapes MarshalJSON output.
	data, err := marshalNoEscape(m)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if strings.Contains(out, `\u0026`) || strings.Contains(out, `\u003e`) {
		t.Errorf("OrderedFileMap should not HTML-escape via marshalNoEscape, got: %s", out)
	}
}

func TestPrintStatusJSON_HybridFormat(t *testing.T) {
	// Verify PrintStatusJSON produces the bash-compatible hybrid format:
	// top-level keys indented, nested objects compact.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	s := StatusJSON{
		Name:        "test-wf",
		Phase:       "2",
		PhaseLabel:  "review",
		Complete:    false,
		Project:     "default",
		Repo:        "/tmp/repo",
		AddDirs:     []string{},
		Repos:       ReposJSON{Primary: "/tmp/repo", Additional: []string{}},
		Description: "test description",
		Created:     "2026-03-13 10:00",
		WorkflowDir: "/home/.crossagent/workflows/test-wf",
		Agents: OrderedAgents{
			Plan:      AgentRefJSON{Name: "claude", DisplayName: "Claude Code"},
			Review:    AgentRefJSON{Name: "codex", DisplayName: "OpenAI Codex"},
			Implement: AgentRefJSON{Name: "claude", DisplayName: "Claude Code"},
			Verify:    AgentRefJSON{Name: "codex", DisplayName: "OpenAI Codex"},
		},
		RetryCount: 0,
		MaxRetries: 3,
		Artifacts: OrderedArtifacts{
			Plan:      ArtifactJSON{Exists: true, Path: "/home/.crossagent/workflows/test-wf/plan.md", Lines: 42},
			Review:    ArtifactJSON{Exists: false, Path: "/home/.crossagent/workflows/test-wf/review.md"},
			Implement: ArtifactJSON{Exists: false, Path: "/home/.crossagent/workflows/test-wf/implement.md"},
			Verify:    ArtifactJSON{Exists: false, Path: "/home/.crossagent/workflows/test-wf/verify.md"},
			Memory:    ArtifactJSON{Exists: true, Path: "/home/.crossagent/workflows/test-wf/memory.md", Lines: 20},
		},
		ChatHistory: OrderedChatHistory{
			Plan:      ChatHistoryEntry{Exists: false, Path: "/home/.crossagent/workflows/test-wf/chat-history/plan.log"},
			Review:    ChatHistoryEntry{Exists: false, Path: "/home/.crossagent/workflows/test-wf/chat-history/review.log"},
			Implement: ChatHistoryEntry{Exists: false, Path: "/home/.crossagent/workflows/test-wf/chat-history/implement.log"},
			Verify:    ChatHistoryEntry{Exists: false, Path: "/home/.crossagent/workflows/test-wf/chat-history/verify.log"},
		},
	}

	err := PrintStatusJSON(s)
	if err != nil {
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("PrintStatusJSON output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Verify hybrid format: top-level indented, agents/artifacts sub-keys indented,
	// nested objects compact (on same line as their key).
	lines := strings.Split(output, "\n")

	// Check that agents sub-keys are indented 4 spaces with compact values
	foundPlanAgent := false
	for _, line := range lines {
		if strings.HasPrefix(line, `    "plan": {"name":`) {
			foundPlanAgent = true
			break
		}
	}
	if !foundPlanAgent {
		t.Errorf("PrintStatusJSON should have agents.plan as 4-space indented compact object.\nOutput:\n%s", output)
	}

	// Check that repos is compact on same line
	foundRepos := false
	for _, line := range lines {
		if strings.HasPrefix(line, `  "repos": {"primary":`) {
			foundRepos = true
			break
		}
	}
	if !foundRepos {
		t.Errorf("PrintStatusJSON should have repos as compact object on same line.\nOutput:\n%s", output)
	}

	// Check that artifacts sub-keys are indented 4 spaces with compact values
	foundPlanArtifact := false
	for _, line := range lines {
		if strings.HasPrefix(line, `    "plan": {"exists":`) {
			foundPlanArtifact = true
			break
		}
	}
	if !foundPlanArtifact {
		t.Errorf("PrintStatusJSON should have artifacts.plan as 4-space indented compact object.\nOutput:\n%s", output)
	}
}

func TestStatusJSON_ChatHistoryField(t *testing.T) {
	s := StatusJSON{
		Name:    "test-wf",
		AddDirs: []string{},
		Repos:   ReposJSON{Additional: []string{}},
		ChatHistory: OrderedChatHistory{
			Plan:      ChatHistoryEntry{Exists: true, Path: "/tmp/chat-history/plan.log", Size: 1024},
			Review:    ChatHistoryEntry{Exists: false, Path: "/tmp/chat-history/review.log"},
			Implement: ChatHistoryEntry{Exists: false, Path: "/tmp/chat-history/implement.log"},
			Verify:    ChatHistoryEntry{Exists: false, Path: "/tmp/chat-history/verify.log"},
		},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	if !strings.Contains(out, `"chat_history"`) {
		t.Errorf("StatusJSON missing chat_history key in output: %s", out)
	}
	if !strings.Contains(out, `"size":1024`) {
		t.Errorf("ChatHistoryEntry with size=1024 should include size, got: %s", out)
	}

	// Verify size is omitted when 0 (exists=false)
	if strings.Count(out, `"size"`) != 1 {
		t.Errorf("ChatHistoryEntry with exists=false should omit size, got: %s", out)
	}
}

func TestStatusJSON_ChatHistoryKeyOrder(t *testing.T) {
	s := StatusJSON{
		AddDirs: []string{},
		Repos:   ReposJSON{Additional: []string{}},
		ChatHistory: OrderedChatHistory{
			Plan:      ChatHistoryEntry{Exists: true, Path: "/tmp/plan.log", Size: 100},
			Review:    ChatHistoryEntry{Exists: false, Path: "/tmp/review.log"},
			Implement: ChatHistoryEntry{Exists: false, Path: "/tmp/implement.log"},
			Verify:    ChatHistoryEntry{Exists: false, Path: "/tmp/verify.log"},
		},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	out := string(data)
	// Find positions within chat_history section
	chStart := strings.Index(out, `"chat_history"`)
	chSection := out[chStart:]
	planIdx := strings.Index(chSection, `"plan"`)
	reviewIdx := strings.Index(chSection, `"review"`)
	implementIdx := strings.Index(chSection, `"implement"`)
	verifyIdx := strings.Index(chSection, `"verify"`)

	if planIdx >= reviewIdx || reviewIdx >= implementIdx || implementIdx >= verifyIdx {
		t.Errorf("chat_history keys not in expected order (plan < review < implement < verify) in: %s", chSection)
	}
}

func TestPrintStatusJSON_ChatHistory(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	s := StatusJSON{
		Name:    "test-wf",
		AddDirs: []string{},
		Repos:   ReposJSON{Primary: "/tmp/repo", Additional: []string{}},
		Agents: OrderedAgents{
			Plan:      AgentRefJSON{Name: "claude", DisplayName: "Claude Code"},
			Review:    AgentRefJSON{Name: "codex", DisplayName: "OpenAI Codex"},
			Implement: AgentRefJSON{Name: "claude", DisplayName: "Claude Code"},
			Verify:    AgentRefJSON{Name: "codex", DisplayName: "OpenAI Codex"},
		},
		ChatHistory: OrderedChatHistory{
			Plan:      ChatHistoryEntry{Exists: true, Path: "/tmp/plan.log", Size: 512},
			Review:    ChatHistoryEntry{Exists: false, Path: "/tmp/review.log"},
			Implement: ChatHistoryEntry{Exists: false, Path: "/tmp/implement.log"},
			Verify:    ChatHistoryEntry{Exists: false, Path: "/tmp/verify.log"},
		},
	}

	err := PrintStatusJSON(s)
	if err != nil {
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should be valid JSON
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("PrintStatusJSON output with chat_history is not valid JSON: %v\nOutput: %s", err, output)
	}

	if _, ok := parsed["chat_history"]; !ok {
		t.Errorf("PrintStatusJSON output should contain chat_history key.\nOutput:\n%s", output)
	}

	// Verify chat_history sub-keys are indented 4 spaces
	foundPlanChat := false
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, `    "plan": {"exists":`) {
			foundPlanChat = true
			break
		}
	}
	if !foundPlanChat {
		t.Errorf("PrintStatusJSON should have chat_history.plan as 4-space indented compact object.\nOutput:\n%s", output)
	}
}

func TestMarshalCompact(t *testing.T) {
	// Verify MarshalCompact produces compact JSON without HTML escaping
	data, err := MarshalCompact(map[string]string{"cmd": "a && b > c"})
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if strings.Contains(out, `\u0026`) || strings.Contains(out, `\u003e`) {
		t.Errorf("MarshalCompact should not HTML-escape, got: %s", out)
	}
	if strings.Contains(out, "\n") {
		t.Errorf("MarshalCompact should be single-line, got: %s", out)
	}
}
