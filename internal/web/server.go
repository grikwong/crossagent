package web

import (
	"io/fs"
	"net/http"
)

// Serve starts the HTTP server on the given address (e.g., ":3456").
func Serve(addr string) error {
	mux := NewMux()
	return http.ListenAndServe(addr, mux)
}

// NewMux creates the HTTP router with all API routes and static file serving.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()

	// ── API routes ──────────────────────────────────────────────────────

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

	// Repos
	mux.HandleFunc("POST /api/repos/add", handleReposAdd)
	mux.HandleFunc("POST /api/repos/remove", handleReposRemove)

	// ── WebSocket terminal ──────────────────────────────────────────────
	mux.HandleFunc("/ws/terminal", handleTerminal)

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
