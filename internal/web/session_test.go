package web

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// testHijackWriter implements http.ResponseWriter and http.Hijacker for
// in-memory WebSocket upgrades without binding to a network port.
type testHijackWriter struct {
	conn   net.Conn
	brw    *bufio.ReadWriter
	header http.Header
}

func (w *testHijackWriter) Header() http.Header                { return w.header }
func (w *testHijackWriter) Write(b []byte) (int, error)        { return w.brw.Write(b) }
func (w *testHijackWriter) WriteHeader(int)                     {}
func (w *testHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.brw, nil
}

// wsConnPair creates an in-memory WebSocket connection pair using net.Pipe,
// avoiding any TCP port binding. Safe for sandboxed test environments.
func wsConnPair(t *testing.T) (server *websocket.Conn, client *websocket.Conn) {
	t.Helper()

	sConn, cConn := net.Pipe()

	var serverWS *websocket.Conn
	done := make(chan error, 1)

	go func() {
		br := bufio.NewReader(sConn)
		req, err := http.ReadRequest(br)
		if err != nil {
			done <- err
			return
		}
		w := &testHijackWriter{
			conn:   sConn,
			brw:    bufio.NewReadWriter(br, bufio.NewWriter(sConn)),
			header: make(http.Header),
		}
		u := websocket.Upgrader{}
		sc, err := u.Upgrade(w, req, nil)
		if err != nil {
			done <- err
			return
		}
		serverWS = sc
		done <- nil
	}()

	dialer := &websocket.Dialer{
		NetDial: func(string, string) (net.Conn, error) {
			return cConn, nil
		},
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}
	cc, _, err := dialer.Dial("ws://test/", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("server upgrade: %v", err)
	}

	return serverWS, cc
}

// startEchoSession creates a session running "cat" (echoes stdin to stdout)
// and returns the session + manager. Caller should defer sm.Kill(s.ID).
func startEchoSession(t *testing.T, sm *SessionManager, wfDir string) *Session {
	t.Helper()

	cmd := exec.Command("cat")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}

	s := sm.Create("test-wf", "plan", cmd, ptmx, wfDir)

	// Start the reader goroutine (mirrors handleSpawn's goroutine with UTF-8 safety)
	go func() {
		defer close(s.Done)

		buf := make([]byte, 32*1024)
		var remainder []byte
		for {
			n, readErr := ptmx.Read(buf)
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
					s.OutputBuf.Write(valid)
					s.appendChat(string(valid))
				}
			}
			if readErr != nil {
				if len(remainder) > 0 {
					s.OutputBuf.Write(remainder)
					s.appendChat(string(remainder))
				}
				break
			}
		}

		exitCode := 0
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}

		s.mu.Lock()
		s.ExitCode = exitCode
		s.Status = "exited"
		s.Ptmx = nil
		s.Cmd = nil
		s.mu.Unlock()

		s.flushChatHistory()
	}()

	return s
}

func TestSessionManagerCreateAndGet(t *testing.T) {
	sm := NewSessionManager()

	cmd := exec.Command("echo", "hello")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer ptmx.Close()
	cmd.Wait()

	s := sm.Create("wf1", "plan", cmd, ptmx, "/tmp/test")
	s.Done = make(chan struct{})
	close(s.Done) // no goroutine for this test

	if s.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if s.Status != "running" {
		t.Fatalf("expected status running, got %s", s.Status)
	}

	got := sm.Get(s.ID)
	if got != s {
		t.Fatal("Get returned different session")
	}

	got = sm.Get("nonexistent")
	if got != nil {
		t.Fatal("expected nil for nonexistent session")
	}
}

func TestSessionManagerGetByWorkflowPhase(t *testing.T) {
	sm := NewSessionManager()

	cmd := exec.Command("echo")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer ptmx.Close()
	cmd.Wait()

	s := sm.Create("wf1", "plan", cmd, ptmx, "/tmp/test")
	s.Done = make(chan struct{})
	close(s.Done)

	found := sm.GetByWorkflowPhase("wf1", "plan")
	if found != s {
		t.Fatal("expected to find session by workflow+phase")
	}

	found = sm.GetByWorkflowPhase("wf1", "review")
	if found != nil {
		t.Fatal("expected nil for wrong phase")
	}

	// Exited sessions should not be found
	s.mu.Lock()
	s.Status = "exited"
	s.mu.Unlock()

	found = sm.GetByWorkflowPhase("wf1", "plan")
	if found != nil {
		t.Fatal("expected nil for exited session")
	}
}

