# Crossagent — AI Agent Instructions

## What This Project Is

Crossagent is a cross-model AI agent orchestrator with a browser-based Web UI. It ships with builtin `claude`, `codex`, and `gemini` agents, and each workflow phase can be reassigned to any registered static agent.

Default phase mapping:

1. **Phase 1 — Plan**: `claude`
2. **Phase 2 — Review**: `codex`
3. **Phase 3 — Implement**: `claude`
4. **Phase 4 — Verify**: `codex`

## Project Structure

```
crossagent/
├── go.mod              # Go module (github.com/grikwong/crossagent)
├── cmd/crossagent/     # Go CLI entry point (fully wired, includes `serve` command)
├── internal/
│   ├── state/          #   Data layer — config, workflow, project, memory
│   ├── agent/          #   Agent registry, phase assignments, CLI launcher
│   ├── cli/            #   JSON types, ordered serialization, hybrid formatting
│   ├── prompt/         #   Template-based prompt generation & memory context
│   │   └── templates/  #     Embedded Go templates (general, plan, review, implement, verify)
│   ├── judge/          #   Verdict parsing for review & verify outputs
│   └── web/            #   Embedded HTTP/WebSocket server + frontend assets
│       ├── embed.go    #     go:embed directive for public/ assets
│       ├── server.go   #     HTTP server setup, routes, static file serving
│       ├── api.go      #     REST API handlers (22 endpoints)
│       ├── terminal.go #     WebSocket + PTY handler, chat history capture
│       └── public/     #     Vanilla JS frontend (HTML, CSS, JS, vendored libs)
├── crossagent-legacy.sh  # Deprecated bash CLI (no longer maintained or tested)
├── test/             # Integration test suite
│   └── integration_test.sh
├── web/              # Legacy Node.js server (retained for reference, no longer runtime-required)
├── docs/             # Architecture decision record
├── CLAUDE.md         # This file
├── README.md         # Human documentation
└── Makefile          # Build, test, install, check, start targets
```

Workflow state is stored in `~/.crossagent/`:
```
~/.crossagent/
├── current                          # Active workflow name
├── agents/<name>                    # Custom static agent definitions
├── projects/                        # Project definitions
│   ├── default/                     #   Default project (auto-created)
│   │   └── memory/
│   │       ├── project-context.md   #   Project-scoped memory
│   │       ├── lessons-learned.md   #   Project-scoped lessons
│   │       └── features/            #   Migrated feature memory files
│   └── <name>/                      #   User-created projects
│       └── memory/
│           ├── project-context.md
│           └── lessons-learned.md
├── memory/                          # Global memory (cross-project)
│   ├── global-context.md            # Patterns, conventions, accumulated knowledge
│   └── lessons-learned.md           # Retrospective insights
└── workflows/<name>/
    ├── config                       # Key-value config (repo, add_dirs, created, project, phase agent assignments)
    ├── description                  # Feature description (multi-line)
    ├── phase                        # Current phase (1-4 or "done")
    ├── memory.md                    # Workflow-scoped memory (decisions, findings, notes)
    ├── plan.md                      # Phase 1 output
    ├── review.md                    # Phase 2 output
    ├── verify.md                    # Phase 4 output
    └── prompts/                     # Generated prompt files
```

## Memory System

Crossagent has a persistent memory system with three tiers:

- **Workflow memory** (`memory.md` in the workflow directory) — per-workflow decisions, findings, and session notes. Initialized on `crossagent new` and updated by each phase agent.
- **Project memory** (`~/.crossagent/projects/<name>/memory/`) — per-project patterns, conventions, domain knowledge, and feature context. Shared across all workflows in a project. May contain a `features/` subdirectory with feature-level memory files.
- **Global memory** (`~/.crossagent/memory/`) — cross-project patterns, conventions, and accumulated knowledge. Updated when agents discover broadly reusable insights.

Memory flows into prompts via memory context builders that inject all three tiers (workflow -> project -> global) into each phase prompt. Each prompt also includes memory update instructions telling the agent how to update memory at each tier after completing its work.

