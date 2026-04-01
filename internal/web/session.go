package web

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

// ringBuffer is a fixed-capacity circular buffer that keeps the last N bytes
// for scrollback replay on reconnect. All methods are safe for concurrent use.
type ringBuffer struct {
	mu   sync.Mutex
	buf  []byte
	size int // capacity
	pos  int // next write position
	full bool
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{buf: make([]byte, size), size: size}
}

func (rb *ringBuffer) Write(p []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	for _, b := range p {
		rb.buf[rb.pos] = b
		rb.pos++
		if rb.pos >= rb.size {
			rb.pos = 0
			rb.full = true
		}
	}
}

// Bytes returns a copy of the buffered content in order.
// When the buffer has wrapped, leading bytes that form an incomplete UTF-8
// codepoint (split by the circular boundary) are skipped so that replay
// output always starts on a valid character boundary.
func (rb *ringBuffer) Bytes() []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if !rb.full {
		out := make([]byte, rb.pos)
		copy(out, rb.buf[:rb.pos])
		return out
	}
	out := make([]byte, rb.size)
	n := copy(out, rb.buf[rb.pos:])
	copy(out[n:], rb.buf[:rb.pos])
	// Skip orphaned UTF-8 continuation bytes at the start caused by
	// the circular boundary splitting a multi-byte codepoint.
	for len(out) > 0 {
		r, size := utf8.DecodeRune(out)
		if r != utf8.RuneError || size != 1 {
			break // valid rune (or empty) — stop skipping
		}
		out = out[1:]
	}
	return out
}

// Session represents a PTY session decoupled from any single WebSocket connection.
type Session struct {
	ID          string
	Workflow    string
	Phase       string
	Cmd         *exec.Cmd
	Ptmx        *os.File
	OutputBuf   *ringBuffer
	Viewers     map[*websocket.Conn]bool
	InputOwner  *websocket.Conn // nil = any viewer can send input
	ChatBuffer  []string
	ChatBufSize int
	ChatFlushed bool
	WorkflowDir string
	CreatedAt   time.Time
	Status      string // "running", "exited"
	ExitCode    int
	Done        chan struct{} // closed when the PTY reader goroutine finishes
	mu          sync.Mutex
}

// Broadcast sends a message to all connected viewers.
func (s *Session) Broadcast(msgType string, fields map[string]any) {
	s.mu.Lock()
	viewers := make([]*websocket.Conn, 0, len(s.Viewers))
	for v := range s.Viewers {
		viewers = append(viewers, v)
	}
	s.mu.Unlock()

	for _, v := range viewers {
		wsSend(v, msgType, fields)
	}
}

// appendChat appends data to the chat buffer if under cap.
func (s *Session) appendChat(data string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.WorkflowDir == "" || s.Phase == "" || s.ChatBufSize >= chatHistoryBufferCap {
		return
	}
	s.ChatBuffer = append(s.ChatBuffer, data)
	s.ChatBufSize += len(data)
}

// flushChatHistory writes buffered chat output to disk. Safe to call multiple times.
func (s *Session) flushChatHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ChatFlushed {
		return
	}
	s.ChatFlushed = true

	if s.WorkflowDir == "" || s.Phase == "" || len(s.ChatBuffer) == 0 {
		return
	}

	dir := filepath.Join(s.WorkflowDir, "chat-history")
	logFile := filepath.Join(dir, s.Phase+".log")
	tmpFile := logFile + ".tmp"

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create chat-history dir: %v", err)
		return
	}

	if err := os.WriteFile(tmpFile, []byte(strings.Join(s.ChatBuffer, "")), 0644); err != nil {
		log.Printf("Failed to write chat history: %v", err)
		os.Remove(tmpFile)
		return
	}

	if err := os.Rename(tmpFile, logFile); err != nil {
		log.Printf("Failed to rename chat history: %v", err)
		os.Remove(tmpFile)
		return
	}

	s.ChatBuffer = nil
	s.ChatBufSize = 0
}

// kill signals the PTY process to terminate by closing the PTY fd and killing
// the process. It does NOT set status or flush chat — the reader goroutine
// handles that after draining remaining output and calling cmd.Wait().
func (s *Session) kill() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Ptmx != nil {
		s.Ptmx.Close()
		// Don't nil out Ptmx here — the reader goroutine's Read will
		// get an error and break out of the loop naturally.
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
	}
}

