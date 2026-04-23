package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"valid-name", true},
		{"valid.name", true},
		{"valid_name", true},
		{"Valid123", true},
		{"1start", true},
		{"", false},
		{"-start", false},
		{".start", false},
		{"has space", false},
		{"has/slash", false},
		{strings.Repeat("a", 129), false},
		{strings.Repeat("a", 128), true},
	}
	for _, tt := range tests {
		if got := validateName(tt.name); got != tt.want {
			t.Errorf("validateName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func setupTestWorkflow(t *testing.T) (string, func()) {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("CROSSAGENT_HOME", tmpHome)

	wfDir := filepath.Join(tmpHome, "workflows", "test-wf")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "description"), []byte("Initial description"), 0o644); err != nil {
		t.Fatal(err)
	}

	return wfDir, func() {}
}

func TestHandleUpdateDescription(t *testing.T) {
	wfDir, cleanup := setupTestWorkflow(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/update-description", handleUpdateDescription)

	t.Run("success", func(t *testing.T) {
		body := `{"workflow":"test-wf","append":"extra guidance"}`
		req := httptest.NewRequest("POST", "/api/update-description", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		data, err := os.ReadFile(filepath.Join(wfDir, "description"))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "Initial description") {
			t.Error("original description missing")
		}
		if !strings.Contains(content, "extra guidance") {
			t.Error("appended text missing")
		}
		if !strings.Contains(content, "---") {
			t.Error("separator missing")
		}
	})

	t.Run("missing workflow field", func(t *testing.T) {
		body := `{"append":"text"}`
		req := httptest.NewRequest("POST", "/api/update-description", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("missing append field", func(t *testing.T) {
		body := `{"workflow":"test-wf"}`
		req := httptest.NewRequest("POST", "/api/update-description", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid workflow name", func(t *testing.T) {
		body := `{"workflow":"bad name!","append":"text"}`
		req := httptest.NewRequest("POST", "/api/update-description", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("nonexistent workflow", func(t *testing.T) {
		body := `{"workflow":"no-such-wf","append":"text"}`
		req := httptest.NewRequest("POST", "/api/update-description", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleWorkflowSetDescription(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("CROSSAGENT_HOME", tmpHome)

	// Prevent runCLI from recursively exec'ing the test binary and hanging.
	orig := crossagentBin
	crossagentBin = "/usr/bin/false"
	t.Cleanup(func() { crossagentBin = orig })

	// Create a workflow at phase 1 with no plan.md (pre-run state).
	wfDir := filepath.Join(tmpHome, "workflows", "test-wf")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "phase"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "description"), []byte("Old description\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/workflow/{name}/description", handleWorkflowSetDescription(sm))

	t.Run("invalid workflow name", func(t *testing.T) {
		// Names starting with '-' fail validateName.
		req := httptest.NewRequest("PUT", "/api/workflow/-bad-name/description", strings.NewReader(`{"description":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("missing description field", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/workflow/test-wf/description", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("nonexistent workflow", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/workflow/no-such-wf/description", strings.NewReader(`{"description":"new"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("rejects workflow at phase 2", func(t *testing.T) {
		wfDir2 := filepath.Join(tmpHome, "workflows", "wf-phase2")
		if err := os.MkdirAll(wfDir2, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir2, "phase"), []byte("2"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir2, "description"), []byte("desc\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("PUT", "/api/workflow/wf-phase2/description", strings.NewReader(`{"description":"new"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != 409 {
			t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects workflow with existing plan.md", func(t *testing.T) {
		wfDir3 := filepath.Join(tmpHome, "workflows", "wf-started")
		if err := os.MkdirAll(wfDir3, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir3, "phase"), []byte("1"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir3, "description"), []byte("desc\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(wfDir3, "plan.md"), []byte("# plan\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("PUT", "/api/workflow/wf-started/description", strings.NewReader(`{"description":"new"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != 409 {
			t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("writes description on success", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/workflow/test-wf/description", strings.NewReader(`{"description":"Updated description"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		// Response may be 200 or 500 depending on whether the crossagent binary
		// is available; either way the description file must have been updated.
		data, err := os.ReadFile(filepath.Join(wfDir, "description"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "Updated description") {
			t.Errorf("description file not updated: %q", string(data))
		}
		if strings.Contains(string(data), "Old description") {
			t.Errorf("old description not replaced: %q", string(data))
		}
	})
}

func TestHandleWorkflowRoundArtifact(t *testing.T) {
	_, cleanup := setupTestWorkflow(t)
	defer cleanup()

	// Create a round directory with an artifact
	tmpHome := os.Getenv("CROSSAGENT_HOME")
	roundDir := filepath.Join(tmpHome, "workflows", "test-wf", "rounds", "1")
	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roundDir, "plan.md"), []byte("# Archived Plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := NewMux(sm)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/artifact/plan", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Archived Plan") {
			t.Error("response should contain archived plan content")
		}
	})

	t.Run("not found round", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/99/artifact/plan", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("invalid artifact type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/artifact/invalid", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleWorkflowRoundChatHistory(t *testing.T) {
	_, cleanup := setupTestWorkflow(t)
	defer cleanup()

	// Create a round directory with chat history
	tmpHome := os.Getenv("CROSSAGENT_HOME")
	chatDir := filepath.Join(tmpHome, "workflows", "test-wf", "rounds", "1", "chat-history")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chatDir, "plan.log"), []byte("terminal output here"), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := NewMux(sm)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/plan", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "terminal output here") {
			t.Error("response should contain chat history content")
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/review", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"exists":false`) {
			t.Error("response should indicate non-existence")
		}
	})

	t.Run("invalid phase", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/invalid", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleWorkflowRoundChatHistoryStream(t *testing.T) {
	_, cleanup := setupTestWorkflow(t)
	defer cleanup()

	tmpHome := os.Getenv("CROSSAGENT_HOME")
	chatDir := filepath.Join(tmpHome, "workflows", "test-wf", "rounds", "1", "chat-history")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "large terminal output stream content"
	if err := os.WriteFile(filepath.Join(chatDir, "plan.log"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := NewMux(sm)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/plan/stream", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), content) {
			t.Error("stream response should contain file content")
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") {
			t.Errorf("expected text/plain content type, got %s", ct)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/review/stream", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleWorkflowRoundChatHistoryStreamWithAttempt(t *testing.T) {
	_, cleanup := setupTestWorkflow(t)
	defer cleanup()

	tmpHome := os.Getenv("CROSSAGENT_HOME")
	chatDir := filepath.Join(tmpHome, "workflows", "test-wf", "rounds", "1", "chat-history")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	attemptContent := "attempt 2 review chat log"
	if err := os.WriteFile(filepath.Join(chatDir, "review.attempt-2.log"), []byte(attemptContent), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := NewMux(sm)

	t.Run("attempt stream success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/review/stream?attempt=2", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), attemptContent) {
			t.Error("stream response should contain attempt file content")
		}
	})

	t.Run("attempt stream not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/review/stream?attempt=99", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleWorkflowRoundArtifactWithAttempt(t *testing.T) {
	_, cleanup := setupTestWorkflow(t)
	defer cleanup()

	tmpHome := os.Getenv("CROSSAGENT_HOME")
	roundDir := filepath.Join(tmpHome, "workflows", "test-wf", "rounds", "1")
	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roundDir, "review.attempt-1.md"), []byte("# Attempt 1 Review"), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := NewMux(sm)

	t.Run("attempt artifact success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/artifact/review?attempt=1", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Attempt 1 Review") {
			t.Error("response should contain attempt artifact content")
		}
	})

	t.Run("attempt artifact not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/artifact/review?attempt=99", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleWorkflowRoundChatHistoryWithAttempt(t *testing.T) {
	_, cleanup := setupTestWorkflow(t)
	defer cleanup()

	tmpHome := os.Getenv("CROSSAGENT_HOME")
	chatDir := filepath.Join(tmpHome, "workflows", "test-wf", "rounds", "1", "chat-history")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chatDir, "review.attempt-1.log"), []byte("attempt chat log"), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := NewMux(sm)

	t.Run("attempt chat history success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/review?attempt=1", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "attempt chat log") {
			t.Error("response should contain attempt chat history content")
		}
	})

	t.Run("attempt chat history not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/rounds/1/chat-history/review?attempt=99", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"exists":false`) {
			t.Error("response should indicate non-existence for missing attempt")
		}
	})
}

func TestHandleWorkflowArtifactWithAttempt(t *testing.T) {
	_, cleanup := setupTestWorkflow(t)
	defer cleanup()

	tmpHome := os.Getenv("CROSSAGENT_HOME")
	wfDir := filepath.Join(tmpHome, "workflows", "test-wf")
	if err := os.WriteFile(filepath.Join(wfDir, "review.attempt-1.md"), []byte("# Current WF Attempt 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := NewMux(sm)

	t.Run("current workflow attempt artifact success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/artifact/review?attempt=1", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "Current WF Attempt 1") {
			t.Error("response should contain attempt artifact content")
		}
	})

	t.Run("current workflow attempt artifact not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/artifact/review?attempt=99", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleWorkflowChatHistoryWithAttempt(t *testing.T) {
	_, cleanup := setupTestWorkflow(t)
	defer cleanup()

	tmpHome := os.Getenv("CROSSAGENT_HOME")
	chatDir := filepath.Join(tmpHome, "workflows", "test-wf", "chat-history")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chatDir, "review.attempt-1.log"), []byte("current wf attempt chat"), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := NewSessionManager()
	mux := NewMux(sm)

	t.Run("current workflow attempt chat history success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/chat-history/review?attempt=1", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "current wf attempt chat") {
			t.Error("response should contain attempt chat history content")
		}
	})

	t.Run("current workflow attempt chat history not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/chat-history/review?attempt=99", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"exists":false`) {
			t.Error("response should indicate non-existence for missing attempt")
		}
	})

	t.Run("current workflow attempt chat history stream", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/chat-history/review/stream?attempt=1", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "current wf attempt chat") {
			t.Error("response should contain streamed attempt chat content")
		}
	})

	t.Run("current workflow attempt chat history stream not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workflow/test-wf/chat-history/review/stream?attempt=99", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}
