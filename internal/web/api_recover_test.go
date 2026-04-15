package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grikwong/crossagent/internal/state"
)

// TestHandleWorkflowCheckFile_RecoversSandboxFallback simulates the gemini
// --sandbox scenario: the agent was denied a write to the workflow dir
// and emitted plan.md into the repo root instead. The check-file handler
// should relocate it into the workflow dir and report exists=true so the
// frontend can auto-advance.
func TestHandleWorkflowCheckFile_RecoversSandboxFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CROSSAGENT_HOME", home)

	// Build a minimal workflow on disk: workflows/<name>/config pointing
	// at a fake repo where the misplaced plan.md lives.
	repo := filepath.Join(home, "fake-repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	wfName := "gemini-sandbox-test"
	wfDir := state.WorkflowDir(wfName)
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "config"),
		[]byte("repo="+repo+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	misplaced := filepath.Join(repo, "plan.md")
	if err := os.WriteFile(misplaced, []byte("# plan\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/workflow/{name}/check-file", handleWorkflowCheckFile)

	expected := filepath.Join(wfDir, "plan.md")
	body := `{"path":"` + expected + `"}`
	req := httptest.NewRequest("POST", "/api/workflow/"+wfName+"/check-file",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v (body=%s)", err, w.Body.String())
	}

	if exists, _ := resp["exists"].(bool); !exists {
		t.Errorf("exists = %v, want true after recovery; body=%s",
			resp["exists"], w.Body.String())
	}
	if recovered, _ := resp["recovered"].(bool); !recovered {
		t.Errorf("recovered = %v, want true; body=%s",
			resp["recovered"], w.Body.String())
	}
	if from, _ := resp["recovered_from"].(string); from != misplaced {
		t.Errorf("recovered_from = %q, want %q", from, misplaced)
	}

	// Physical verification: file moved, not copied.
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected plan.md in wfDir: %v", err)
	}
	if _, err := os.Stat(misplaced); !os.IsNotExist(err) {
		t.Errorf("misplaced plan.md should have been removed: err=%v", err)
	}
}

// TestHandleWorkflowCheckFile_IgnoresPathsOutsideWorkflow pins the safety
// guard: check-file must never move files whose expected path is outside
// the workflow directory. Otherwise a malicious or buggy caller could
// cause crossagent to relocate arbitrary files.
func TestHandleWorkflowCheckFile_IgnoresPathsOutsideWorkflow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CROSSAGENT_HOME", home)

	wfName := "outside-path-test"
	wfDir := state.WorkflowDir(wfName)
	os.MkdirAll(wfDir, 0755)
	os.WriteFile(filepath.Join(wfDir, "config"),
		[]byte("repo="+home+"\n"), 0644)

	// Stage a plan.md in repo root, but ask about a path outside wfDir.
	os.WriteFile(filepath.Join(home, "plan.md"), []byte("x"), 0644)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/workflow/{name}/check-file", handleWorkflowCheckFile)

	outside := filepath.Join(home, "somewhere-else", "plan.md")
	body := `{"path":"` + outside + `"}`
	req := httptest.NewRequest("POST", "/api/workflow/"+wfName+"/check-file",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if recovered, _ := resp["recovered"].(bool); recovered {
		t.Errorf("must not recover for paths outside wfDir; body=%s", w.Body.String())
	}
}