// SessionInfo is the JSON-serializable session metadata.
type SessionInfo struct {
	ID        string `json:"id"`
	Workflow  string `json:"workflow"`
	Phase     string `json:"phase"`
	Status    string `json:"status"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Viewers   int    `json:"viewers"`
	CreatedAt string `json:"created_at"`
}

// SessionManager tracks all active PTY sessions.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	counter  int // simple incrementing ID
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// Create registers a new session. Returns the session.
func (sm *SessionManager) Create(workflow, phase string, cmd *exec.Cmd, ptmx *os.File, wfDir string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.counter++
	id := fmt.Sprintf("%s-%s-%d", workflow, phase, sm.counter)

	s := &Session{
		ID:          id,
		Workflow:    workflow,
		Phase:       phase,
		Cmd:         cmd,
		Ptmx:        ptmx,
		OutputBuf:   newRingBuffer(1024 * 1024), // 1MB scrollback
		Viewers:     make(map[*websocket.Conn]bool),
		WorkflowDir: wfDir,
		CreatedAt:   time.Now(),
		Status:      "running",
		Done:        make(chan struct{}),
	}
	sm.sessions[id] = s
	return s
}

// Get returns a session by ID.
func (sm *SessionManager) Get(id string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[id]
}

// GetByWorkflowPhase finds an active (running) session for a workflow+phase.
func (sm *SessionManager) GetByWorkflowPhase(workflow, phase string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, s := range sm.sessions {
		if s.Workflow == workflow && s.Phase == phase && s.Status == "running" {
			return s
		}
	}
	return nil
}

// Attach adds a WebSocket viewer to a session and replays the scrollback buffer.
// Replay is sent BEFORE the connection is added to the live broadcast set so
// that live PTY output cannot arrive at the viewer ahead of the scrollback.
func (sm *SessionManager) Attach(id string, conn *websocket.Conn) *Session {
	s := sm.Get(id)
	if s == nil {
		return nil
	}

	// Snapshot scrollback and determine ownership under the lock, but do NOT
	// add to Viewers yet — that would let the PTY reader broadcast live output
	// to this connection before the replay message is delivered.
	s.mu.Lock()
	scrollback := s.OutputBuf.Bytes()
	isOwner := s.InputOwner == nil
	if isOwner {
		s.InputOwner = conn
	}
	status := s.Status
	s.mu.Unlock()

	// Send replay and attached messages while NOT in the broadcast set.
	if len(scrollback) > 0 {
		wsSend(conn, "replay", map[string]any{"data": string(scrollback)})
	}
	wsSend(conn, "attached", map[string]any{
		"sessionID":  id,
		"inputOwner": isOwner,
		"status":     status,
	})

	// NOW add to viewers so subsequent live broadcasts reach this connection.
	s.mu.Lock()
	s.Viewers[conn] = true
	s.mu.Unlock()

	return s
}

// Detach removes a WebSocket viewer from a session.
// Does NOT kill the PTY — session stays alive for other viewers or reconnection.
func (sm *SessionManager) Detach(id string, conn *websocket.Conn) {
	s := sm.Get(id)
	if s == nil {
		return
	}

	s.mu.Lock()
	delete(s.Viewers, conn)

	// If the input owner disconnected, transfer to another viewer
	if s.InputOwner == conn {
		s.InputOwner = nil
		for v := range s.Viewers {
			s.InputOwner = v
			break
		}
	}
	newOwner := s.InputOwner
	s.mu.Unlock()

	// Notify the new input owner
	if newOwner != nil {
		wsSend(newOwner, "input-claimed", map[string]any{"sessionID": id})
	}
}

// ClaimInput transfers input ownership to the requesting connection.
func (sm *SessionManager) ClaimInput(id string, conn *websocket.Conn) bool {
	s := sm.Get(id)
	if s == nil {
		return false
	}

	s.mu.Lock()
	if !s.Viewers[conn] {
		s.mu.Unlock()
		return false
	}
	oldOwner := s.InputOwner
	s.InputOwner = conn
	s.mu.Unlock()

	// Notify old owner they lost input
	if oldOwner != nil && oldOwner != conn {
		wsSend(oldOwner, "input-released", map[string]any{"sessionID": id})
	}
	wsSend(conn, "input-claimed", map[string]any{"sessionID": id})
	return true
}

// Kill terminates a session's PTY process. The PTY reader goroutine will
// detect the closed PTY, call cmd.Wait() for the real exit code, flush chat
// history with all output captured, and broadcast the exit event.
func (sm *SessionManager) Kill(id string) {
	s := sm.Get(id)
	if s == nil {
		return
	}

	s.kill()
	// Wait for the reader goroutine to finish draining output, set exit code,
	// flush chat history, and broadcast exit.
	<-s.Done
}

// Remove deletes a session from the registry (after it's exited and no longer needed).
func (sm *SessionManager) Remove(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, id)
}

// List returns info for all sessions.
func (sm *SessionManager) List() []SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]SessionInfo, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		s.mu.Lock()
		info := SessionInfo{
			ID:        s.ID,
			Workflow:  s.Workflow,
			Phase:     s.Phase,
			Status:    s.Status,
			ExitCode:  s.ExitCode,
			Viewers:   len(s.Viewers),
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		}
		s.mu.Unlock()
		result = append(result, info)
	}
	return result
}

// ShutdownAll gracefully kills all active sessions. It signals each PTY process,
// waits for the reader goroutines to drain output and flush chat history, then
// closes all viewer WebSocket connections.
func (sm *SessionManager) ShutdownAll() {
	sm.mu.RLock()
	sessions := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	sm.mu.RUnlock()

	// Signal all processes to stop
	for _, s := range sessions {
		s.kill()
	}

	// Wait for all reader goroutines to finish (drain output, flush, broadcast exit)
	for _, s := range sessions {
		<-s.Done
	}

	// Close all viewer connections
	for _, s := range sessions {
		s.mu.Lock()
		for v := range s.Viewers {
			v.Close()
		}
		s.Viewers = make(map[*websocket.Conn]bool)
		s.mu.Unlock()
	}
}
