package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// Serve starts the HTTP server on the given address (e.g., ":3456").
// It sets up graceful shutdown to kill all active PTY sessions on SIGINT/SIGTERM.
func Serve(addr string) error {
	sm := NewSessionManager()
	mux := NewMux(sm)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown on SIGINT/SIGTERM
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		// Server stopped on its own (e.g., port in use)
		sm.ShutdownAll()
		return err
	case <-done:
		log.Println("Shutting down: killing all sessions...")
		sm.ShutdownAll()
		if err := srv.Shutdown(context.Background()); err != nil {
			return err
		}
		return nil // clean shutdown, don't propagate ErrServerClosed
	}
}

// NewMux creates the HTTP router with all API routes and static file serving.
func NewMux(sm *SessionManager) *http.ServeMux {
	mux := http.NewServeMux()

	// ── API routes ──────────────────────────────────────────────────────

	// Version
	mux.HandleFunc("GET /api/version", handleVersion)

	// Status & list
	mux.HandleFunc("GET /api/status", handleStatus)
	mux.HandleFunc("GET /api/list", handleList)

	// Workflow lifecycle
	mux.HandleFunc("POST /api/use/{name}", handleUse)
	mux.HandleFunc("POST /api/advance", handleAdvance)
	mux.HandleFunc("POST /api/done", handleDone)
	mux.HandleFunc("POST /api/new", handleNew)

	// Phase commands & artifacts
	mux.HandleFunc("GET /api/phase-cmd/{phase}", handlePhaseCmd)
	mux.HandleFunc("GET /api/artifact/{type}", handleArtifact)

	// Projects
	mux.HandleFunc("GET /api/projects", handleProjectsList)
	mux.HandleFunc("POST /api/projects/new", handleProjectsNew)
	mux.HandleFunc("POST /api/projects/delete", handleProjectsDelete)
	mux.HandleFunc("GET /api/projects/{name}", handleProjectsShow)
	mux.HandleFunc("POST /api/projects/rename", handleProjectsRename)

	// Workflow management
	mux.HandleFunc("POST /api/move", handleMove)
	mux.HandleFunc("POST /api/suggest-project", handleSuggestProject)

	// Supervise & revert
	mux.HandleFunc("POST /api/supervise", handleSupervise)
	mux.HandleFunc("POST /api/revert", handleRevert)

	// Chat history
	mux.HandleFunc("GET /api/chat-history/{phase}", handleChatHistory)
	mux.HandleFunc("GET /api/chat-history/{phase}/stream", handleChatHistoryStream)

	// File checks
	mux.HandleFunc("POST /api/check-file", handleCheckFile)
	mux.HandleFunc("POST /api/check-advance", handleCheckAdvance)

	// Description update (elicitation)
	mux.HandleFunc("POST /api/update-description", handleUpdateDescription)

	// Repos
	mux.HandleFunc("POST /api/repos/add", handleReposAdd)
	mux.HandleFunc("POST /api/repos/remove", handleReposRemove)

	// Sessions
	mux.HandleFunc("GET /api/sessions", handleSessions(sm))

	// Agents (model management)
	mux.HandleFunc("GET /api/agents", handleAgentsList)
	mux.HandleFunc("POST /api/agents", handleAgentsCreate)
	mux.HandleFunc("DELETE /api/agents/{name}", handleAgentsDelete)
	mux.HandleFunc("GET /api/adapters", handleAdaptersList)

	// ── Workflow-scoped routes ──────────────────────────────────────────
	// These eliminate dependence on the global ~/.crossagent/current file,
	// enabling multiple browsers to operate on different workflows safely.
	mux.HandleFunc("GET /api/workflow/{name}/status", handleWorkflowStatus)
	mux.HandleFunc("GET /api/workflow/{name}/phase-cmd/{phase}", handleWorkflowPhaseCmd)
	mux.HandleFunc("GET /api/workflow/{name}/artifact/{type}", handleWorkflowArtifact)
	mux.HandleFunc("POST /api/workflow/{name}/advance", handleWorkflowAdvance)
	mux.HandleFunc("POST /api/workflow/{name}/done", handleWorkflowDone)
	mux.HandleFunc("POST /api/workflow/{name}/supervise", handleWorkflowSupervise)
	mux.HandleFunc("POST /api/workflow/{name}/revert", handleWorkflowRevert)
	mux.HandleFunc("POST /api/workflow/{name}/check-file", handleWorkflowCheckFile)
	mux.HandleFunc("POST /api/workflow/{name}/check-advance", handleWorkflowCheckAdvance)
	mux.HandleFunc("GET /api/workflow/{name}/chat-history/{phase}", handleWorkflowChatHistory)
	mux.HandleFunc("GET /api/workflow/{name}/chat-history/{phase}/stream", handleWorkflowChatHistoryStream)
	mux.HandleFunc("POST /api/workflow/{name}/repos/add", handleWorkflowReposAdd)
	mux.HandleFunc("POST /api/workflow/{name}/repos/remove", handleWorkflowReposRemove)
	mux.HandleFunc("PUT /api/workflow/{name}/description", handleWorkflowSetDescription(sm))

	// Workflow-scoped agent assignments
	mux.HandleFunc("GET /api/workflow/{name}/agents", handleWorkflowAgentsGet)
	mux.HandleFunc("POST /api/workflow/{name}/agents", handleWorkflowAgentsSet)
	mux.HandleFunc("POST /api/workflow/{name}/agents/autoselect", handleWorkflowAgentsAutoselect)

	// Followup & round history
	mux.HandleFunc("POST /api/followup", handleFollowup)
	mux.HandleFunc("POST /api/workflow/{name}/followup", handleWorkflowFollowup)
	mux.HandleFunc("GET /api/workflow/{name}/rounds/{n}/artifact/{type}", handleWorkflowRoundArtifact)
	mux.HandleFunc("GET /api/workflow/{name}/rounds/{n}/chat-history/{phase}", handleWorkflowRoundChatHistory)
	mux.HandleFunc("GET /api/workflow/{name}/rounds/{n}/chat-history/{phase}/stream", handleWorkflowRoundChatHistoryStream)

	// ── WebSocket terminal ──────────────────────────────────────────────
	mux.HandleFunc("/ws/terminal", handleTerminal(sm))

	// ── Static files from embedded FS ───────────────────────────────────
	publicFS, _ := fs.Sub(frontendFS, "public")
	fileServer := http.FileServer(http.FS(publicFS))

	// Serve /assets/ from embedded public/assets/
	mux.Handle("/assets/", fileServer)
	// Serve /vendor/ from embedded public/vendor/
	mux.Handle("/vendor/", fileServer)
	// Serve everything else (index.html, app.js, style.css, etc.)
	mux.Handle("/", fileServer)

	return mux
}

// handleSessions returns the list of active/exited sessions.
func handleSessions(sm *SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessions := sm.List()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	}
}
