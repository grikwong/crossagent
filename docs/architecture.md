# Crossagent — Architecture

> Date: 2026-03-18 | Status: Implemented

## System Overview

Crossagent is a cross-model AI agent orchestrator that routes each workflow phase to a different AI model. The default phase mapping is:

1. **Plan** — Claude Code
2. **Review** — Codex CLI
3. **Implement** — Claude Code
4. **Verify** — Codex CLI

Phase assignments are configurable per workflow via the `agents assign` command.

## Layered Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Web UI Layer                                           │
│  Embedded Go HTTP/WebSocket server + vanilla JS frontend│
│  internal/web/                                          │
├─────────────────────────────────────────────────────────┤
│  Integration Layer                                      │
│  --json output on status, list, phase-cmd, agents       │
├─────────────────────────────────────────────────────────┤
│  Core Layer                                             │
│  Go packages: state, agent, prompt, judge, cli          │
│  cmd/crossagent/main.go: CLI entry point                │
└─────────────────────────────────────────────────────────┘
```

### Core Layer

Go packages in `internal/` provide all business logic:

- **`internal/state/`** — Config, workflow, project, and memory file operations. Uses atomic writes (temp file + rename) and file locking (`syscall.Flock`) for concurrent access safety.
- **`internal/agent/`** — Agent registry, phase assignments, CLI launcher. Supports `claude`, `codex`, and `gemini` adapters. Builds launch arguments including `--add-dir` flags and sandbox settings.
- **`internal/cli/`** — JSON types, ordered serialization, hybrid formatting for CLI output.
- **`internal/prompt/`** — Template-based prompt generation using embedded Go templates (`embed.FS`). Injects three-tier memory context into each phase prompt.
- **`internal/judge/`** — Verdict parsing for review and verify outputs. Handles case-insensitive matching across various phrasings.

The CLI entry point (`cmd/crossagent/main.go`) handles all command dispatch.

### Integration Layer

Machine-readable `--json` output on `status`, `list`, `phase-cmd`, and `agents` commands. This layer enables the Web UI and external tooling to consume CLI output programmatically.

### Web UI Layer

An embedded Go HTTP/WebSocket server in `internal/web/`:

- **`embed.go`** — `go:embed all:public` directive bundles the vanilla JS frontend into the binary.
- **`server.go`** — HTTP router with API routes and static file serving.
- **`api.go`** — REST API handlers. These shell out to the `crossagent` binary for operations that produce complex JSON, guaranteeing identical output between CLI and API.
- **`terminal.go`** — WebSocket + PTY handler using `creack/pty` and `gorilla/websocket`. Manages interactive terminal sessions for each phase agent.

The frontend (`internal/web/public/`) is plain HTML, CSS, and vanilla JS with no build step. Browser dependencies (xterm.js, xterm-addon-fit, marked) are vendored in `public/vendor/`.

## Data Architecture

All state is stored as plain files in `~/.crossagent/`:

```
~/.crossagent/
├── current                          # Active workflow name
├── agents/<name>                    # Custom static agent definitions
├── projects/                        # Project definitions
│   ├── default/                     #   Default project (auto-created)
│   │   └── memory/                  #   Project-scoped memory
│   └── <name>/                      #   User-created projects
│       └── memory/
├── memory/                          # Global memory (cross-project)
│   ├── global-context.md
│   └── lessons-learned.md
└── workflows/<name>/
    ├── config                       # Key-value config
    ├── description                  # Feature description
    ├── phase                        # Current phase (1-4 or "done")
    ├── memory.md                    # Workflow-scoped memory
    ├── plan.md / review.md / verify.md  # Phase artifacts
    └── prompts/                     # Generated prompt files
```

### Three-Tier Memory System

1. **Workflow memory** (`memory.md`) — per-workflow decisions, findings, session notes.
2. **Project memory** (`~/.crossagent/projects/<name>/memory/`) — project-specific patterns and conventions shared across workflows.
3. **Global memory** (`~/.crossagent/memory/`) — cross-project patterns and accumulated knowledge.

Memory flows into prompts via `prompt.BuildMemoryContext()` and `prompt.BuildMemoryUpdateInstructions()`.

### File Safety

- **Atomic writes** — all state mutations use a temp file + rename pattern to prevent partial-write corruption.
- **File locking** — `syscall.Flock` for concurrent `SetConf` calls.

## WebSocket + PTY Protocol

The Web UI spawns interactive agent sessions via WebSocket:

1. Client fetches launch params: `GET /api/phase-cmd/{phase}` → returns command, args, cwd, prompt.
2. Client sends `spawn` message → server creates a PTY via `creack/pty` and starts the agent process.
3. Bidirectional streaming: `input` (client → server), `output` (server → client).
4. Client can send `resize` (terminal dimensions) and `kill` (terminate session).
5. Server sends `spawned`, `exit`, and `error` messages for lifecycle events.
6. Chat history is captured per-session with a 50MB buffer cap, flushed atomically on exit/kill/disconnect.

## Agent Adapters

Three built-in adapters:

- `claude` (Claude Code CLI) — `--permission-mode auto` auto-approves tool calls; a generated `--settings` JSON enables the built-in sandbox with `allowWrite` pinned to {workflow dir, repo, global/project memory, add_dirs}; repeated `--add-dir` flags surface those dirs as workspace roots.
- `codex` (Codex CLI) — `--full-auto` auto-approves tool calls; `-c sandbox_mode="workspace-write"` forces the workspace-write sandbox (overriding any user default), and `-c sandbox_workspace_write.writable_roots=[…]` explicitly lists every add-dir (memory + add_dirs) as writable, so writes outside `{repo} ∪ writable_roots` are blocked; a pre-trust override avoids the "Do you trust this folder?" prompt; the full prompt content is inlined (general + phase concatenated).
- `gemini` (Google Gemini CLI) — `--yolo` auto-approves tool calls, `--sandbox` runs the agent under an OS-level sandbox (sandbox-exec on macOS, Docker/Podman on Linux) scoped to the project dir plus `--include-directories <comma-list>`; the bootstrap prompt is passed via `-p`.

All three adapters enforce the same invariant: **no writes outside the directories crossagent explicitly hands to the agent** (workflow dir, repo, global/project memory, and any configured `add_dirs`).

Each adapter constructs launch arguments including:
- Workspace directory flags (`--add-dir` for claude/codex, `--include-directories` for gemini) covering the workflow directory, global/project memory, and any extra `add_dirs`
- Auto-approval / sandbox settings appropriate for the adapter
- The prompt (file path or inlined content) pointing to the generated phase prompt

Custom agents can be registered via `crossagent agents add` with an adapter type and optional custom command.

## CI/CD Pipeline

- **CI** (`.github/workflows/ci.yml`) — `go vet`, build, unit tests, integration tests, GoReleaser config validation.
- **Security** (`.github/workflows/security.yml`) — `govulncheck`, `go vet`, `staticcheck` with pinned versions.
- **Release** (`.github/workflows/release.yml`) — `release-please` for automated semantic versioning + GoReleaser for cross-platform binary builds and Homebrew formula updates.

## Architectural Boundaries

1. The Go binary handles all command dispatch and state management.
2. The Web UI never writes to `~/.crossagent/` directly — it calls `crossagent advance`/`done` for state changes.
3. The Web UI uses `phase-cmd --json` for launch params — no duplicated phase logic.
4. The Web UI exposes agent management through CLI commands, not direct config writes.
5. Every workflow action remains possible via CLI alone.
