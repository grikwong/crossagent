package agent

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
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

	// resolve helper to match NewLaunchContext behavior
	resolve := func(p string) string {
		if abs, err := filepath.EvalSymlinks(p); err == nil {
			return abs
		}
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}

	// Should contain --add-dir wfDir --add-dir home
	// globalMemDir is skipped as it is under home
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}
	if args[0] != "--add-dir" || args[1] != resolve(wfDir) {
		t.Errorf("first --add-dir should be resolved wfDir, got %v", args[:2])
	}
	if args[2] != "--add-dir" || args[3] != resolve(home) {
		t.Errorf("second --add-dir should be resolved home, got %v", args[2:4])
	}
}

func TestBuildLaunchArgsWithProjectMem(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(wfDir, 0755)

	projMemDir := filepath.Join(home, "projects", "default", "memory")
	// projMemDir already created by setupTestHome

	args := BuildLaunchArgs(wfDir, nil, projMemDir)

	// resolve helper
	resolve := func(p string) string {
		if abs, err := filepath.EvalSymlinks(p); err == nil {
			return abs
		}
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}

	// Should contain 2 --add-dir entries: wfDir and home
	// projMemDir is skipped as it is under home
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}
	if args[2] != "--add-dir" || args[3] != resolve(home) {
		t.Errorf("second --add-dir should be resolved home, got %v", args[2:4])
	}
}

