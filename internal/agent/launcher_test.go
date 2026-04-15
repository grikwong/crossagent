package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grikwong/crossagent/internal/state"
)

func TestBuildLaunchArgs(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(wfDir, 0755)

	args := BuildLaunchArgs(wfDir, nil, "")

	// Should contain --add-dir wfDir --add-dir globalMemDir
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}
	if args[0] != "--add-dir" || args[1] != wfDir {
		t.Errorf("first --add-dir should be wfDir, got %v", args[:2])
	}
	globalMemDir := state.GlobalMemoryDir()
	if args[2] != "--add-dir" || args[3] != globalMemDir {
		t.Errorf("second --add-dir should be globalMemDir, got %v", args[2:4])
	}
}

func TestBuildLaunchArgsWithProjectMem(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(wfDir, 0755)

	projMemDir := filepath.Join(home, "projects", "default", "memory")
	// projMemDir already created by setupTestHome

	args := BuildLaunchArgs(wfDir, nil, projMemDir)

	// Should contain 3 --add-dir entries
	if len(args) != 6 {
		t.Fatalf("expected 6 args, got %d: %v", len(args), args)
	}
	if args[4] != "--add-dir" || args[5] != projMemDir {
		t.Errorf("third --add-dir should be projMemDir, got %v", args[4:6])
	}
}

func TestBuildLaunchArgsWithAddDirs(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(wfDir, 0755)

	args := BuildLaunchArgs(wfDir, []string{"/extra/dir1", "/extra/dir2"}, "")

	// wfDir + globalMem + 2 extra = 4 entries = 8 args
	if len(args) != 8 {
		t.Fatalf("expected 8 args, got %d: %v", len(args), args)
	}
}

func TestGenSandboxSettings(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)

	repo := "/Users/test/repo"

	path, err := GenSandboxSettings(wfDir, repo, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify sandbox structure
	sandbox, ok := parsed["sandbox"].(map[string]interface{})
	if !ok {
		t.Fatal("missing sandbox key")
	}
	if enabled, ok := sandbox["enabled"].(bool); !ok || !enabled {
		t.Error("sandbox.enabled should be true")
	}

	fs, ok := sandbox["filesystem"].(map[string]interface{})
	if !ok {
		t.Fatal("missing filesystem key")
	}
	allowWrite, ok := fs["allowWrite"].([]interface{})
	if !ok {
		t.Fatal("missing allowWrite key")
	}

	// Should have wfDir, repo, globalMemDir
	if len(allowWrite) < 3 {
		t.Fatalf("expected at least 3 allowWrite entries, got %d", len(allowWrite))
	}

	// Check // prefix convention
	for _, p := range allowWrite {
		s, ok := p.(string)
		if !ok {
			t.Error("allowWrite entry should be string")
			continue
		}
		if !strings.HasPrefix(s, "//") {
			t.Errorf("allowWrite path should start with //, got %q", s)
		}
	}
}

func TestGenSandboxSettingsWithAddDirs(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)

	repo := "/Users/test/repo"
	projMemDir := filepath.Join(home, "projects", "default", "memory")

	path, err := GenSandboxSettings(wfDir, repo, []string{"/extra"}, projMemDir)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	sandbox := parsed["sandbox"].(map[string]interface{})
	fs := sandbox["filesystem"].(map[string]interface{})
	allowWrite := fs["allowWrite"].([]interface{})

	// wfDir + repo + globalMem + projMem + extra = 5
	if len(allowWrite) != 5 {
		t.Fatalf("expected 5 allowWrite entries, got %d: %v", len(allowWrite), allowWrite)
	}
}

func TestRequireAgentCommandBuiltin(t *testing.T) {
	// Test with a command that exists on PATH (e.g., "ls")
	agent := &Agent{Name: "test", Command: "ls"}
	err := RequireAgentCommand(agent)
	if err != nil {
		t.Errorf("expected no error for 'ls', got: %v", err)
	}
}

