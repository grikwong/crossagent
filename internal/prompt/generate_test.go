package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grikwong/crossagent/internal/state"
)

// setupTestEnv creates a temporary CROSSAGENT_HOME with required structure.
func setupTestEnv(t *testing.T) (home string, wfDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("CROSSAGENT_HOME", home)

	// Create required directories
	wfDir = filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.MkdirAll(filepath.Join(home, "agents"), 0755)
	os.MkdirAll(filepath.Join(home, "memory"), 0755)
	os.MkdirAll(filepath.Join(home, "projects", "test-proj", "memory"), 0755)

	// Write config
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/test-repo\nproject=test-proj\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature description\n"), 0644)

	// Initialize global memory
	state.InitGlobalMemory()
	state.InitProjectMemory("test-proj")

	return home, wfDir
}

func TestLoadTemplate(t *testing.T) {
	templates := []string{"general", "plan", "review", "implement", "verify"}
	for _, name := range templates {
		tmpl, err := LoadTemplate(name)
		if err != nil {
			t.Errorf("LoadTemplate(%q): %v", name, err)
			continue
		}
		if tmpl == nil {
			t.Errorf("LoadTemplate(%q) returned nil", name)
		}
	}
}

func TestLoadTemplateNotFound(t *testing.T) {
	_, err := LoadTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestGenerateGeneralInstructions(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GenerateGeneralInstructions(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Check key sections exist
	checks := []string{
		"# Crossagent — General Instructions",
		"## 1. Output File Gate",
		"## 2. Execution Mode",
		"## 3. Workflow Context",
		"## 9. Workspace Directories",
		"/add-dir " + wfDir,
		"/add-dir /tmp/test-repo",
		"Workflow directory",
		"Primary repository",
		"Global memory",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("general.md missing expected content: %q", check)
		}
	}
}

func TestGeneratePlanPrompt(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GeneratePlanPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"Phase 1: Create Implementation Plan",
		"## Feature Request",
		"Test feature description",
		"## Step 1: Understand the Request",
		"## Step 2: Write the Plan",
		filepath.Join(wfDir, "plan.md"),
		"## Memory Update Instructions",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("plan.md missing expected content: %q", check)
		}
	}

	// Should NOT have retry content
	if strings.Contains(content, "RETRY MODE") {
		t.Error("plan.md should not contain RETRY MODE without revert context")
	}
}

func TestGenerateReviewPrompt(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GenerateReviewPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"Phase 2: Review Implementation Plan",
		"## Review Criteria",
		"### Verdict",
		"**APPROVE** | **APPROVE WITH CHANGES** | **REQUEST REWORK**",
		filepath.Join(wfDir, "review.md"),
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("review.md missing expected content: %q", check)
		}
	}
}

func TestGenerateImplementPrompt(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GenerateImplementPrompt(wfDir, cfg, 2)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"Phase 3: Implement (Phase 2)",
		"Implement **Phase 2** of the plan",
		"Do NOT implement phases beyond Phase 2",
		filepath.Join(wfDir, "plan.md"),
		filepath.Join(wfDir, "review.md"),
		filepath.Join(wfDir, "implement.md"),
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("implement.md missing expected content: %q", check)
		}
	}
}

func TestGenerateVerifyPrompt(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GenerateVerifyPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"Phase 4: Verify Implementation",
		"## Verification Checklist",
		"### Status",
		"### Recommendation",
		"**Ship it** | **Fix issues first** | **Needs rework**",
		filepath.Join(wfDir, "verify.md"),
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("verify.md missing expected content: %q", check)
		}
	}
}

func TestGeneratePlanPromptRetryMode(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	// Create revert context to trigger retry mode
	os.WriteFile(filepath.Join(wfDir, "prompts", "revert-context.md"), []byte("Previous issues here\n"), 0644)
	// Update config with retry count
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/test-repo\nproject=test-proj\nretry_count=2\n"), 0644)

	cfg, _ := state.ReadConfig(wfDir)

	path, err := GeneratePlanPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "RETRY MODE") {
		t.Error("plan.md should contain RETRY MODE with revert context")
	}
	if !strings.Contains(content, "Attempt 2") {
		t.Error("plan.md should reference attempt number 2")
	}
	if !strings.Contains(content, "Previous issues here") {
		t.Error("plan.md should contain revert context content")
	}
}