- `prompt.BuildMemoryContext()`, `prompt.BuildMemoryUpdateInstructions()`, `agent.BuildLaunchArgs()`, `agent.GenSandboxSettings()`, `prompt.GenerateGeneralInstructions()`

Project memory is also wired into:
- Launch args — added as `--add-dir` so agents can read/write project memory
- Sandbox settings — added to `allowWrite` so agents have write access
- General instructions — listed in workspace directories and memory system description

The `crossagent memory` CLI subcommand manages memory:
- `crossagent memory show [--global|--project [name]] [--json]` — display memory content
- `crossagent memory list [--global|--project [name]] [--json]` — list memory files
- `crossagent memory edit [--global|--project [name]]` — open memory in editor

## Projects

Workflows are grouped under projects. Each workflow has exactly one parent project (defaults to `default`). Projects provide:
- Scoped memory shared across the project's workflows
- Organization of related workflows
- Auto-suggestion of the best project for new workflows

The `crossagent projects` CLI subcommand manages projects:
- `crossagent projects list [--json]` — list all projects
- `crossagent projects new <name>` — create a new project
- `crossagent projects delete <name>` — delete (moves workflows to default)
- `crossagent projects show <name> [--json]` — show project details
- `crossagent projects rename <old> <new>` — rename a project
- `crossagent projects suggest [--description <text>] [--json]` — suggest matching project
- `crossagent move <workflow> --project <project>` — move workflow to project

## Go Conventions

- Two external dependencies: `github.com/creack/pty` (Unix PTY) and `github.com/gorilla/websocket` (WebSocket server) — both for the embedded web server
- Atomic writes via temp file + rename pattern to prevent partial-write corruption
- File locking (flock) for concurrent `SetConf` calls
- Embedded templates via `embed.FS` in `internal/prompt/templates/`
- State files use a simple key=value format
- Tests use `t.TempDir()` with swappable `homeDir` for isolation

## Web UI Conventions

- The Web UI is an embedded Go HTTP/WebSocket server in `internal/web/`, served via `crossagent serve`
- API handlers shell out to the crossagent binary for complex operations (guarantees JSON compatibility)
- All user input is validated server-side before processing
- PTY sessions use `creack/pty` + `gorilla/websocket`; launch params come from `crossagent phase-cmd --json`
- Frontend uses vanilla JS with native ES modules (`<script type="module">`), no build step — xterm.js, addon-fit, and marked are vendored in `public/vendor/` as classic scripts that load before the module graph. Frontend modules live under `public/js/` (`state.js`, `api.js`, `util.js`, `derive.js`, `modals.js`, `v2.js`) with per-region modules under `public/js/regions/` (`titlebar.js`, `workflow-list.js`, `pipeline-header.js`, `pipeline-board.js`, `artifact-reader.js`, `artifact-info-rail.js`, `terminal-drawer.js`). Each region exports `mount(el)` and `render()`; all UI truth flows through the `store` in `state.js`. The default layout is the pipeline-timeline (Variation B) defined by `.app-v2` in `index.html` and styled by `pipeline.css` + `terminal-drawer.css`. The legacy stacked-sidebar `.app` shell is kept hidden as a compat layer that exposes the existing modal buttons — v2 regions delegate to it via `element.click()`.
- WebSocket protocol: `spawn`, `input`, `resize`, `kill` from client; `output`, `spawned`, `exit`, `error` from server
- Chat history is captured per-session with a 50MB buffer cap, flushed atomically on exit/kill/disconnect

## Layered Architecture

See [docs/architecture.md](docs/architecture.md) for the full decision record.

1. **Core layer** — Go packages (`internal/`) provide state management, agent orchestration, prompt generation, and verdict judging. The CLI entry point is `cmd/crossagent/main.go`.
2. **Integration layer** — `--json` output on `status`, `list`, `phase-cmd`, and `agents`.
3. **Web UI layer** — Embedded Go HTTP/WebSocket server in `internal/web/`. Serves frontend assets, REST API, and terminal sessions.

Critical boundaries:
- Web UI never writes to `~/.crossagent/` directly — it calls `crossagent advance`/`done` for state changes
- Web UI owns PTY sessions; the Go binary provides launch params via `crossagent phase-cmd <phase> --json`
- Web UI exposes agent management through CLI `agents` commands, not direct config writes
- No orchestration logic is duplicated outside the Go core