func TestSessionManagerList(t *testing.T) {
	sm := NewSessionManager()

	cmd := exec.Command("echo")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer ptmx.Close()
	cmd.Wait()

	sm.Create("wf1", "plan", cmd, ptmx, "/tmp/test")
	list := sm.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].Workflow != "wf1" || list[0].Phase != "plan" {
		t.Fatalf("unexpected session info: %+v", list[0])
	}
}

func TestRingBuffer(t *testing.T) {
	rb := newRingBuffer(8)
	rb.Write([]byte("hello"))
	if string(rb.Bytes()) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(rb.Bytes()))
	}

	// Overflow: wrap around
	rb.Write([]byte("worldXYZ"))
	got := string(rb.Bytes())
	// Buffer is 8 bytes, wrote "helloworld" (13 bytes total), last 8 should be "orldXYZ" wait...
	// "hello" = 5 bytes at positions 0-4, pos=5
	// "worldXYZ" = 8 bytes: w=5,o=6,r=7,l=0(wrap,full=true),d=1,X=2,Y=3,Z=4, pos=5
	// Bytes(): full=true, so [pos:] + [:pos] = buf[5:8] + buf[0:5] = "XYZ" + "ldXYZ"
	// Wait, let me re-trace: buf after "hello" = [h,e,l,l,o,0,0,0], pos=5
	// Writing "worldXYZ": w@5, o@6, r@7, l@0(wrap,full), d@1, X@2, Y@3, Z@4, pos=5
	// buf = [l,d,X,Y,Z,w,o,r], Bytes = buf[5:8]+buf[0:5] = "wor"+"ldXYZ" = "worldXYZ"
	if got != "worldXYZ" {
		t.Fatalf("expected 'worldXYZ', got %q", got)
	}
}

func TestKillWaitsForGoroutine(t *testing.T) {
	sm := NewSessionManager()
	wfDir := t.TempDir()
	s := startEchoSession(t, sm, wfDir)

	// Kill should wait for the goroutine to finish
	sm.Kill(s.ID)

	// After Kill returns, the goroutine must have finished
	select {
	case <-s.Done:
		// good
	default:
		t.Fatal("Done channel not closed after Kill")
	}

	s.mu.Lock()
	status := s.Status
	s.mu.Unlock()

	if status != "exited" {
		t.Fatalf("expected exited status, got %s", status)
	}
}

func TestKillFlushesCompleteHistory(t *testing.T) {
	sm := NewSessionManager()
	wfDir := t.TempDir()
	s := startEchoSession(t, sm, wfDir)

	// Write some data to the PTY
	s.mu.Lock()
	p := s.Ptmx
	s.mu.Unlock()

	if p != nil {
		p.Write([]byte("test input\n"))
	}

	// Give the echo some time to process
	time.Sleep(100 * time.Millisecond)

	// Kill and wait
	sm.Kill(s.ID)

	// Chat history should have been flushed after all output was captured
	logFile := filepath.Join(wfDir, "chat-history", "plan.log")
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("chat history not flushed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("chat history is empty")
	}
}

func TestShutdownAll(t *testing.T) {
	sm := NewSessionManager()
	wfDir := t.TempDir()

	s1 := startEchoSession(t, sm, wfDir)
	s2 := startEchoSession(t, sm, wfDir)

	sm.ShutdownAll()

	// Both sessions should be exited
	for _, s := range []*Session{s1, s2} {
		select {
		case <-s.Done:
		default:
			t.Fatal("session Done not closed after ShutdownAll")
		}
		s.mu.Lock()
		st := s.Status
		s.mu.Unlock()
		if st != "exited" {
			t.Fatalf("expected exited, got %s", st)
		}
	}
}

