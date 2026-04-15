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

### Adapter plugin model

Adapters live behind a small interface in `internal/agent/adapter.go`:

```go
type Adapter interface {
    Name() string
    DisplayName() string
    DefaultCommand() string
    Plan(ctx *LaunchContext) (*LaunchPlan, error)
}
```

Each adapter is a single file (`adapter_claude.go`, `adapter_codex.go`, `adapter_gemini.go`) that implements the interface and calls `RegisterAdapter(<yourAdapter>{})` from `init()`. Everything downstream is derived from the registry, so adding a new adapter is a one-file change:

- `LaunchAgent` and `BuildPhaseCmd` dispatch via `AdapterFor(name).Plan(ctx)` — no switch statements to update.
- `ListAgents` iterates `AdapterNames()` and builds one builtin per adapter.
- CLI usage strings (`--adapter <…>`) and validation messages are composed from `AdapterNames()`.
- The Web UI dropdown fetches `GET /api/adapters` on open and populates `<option>` tags dynamically.
- `IsBuiltinAgent(name)` and `ValidAdapter(name)` replace the old duplicated allowlists.
- `TestAdapterRegistry_ContractsHoldForAllBuiltins` sweeps every registered adapter and enforces the interface contract, so a new adapter that forgets to wire up DisplayName / DefaultCommand / registration fails loudly in CI.

`LaunchContext` aggregates every input an adapter could need (Repo, WorkflowDir, PromptFile, AllDirs). Adding a new field is backward-compatible — existing adapters simply ignore what they don't read. `LaunchPlan` is the adapter's answer: argv, prompt text for display, and an optional `WorkDir` override (codex leaves it empty because it uses `-C <repo>`; claude and gemini set it to the repo).

The three current adapters are good worked examples:

- `adapter_claude.go` writes a sandbox settings JSON as a side effect in Plan().
- `adapter_codex.go` emits multiple `-c` TOML overrides and inlines the full general+phase prompt.
- `adapter_gemini.go` translates the ordered `AllDirs` list into a comma-joined `--include-directories` value.

### Sandbox-fallback artifact recovery

OS-level sandboxes (notably gemini's `--sandbox` seatbelt profile on macOS, which in practice only grants write access to the working directory) can deny the agent's attempts to write phase outputs into `~/.crossagent/workflows/<name>/`. When that happens the agent falls back to emitting `plan.md` / `review.md` / `implement.md` / `verify.md` (and a gemini-specific `memory_updates.md` staging file) into the repo root — the one location inside its sandbox it is allowed to write.

The `internal/state` package exposes two recovery helpers:

- `RecoverMisplacedOutput(wfDir, repo, basename)` — atomically relocates a single file from the repo root into the workflow dir (os.Rename on the same filesystem, copy+remove fallback across filesystems). No-op when the workflow copy already exists, the repo copy is missing, or the two paths resolve to the same file.
- `RecoverWorkflowOutputs(wfDir, repo)` — sweeps every entry in `RecoverableArtifacts` (`plan.md`, `review.md`, `implement.md`, `verify.md`, `memory_updates.md`).

Recovery is invoked from three call sites so every downstream check sees a consistent filesystem view:

1. `crossagent advance` (CLI) — sweep before the phase-number bump.
2. `POST /api/workflow/{name}/check-file` (Web UI poll) — recover the single requested artifact, then stat. The response carries `{recovered: bool, recovered_from: string}` so the frontend prints a yellow "Sandbox-fallback: relocated …" line.
3. `POST /api/workflow/{name}/check-advance` — same recovery before the advance decision.

A path-scope guard ensures recovery only ever moves files into the workflow directory of the caller's workflow; paths outside that directory are left alone.

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
