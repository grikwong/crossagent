# Contributing to Crossagent

## Scope

Crossagent is a local-first orchestration tool for multi-agent development workflows. Contributions should preserve these core properties:

- The Bash CLI remains the source of truth for workflow state and orchestration.
- The Web UI remains a companion layer that consumes CLI output rather than duplicating orchestration logic.
- Changes should keep local operator control explicit and visible.

## Before You Start

- Open an issue for significant features, workflow changes, or architecture changes before starting implementation.
- Keep pull requests focused. Separate bug fixes, refactors, and feature work when possible.
- Avoid unrelated formatting churn.

## Development Setup

```bash
git clone <repo-url>
cd crossagent
make install-ui
make check
```

If you want the CLI available on your shell `PATH`:

```bash
make install PREFIX=$HOME/.local
```

## Change Guidelines

- Update documentation when behavior, commands, flags, or workflow expectations change.
- Preserve backward compatibility for documented CLI behavior unless the change is intentional and documented.
- Prefer small, reviewable increments.
- Keep prompts, workflow artifacts, and operator-facing text clear and direct.
- Do not add dependencies without a concrete need.

## Validation

At minimum, contributors should run the checks relevant to their change:

```bash
make check
bash -n crossagent
node --check web/server.js
node --check web/public/app.js
```

If a change affects behavior that is not covered by the commands above, include the exact manual verification steps in the pull request description.

## Pull Requests

- Describe the problem being solved.
- Summarize the approach and notable tradeoffs.
- List validation performed.
- Link related issues.
- Include screenshots or terminal captures for UI changes when useful.

## Licensing

By submitting a contribution, you agree that your work will be licensed under the GNU Affero General Public License, version 3 or later (`AGPL-3.0-or-later`).