// TestRingBufferConcurrent verifies that concurrent reads and writes on a
// ringBuffer protected by its internal mutex do not race.
// Run with: go test -race ./internal/web/...
func TestRingBufferConcurrent(t *testing.T) {
	rb := newRingBuffer(1024)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 5000; i++ {
			rb.Write([]byte("data"))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 5000; i++ {
			_ = rb.Bytes()
		}
	}()
	wg.Wait()

	// Bytes must return a valid snapshot after all writes
	got := rb.Bytes()
	if len(got) == 0 {
		t.Fatal("expected non-empty buffer after concurrent writes")
	}
}

// TestAttachReplayBeforeLiveOutput verifies that a newly attached viewer
// receives the "replay" message before any live "output" messages.
// Uses in-memory net.Pipe WebSocket pairs — no TCP port binding required.
func TestAttachReplayBeforeLiveOutput(t *testing.T) {
	sm := NewSessionManager()

	// Create a session directly with controlled content (no real PTY needed).
	s := &Session{
		ID:        "test-replay-1",
		Workflow:  "test-wf",
		Phase:     "plan",
		OutputBuf: newRingBuffer(4096),
		Viewers:   make(map[*websocket.Conn]bool),
		Status:    "running",
		Done:      make(chan struct{}),
	}
	sm.mu.Lock()
	sm.sessions[s.ID] = s
	sm.mu.Unlock()

	// Seed the scrollback buffer with known prior output.
	s.OutputBuf.Write([]byte("previous output data"))

	// Create an in-memory WebSocket pair (no network port).
	serverWS, clientWS := wsConnPair(t)
	defer serverWS.Close()
	defer clientWS.Close()

	// Start the client reader goroutine BEFORE Attach — net.Pipe is
	// synchronous so the write in wsSend blocks until the other side reads.
	var messages []string
	readDone := make(chan struct{})

	go func() {
		defer close(readDone)
		for i := 0; i < 10; i++ {
			clientWS.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, raw, err := clientWS.ReadMessage()
			if err != nil {
				break
			}
			var msg map[string]any
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			msgType, _ := msg["type"].(string)
			messages = append(messages, msgType)
		}
	}()

	// Attach sends replay + attached through the pipe, consumed by the reader.
	sm.Attach(s.ID, serverWS)

	// Simulate live PTY output arriving after attach — now that the connection
	// is in the Viewers set, Broadcast delivers to it.
	s.Broadcast("output", map[string]any{"data": "live data"})

	<-readDone
	close(s.Done)

	if len(messages) == 0 {
		t.Fatal("no messages received")
	}

	// Verify ordering: "replay" must come before any "output".
	replayIdx := -1
	outputIdx := -1
	for i, m := range messages {
		if m == "replay" && replayIdx == -1 {
			replayIdx = i
		}
		if m == "output" && outputIdx == -1 {
			outputIdx = i
		}
	}

	if replayIdx == -1 {
		t.Fatalf("no replay message received; got: %v", messages)
	}
	if outputIdx != -1 && outputIdx < replayIdx {
		t.Fatalf("received 'output' before 'replay'; message order: %v", messages)
	}
}

