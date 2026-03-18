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