func TestRequireAgentCommandNotFound(t *testing.T) {
	agent := &Agent{Name: "test", Command: "nonexistent-command-xyz"}
	err := RequireAgentCommand(agent)
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestRequireAgentCommandEmpty(t *testing.T) {
	agent := &Agent{Name: "test", Command: ""}
	err := RequireAgentCommand(agent)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestRequireAgentCommandAbsolutePath(t *testing.T) {
	// Create a temp executable
	dir := t.TempDir()
	cmdPath := filepath.Join(dir, "test-cmd")
	os.WriteFile(cmdPath, []byte("#!/bin/sh\n"), 0755)

	agent := &Agent{Name: "test", Command: cmdPath}
	err := RequireAgentCommand(agent)
	if err != nil {
		t.Errorf("expected no error for executable path, got: %v", err)
	}
}

func TestRequireAgentCommandAbsolutePathNotExecutable(t *testing.T) {
	dir := t.TempDir()
	cmdPath := filepath.Join(dir, "test-cmd")
	os.WriteFile(cmdPath, []byte("not executable"), 0644)

	agent := &Agent{Name: "test", Command: cmdPath}
	err := RequireAgentCommand(agent)
	if err == nil {
		t.Error("expected error for non-executable file")
	}
}

func TestPhaseCmdResultJSON(t *testing.T) {
	outputFile := "/tmp/test/plan.md"
	result := &PhaseCmdResult{
		Agent: PhaseCmdAgent{
			Name:        "claude",
			DisplayName: "Claude Code",
			Adapter:     "claude",
		},
		Command:     "claude",
		Args:        []string{"--permission-mode", "auto", "--", "prompt"},
		Cwd:         "/test/repo",
		Prompt:      "Read and follow the instructions at /tmp/test.md",
		PromptFile:  "/tmp/test/prompts/plan.md",
		OutputFile:  &outputFile,
		Phase:       1,
		PhaseLabel:  "plan",
		Workflow:    "test-wf",
		WorkflowDir: "/tmp/test",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	// Verify JSON has all expected keys
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	expectedKeys := []string{"agent", "command", "args", "cwd", "prompt", "prompt_file", "output_file", "phase", "phase_label", "workflow", "workflow_dir"}
	for _, key := range expectedKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing expected key %q in JSON", key)
		}
	}

	// Verify nested agent object
	agentObj, ok := parsed["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("agent should be a nested object")
	}
	if agentObj["name"] != "claude" {
		t.Errorf("agent.name = %v, want %q", agentObj["name"], "claude")
	}
}

func TestBuildPhaseCmdPlanPhase(t *testing.T) {
	home := setupTestHome(t)

	// Create a workflow with phase=1
	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("1\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\nproject=default\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature\n"), 0644)

	// Create the prompt file so the codex adapter can read it
	os.WriteFile(filepath.Join(wfDir, "prompts", "plan.md"), []byte("Plan prompt\n"), 0644)

	result, err := BuildPhaseCmd(wfDir, "test-wf", "plan", false, 1)
	if err != nil {
		t.Fatal(err)
	}

	if result.Phase != 1 {
		t.Errorf("phase = %d, want 1", result.Phase)
	}
	if result.PhaseLabel != "plan" {
		t.Errorf("phase_label = %q, want %q", result.PhaseLabel, "plan")
	}
	if result.Agent.Name != "claude" {
		t.Errorf("agent.name = %q, want %q", result.Agent.Name, "claude")
	}
	if result.Workflow != "test-wf" {
		t.Errorf("workflow = %q, want %q", result.Workflow, "test-wf")
	}
}

func TestBuildPhaseCmdReviewRequiresPlan(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("1\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\n"), 0644)

	_, err := BuildPhaseCmd(wfDir, "test-wf", "review", false, 1)
	if err == nil {
		t.Fatal("expected error: review requires phase >= 2")
	}
}

func TestBuildPhaseCmdReviewRequiresPlanFile(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("2\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\n"), 0644)
	// No plan.md file

	_, err := BuildPhaseCmd(wfDir, "test-wf", "review", false, 1)
	if err == nil {
		t.Fatal("expected error: plan.md missing")
	}
}

func TestBuildPhaseCmdImplementRequiresPositiveSubPhase(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("3\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\nproject=default\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature\n"), 0644)

	// implPhase=0 should fail
	_, err := BuildPhaseCmd(wfDir, "test-wf", "implement", false, 0)
	if err == nil {
		t.Fatal("expected error for implPhase=0")
	}
	if !strings.Contains(err.Error(), "positive integer") {
		t.Errorf("error should mention positive integer, got: %v", err)
	}

	// implPhase=-1 should fail
	_, err = BuildPhaseCmd(wfDir, "test-wf", "implement", false, -1)
	if err == nil {
		t.Fatal("expected error for implPhase=-1")
	}

	// implPhase=1 should succeed
	state.InitGlobalMemory()
	state.InitProjectMemory("default")
	result, err := BuildPhaseCmd(wfDir, "test-wf", "implement", false, 1)
	if err != nil {
		t.Fatalf("expected success for implPhase=1, got: %v", err)
	}
	if result.Phase != 3 {
		t.Errorf("phase = %d, want 3", result.Phase)
	}
}