// TestRingBufferUTF8Boundary verifies that Bytes() returns valid UTF-8 even
// when the circular buffer wraps in the middle of a multi-byte codepoint.
func TestRingBufferUTF8Boundary(t *testing.T) {
	// Use a small buffer so we can force a wrap easily.
	// "✓" is U+2713, encoded as 3 bytes: 0xE2 0x9C 0x93
	checkmark := "✓"
	if len(checkmark) != 3 {
		t.Fatalf("expected 3-byte checkmark, got %d", len(checkmark))
	}

	// Buffer of size 5. Write "ABC✓" (6 bytes) to force wrap.
	rb := newRingBuffer(5)
	rb.Write([]byte("ABC" + checkmark)) // 6 bytes wraps around a 5-byte buffer

	got := rb.Bytes()
	if !utf8.Valid(got) {
		t.Fatalf("Bytes() returned invalid UTF-8: %q (hex: %x)", got, got)
	}

	// The first byte(s) of the checkmark should be skipped since they're orphaned
	// continuation bytes from the circular split. The result should end with
	// the complete checkmark's last byte(s) or skip the broken codepoint entirely.
	// With buffer size 5, "ABC✓" = [0x41,0x42,0x43,0xE2,0x9C,0x93]
	// After wrap: buf = [0x93,0x42,0x43,0xE2,0x9C], pos=1, full=true
	// Bytes raw = buf[1:] + buf[:1] = [0x42,0x43,0xE2,0x9C,0x93] = "BC✓"
	// "BC✓" is valid UTF-8 so no bytes should be skipped.
	if string(got) != "BC"+checkmark {
		t.Fatalf("expected %q, got %q", "BC"+checkmark, string(got))
	}

	// Now test a case where the split actually orphans continuation bytes.
	// Buffer of size 4: write "AB✓" (5 bytes) to force wrap mid-codepoint.
	rb2 := newRingBuffer(4)
	rb2.Write([]byte("AB" + checkmark)) // 5 bytes wraps around 4-byte buffer
	// buf = [0x93,0x42,0xE2,0x9C], pos=1, full=true
	// Bytes raw = buf[1:] + buf[:1] = [0x42,0xE2,0x9C,0x93] = "B✓"
	// This is actually valid UTF-8!
	got2 := rb2.Bytes()
	if !utf8.Valid(got2) {
		t.Fatalf("Bytes() returned invalid UTF-8: %q (hex: %x)", got2, got2)
	}

	// Force orphaned bytes: buffer of size 3, write "A✓" (4 bytes).
	// buf = [0x93,0x41,0xE2], pos=1, full=true
	// Bytes raw = buf[1:] + buf[:1] = [0x41,0xE2,0x93] = "A" + 0xE2 + 0x93
	// 0xE2 starts a 3-byte sequence but 0x93 is a continuation byte —
	// DecodeRune("A...") returns 'A' (valid), then DecodeRune([0xE2,0x93])
	// returns RuneError. But our skip logic only skips leading invalid bytes.
	// Let's use a buffer of 2 to get a clear orphan case.
	rb3 := newRingBuffer(2)
	rb3.Write([]byte(checkmark)) // 3 bytes into 2-byte buffer
	// buf = [0x93,0xE2], pos=1 (wraps: 0xE2@0, 0x9C@1(wrap,full), 0x93@0, pos=1)
	// Wait, let me retrace: Write byte-by-byte: 0xE2→pos0, 0x9C→pos1(wrap,full=true), 0x93→pos0, pos=1
	// buf = [0x93, 0x9C], pos=1, full=true
	// Bytes raw = buf[1:] + buf[:1] = [0x9C, 0x93]
	// Both are continuation bytes (0x80-0xBF) — should be skipped entirely.
	got3 := rb3.Bytes()
	if !utf8.Valid(got3) {
		t.Fatalf("Bytes() returned invalid UTF-8: %q (hex: %x)", got3, got3)
	}
	if len(got3) != 0 {
		t.Fatalf("expected empty result after skipping orphaned bytes, got %q (hex: %x)", got3, got3)
	}
}

// TestSplitUTF8 verifies the splitUTF8 helper correctly separates valid UTF-8
// from trailing incomplete multi-byte sequences.
func TestSplitUTF8(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantValid string
		wantRem   int // expected remainder length
	}{
		{"ascii only", []byte("hello"), "hello", 0},
		{"complete multibyte", []byte("hello✓"), "hello✓", 0},
		{"trailing 1 of 3", []byte{'h', 'i', 0xE2}, "hi", 1},
		{"trailing 2 of 3", []byte{'h', 'i', 0xE2, 0x9C}, "hi", 2},
		{"empty", []byte{}, "", 0},
		{"only incomplete", []byte{0xE2, 0x9C}, "", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, rem := splitUTF8(tt.input)
			if string(valid) != tt.wantValid {
				t.Errorf("valid = %q, want %q", valid, tt.wantValid)
			}
			if len(rem) != tt.wantRem {
				t.Errorf("remainder len = %d, want %d (rem = %x)", len(rem), tt.wantRem, rem)
			}
		})
	}
}
