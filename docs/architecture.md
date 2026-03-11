# Crossagent — Architecture Decision Record

> Date: 2026-03-11 | Status: Implemented

## Decision

Crossagent uses a **layered architecture**:

1. **Core** — Bash CLI (`crossagent`). Source of truth for orchestration, state, and prompt generation.
2. **Integration** — Machine-readable CLI output (`--json` on `status`, `list`, `phase-cmd`, `agents`).
3. **Web UI** — Local Node.js companion (`web/`). Embeds terminals, renders artifacts, manages workflows visually.

The bash CLI is the engine. The Web UI is the primary operator experience. Both are shipped.

## Why This Architecture

Two separate questions led here:

1. **Best stack for embedded interactive terminals?**
   Node.js + `node-pty` + `xterm.js` — same stack VS Code uses. Nothing else comes close for PTY embedding.

2. **Best product architecture given what exists?**
   Keep bash as the core engine. Don't rewrite working orchestration into Node. Add the GUI as a companion that consumes the CLI's structured output.

This avoids rewrite risk while using the strongest PTY tooling where it matters.

## Technology Assessment

| Stack | PTY | Multi-panel | Build Time | Fit |
|-------|-----|-------------|------------|-----|
| **Node.js + xterm.js** | Excellent | Excellent | 1-2 wk | **Best** |
| Electron | Excellent | Excellent | 2-3 wk | Runner-up |
| Go + Bubbletea | Fair* | Good | 2-3 wk | Best TUI |
| Tauri | Good | Excellent | 3-5 wk | Overengineered |
| Swift + SwiftUI | Fair | Good | 4-6 wk | Mac only |

*Uses suspend/exec pattern, not embedding

## PTY Ownership Boundary

The UI embeds interactive `claude`/`codex` sessions. The **UI process owns the PTY**, but prompt generation and phase gating live in bash.

Resolution: `phase-cmd --json` returns everything the UI needs to spawn the session itself:

```json
{
  "command": "claude",
  "args": ["--add-dir", "/path/to/wf-dir"],
  "cwd": "/path/to/repo",
  "prompt": "Read and follow the instructions at /path/to/prompts/plan.md",
  "output_file": "/path/to/.crossagent/workflows/name/plan.md",
  "phase": 1
}
```

UI flow:
1. Call `crossagent phase-cmd <phase> --json` to get launch config
2. Spawn the command in its own PTY (embedded in xterm.js)
3. When session ends, call `crossagent advance` or check for output file
4. Read updated state via `crossagent status --json`

No logic duplication. Bash owns orchestration. UI owns presentation and PTY lifecycle.

## Data Ownership

| Data | Owner | Storage |
|------|-------|---------|
| Workflow state | Bash CLI | `~/.crossagent/` files |
| Markdown artifacts | Bash CLI | `~/.crossagent/workflows/<name>/*.md` |
| Agent definitions | Bash CLI | `~/.crossagent/agents/` |
| PTY sessions | Web UI | Transient (in-memory) |
| Presentation state | Web UI | Transient (browser) |

## Architectural Boundaries

1. Bash CLI is the single source of truth for workflow state
2. Web UI never writes to `~/.crossagent/` directly — it calls `crossagent advance`/`done`
3. Web UI uses `phase-cmd --json` for launch params — no duplicated phase logic
4. Web UI exposes agent management through CLI commands, not direct config writes
5. Every workflow action remains possible via CLI alone
