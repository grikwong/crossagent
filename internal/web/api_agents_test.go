package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests cover request validation in the agents handlers. They do not
// exercise the CLI shell-out path (which requires a built `crossagent`
// binary); that is covered end-to-end by the integration suite. The CLI
// heuristic itself is unit-tested in internal/agent/autoselect_test.go.

func TestHandleAgentsCreate_Validation(t *testing.T) {
	t.Setenv("CROSSAGENT_HOME", t.TempDir())

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/agents", handleAgentsCreate)

	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"bad name", `{"name":"bad name","adapter":"claude"}`, "Invalid agent name"},
		{"missing adapter", `{"name":"gemini-pro"}`, "adapter must be one of"},
		{"bad adapter", `{"name":"gemini-pro","adapter":"vertex"}`, "adapter must be one of"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/agents", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != 400 {
				t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantErr) {
				t.Errorf("want error containing %q, got %s", tc.wantErr, w.Body.String())
			}
		})
	}
}

func TestHandleAgentsDelete_Validation(t *testing.T) {
	t.Setenv("CROSSAGENT_HOME", t.TempDir())

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/agents/{name}", handleAgentsDelete)

	req := httptest.NewRequest("DELETE", "/api/agents/bad%20name", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestHandleWorkflowAgentsSet_Validation(t *testing.T) {
	t.Setenv("CROSSAGENT_HOME", t.TempDir())

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/workflow/{name}/agents", handleWorkflowAgentsSet)

	cases := []struct {
		name string
		path string
		body string
	}{
		{"bad workflow", "/api/workflow/bad%20wf/agents", `{"phase":"plan","agent":"claude"}`},
		{"bad phase", "/api/workflow/test/agents", `{"phase":"design","agent":"claude"}`},
		{"bad agent", "/api/workflow/test/agents", `{"phase":"plan","agent":"bad name"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != 400 {
				t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
