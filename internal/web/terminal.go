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

// termSession tracks a single PTY session on a WebSocket connection.
type termSession struct {
	mu             sync.Mutex
	ptmx           *os.File     // PTY master
	cmd            *exec.Cmd    // spawned process
	chatBuffer     []string     // buffered output chunks
	chatBufferSize int          // total bytes buffered
	chatWorkflowDir string
	chatPhaseName  string
	chatFlushed    bool
}

// flushChatHistory writes buffered chat output to disk. Safe to call multiple times.
func (s *termSession) flushChatHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.chatFlushed {
		return
	}
	s.chatFlushed = true

	if s.chatWorkflowDir == "" || s.chatPhaseName == "" || len(s.chatBuffer) == 0 {
		return
	}

	dir := filepath.Join(s.chatWorkflowDir, "chat-history")
	logFile := filepath.Join(dir, s.chatPhaseName+".log")
	tmpFile := logFile + ".tmp"

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create chat-history dir: %v", err)
		return
	}

	if err := os.WriteFile(tmpFile, []byte(strings.Join(s.chatBuffer, "")), 0644); err != nil {
		log.Printf("Failed to write chat history: %v", err)
		os.Remove(tmpFile)
		return
	}

	if err := os.Rename(tmpFile, logFile); err != nil {
		log.Printf("Failed to rename chat history: %v", err)
		os.Remove(tmpFile)
		return
	}

	// Release buffer memory
	s.chatBuffer = nil
	s.chatBufferSize = 0
}

// appendChat appends data to the chat buffer if under cap.
func (s *termSession) appendChat(data string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.chatWorkflowDir == "" || s.chatPhaseName == "" || s.chatBufferSize >= chatHistoryBufferCap {
		return
	}
	s.chatBuffer = append(s.chatBuffer, data)
	s.chatBufferSize += len(data)
}

// kill terminates the PTY process if running.
func (s *termSession) kill() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ptmx != nil {
		s.ptmx.Close()
		s.ptmx = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd = nil
	}
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
}

// phaseCmdResult mirrors the JSON output from `crossagent phase-cmd --json`.
type phaseCmdResult struct {
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Cwd         string   `json:"cwd"`
	WorkflowDir string   `json:"workflow_dir"`
	OutputFile  string   `json:"output_file"`
	Error       string   `json:"error"`
}

func wsSend(ws *websocket.Conn, msgType string, fields map[string]any) {
	fields["type"] = msgType
	data, _ := json.Marshal(fields)
	ws.WriteMessage(websocket.TextMessage, data)
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

// handleTerminal handles WebSocket connections for the terminal.
func handleTerminal(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	session := &termSession{}

	defer func() {
		session.flushChatHistory()
		session.kill()
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
			// Flush any previous session's chat history
			session.flushChatHistory()
			session.kill()

			if msg.PhaseName == "" || !validPhases[msg.PhaseName] {
				wsSend(ws, "error", map[string]any{"message": "Invalid or missing phaseName"})
				continue
			}

			// Derive the command entirely server-side via phase-cmd
			cliArgs := []string{"phase-cmd", msg.PhaseName, "--json"}
			if msg.Subphase != "" {
				// Validate subphase is numeric
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
				continue
			}

			var phaseCmd phaseCmdResult
			if err := json.Unmarshal(phaseCmdOut, &phaseCmd); err != nil {
				wsSend(ws, "error", map[string]any{"message": "Failed to parse phase command"})
				continue
			}
			if phaseCmd.Error != "" {
				wsSend(ws, "error", map[string]any{"message": phaseCmd.Error})
				continue
			}
			if phaseCmd.Command == "" {
				wsSend(ws, "error", map[string]any{"message": "No command for phase"})
				continue
			}

			// Reset chat history state
			session.mu.Lock()
			session.chatBuffer = nil
			session.chatBufferSize = 0
			session.chatFlushed = false
			session.chatWorkflowDir = phaseCmd.WorkflowDir
			session.chatPhaseName = msg.PhaseName
			session.mu.Unlock()

			cols := clamp(msg.Cols, 10, 500)
			rows := clamp(msg.Rows, 5, 200)
			if cols == 0 {
				cols = 120
			}
			if rows == 0 {
				rows = 30
			}

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
				continue
			}

			session.mu.Lock()
			session.ptmx = ptmx
			session.cmd = cmd
			session.mu.Unlock()

			wsSend(ws, "spawned", map[string]any{"pid": cmd.Process.Pid})

			// Read PTY output in a goroutine
			go func() {
				buf := make([]byte, 32*1024)
				for {
					n, err := ptmx.Read(buf)
					if n > 0 {
						data := string(buf[:n])
						wsSend(ws, "output", map[string]any{"data": data})
						session.appendChat(data)
					}
					if err != nil {
						break
					}
				}

				// PTY closed — wait for process to finish
				exitCode := 0
				if err := cmd.Wait(); err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ExitCode()
					}
				}

				session.flushChatHistory()
				wsSend(ws, "exit", map[string]any{"code": exitCode})

				session.mu.Lock()
				session.ptmx = nil
				session.cmd = nil
				session.mu.Unlock()
			}()

		case "input":
			session.mu.Lock()
			p := session.ptmx
			session.mu.Unlock()
			if p != nil {
				fmt.Fprint(p, msg.Data)
				// Also capture input for non-echoed cases
				session.appendChat(msg.Data)
			}

		case "kill":
			session.flushChatHistory()
			session.kill()

		case "resize":
			session.mu.Lock()
			p := session.ptmx
			session.mu.Unlock()
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