## When Modifying the CLI

- Supported agent adapters are `claude`, `codex`, and `gemini`
- Sandbox-fallback artifact recovery: OS-level sandboxes (notably gemini's `--sandbox` seatbelt profile on macOS) may deny writes to `~/.crossagent/workflows/<name>/` if paths are not canonicalized. Crossagent now automatically resolves all symlinks (e.g. `/Users` vs `/private/var/users`) and explicitly authorizes the `~/.crossagent` home directory to ensure write access to all state. If a fallback still occurs, the state helpers `state.RecoverMisplacedOutput` and `state.RecoverWorkflowOutputs` relocate files from the repo root (agent CWD) back into the workflow directory. Recovery is wired into `crossagent advance`, the Web UI's `/api/workflow/{name}/check-file` polling, and `/check-advance`; users see a yellow "Sandbox-fallback: relocated …" line in the terminal when it triggers. Any new code path that asserts the presence of a phase-output file should run recovery first via `state.RecoverWorkflowOutputs(wfDir, repo)` so behavior is consistent.
- Maintain the phase gate pattern: each phase checks prerequisites before running
- All output files (plan.md, review.md, verify.md) are written by the launched AI, not by crossagent itself
- The workflow dir is always passed as `--add-dir` to both adapters
- Agent adapters are plugins: create `internal/agent/adapter_<name>.go` implementing the `Adapter` interface (Name/DisplayName/DefaultCommand/Plan) and call `RegisterAdapter(<yourAdapter>{})` from `init()`. Everything downstream (builtin-agent list, CLI validation messages, Web UI dropdown, `LaunchAgent` dispatch, `BuildPhaseCmd` dispatch, `TestAdapterRegistry_*` contracts) reads from the registry — no other code changes required. The current builtins are `claude`, `codex`, and `gemini`
- Prompt templates live in `internal/prompt/templates/` as embedded `.md.tmpl` files
- Verdict parsing must handle case-insensitive matching and various phrasings
- Run tests with `make test` (runs `go test ./...` then integration tests)

## When Modifying Go Packages

- Use atomic writes (`atomicWrite`) for any state mutation
- Agent adapters are plugins: create `internal/agent/adapter_<name>.go` implementing the `Adapter` interface (Name/DisplayName/DefaultCommand/Plan) and call `RegisterAdapter(<yourAdapter>{})` from `init()`. Everything downstream (builtin-agent list, CLI validation messages, Web UI dropdown, `LaunchAgent` dispatch, `BuildPhaseCmd` dispatch, `TestAdapterRegistry_*` contracts) reads from the registry — no other code changes required. The current builtins are `claude`, `codex`, and `gemini`
- Prompt templates live in `internal/prompt/templates/` as embedded `.md.tmpl` files
- Verdict parsing must handle case-insensitive matching and various phrasings
- Run tests with `go test ./internal/...`

## When Modifying the Web UI

- The web server lives in `internal/web/` — `server.go` (routing), `api.go` (handlers), `terminal.go` (WebSocket+PTY)
- API handlers use `exec.Command` to shell out to the crossagent binary for JSON-producing operations
- Validate all inputs server-side (names, phases, artifact types)
- Don't add a frontend build step — keep it as plain HTML/CSS/JS
- Frontend assets are embedded via `go:embed all:public` in `embed.go`
- Vendored browser libraries (xterm.js, marked) live in `public/vendor/`
- Test by running `make start` (builds binary then launches `crossagent serve`)

## Before Pushing

Always run the full CI check suite locally before pushing to avoid CI failures:

```bash
go vet ./...
go test ./...
staticcheck ./...                    # install: go install honnef.co/go/tools/cmd/staticcheck@v0.6.1
govulncheck ./...                    # install: go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
bash test/integration_test.sh ./crossagent   # requires a fresh build first
```

The GitHub CI runs all of the above (see `.github/workflows/ci.yml` and `security.yml`). Catching issues locally saves a push-fix cycle.
