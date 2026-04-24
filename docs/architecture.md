# Crossagent — Architecture

> Date: 2026-04-24 | Status: Implemented

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
- **`session.go`** — `SessionManager` struct: tracks active PTY sessions, implements atomic spawn deduplication via `TryClaimSpawn`/`ReleaseSpawnClaim`, and manages scrollback replay on reconnect.

The frontend (`internal/web/public/`) is plain HTML, CSS, and vanilla JS with no build step. Browser dependencies (xterm.js, xterm-addon-fit, marked) are vendored in `public/vendor/`. The default layout is the **pipeline-timeline** (`.app-v2` shell in `index.html`), styled by `pipeline.css` and `terminal-drawer.css`. The legacy stacked-sidebar `.app` shell is kept hidden as a compat layer.

### Frontend Module Architecture

The frontend uses native ES modules (`<script type="module">`). Modules under `public/js/`:

| Module | Role |
|--------|------|
| `state.js` | Single store with `setState(patch)` / `subscribe(fn)` — all UI truth lives here |
| `api.js` | `api(path, opts)` and `wfApi(name, path, opts)` fetch helpers |
| `util.js` | Shared constants (`PHASE_NAMES`) and pure helpers (`esc`, `capitalize`) |
| `derive.js` | Computed values derived from the store (selected workflow, phase status, etc.) |
| `modals.js` | Modal open/close helpers |
| `v2.js` | Mounts and wires all region components for the pipeline-timeline layout |

Region components live under `public/js/regions/` and each exports `mount(el)` + `render()`:

| Region | Role |
|--------|------|
| `titlebar.js` | Top bar — workflow selector, search, session status dot |
| `workflow-list.js` | Collapsible workflow list with project groups |
| `pipeline-header.js` | Workflow name, description (editable pre-run), phase progress |
| `pipeline-board.js` | Four-phase card board with run/advance/done controls |
| `artifact-reader.js` | Rendered markdown viewer for phase artifacts |
| `artifact-info-rail.js` | Metadata sidebar for the selected artifact |
| `terminal-drawer.js` | Collapsible xterm.js terminal drawer |

Vendored classic scripts (`public/vendor/`) load before the module graph so xterm.js globals are available to region modules.

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

## Hardened State Management

### Atomic State & Concurrency
Crossagent is designed to be safe for simultaneous use by the CLI and Web UI. All state transitions (phase bumps, configuration changes, memory updates) use an atomic write pattern (temp file + rename) and OS-level file locking (`syscall.Flock`). This ensures that:
- **Zero-Partial Writes**: A crash or interruption during a state write never leaves a corrupted file.
- **Race Condition Prevention**: Concurrent CLI commands and Web API calls are serialized at the file level.
- **Consistent View**: Recovery and state-sync logic run before every phase-advancement check, providing a unified view of the workflow filesystem.

### Multi-Round Workflows
Workflows can be iterated in multiple "rounds." When a workflow is completed (Phase 4 → Done), the **Follow-up** feature archives all phase artifacts (`plan.md`, `review.md`, etc.), retry attempts, and chat logs into a `rounds/N/` directory. 
- **Context Preservation**: The reset to Phase 1 for the new round automatically generates a `followup-context.md` that summarizes previous findings to guide the next iteration.
- **Deep History**: The Web UI allows browsing all previous rounds, including their specific terminal session logs and interim retry attempts.

### Hardened Revert Operation

`crossagent revert` follows a strict ordering to avoid collisions on concurrent retries:

1. Archive the current phase artifact (e.g. `review.md → review.attempt-N.md`) with explicit rename error surfacing.
2. Check `state.LooksSubstantive(sourceArtifact)` before reading the source for revert context — if the artifact is not substantive a warning is emitted and the revert context is generated without issue details.
3. Write `revert-context.md` atomically; abort the whole operation if the write fails (avoids leaving the workflow in a partially-reverted state).
4. Write `retry_count` **before** `SetPhase` so that if `SetPhase` fails the next revert attempt uses the new attempt number (no collision on the archive file name).