func TestBuildPhaseCmdGeneratesPromptFiles(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("1\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\nproject=default\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature\n"), 0644)

	state.InitGlobalMemory()
	state.InitProjectMemory("default")

	// Remove any pre-existing prompt file to prove BuildPhaseCmd generates it
	promptFile := filepath.Join(wfDir, "prompts", "plan.md")
	os.Remove(promptFile)

	result, err := BuildPhaseCmd(wfDir, "test-wf", "plan", false, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Verify prompt file was generated (not just referenced)
	if _, err := os.Stat(result.PromptFile); os.IsNotExist(err) {
		t.Fatal("BuildPhaseCmd should generate the prompt file, but it does not exist")
	}

	// Verify the generated prompt has substantive content
	data, err := os.ReadFile(result.PromptFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Phase 1: Create Implementation Plan") {
		t.Error("generated prompt should contain plan phase header")
	}
	if !strings.Contains(content, "Test feature") {
		t.Error("generated prompt should contain the workflow description")
	}

	// Verify general.md was also generated
	generalFile := filepath.Join(wfDir, "prompts", "general.md")
	if _, err := os.Stat(generalFile); os.IsNotExist(err) {
		t.Fatal("BuildPhaseCmd should also generate general.md")
	}
}

func TestBuildPhaseCmdJSONParity(t *testing.T) {
	// Verify BuildPhaseCmd output has the exact JSON shape expected by the Web UI,
	// matching bash phase-cmd --json contract.
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("1\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\nproject=default\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature\n"), 0644)

	state.InitGlobalMemory()
	state.InitProjectMemory("default")

	result, err := BuildPhaseCmd(wfDir, "test-wf", "plan", false, 1)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	// All keys from the bash phase-cmd --json contract
	bashContractKeys := []string{
		"agent", "command", "args", "cwd", "prompt", "prompt_file",
		"output_file", "phase", "phase_label", "workflow", "workflow_dir",
	}
	for _, key := range bashContractKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("phase-cmd JSON parity failure: missing key %q", key)
		}
	}

	// Verify nested agent sub-keys
	agentObj, ok := parsed["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("agent should be a nested object")
	}
	for _, key := range []string{"name", "display_name", "adapter"} {
		if _, ok := agentObj[key]; !ok {
			t.Errorf("agent object parity failure: missing key %q", key)
		}
	}

	// Verify prompt_file points to a generated file that exists
	promptFile, _ := parsed["prompt_file"].(string)
	if _, err := os.Stat(promptFile); os.IsNotExist(err) {
		t.Error("prompt_file should point to a generated file that exists on disk")
	}

	// Verify prompt content for claude adapter
	promptText, _ := parsed["prompt"].(string)
	if !strings.Contains(promptText, "Read and follow the instructions at") {
		t.Error("prompt should contain claude-style instruction text")
	}

	// Verify generated prompt file has substantive plan prompt content
	promptData, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(promptData), "Phase 1: Create Implementation Plan") {
		t.Error("generated prompt file should contain plan phase content")
	}
}

func TestCodexTrustArgs(t *testing.T) {
	repo := "/Users/test/repo"
	got := codexTrustArgs(repo)

	if len(got) != 2 {
		t.Fatalf("codexTrustArgs should return 2 args, got %d: %v", len(got), got)
	}
	if got[0] != "-c" {
		t.Errorf("codexTrustArgs[0] = %q, want %q", got[0], "-c")
	}
	want := `projects."/Users/test/repo".trust_level="trusted"`
	if got[1] != want {
		t.Errorf("codexTrustArgs[1] = %q, want %q", got[1], want)
	}
}

func TestCodexTrustArgsWithSpaces(t *testing.T) {
	// Repo paths with spaces must survive the %q escape as a valid TOML key segment.
	repo := "/Users/test/my repo"
	got := codexTrustArgs(repo)
	want := `projects."/Users/test/my repo".trust_level="trusted"`
	if got[1] != want {
		t.Errorf("codexTrustArgs[1] = %q, want %q", got[1], want)
	}
}

