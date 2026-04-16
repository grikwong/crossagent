package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

const chatHistoryBufferCap = 50 * 1024 * 1024 // 50MB cap per session

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser clients (curl, etc.)
		}
		// Allow same-origin requests: origin host must match the Host header
		// This is a localhost-only server so we compare against the request host.
		host := r.Host
		// Origin is a full URL (e.g., "http://localhost:3456"), extract host part
		for _, prefix := range []string{"https://", "http://"} {
			if strings.HasPrefix(origin, prefix) {
				origin = origin[len(prefix):]
				break
			}
		}
		return origin == host
	},
}

// wsMsg is a WebSocket message envelope.
type wsMsg struct {
	Type      string `json:"type"`
	Cols      uint16 `json:"cols,omitempty"`
	Rows      uint16 `json:"rows,omitempty"`
	Data      string `json:"data,omitempty"`
	PhaseName string `json:"phaseName,omitempty"`
	Subphase  string `json:"subphase,omitempty"`
	Force     bool   `json:"force,omitempty"`
	SessionID string `json:"sessionID,omitempty"`
	Workflow  string `json:"workflow,omitempty"` // Target workflow name (avoids global state)
}

// phaseCmdResult mirrors the JSON output from `crossagent phase-cmd --json`.
type phaseCmdResult struct {
	Agent       struct {
		Adapter string `json:"adapter"`
	} `json:"agent"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Cwd              string   `json:"cwd"`
	WorkflowDir      string   `json:"workflow_dir"`
	OutputFile       string   `json:"output_file"`
	ExtractionStatus string   `json:"extraction_status"`
	Error            string   `json:"error"`
}

// connWriteMu provides per-connection write serialization.
// gorilla/websocket does not allow concurrent writers on a single connection.
var connWriteMu sync.Map // map[*websocket.Conn]*sync.Mutex

func getConnMu(ws *websocket.Conn) *sync.Mutex {
	if mu, ok := connWriteMu.Load(ws); ok {
		return mu.(*sync.Mutex)
	}
	mu, _ := connWriteMu.LoadOrStore(ws, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func wsSend(ws *websocket.Conn, msgType string, fields map[string]any) {
	fields["type"] = msgType
	data, _ := json.Marshal(fields)
	mu := getConnMu(ws)
	mu.Lock()
	ws.WriteMessage(websocket.TextMessage, data)
	mu.Unlock()
}

func clamp(val, min, max uint16) uint16 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// splitUTF8 splits p into a valid UTF-8 prefix and a trailing incomplete
// multi-byte sequence (if any). The remainder should be prepended to the next
// read to reassemble the full codepoint.
func splitUTF8(p []byte) (valid, remainder []byte) {
	if utf8.Valid(p) {
		return p, nil
	}
	// Walk backwards (up to utf8.UTFMax bytes) to find the incomplete trailing rune.
	for i := 1; i <= utf8.UTFMax && i <= len(p); i++ {
		if utf8.Valid(p[:len(p)-i]) {
			return p[:len(p)-i], p[len(p)-i:]
		}
	}
	// Entire slice is invalid — return as-is to avoid swallowing data.
	return p, nil
}

// cleanEnv returns the current environment with Claude Code internal vars removed
// and TERM set to xterm-256color.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") || strings.HasPrefix(e, "CLAUDE_CODE_ENTRYPOINT=") {
			continue
		}
		env = append(env, e)
	}
	return append(env, "TERM=xterm-256color")
}

// connState tracks the per-connection session attachment.
type connState struct {
	mu        sync.Mutex
	sessionID string
}

// handleTerminal handles WebSocket connections for the terminal.
// It uses the SessionManager to decouple PTY sessions from individual connections.
func handleTerminal(sm *SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}
		defer ws.Close()
		defer connWriteMu.Delete(ws) // clean up per-connection write mutex

		cs := &connState{}

		defer func() {
			// Detach from any session on disconnect
			cs.mu.Lock()
			sid := cs.sessionID
			cs.mu.Unlock()
			if sid != "" {
				sm.Detach(sid, ws)
			}
		}()

		for {
			_, rawMsg, err := ws.ReadMessage()
			if err != nil {
				break
			}

			var msg wsMsg
			if err := json.Unmarshal(rawMsg, &msg); err != nil {
				wsSend(ws, "error", map[string]any{"message": "Invalid JSON"})
				continue
			}

			switch msg.Type {
			case "spawn":
				handleSpawn(sm, ws, cs, &msg)

			case "attach":
				handleAttach(sm, ws, cs, &msg)

			case "detach":
				cs.mu.Lock()
				sid := cs.sessionID
				cs.sessionID = ""
				cs.mu.Unlock()
				if sid != "" {
					sm.Detach(sid, ws)
				}

			case "claim-input":
				cs.mu.Lock()
				sid := cs.sessionID
				cs.mu.Unlock()
				if sid != "" {
					if !sm.ClaimInput(sid, ws) {
						wsSend(ws, "error", map[string]any{"message": "Cannot claim input"})
					}
				}

			case "input":
				cs.mu.Lock()
				sid := cs.sessionID
				cs.mu.Unlock()
				if sid == "" {
					continue
				}
				s := sm.Get(sid)
				if s == nil {
					continue
				}
				s.mu.Lock()
				// Allow input if this conn is the input owner or if no owner is set
				canInput := s.InputOwner == nil || s.InputOwner == ws
				p := s.Ptmx
				s.mu.Unlock()
				if canInput && p != nil {
					fmt.Fprint(p, msg.Data)
					s.appendChat(msg.Data)
				}

			case "kill":
				cs.mu.Lock()
				sid := cs.sessionID
				cs.mu.Unlock()
				if sid != "" {
					sm.Kill(sid)
				}

			case "resize":
				cs.mu.Lock()
				sid := cs.sessionID
				cs.mu.Unlock()
				if sid == "" {
					continue
				}
				s := sm.Get(sid)
				if s == nil {
					continue
				}
				s.mu.Lock()
				p := s.Ptmx
				s.mu.Unlock()
				if p != nil && msg.Cols > 0 && msg.Rows > 0 {
					cols := clamp(msg.Cols, 10, 500)
					rows := clamp(msg.Rows, 5, 200)
					pty.Setsize(p, &pty.Winsize{Rows: rows, Cols: cols})
				}

			default:
				wsSend(ws, "error", map[string]any{"message": fmt.Sprintf("Unknown type: %s", msg.Type)})
			}
		}
	}
}

// handleSpawn creates a new PTY session via the SessionManager.
func handleSpawn(sm *SessionManager, ws *websocket.Conn, cs *connState, msg *wsMsg) {
	// Detach from previous session if any
	cs.mu.Lock()
	oldSID := cs.sessionID
	cs.sessionID = ""
	cs.mu.Unlock()
	if oldSID != "" {
		sm.Detach(oldSID, ws)
	}

	if msg.PhaseName == "" || !validPhases[msg.PhaseName] {
		wsSend(ws, "error", map[string]any{"message": "Invalid or missing phaseName"})
		return
	}

	// Derive the command entirely server-side via phase-cmd
	cliArgs := []string{"phase-cmd", msg.PhaseName, "--json"}
	if msg.Workflow != "" {
		cliArgs = append(cliArgs, "--workflow", msg.Workflow)
	}
	if msg.Subphase != "" {
		validSub := true
		for _, c := range msg.Subphase {
			if c < '0' || c > '9' {
				validSub = false
				break
			}
		}
		if validSub {
			cliArgs = append(cliArgs, "--phase", msg.Subphase)
		}
	}
	if msg.Force {
		cliArgs = append(cliArgs, "--force")
	}

	phaseCmdOut, err := runCLI(cliArgs...)
	if err != nil {
		wsSend(ws, "error", map[string]any{"message": "Failed to get phase command: " + err.Error()})
		return
	}

	var phaseCmd phaseCmdResult
	if err := json.Unmarshal(phaseCmdOut, &phaseCmd); err != nil {
		wsSend(ws, "error", map[string]any{"message": "Failed to parse phase command"})
		return
	}
	if phaseCmd.Error != "" {
		wsSend(ws, "error", map[string]any{"message": phaseCmd.Error})
		return
	}
	if phaseCmd.Command == "" {
		wsSend(ws, "error", map[string]any{"message": "No command for phase"})
		return
	}

	cols := msg.Cols
	rows := msg.Rows
	if cols == 0 {
		cols = 120
	}
	if rows == 0 {
		rows = 30
	}
	cols = clamp(cols, 10, 500)
	rows = clamp(rows, 5, 200)

	cwd := phaseCmd.Cwd
	if cwd == "" {
		cwd, _ = os.UserHomeDir()
	}

	cmd := exec.Command(phaseCmd.Command, phaseCmd.Args...)
	cmd.Dir = cwd
	cmd.Env = cleanEnv()

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if err != nil {
		wsSend(ws, "error", map[string]any{"message": err.Error()})
		return
	}

	// Determine workflow name from workflow dir
	workflow := filepath.Base(phaseCmd.WorkflowDir)

	// Create session in the manager
	session := sm.Create(workflow, msg.PhaseName, cmd, ptmx, phaseCmd.WorkflowDir)

	// Attach this connection as the first viewer (and input owner)
	session.mu.Lock()
	session.Viewers[ws] = true
	session.InputOwner = ws
	session.mu.Unlock()

	cs.mu.Lock()
	cs.sessionID = session.ID
	cs.mu.Unlock()

	wsSend(ws, "spawned", map[string]any{
		"pid":               cmd.Process.Pid,
		"sessionID":         session.ID,
		"adapter":           phaseCmd.Agent.Adapter,
		"extraction_status": phaseCmd.ExtractionStatus,
	})

	// Read PTY output in a goroutine, broadcast to all viewers.
	// Uses a two-goroutine design: an inner goroutine reads from the PTY with
	// UTF-8 boundary safety, and an outer goroutine coalesces reads before
	// broadcasting to reduce WebSocket message storms during TUI redraws.
	go func() {
		defer close(session.Done)

		const (
			coalesceInterval = 8 * time.Millisecond // just over half a 60fps frame
			coalesceMaxSize  = 64 * 1024            // force-flush at 64KB
		)

		type readResult struct {
			data []byte // nil on error/EOF
			err  error
		}
		readCh := make(chan readResult, 4)

		// Inner goroutine: reads from PTY, handles UTF-8 boundaries.
		go func() {
			buf := make([]byte, 32*1024)
			var remainder []byte
			for {
				n, err := ptmx.Read(buf)
				if n > 0 {
					var chunk []byte
					if len(remainder) > 0 {
						chunk = append(remainder, buf[:n]...)
						remainder = nil
					} else {
						chunk = make([]byte, n)
						copy(chunk, buf[:n])
					}
					valid, rem := splitUTF8(chunk)
					remainder = rem
					if len(valid) > 0 {
						readCh <- readResult{data: valid}
					}
				}
				if err != nil {
					// Flush any incomplete UTF-8 remainder on PTY close.
					if len(remainder) > 0 {
						readCh <- readResult{data: remainder}
					}
					readCh <- readResult{err: err}
					return
				}
			}
		}()

		// Outer loop: coalesces output before broadcasting.
		var coalesceBuf []byte
		coalesceTimer := time.NewTimer(coalesceInterval)
		coalesceTimer.Stop()

		flushCoalesced := func() {
			if len(coalesceBuf) == 0 {
				return
			}
			session.Broadcast("output", map[string]any{"data": string(coalesceBuf)})
			coalesceBuf = coalesceBuf[:0]
		}

	loop:
		for {
			select {
			case r := <-readCh:
				if r.err != nil {
					flushCoalesced()
					break loop
				}
				session.OutputBuf.Write(r.data)
				session.appendChat(string(r.data))
				coalesceBuf = append(coalesceBuf, r.data...)
				if len(coalesceBuf) >= coalesceMaxSize {
					coalesceTimer.Stop()
					flushCoalesced()
				} else {
					coalesceTimer.Reset(coalesceInterval)
				}
			case <-coalesceTimer.C:
				flushCoalesced()
			}
		}

		coalesceTimer.Stop()

		// PTY closed — wait for process to finish and get real exit code
		exitCode := 0
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}

		session.mu.Lock()
		session.ExitCode = exitCode
		session.Status = "exited"
		session.Ptmx = nil
		session.Cmd = nil
		session.mu.Unlock()

		// Flush chat history AFTER all output is drained and exit code is set
		session.flushChatHistory()
		session.Broadcast("exit", map[string]any{"code": exitCode})
	}()
}

// handleAttach connects a WebSocket to an existing session.
func handleAttach(sm *SessionManager, ws *websocket.Conn, cs *connState, msg *wsMsg) {
	// Detach from previous session if any
	cs.mu.Lock()
	oldSID := cs.sessionID
	cs.sessionID = ""
	cs.mu.Unlock()
	if oldSID != "" {
		sm.Detach(oldSID, ws)
	}

	if msg.SessionID == "" {
		wsSend(ws, "error", map[string]any{"message": "Missing sessionID"})
		return
	}

	s := sm.Attach(msg.SessionID, ws)
	if s == nil {
		wsSend(ws, "error", map[string]any{"message": "Session not found: " + msg.SessionID})
		return
	}

	cs.mu.Lock()
	cs.sessionID = msg.SessionID
	cs.mu.Unlock()
}