`state.LooksSubstantive` is exported (`LooksSubstantive`) so `cmdRevert` can call it without duplicating the threshold logic.

### Robust Artifact Recovery
Sandboxed agents (especially Gemini on macOS) may be forced to write artifacts into the repo root (their CWD) rather than the workflow directory. Crossagent's recovery system:
- **Canonical Path Resolution**: Automatically evaluates symlinks (e.g., `/Users` vs `/private/var/users`) to ensure sandbox profiles correctly authorize the workflow directory.
- **Atomic Relocation**: Moves misplaced artifacts back to their canonical location using a transactional pattern (rollback on failure).
- **Probe Guarding**: Identifies and quarantines 4-byte sandbox probe files (e.g., `test`) to prevent them from overwriting legitimate artifacts.

## WebSocket + PTY Protocol

The Web UI spawns interactive agent sessions via WebSocket:

1. Client fetches launch params: `GET /api/phase-cmd/{phase}` → returns command, args, cwd, prompt.
2. Client sends `spawn` message → server calls `SessionManager.TryClaimSpawn(workflow, phase)` under a lock:
   - If a running session already exists, the server reattaches the connection via `sm.Attach` and replays the scrollback buffer (no second PTY is created).
   - If the slot is unclaimed, the claim is reserved and a PTY is created via `creack/pty`; `ReleaseSpawnClaim` is deferred to release the reservation on exit or error.
   - If another goroutine holds the claim (concurrent spawn race), the server returns an `error` message and the client retries.
3. Bidirectional streaming: `input` (client → server), `output` (server → client).
4. Client can send `resize` (terminal dimensions) and `kill` (terminate session).
5. Server sends `spawned`, `exit`, and `error` messages for lifecycle events.
6. Chat history is captured per-session with a 50MB buffer cap, flushed atomically on exit/kill/disconnect.

## Agent Adapters

Three built-in adapters:

- `claude` (Claude Code CLI) — `--permission-mode auto` auto-approves tool calls; a generated `--settings` JSON enables the built-in sandbox with `allowWrite` pinned to {workflow dir, repo, global/project memory, add_dirs}; repeated `--add-dir` flags surface those dirs as workspace roots.
- `codex` (Codex CLI) — `--full-auto` auto-approves tool calls; `-c sandbox_mode="workspace-write"` forces the workspace-write sandbox (overriding any user default), and `-c sandbox_workspace_write.writable_roots=[…]` explicitly lists every add-dir (memory + add_dirs) as writable, so writes outside `{repo} ∪ writable_roots` are blocked; a pre-trust override avoids the "Do you trust this folder?" prompt; the full prompt content is inlined (general + phase concatenated).
- `gemini` (Google Gemini CLI) — `--yolo` auto-approves tool calls, `--sandbox` runs the agent under an OS-level sandbox (sandbox-exec on macOS, Docker/Podman on Linux) scoped to the project dir plus `--include-directories <comma-list>`. Crossagent automatically **consolidates** subdirectories under their parent and **caps** the list to 5 slots to stay within the macOS seatbelt capacity, while ensuring the workflow and home directories always take the first slots. The bootstrap prompt is passed via `-p`.

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