func TestBuildMemoryContext(t *testing.T) {
	home, wfDir := setupTestEnv(t)

	// Create workflow memory with substantive content
	os.WriteFile(filepath.Join(wfDir, "memory.md"), []byte("# Workflow Memory\n\n## Task Summary\nTest task\n\n## Decisions\nDecision 1\n"), 0644)

	ctx, err := BuildMemoryContext(wfDir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(ctx, "<workflow-memory>") {
		t.Error("missing workflow-memory tags")
	}
	if !strings.Contains(ctx, "Workflow Memory") {
		t.Error("missing workflow memory content")
	}

	// Global context with default template should NOT be included (only 1 header row)
	if strings.Contains(ctx, "<global-context>") {
		t.Error("should not include default global context template")
	}

	// Add substantive global context
	globalCtx := filepath.Join(home, "memory", "global-context.md")
	os.WriteFile(globalCtx, []byte(`# Global Context

## Changelog

| Date | Change | Source Workflow |
|------|--------|----------------|
| 2026-03-01 | Added pattern | test-wf |
`), 0644)

	ctx, err = BuildMemoryContext(wfDir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(ctx, "<global-context>") {
		t.Error("should include global context with real entries")
	}
}

func TestBuildMemoryUpdateInstructions(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	instr, err := BuildMemoryUpdateInstructions(wfDir, "Implement")
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"## Memory Update Instructions",
		"Implement work",
		"### Workflow Memory (REQUIRED)",
		"### Project Memory",
		"### Global Context",
		"### Lessons Learned",
		filepath.Join(wfDir, "memory.md"),
	}

	for _, check := range checks {
		if !strings.Contains(instr, check) {
			t.Errorf("memory update instructions missing: %q", check)
		}
	}
}

// --- Parity tests: verify Go-generated prompts match bash output semantics ---
// These tests compare specific text blocks that appear verbatim in the bash
// heredoc output, ensuring the Go templates produce equivalent prompts.

func TestGeneralPromptParityWithBash(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GenerateGeneralInstructions(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// These are verbatim text blocks from the bash heredoc (gen_general_instructions).
	// If any of these fail, the Go template has drifted from bash.
	parityBlocks := []string{
		// Title (exact)
		"# Crossagent — General Instructions",
		// Output File Gate table (exact content)
		"| Plan      | `plan.md`        |",
		"| Review    | `review.md`      |",
		"| Implement | `implement.md`   |",
		"| Verify    | `verify.md`      |",
		// Skip gate instruction (exact)
		`"Output file already exists — skipping this phase." and exit.`,
		// Execution mode key phrase
		"You are running in **sandboxed auto-accept mode**.",
		// Workflow phase list (exact)
		"1. **Plan** — create a structured implementation plan",
		"2. **Review** — critically review the plan against the codebase",
		"3. **Implement** — execute the reviewed plan",
		"4. **Verify** — independently verify the implementation",
		// Memory system tiers (exact text from bash — note backticks)
		"- **Workflow memory** (\\`memory.md\\` in the workflow directory)",
		"- **Global memory** (\\`~/.crossagent/memory/\\`)",
		// Workspace directory formatting
		"/add-dir " + wfDir,
		"/add-dir /tmp/test-repo",
		"- **Workflow directory**: `" + wfDir + "` — where phase artifacts (plan.md, review.md, etc.) live",
		"- **Primary repository**: `/tmp/test-repo` — the main codebase",
	}

	for _, block := range parityBlocks {
		if !strings.Contains(content, block) {
			t.Errorf("general.md parity failure — missing verbatim bash block:\n  %q", block)
		}
	}
}

func TestPlanPromptParityWithBash(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GeneratePlanPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	planPath := filepath.Join(wfDir, "plan.md")
	descPath := filepath.Join(wfDir, "description")

	// Verbatim text blocks from bash gen_plan_prompt() heredoc
	parityBlocks := []string{
		"# Crossagent Phase 1: Create Implementation Plan",
		"You are the **Planner** in a dual-model AI workflow.",
		"## Feature Request\nTest feature description",
		"(Full description file: `" + descPath + "`)",
		"## Step 1: Understand the Request",
		"- Read the feature request above carefully",
		"- The scope of the change (what is in/out of scope)",
		"## Step 2: Write the Plan",
		"Create a detailed, actionable implementation plan and **write it to:**\n`" + planPath + "`",
		"#### 1. Overview",
		"#### 2. Affected Files",
		"#### 3. Implementation Phases",
		"#### 4. Risks & Edge Cases",
		"#### 5. Open Questions",
		"## CRITICAL\nYou MUST write the completed plan as a markdown file to:\n`" + planPath + "`",
		"The next workflow phase depends on this file existing.",
	}

	for _, block := range parityBlocks {
		if !strings.Contains(content, block) {
			t.Errorf("plan.md parity failure — missing verbatim bash block:\n  %q", block)
		}
	}

	// Negative: no retry content without revert context
	if strings.Contains(content, "RETRY MODE") {
		t.Error("plan.md should NOT contain RETRY MODE without revert context")
	}
}

func TestPlanPromptRetryParityWithBash(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	os.WriteFile(filepath.Join(wfDir, "prompts", "revert-context.md"), []byte("Previous review feedback here\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/test-repo\nproject=test-proj\nretry_count=3\n"), 0644)

	cfg, _ := state.ReadConfig(wfDir)

	path, err := GeneratePlanPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Verbatim bash retry block text
	parityBlocks := []string{
		"# RETRY MODE — Surgical Plan Update (Attempt 3)",
		"**This is NOT a fresh planning session.**",
		"`plan.attempt-3.md`",
		"Read the previous plan at `" + wfDir + "/plan.attempt-3.md`",
		"Update ONLY the parts of the plan that address the specific issues",
		"Keep all sections that were NOT flagged — do not rewrite them",
		"Previous review feedback here",
	}

	for _, block := range parityBlocks {
		if !strings.Contains(content, block) {
			t.Errorf("plan.md retry parity failure — missing verbatim bash block:\n  %q", block)
		}
	}
}

func TestReviewPromptParityWithBash(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GenerateReviewPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	planPath := filepath.Join(wfDir, "plan.md")
	reviewPath := filepath.Join(wfDir, "review.md")

	parityBlocks := []string{
		"# Crossagent Phase 2: Review Implementation Plan",
		"You are an **Adversarial Reviewer** in a dual-model AI workflow.",
		"Read the plan at: `" + planPath + "`",
		"## Review Criteria",
		"- **Completeness** — What files, steps, or dependencies did the planner miss?",
		"- **Correctness** — Does the approach actually match existing code patterns",
		"### Issues (Must Fix)",
		"### Suggestions (Nice to Have)",
		"### Phase Assessment",
		"### Verdict",
		"One of: **APPROVE** | **APPROVE WITH CHANGES** | **REQUEST REWORK**",
		"You MUST write your review to: `" + reviewPath + "`",
		"The implementation phase depends on this file.",
	}

	for _, block := range parityBlocks {
		if !strings.Contains(content, block) {
			t.Errorf("review.md parity failure — missing verbatim bash block:\n  %q", block)
		}
	}
}

func TestImplementPromptParityWithBash(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GenerateImplementPrompt(wfDir, cfg, 3)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	parityBlocks := []string{
		"# Crossagent Phase 3: Implement (Phase 3)",
		"You are the **Implementer** in a dual-model AI workflow.",
		"Implement **Phase 3** of the plan.",
		"Do NOT implement phases beyond Phase 3",
		"`" + filepath.Join(wfDir, "plan.md") + "`",
		"`" + filepath.Join(wfDir, "review.md") + "`",
		"`" + filepath.Join(wfDir, "implement.md") + "`",
		"Run tests after implementation to confirm test gates pass",
	}

	for _, block := range parityBlocks {
		if !strings.Contains(content, block) {
			t.Errorf("implement.md parity failure — missing verbatim bash block:\n  %q", block)
		}
	}
}

func TestVerifyPromptParityWithBash(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GenerateVerifyPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	parityBlocks := []string{
		"# Crossagent Phase 4: Verify Implementation",
		"You are a **Red-Team Verifier** in a dual-model AI workflow.",
		"## Verification Checklist",
		"1. **Plan Adherence** — Does implementation match the plan?",
		"2. **Review Compliance** — Were review \"Must Fix\" issues actually resolved",
		"### Status",
		"One of: **PASS** | **PASS WITH NOTES** | **FAIL**",
		"### Plan Drift",
		"### Issues Found",
		"### Positive Notes",
		"### Recommendation",
		"One of: **Ship it** | **Fix issues first** | **Needs rework**",
		"**Original plan:** `" + filepath.Join(wfDir, "plan.md") + "`",
		"**Review notes:** `" + filepath.Join(wfDir, "review.md") + "`",
		"You MUST write the verification report to: `" + filepath.Join(wfDir, "verify.md") + "`",
	}

	for _, block := range parityBlocks {
		if !strings.Contains(content, block) {
			t.Errorf("verify.md parity failure — missing verbatim bash block:\n  %q", block)
		}
	}
}

func TestGeneratePlanPromptFollowupMode(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	// Create followup context to trigger followup mode
	os.WriteFile(filepath.Join(wfDir, "prompts", "followup-context.md"),
		[]byte("# Previous Round 1 Context\n\nPlan was to add feature X.\n"), 0644)
	// Update config with followup_round
	os.WriteFile(filepath.Join(wfDir, "config"),
		[]byte("repo=/tmp/test-repo\nproject=test-proj\nfollowup_round=1\n"), 0644)

	cfg, _ := state.ReadConfig(wfDir)

	path, err := GeneratePlanPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "FOLLOWUP MODE") {
		t.Error("plan.md should contain FOLLOWUP MODE with followup context")
	}
	if !strings.Contains(content, "Round 1") {
		t.Error("plan.md should reference round number 1")
	}
	if !strings.Contains(content, "Previous Round 1 Context") {
		t.Error("plan.md should contain followup context content")
	}
	// Should NOT have retry content
	if strings.Contains(content, "RETRY MODE") {
		t.Error("plan.md should not contain RETRY MODE in followup mode")
	}
}

func TestGeneratePlanPromptRetryTakesPrecedenceOverFollowup(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	// Create both revert context AND followup context
	os.WriteFile(filepath.Join(wfDir, "prompts", "revert-context.md"),
		[]byte("Previous issues here\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "prompts", "followup-context.md"),
		[]byte("# Previous Round 1 Context\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"),
		[]byte("repo=/tmp/test-repo\nproject=test-proj\nretry_count=1\nfollowup_round=1\n"), 0644)

	cfg, _ := state.ReadConfig(wfDir)

	path, err := GeneratePlanPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "RETRY MODE") {
		t.Error("plan.md should contain RETRY MODE when both contexts exist")
	}
	if strings.Contains(content, "FOLLOWUP MODE") {
		t.Error("plan.md should NOT contain FOLLOWUP MODE when retry takes precedence")
	}
}

func TestGeneratePlanPromptNoFollowupOrRetry(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
	}

	path, err := GeneratePlanPrompt(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if strings.Contains(content, "FOLLOWUP MODE") {
		t.Error("plan.md should not contain FOLLOWUP MODE without followup context")
	}
	if strings.Contains(content, "RETRY MODE") {
		t.Error("plan.md should not contain RETRY MODE without revert context")
	}
}

func TestGenerateGeneralWithAddDirs(t *testing.T) {
	_, wfDir := setupTestEnv(t)

	cfg := &state.Config{
		Repo:    "/tmp/test-repo",
		Project: "test-proj",
		AddDirs: []string{"/extra/dir1", "/extra/dir2"},
	}

	path, err := GenerateGeneralInstructions(wfDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "/add-dir /extra/dir1") {
		t.Error("should contain add-dir for extra dir1")
	}
	if !strings.Contains(content, "/add-dir /extra/dir2") {
		t.Error("should contain add-dir for extra dir2")
	}
}
