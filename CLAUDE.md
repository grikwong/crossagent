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
├── crossagent          # Bash CLI — core engine (orchestration, state, prompts)
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
└── Makefile          # Install, check, start targets
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

Memory flows into prompts via `gen_memory_context()`, which injects all three tiers (workflow -> project -> global) into each phase prompt. Each prompt also includes `gen_memory_update_instructions()` telling the agent how to update memory at each tier after completing its work.

Project memory is also wired into:
- `_build_launch_args()` — added as `--add-dir` so agents can read/write project memory
- `_gen_sandbox_settings()` — added to `allowWrite` so agents have write access
- `gen_general_instructions()` — listed in workspace directories (section 9) and memory system description (section 8)

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

## Bash CLI Conventions

- Pure bash with `set -euo pipefail`, compatible with bash 3.2+ (macOS system bash)
- Functions prefixed by purpose: `cmd_*` (commands), `gen_*` (prompt generation), `get_*/set_*` (state)
- No namerefs (`local -n`), no `${,,}` lowercase — these require bash 4.x+
- Colors via ANSI escape codes, stored in uppercase variables
- State is stored as simple text files under `~/.crossagent/`
- JSON output is generated manually; do not add a JSON dependency
- Static agents stored as config files under `~/.crossagent/agents/`
- CLI launches use `|| true` to survive non-zero exits (Ctrl+C, errors)

## Web UI Conventions

- Node.js server uses `execFileSync` (not `execSync`) to call the CLI — no shell interpolation
- All user input is validated server-side before passing to CLI commands
- PTY sessions are owned by the Web UI; launch params come from `crossagent phase-cmd --json`
- Frontend uses vanilla JS, no build step — CDN for xterm.js and marked
- WebSocket protocol: `spawn`, `input`, `resize` from client; `output`, `spawned`, `exit`, `error` from server

## Layered Architecture

See [docs/architecture.md](docs/architecture.md) for the full decision record.

1. **Core layer** — bash CLI (`crossagent`). Source of truth for orchestration, state, and prompt generation.
2. **Integration layer** — `--json` output on `status`, `list`, `phase-cmd`, and `agents`.
3. **Web UI layer** — Node.js companion in `web/`. Embeds terminals, renders artifacts, manages workflows.

Critical boundaries:
- Web UI never writes to `~/.crossagent/` directly — it calls `crossagent advance`/`done` for state changes
- Web UI owns PTY sessions; bash provides launch params via `crossagent phase-cmd <phase> --json`
- Web UI exposes agent management through CLI `agents` commands, not direct config writes
- No orchestration logic is duplicated outside the bash core

## When Modifying the Bash CLI

- Keep zero external dependencies — only bash built-ins, coreutils, and the supported agent CLIs
- Supported agent adapters are `claude` and `codex`
- Maintain the phase gate pattern: each phase checks prerequisites before running
- All output files (plan.md, review.md, verify.md) are written by the launched AI, not by crossagent itself
- The workflow dir is always passed as `--add-dir` to both adapters
- Prompt templates are the most impactful thing to improve
- Test changes with `crossagent new test-workflow --repo /tmp/test-repo`

## When Modifying the Web UI

- Keep the server thin — it should only proxy CLI commands and manage PTY sessions
- Validate all inputs server-side (names, phases, artifact types)
- Use `execFileSync` with array args to prevent command injection
- Don't add a frontend build step — keep it as plain HTML/CSS/JS
- Test by running `make start` (runs preflight checks then launches server)