OS-level sandboxes (notably gemini's `--sandbox` seatbelt profile on macOS) can deny write attempts to `~/.crossagent/workflows/<name>/` if paths are not canonicalized. Crossagent now automatically **resolves all symlinks** (e.g. `/Users` vs `/private/var/users`) and explicitly **authorizes the crossagent home directory** in the adapter launch context. The seatbelt profile bundled with gemini (`bundle/sandbox-macos-*.sb`) grants writes to `INCLUDE_DIR_0..4` populated from `--include-directories`, and `ctx.AllDirs` always puts the workflow dir in the first slot — so in the common path writes to the workflow dir succeed. 

Fallbacks are now rare and handled by `internal/state` recovery helpers:

- `RecoverMisplacedOutput(wfDir, repo, basename)` — atomically relocates a single file from the repo root into the workflow dir (os.Rename on the same filesystem, copy+remove fallback across filesystems). No-op when the workflow copy already exists, the repo copy is missing, or the two paths resolve to the same file. **Probe guard:** candidates that fail `looksSubstantive` (≥ 256 B plus a markdown header for phase outputs) are quarantined in-place with a `.sandbox-probe` suffix instead of being promoted. This prevents sandbox probe files (e.g. a 4-byte `"test"` gemini writes to verify CWD access) from overwriting the canonical artifact slot and stalling the workflow in a plan↔review retry loop.
- `RecoverWorkflowOutputs(wfDir, repo)` — sweeps every entry in `RecoverableArtifacts` (`plan.md`, `review.md`, `implement.md`, `verify.md`, `memory_updates.md`).

Recovery is invoked from three call sites so every downstream check sees a consistent filesystem view:

1. `crossagent advance` (CLI) — sweep before the phase-number bump.
2. `POST /api/workflow/{name}/check-file` (Web UI poll) — recover the single requested artifact, then stat. The response carries `{recovered: bool, recovered_from: string}` so the frontend prints a yellow "Sandbox-fallback: relocated …" line.
3. `POST /api/workflow/{name}/check-advance` — same recovery before the advance decision.

A path-scope guard ensures recovery only ever moves files into the workflow directory of the caller's workflow; paths outside that directory are left alone.

### Advance verdict gate

`crossagent advance` (phase 2→3 and phase 4→done) inspects the corresponding `review.md` / `verify.md` verdict via `internal/judge` and refuses to bump the phase when the parsed verdict is `Rework` (or `Fix` for verify). This closes a half-state window where the Web UI had bumped the phase but the follow-up `/supervise` call had not yet run the revert — which is how workflows could end up at phase=3 with no approval on record. A missing artifact preserves the legacy dry-advance behavior so integration tests and CLI-only flows that advance phases without producing artifacts still work.

### Phase-consistency guard

`crossagent advance` accepts an optional `--expected-phase <1-4>` flag. When provided, the command no-ops if the current phase does not match — preventing TOCTOU races where a concurrent revert or supervise changed the phase between the caller's artifact-existence check and the advance call. The Web UI's `check-advance` and `check-file` handlers always pass this flag, derived from a filename→phase map (`plan.md→1`, `review.md→2`, `implement.md→3`, `verify.md→4`).

The `advanced` field in `check-advance` responses reflects an **actual state change** (pre/post phase comparison) rather than unconditionally returning `true`. This allows the frontend to distinguish a real phase transition from a legitimate no-op (verdict gate or phase mismatch guard).

### Description edit guard

`PUT /api/workflow/{name}/description` atomically replaces a workflow's description file. The handler enforces server-side that editing is only allowed when phase is `"1"`, `plan.md` does not yet exist, and no plan PTY session is running. This prevents the description from drifting out of sync with in-progress work.

Each adapter constructs launch arguments including:
- Workspace directory flags (`--add-dir` for claude/codex, `--include-directories` for gemini) covering the workflow directory, global/project memory, and any extra `add_dirs`
- Auto-approval / sandbox settings appropriate for the adapter
- The prompt (file path or inlined content) pointing to the generated phase prompt

Custom agents can be registered via `crossagent agents add` with an adapter type and optional custom command.

## CI/CD Pipeline

- **CI** (`.github/workflows/ci.yml`) — `go vet`, build, unit tests (`internal/web/session_test.go`, `api_test.go`, `ui_regression_test.go`, …), integration tests, GoReleaser config validation.
- **Security** (`.github/workflows/security.yml`) — `govulncheck`, `go vet`, `staticcheck` with pinned versions.
- **Release** (`.github/workflows/release.yml`) — `release-please` for automated semantic versioning + GoReleaser for cross-platform binary builds and Homebrew formula updates.

## Architectural Boundaries

1. The Go binary handles all command dispatch and state management.
2. The Web UI never writes to `~/.crossagent/` directly — it calls `crossagent advance`/`done` for state changes.
3. The Web UI uses `phase-cmd --json` for launch params — no duplicated phase logic.
4. The Web UI exposes agent management through CLI commands, not direct config writes.
5. Every workflow action remains possible via CLI alone.