func TestBuildLaunchArgsWithAddDirs(t *testing.T) {
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(wfDir, 0755)

	args := BuildLaunchArgs(wfDir, []string{"/extra/dir1", "/extra/dir2"}, "")

	// wfDir + home + 2 extra = 4 entries = 8 args
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
		if !strings.HasPrefix(s, "/") {
			t.Errorf("allowWrite path should start with /, got %q", s)
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

	// repo + wfDir + home + extra = 4
	// globalMem and projMem are skipped as they are under home
	if len(allowWrite) != 4 {
		t.Fatalf("expected 4 allowWrite entries, got %d: %v", len(allowWrite), allowWrite)
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
		"extraction_status",
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
	//   --full-auto -C <repo> -c <trust> -c <sandbox_mode> -c <writable_roots>
	//   --add-dir ... -- <prompt>
	wantPrefix := []string{
		"--full-auto", "-C", repo,
		"-c", `projects."/tmp/repo".trust_level="trusted"`,
		"-c", `sandbox_mode="workspace-write"`,
		"-c", `sandbox_workspace_write.writable_roots=["/tmp/repo","/tmp/wf","/tmp/mem"]`,
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

func TestBuildPhaseCmdCodexAdapterPinsSandboxScope(t *testing.T) {
	// codex must be forced into workspace-write sandbox mode with an
	// explicit writable_roots list covering every directory crossagent
	// passes via --add-dir. This guarantees "sandbox limited to directories
	// explicitly given" regardless of the user's codex TOML defaults.
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

	var haveMode, haveRoots bool
	var rootsVal string
	for i, a := range result.Args {
		if a == "-c" && i+1 < len(result.Args) {
			v := result.Args[i+1]
			if v == `sandbox_mode="workspace-write"` {
				haveMode = true
			}
			if strings.HasPrefix(v, "sandbox_workspace_write.writable_roots=[") {
				haveRoots = true
				rootsVal = v
			}
		}
	}
	if !haveMode {
		t.Errorf(`codex Args missing sandbox_mode="workspace-write" override; got %v`, result.Args)
	}
	if !haveRoots {
		t.Fatalf("codex Args missing sandbox_workspace_write.writable_roots override; got %v", result.Args)
	}
	if !strings.Contains(rootsVal, wfDir) {
		t.Errorf("writable_roots should contain workflow dir %q; got %q", wfDir, rootsVal)
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

func TestBuildPhaseCmdGeminiAdapter(t *testing.T) {
	// Gemini uses --yolo to auto-approve, --include-directories with a
	// comma-separated list (not repeated --add-dir flags), and -p for the
	// bootstrap prompt. Guards the Web UI / phase-cmd --json launch path.
	home := setupTestHome(t)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	os.WriteFile(filepath.Join(wfDir, "phase"), []byte("1\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/repo\nproject=default\n"), 0644)
	os.WriteFile(filepath.Join(wfDir, "description"), []byte("Test feature\n"), 0644)

	state.InitGlobalMemory()
	state.InitProjectMemory("default")

	if err := SetPhaseAgent(wfDir, "plan", "gemini"); err != nil {
		t.Fatalf("SetPhaseAgent: %v", err)
	}

	result, err := BuildPhaseCmd(wfDir, "test-wf", "plan", false, 1)
	if err != nil {
		t.Fatal(err)
	}

	if result.Agent.Adapter != "gemini" {
		t.Fatalf("agent.adapter = %q, want %q", result.Agent.Adapter, "gemini")
	}

	var approvalIdx, approvalVal, sandboxIdx, includeIdx, includeVal, promptIdx, allowedIdx, allowedVal = -1, -1, -1, -1, -1, -1, -1, -1
	for i, a := range result.Args {
		switch a {
		case "--approval-mode":
			approvalIdx = i
			approvalVal = i + 1
		case "--sandbox":
			if sandboxIdx == -1 {
				sandboxIdx = i
			}
		case "--include-directories":
			if includeIdx == -1 {
				includeIdx = i
				includeVal = i + 1
			}
		case "-p":
			if promptIdx == -1 {
				promptIdx = i
			}
		case "--allowed-tools":
			allowedIdx = i
			allowedVal = i + 1
		}
	}

	if approvalIdx == -1 || approvalVal >= len(result.Args) || result.Args[approvalVal] != "yolo" {
		t.Errorf("gemini Args missing --approval-mode yolo; got %v", result.Args)
	}
	if allowedIdx == -1 || allowedVal >= len(result.Args) || result.Args[allowedVal] != "*" {
		t.Errorf("gemini Args missing --allowed-tools \"*\"; got %v", result.Args)
	}
	if sandboxIdx == -1 {
		t.Errorf("gemini Args missing --sandbox (writes must be OS-sandboxed); got %v", result.Args)
	}
	if includeIdx == -1 || includeVal >= len(result.Args) {
		t.Fatalf("gemini Args missing --include-directories <list>; got %v", result.Args)
	}
	list := result.Args[includeVal]
	if !strings.Contains(list, wfDir) {
		t.Errorf("include-directories list should contain workflow dir %q; got %q", wfDir, list)
	}
	if strings.Contains(list, " ") {
		t.Errorf("include-directories list should be comma-separated (no spaces); got %q", list)
	}
	if promptIdx == -1 || promptIdx+1 >= len(result.Args) {
		t.Fatalf("gemini Args missing -p <prompt>; got %v", result.Args)
	}
	if !strings.HasPrefix(result.Args[promptIdx+1], "Do not ask for confirmation or agreement;") {
		t.Errorf("gemini -p prompt should be a bootstrap instruction; got %q", result.Args[promptIdx+1])
	}
	// Claude-specific flags must not leak.
	for _, a := range result.Args {
		if a == "--permission-mode" || a == "--settings" || a == "--add-dir" || a == "--full-auto" {
			t.Errorf("gemini Args should not contain non-gemini flag %q; got %v", a, result.Args)
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

// TestClaudeAdapterImplementSandboxRestriction verifies that claude's
// implement-phase sandbox allowWrite list is pinned to the canonicalized
// affected files plus the workflow implement.md and memory dirs — and
// that an out-of-repo escape reaching the adapter is defensively dropped.
func TestClaudeAdapterImplementSandboxRestriction(t *testing.T) {
	home := setupTestHome(t)
	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)

	repo := filepath.Join(home, "repo")
	os.MkdirAll(filepath.Join(repo, "internal/agent"), 0755)
	os.MkdirAll(filepath.Join(repo, "cmd/crossagent"), 0755)
	os.WriteFile(filepath.Join(repo, "internal/agent/launcher.go"), []byte("package agent"), 0644)
	os.WriteFile(filepath.Join(repo, "cmd/crossagent/main.go"), []byte("package main"), 0644)

	affected := []string{
		filepath.Join(repo, "internal/agent/launcher.go"),
		filepath.Join(repo, "cmd/crossagent/main.go"),
		// Defensive: an adapter must ignore out-of-repo paths even if
		// upstream canonicalization is ever bypassed.
		"/etc/passwd",
	}
	ctx := NewLaunchContext(repo, wfDir, "/tmp/prompt.md", nil, "", affected, "implement", ExtractionOK)

	plan, err := claudeAdapter{}.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// resolve helper
	resolve := func(p string) string {
		if abs, err := filepath.EvalSymlinks(p); err == nil {
			return abs
		}
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}

	// Read generated sandbox settings and inspect allowWrite.
	var settingsFile string
	for i, a := range plan.Args {
		if a == "--settings" && i+1 < len(plan.Args) {
			settingsFile = plan.Args[i+1]
			break
		}
	}
	if settingsFile == "" {
		t.Fatal("claude args missing --settings <file>")
	}
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Sandbox struct {
			Filesystem struct {
				AllowWrite []string `json:"allowWrite"`
			} `json:"filesystem"`
		} `json:"sandbox"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	aw := strings.Join(parsed.Sandbox.Filesystem.AllowWrite, ",")

	if !strings.Contains(aw, resolve(repo)+"/internal/agent/launcher.go") {
		t.Errorf("allowWrite missing canonical affected file; got %s", aw)
	}
	if strings.Contains(aw, "/etc/passwd") {
		t.Errorf("allowWrite leaked out-of-repo path; got %s", aw)
	}
	if !strings.Contains(aw, resolve(filepath.Join(wfDir, "implement.md"))) {
		t.Errorf("allowWrite missing workflow implement.md; got %s", aw)
	}
	// The repo itself must NOT be blanket-writable when affected files
	// are supplied — that's the whole point of the restriction.
	repoEntry := "/" + strings.TrimPrefix(resolve(repo), "/")
	for _, p := range parsed.Sandbox.Filesystem.AllowWrite {
		if p == repoEntry {
			t.Errorf("allowWrite should not include repo root when affected files present; got %s", aw)
		}
	}
}

// TestCodexAdapterImplementSandboxRestriction mirrors the claude test
// for codex's writable_roots. Both adapters must apply the same rule so
// one family cannot silently regress into repo-wide writes.
func TestCodexAdapterImplementSandboxRestriction(t *testing.T) {
	home := setupTestHome(t)
	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755)
	// codex adapter reads general.md from the prompts dir.
	os.WriteFile(filepath.Join(wfDir, "prompts", "general.md"), []byte("general"), 0644)

	repo := filepath.Join(home, "repo")
	os.MkdirAll(filepath.Join(repo, "internal/agent"), 0755)
	os.WriteFile(filepath.Join(repo, "internal/agent/launcher.go"), []byte("package agent"), 0644)

	promptFile := filepath.Join(wfDir, "prompts", "implement.md")
	os.WriteFile(promptFile, []byte("implement prompt"), 0644)

	affected := []string{
		filepath.Join(repo, "internal/agent/launcher.go"),
		"/etc/passwd", // defensive-assertion target
	}
	ctx := NewLaunchContext(repo, wfDir, promptFile, nil, "", affected, "implement", ExtractionOK)

	plan, err := codexAdapter{}.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// resolve helper
	resolve := func(p string) string {
		if abs, err := filepath.EvalSymlinks(p); err == nil {
			return abs
		}
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}

	var rootsVal string
	for i, a := range plan.Args {
		if a == "-c" && i+1 < len(plan.Args) &&
			strings.HasPrefix(plan.Args[i+1], "sandbox_workspace_write.writable_roots=[") {
			rootsVal = plan.Args[i+1]
			break
		}
	}
	if rootsVal == "" {
		t.Fatalf("codex args missing writable_roots override; got %v", plan.Args)
	}

	if !strings.Contains(rootsVal, resolve(repo)+"/internal/agent/launcher.go") {
		t.Errorf("writable_roots missing canonical affected file; got %s", rootsVal)
	}
	if strings.Contains(rootsVal, "/etc/passwd") {
		t.Errorf("writable_roots leaked out-of-repo path; got %s", rootsVal)
	}
	if !strings.Contains(rootsVal, resolve(filepath.Join(wfDir, "implement.md"))) {
		t.Errorf("writable_roots missing workflow implement.md; got %s", rootsVal)
	}
	// Repo root itself must not appear as a standalone writable root
	// once affected-file restriction is in effect.
	if strings.Contains(rootsVal, `"`+resolve(repo)+`"`) {
		t.Errorf("writable_roots should not include repo root when affected files present; got %s", rootsVal)
	}
}

func TestExtractionStatusClassification(t *testing.T) {
	home := setupTestHome(t)
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	wfDir := filepath.Join(home, "workflows", "wf")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		planBody  string
		writePlan bool
		phase     string
		want      ExtractionStatus
	}{
		{
			name:  "plan_phase_always_ok",
			phase: "plan",
			want:  ExtractionOK,
		},
		{
			name:      "missing_plan_file",
			writePlan: false,
			phase:     "review",
			want:      ExtractionMissing,
		},
		{
			name: "section_absent",
			planBody: `# Plan

Some prose without an Affected Files header.
`,
			writePlan: true,
			phase:     "review",
			want:      ExtractionMissing,
		},
		{
			name: "empty_section",
			planBody: `# Plan

## Affected Files

(none)
`,
			writePlan: true,
			phase:     "review",
			want:      ExtractionEmpty,
		},
		{
			name: "malformed_section",
			planBody: `# Plan

## Affected Files

- ../../escape/path
- .
`,
			writePlan: true,
			phase:     "review",
			want:      ExtractionMalformed,
		},
		{
			name: "ok_section",
			planBody: `# Plan

## Affected Files

- ` + "`internal/agent/launcher.go`" + `
`,
			writePlan: true,
			phase:     "review",
			want:      ExtractionOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			planPath := filepath.Join(wfDir, "plan.md")
			os.Remove(planPath)
			if tc.writePlan {
				if err := os.WriteFile(planPath, []byte(tc.planBody), 0644); err != nil {
					t.Fatal(err)
				}
			}
			_, status, err := extractAffectedFilesForPhase(wfDir, repo, tc.phase)
			if err != nil {
				t.Fatalf("extractAffectedFilesForPhase: %v", err)
			}
			if status != tc.want {
				t.Errorf("status = %q, want %q", status, tc.want)
			}
		})
	}
}

// TestLaunchAgentDegradedSandboxWarning locks in the stderr warning
// emission in LaunchAgent() when extraction status is anything other
// than ok. Only the direct launch path emits this warning — BuildPhaseCmd
// surfaces the status via JSON — so this regression must exercise
// LaunchAgent directly.
func TestLaunchAgentDegradedSandboxWarning(t *testing.T) {
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("/bin/true not available on this system")
	}

	home := setupTestHome(t)
	wfDir := filepath.Join(home, "workflows", "test-wf")
	if err := os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}

	// plan.md without an Affected Files section -> ExtractionMissing
	// when LaunchAgent runs extractAffectedFilesForPhase for "implement".
	planBody := "# Plan\n\nSome prose without an Affected Files header.\n"
	if err := os.WriteFile(filepath.Join(wfDir, "plan.md"), []byte(planBody), 0644); err != nil {
		t.Fatal(err)
	}

	promptFile := filepath.Join(wfDir, "prompts", "implement.md")
	if err := os.WriteFile(promptFile, []byte("noop prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	ag := &Agent{
		Name:    "noop",
		Command: truePath,
		Adapter: "claude",
	}

	// Redirect os.Stderr to a pipe so we can capture the WARNING line.
	// LaunchAgent wires cmd.Stderr = os.Stderr and also writes its
	// degraded-sandbox warning through fmt.Fprintf(os.Stderr, ...).
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	done := make(chan error, 1)
	go func() {
		done <- LaunchAgent(ag, repo, promptFile, wfDir, nil, "", "implement")
	}()

	captured := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		captured <- b
	}()

	launchErr := <-done
	w.Close()
	os.Stderr = origStderr
	out := string(<-captured)

	if launchErr != nil {
		t.Fatalf("LaunchAgent failed: %v (stderr: %s)", launchErr, out)
	}
	if !strings.Contains(out, "WARNING") {
		t.Errorf("stderr missing WARNING prefix; got %q", out)
	}
	if !strings.Contains(out, "degraded sandbox") {
		t.Errorf("stderr missing degraded-sandbox message; got %q", out)
	}
	if !strings.Contains(out, string(ExtractionMissing)) {
		t.Errorf("stderr missing extraction status %q; got %q", ExtractionMissing, out)
	}
}

// TestClaudeAdapterFailClosed pins claudeAdapter.Plan to fail-closed
// behavior when the implement phase runs with an empty AffectedFiles
// list (the degraded-extraction case). The sandbox allowWrite list
// must NEVER contain the repo root: a fallback would collapse the
// sandbox back to repo-wide writes at exactly the moment extraction
// is least trustworthy.
func TestClaudeAdapterFailClosed(t *testing.T) {
	home := setupTestHome(t)
	wfDir := filepath.Join(home, "workflows", "test-wf")
	if err := os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	// resolve helper
	resolve := func(p string) string {
		if abs, err := filepath.EvalSymlinks(p); err == nil {
			return abs
		}
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}

	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	// Create implement.md so resolve can canonicalize it.
	if err := os.WriteFile(filepath.Join(wfDir, "implement.md"), []byte("implement"), 0644); err != nil {
		t.Fatal(err)
	}
	repoEntry := "/" + strings.TrimPrefix(resolve(repo), "/")

	cases := []struct {
		name   string
		status ExtractionStatus
	}{
		{"missing", ExtractionMissing},
		{"empty", ExtractionEmpty},
		{"malformed", ExtractionMalformed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewLaunchContext(repo, wfDir, "/tmp/prompt.md", nil, "", nil, "implement", tc.status)

			plan, err := claudeAdapter{}.Plan(ctx)
			if err != nil {
				t.Fatal(err)
			}

			var settingsFile string
			for i, a := range plan.Args {
				if a == "--settings" && i+1 < len(plan.Args) {
					settingsFile = plan.Args[i+1]
					break
				}
			}
			if settingsFile == "" {
				t.Fatal("claude args missing --settings <file>")
			}
			data, err := os.ReadFile(settingsFile)
			if err != nil {
				t.Fatal(err)
			}
			var parsed struct {
				Sandbox struct {
					Filesystem struct {
						AllowWrite []string `json:"allowWrite"`
					} `json:"filesystem"`
				} `json:"sandbox"`
			}
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatal(err)
			}

			// resolve helper
			resolve := func(p string) string {
				if abs, err := filepath.EvalSymlinks(p); err == nil {
					return abs
				}
				if abs, err := filepath.Abs(p); err == nil {
					return abs
				}
				return p
			}

			for _, p := range parsed.Sandbox.Filesystem.AllowWrite {
				if p == repoEntry {
					t.Errorf("allowWrite must not include repo root on degraded extraction (%s); got %v",
						tc.status, parsed.Sandbox.Filesystem.AllowWrite)
				}
			}
			// The implement.md report under the workflow dir must always
			// remain writable so the agent can signal completion.
			implementReport := "/" + strings.TrimPrefix(resolve(filepath.Join(wfDir, "implement.md")), "/")
			found := false
			for _, p := range parsed.Sandbox.Filesystem.AllowWrite {
				if p == implementReport {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("allowWrite missing workflow implement.md on degraded extraction (%s); got %v",
					tc.status, parsed.Sandbox.Filesystem.AllowWrite)
			}
		})
	}
}

// TestCodexAdapterFailClosed mirrors TestClaudeAdapterFailClosed for
// codex's sandbox_workspace_write.writable_roots. Both adapters must
// apply the same rule so one family cannot silently regress into
// repo-wide writes when extraction is degraded.
func TestCodexAdapterFailClosed(t *testing.T) {
	home := setupTestHome(t)
	wfDir := filepath.Join(home, "workflows", "test-wf")
	if err := os.MkdirAll(filepath.Join(wfDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "prompts", "general.md"), []byte("general"), 0644); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	promptFile := filepath.Join(wfDir, "prompts", "implement.md")
	if err := os.WriteFile(promptFile, []byte("implement prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create implement.md so resolve can canonicalize it.
	if err := os.WriteFile(filepath.Join(wfDir, "implement.md"), []byte("implement"), 0644); err != nil {
		t.Fatal(err)
	}

	// resolve helper
	resolve := func(p string) string {
		if abs, err := filepath.EvalSymlinks(p); err == nil {
			return abs
		}
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}

	cases := []struct {
		name   string
		status ExtractionStatus
	}{
		{"missing", ExtractionMissing},
		{"empty", ExtractionEmpty},
		{"malformed", ExtractionMalformed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewLaunchContext(repo, wfDir, promptFile, nil, "", nil, "implement", tc.status)

			plan, err := codexAdapter{}.Plan(ctx)
			if err != nil {
				t.Fatal(err)
			}

			var rootsVal string
			for i, a := range plan.Args {
				if a == "-c" && i+1 < len(plan.Args) &&
					strings.HasPrefix(plan.Args[i+1], "sandbox_workspace_write.writable_roots=[") {
					rootsVal = plan.Args[i+1]
					break
				}
			}
			if rootsVal == "" {
				t.Fatalf("codex args missing writable_roots override; got %v", plan.Args)
			}

			// The repo root must not appear as a standalone quoted entry.
			// Substring search would also match repo-prefixed files, so
			// we look for the exact quoted repo token.
			if strings.Contains(rootsVal, `"`+resolve(repo)+`"`) {
				t.Errorf("writable_roots must not include repo root on degraded extraction (%s); got %s",
					tc.status, rootsVal)
			}
			if !strings.Contains(rootsVal, resolve(filepath.Join(wfDir, "implement.md"))) {
				t.Errorf("writable_roots missing workflow implement.md on degraded extraction (%s); got %s",
					tc.status, rootsVal)
			}
		})
	}
}