func TestBuildCodexSpawnArgsOrdering(t *testing.T) {
	// The shared codex argv builder is used by both LaunchAgent and
	// BuildPhaseCmd, so this test guards the ordering contract for both.
	repo := "/tmp/repo"
	launchArgs := []string{"--add-dir", "/tmp/wf", "--add-dir", "/tmp/mem"}
	prompt := "do the thing"

	args := buildCodexSpawnArgs(repo, launchArgs, prompt)

	// Expected shape:
	//   --full-auto -C <repo> -c <trust-override> --add-dir ... -- <prompt>
	wantPrefix := []string{
		"--full-auto", "-C", repo,
		"-c", `projects."/tmp/repo".trust_level="trusted"`,
		"--add-dir", "/tmp/wf", "--add-dir", "/tmp/mem",
		"--", prompt,
	}
	if len(args) != len(wantPrefix) {
		t.Fatalf("len mismatch: got %d want %d: %v", len(args), len(wantPrefix), args)
	}
	for i, w := range wantPrefix {
		if args[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, args[i], w)
		}
	}

	// Explicitly assert the trust-override comes AFTER -C <repo> and BEFORE
	// any --add-dir, because losing that ordering would regress the fix.
	trustIdx := -1
	addDirIdx := -1
	for i, a := range args {
		if a == "-c" && trustIdx == -1 {
			trustIdx = i
		}
		if a == "--add-dir" && addDirIdx == -1 {
			addDirIdx = i
		}
	}
	if trustIdx != 3 {
		t.Errorf("trust -c should be at index 3 (after --full-auto -C <repo>), got %d", trustIdx)
	}
	if addDirIdx == -1 || addDirIdx < trustIdx {
		t.Errorf("--add-dir should appear after -c trust override; addDirIdx=%d trustIdx=%d", addDirIdx, trustIdx)
	}
}

func TestBuildPhaseCmdCodexAdapterIncludesTrustOverride(t *testing.T) {
	// Integration check: BuildPhaseCmd for a codex-assigned phase emits the
	// trust override. Guards the Web UI / phase-cmd --json launch path.
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("1\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\nproject=default\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature\n"), 0644)

	state.InitGlobalMemory()
	state.InitProjectMemory("default")

	if err := SetPhaseAgent(wfDir, "plan", "codex"); err != nil {
		t.Fatalf("SetPhaseAgent: %v", err)
	}

	result, err := BuildPhaseCmd(wfDir, "test-wf", "plan", false, 1)
	if err != nil {
		t.Fatal(err)
	}

	wantTrust := `projects."/tmp/repo".trust_level="trusted"`
	var trustIdx, repoIdx, addDirIdx, dashIdx = -1, -1, -1, -1
	for i, a := range result.Args {
		switch {
		case a == "-C" && repoIdx == -1:
			repoIdx = i
		case a == wantTrust && trustIdx == -1:
			trustIdx = i
		case a == "--add-dir" && addDirIdx == -1:
			addDirIdx = i
		case a == "--" && dashIdx == -1:
			dashIdx = i
		}
	}
	if trustIdx == -1 {
		t.Fatalf("codex Args missing trust override %q; got %v", wantTrust, result.Args)
	}
	if repoIdx == -1 || trustIdx < repoIdx {
		t.Errorf("trust override should come after -C <repo>; repoIdx=%d trustIdx=%d", repoIdx, trustIdx)
	}
	if addDirIdx != -1 && addDirIdx < trustIdx {
		t.Errorf("trust override should come before --add-dir; addDirIdx=%d trustIdx=%d", addDirIdx, trustIdx)
	}
	if dashIdx == -1 || dashIdx < trustIdx {
		t.Errorf("trust override should come before --; dashIdx=%d trustIdx=%d", dashIdx, trustIdx)
	}
}

func TestBuildPhaseCmdClaudeAdapterOmitsTrustOverride(t *testing.T) {
	// The trust override is codex-only; claude argv must not carry it.
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("1\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\nproject=default\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature\n"), 0644)

	state.InitGlobalMemory()
	state.InitProjectMemory("default")

	result, err := BuildPhaseCmd(wfDir, "test-wf", "plan", false, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Agent.Adapter != "claude" {
		t.Fatalf("default plan agent should be claude, got %q", result.Agent.Adapter)
	}
	for _, a := range result.Args {
		if strings.Contains(a, "trust_level") {
			t.Errorf("claude Args must not contain trust_level override: %v", result.Args)
			break
		}
	}
}

func TestBuildPhaseCmdForce(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("3\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\nproject=default\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature\n"), 0644)

	state.InitGlobalMemory()
	state.InitProjectMemory("default")

	// Without force, plan should fail (already completed)
	_, err := BuildPhaseCmd(wfDir, "test-wf", "plan", false, 1)
	if err == nil {
		t.Fatal("expected error without --force")
	}

	// With force, should succeed
	result, err := BuildPhaseCmd(wfDir, "test-wf", "plan", true, 1)
	if err != nil {
		t.Fatalf("expected success with --force, got: %v", err)
	}
	if result.Phase != 1 {
		t.Errorf("phase = %d, want 1", result.Phase)
	}
}
