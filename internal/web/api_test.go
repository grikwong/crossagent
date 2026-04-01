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
