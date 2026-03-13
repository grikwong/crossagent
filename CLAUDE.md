# Crossagent — AI Agent Instructions

## What This Project Is

Crossagent is a cross-model AI agent orchestrator with a browser-based Web UI. It ships with builtin `claude` and `codex` agents, and each workflow phase can be reassigned to any registered static agent.

Default phase mapping:

1. **Phase 1 — Plan**: `claude`
2. **Phase 2 — Review**: `codex`
3. **Phase 3 — Implement**: `claude`
4. **Phase 4 — Verify**: `codex`

## Project Structure

```
crossagent/
├── go.mod              # Go module (github.com/pvotal-tech/crossagent)
├── cmd/crossagent/     # Go CLI entry point (fully wired)
├── internal/
│   ├── state/          #   Data layer — config, workflow, project, memory
│   ├── agent/          #   Agent registry, phase assignments, CLI launcher
│   ├── cli/            #   JSON types, ordered serialization, hybrid formatting
│   ├── prompt/         #   Template-based prompt generation & memory context
│   │   └── templates/  #     Embedded Go templates (general, plan, review, implement, verify)
│   └── judge/          #   Verdict parsing for review & verify outputs
├── crossagent-legacy.sh  # Deprecated bash CLI (retained for compatibility testing)
├── test/             # Integration test suite
│   └── integration_test.sh
├── web/              # Web UI — Node.js companion app
│   ├── server.js     #   Express + WebSocket + PTY server
│   ├── package.json  #   Dependencies (express, node-pty, ws)
│   └── public/       #   Browser frontend (HTML, CSS, JS)
│       ├── index.html
│       ├── app.js
│       └── style.css
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

- Zero external dependencies — only the Go standard library
- Atomic writes via temp file + rename pattern to prevent partial-write corruption
- File locking (flock) for concurrent `SetConf` calls
- Embedded templates via `embed.FS` in `internal/prompt/templates/`
- State files use a simple key=value format
- Tests use `t.TempDir()` with swappable `homeDir` for isolation

## Web UI Conventions

- Node.js server uses `execFileSync` (not `execSync`) to call the CLI — no shell interpolation
- All user input is validated server-side before passing to CLI commands
- PTY sessions are owned by the Web UI; launch params come from `crossagent phase-cmd --json`
- Frontend uses vanilla JS, no build step — CDN for xterm.js and marked
- WebSocket protocol: `spawn`, `input`, `resize` from client; `output`, `spawned`, `exit`, `error` from server

## Layered Architecture

See [docs/architecture.md](docs/architecture.md) for the full decision record.

1. **Core layer** — Go packages (`internal/`) provide state management, agent orchestration, prompt generation, and verdict judging. The CLI entry point is `cmd/crossagent/main.go`.
2. **Integration layer** — `--json` output on `status`, `list`, `phase-cmd`, and `agents`.
3. **Web UI layer** — Node.js companion in `web/`. Embeds terminals, renders artifacts, manages workflows.

Critical boundaries:
- Web UI never writes to `~/.crossagent/` directly — it calls `crossagent advance`/`done` for state changes
- Web UI owns PTY sessions; the Go binary provides launch params via `crossagent phase-cmd <phase> --json`
- Web UI exposes agent management through CLI `agents` commands, not direct config writes
- No orchestration logic is duplicated outside the Go core

## When Modifying the CLI

- Keep zero external dependencies — only the Go standard library
- Supported agent adapters are `claude` and `codex`
- Maintain the phase gate pattern: each phase checks prerequisites before running
- All output files (plan.md, review.md, verify.md) are written by the launched AI, not by crossagent itself
- The workflow dir is always passed as `--add-dir` to both adapters
- Agent adapters: "claude" and "codex" — new adapters require updates in both `agent.go` and `launcher.go`
- Prompt templates live in `internal/prompt/templates/` as embedded `.md.tmpl` files
- Verdict parsing must handle case-insensitive matching and various phrasings
- Run tests with `make test` (runs `go test ./...` then integration tests)

## When Modifying Go Packages

- Keep zero external dependencies — only the Go standard library
- Use atomic writes (`atomicWrite`) for any state mutation
- Agent adapters: "claude" and "codex" — new adapters require updates in both `agent.go` and `launcher.go`
- Prompt templates live in `internal/prompt/templates/` as embedded `.md.tmpl` files
- Verdict parsing must handle case-insensitive matching and various phrasings
- Run tests with `go test ./internal/...`

## When Modifying the Web UI

- Keep the server thin — it should only proxy CLI commands and manage PTY sessions
- Validate all inputs server-side (names, phases, artifact types)
- Use `execFileSync` with array args to prevent command injection
- Don't add a frontend build step — keep it as plain HTML/CSS/JS
- Test by running `make start` (runs preflight checks then launches server)
