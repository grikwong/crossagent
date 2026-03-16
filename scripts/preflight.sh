#!/usr/bin/env bash
# Crossagent preflight — dependency detection and auto-install for macOS.
# Called by `make check` and `make start`.
#
# Environment variables:
#   CROSSAGENT_ROOT          Project root directory (set by Makefile)
#   CROSSAGENT_AUTO_INSTALL  1 = auto-yes, 0 = report-only, unset = prompt if TTY
set -euo pipefail

# ── Section 1: Configuration and platform detection ─────────────────────────

ROOT="${CROSSAGENT_ROOT:-.}"
cd "$ROOT"

OS="$(uname -s)"
IS_MAC=false
[ "$OS" = "Darwin" ] && IS_MAC=true

# Determine interactive mode
AUTO_INSTALL="${CROSSAGENT_AUTO_INSTALL:-}"
is_interactive() {
  if [ "$AUTO_INSTALL" = "1" ]; then return 0; fi
  if [ "$AUTO_INSTALL" = "0" ]; then return 1; fi
  [ -t 0 ]  # stdin is a terminal
}

# Prompt helper — returns 0 for yes, 1 for no
# Usage: prompt_yn "message" default_yes|default_no
prompt_yn() {
  local msg="$1" default="$2"
  if [ "$AUTO_INSTALL" = "1" ]; then return 0; fi
  if [ "$AUTO_INSTALL" = "0" ]; then
    [ "$default" = "default_yes" ] && return 0
    return 1
  fi
  if ! [ -t 0 ]; then
    [ "$default" = "default_yes" ] && return 0
    return 1
  fi
  local yn
  printf "  %s " "$msg" >/dev/tty
  read -r yn </dev/tty
  case "$yn" in
    [Yy]*) return 0 ;;
    [Nn]*) return 1 ;;
    "")
      [ "$default" = "default_yes" ] && return 0
      return 1
      ;;
    *) return 1 ;;
  esac
}

# ── Section 2: Tier 1 — External prerequisite checks ───────────────────────

MISSING=()
MISSING_LABELS=()
MISSING_INSTALL=()
PASS=true

echo ""
echo "  Crossagent — Preflight Checks"
echo "  ─────────────────────────────────────────"
echo ""

# --- go (1.22+) ---
printf "  %-28s" "go (1.22+)"
if command -v go >/dev/null 2>&1; then
  V="$(go version | awk '{print $3}')"
  MAJOR=$(echo "$V" | sed 's/^go//' | cut -d. -f1)
  MINOR=$(echo "$V" | sed 's/^go//' | cut -d. -f2)
  if [ "${MAJOR:-0}" -ge 1 ] 2>/dev/null && [ "${MINOR:-0}" -ge 22 ] 2>/dev/null; then
    echo "✓  $V"
  else
    echo "✗  $V (need 1.22+)"
    PASS=false
    MISSING+=("go")
    MISSING_LABELS+=("go (upgrade to 1.22+)")
    MISSING_INSTALL+=("brew install go")
  fi
else
  echo "✗  not found"
  PASS=false
  MISSING+=("go")
  MISSING_LABELS+=("go")
  MISSING_INSTALL+=("brew install go")
fi

# --- node (18+) ---
printf "  %-28s" "node (18+)"
if command -v node >/dev/null 2>&1; then
  V="$(node --version)"
  MAJOR=$(echo "$V" | sed 's/^v//' | cut -d. -f1)
  if [ "${MAJOR:-0}" -ge 18 ] 2>/dev/null; then
    echo "✓  $V"
  else
    echo "✗  $V (need v18+)"
    PASS=false
    MISSING+=("node")
    MISSING_LABELS+=("node (upgrade to 18+)")
    MISSING_INSTALL+=("brew install node")
  fi
else
  echo "✗  not found"
  PASS=false
  MISSING+=("node")
  MISSING_LABELS+=("node")
  MISSING_INSTALL+=("brew install node")
fi

# --- npm ---
printf "  %-28s" "npm"
if command -v npm >/dev/null 2>&1; then
  echo "✓  $(npm --version)"
else
  echo "✗  not found"
  PASS=false
  MISSING+=("npm")
  MISSING_LABELS+=("npm (installed with node)")
  MISSING_INSTALL+=("brew install node")
fi

# --- claude (Claude Code CLI) ---
printf "  %-28s" "claude (Claude Code CLI)"
if command -v claude >/dev/null 2>&1; then
  echo "✓  found"
else
  echo "✗  not found"
  PASS=false
  MISSING+=("claude")
  MISSING_LABELS+=("claude (Claude Code CLI)")
  MISSING_INSTALL+=("npm install -g @anthropic-ai/claude-code")
fi

# --- codex (Codex CLI) ---
printf "  %-28s" "codex (Codex CLI)"
if command -v codex >/dev/null 2>&1; then
  echo "✓  found"
else
  echo "✗  not found"
  PASS=false
  MISSING+=("codex")
  MISSING_LABELS+=("codex (Codex CLI)")
  MISSING_INSTALL+=("npm install -g @openai/codex")
fi

# ── Section 3: Report (Tier 1) ─────────────────────────────────────────────

# Filter to only the installable missing deps (dedup brew install node for npm)
UNIQUE_INSTALLS=()
UNIQUE_LABELS=()
seen_node_brew=false
for i in "${!MISSING[@]}"; do
  inst="${MISSING_INSTALL[$i]}"
  # Deduplicate: if npm is missing and node is also missing, one brew install node covers both
  if [ "$inst" = "brew install node" ]; then
    if $seen_node_brew; then continue; fi
    seen_node_brew=true
  fi
  UNIQUE_INSTALLS+=("$inst")
  UNIQUE_LABELS+=("${MISSING_LABELS[$i]}")
done

if [ ${#MISSING[@]} -gt 0 ]; then
  echo ""
  echo "  ${#UNIQUE_INSTALLS[@]} missing dependencies can be auto-installed."
fi

# ── Section 4: Prompt and install (Tier 1) ──────────────────────────────────

install_tier1() {
  # Separate brew-based and npm-based installs
  local brew_installs=()
  local npm_installs=()
  for inst in "${UNIQUE_INSTALLS[@]}"; do
    case "$inst" in
      brew\ *) brew_installs+=("$inst") ;;
      npm\ *)  npm_installs+=("$inst") ;;
    esac
  done

  # Check Homebrew availability if we need it
  if [ ${#brew_installs[@]} -gt 0 ]; then
    if ! command -v brew >/dev/null 2>&1; then
      echo ""
      echo "  Homebrew is required to install: ${brew_installs[*]}"
      echo "  Installing Homebrew runs a remote bootstrap script and modifies system state."
      if prompt_yn "Install Homebrew now? [y/N]" "default_no"; then
        echo "  Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        # Add brew to PATH for this session (Apple Silicon vs Intel)
        if [ -x /opt/homebrew/bin/brew ]; then
          eval "$(/opt/homebrew/bin/brew shellenv)"
        elif [ -x /usr/local/bin/brew ]; then
          eval "$(/usr/local/bin/brew shellenv)"
        fi
      else
        echo ""
        echo "  Skipping Homebrew install. Manual install instructions:"
        for inst in "${brew_installs[@]}"; do
          echo "    - $inst  (requires Homebrew: https://brew.sh)"
        done
        # Clear brew installs — can't install them without brew
        brew_installs=()
      fi
    fi
  fi

  # Install brew-based deps
  local any_failed=false
  for inst in "${brew_installs[@]}"; do
    echo "  → $inst"
    if ! $inst; then
      echo "  ✗ Failed: $inst"
      any_failed=true
    fi
  done

  # Check that node/npm are available before npm-based installs
  if [ ${#npm_installs[@]} -gt 0 ]; then
    if ! command -v npm >/dev/null 2>&1; then
      echo ""
      echo "  npm is not available — skipping npm-based installs."
      echo "  Install Node.js first, then re-run."
      any_failed=true
      npm_installs=()
    fi
  fi

  # Install npm-based deps
  for inst in "${npm_installs[@]}"; do
    echo "  → $inst"
    if ! $inst 2>/dev/null; then
      echo "  npm install -g failed. You may need to fix npm prefix permissions."
      echo "  See: https://docs.npmjs.com/resolving-eacces-permissions-errors-when-installing-packages-globally"
      any_failed=true
    fi
  done

  if $any_failed; then
    echo ""
    echo "  ⚠ Some installs failed — see above for details."
  fi
}

if [ ${#MISSING[@]} -gt 0 ]; then
  if $IS_MAC; then
    if is_interactive; then
      echo ""
      echo "  The following will be installed:"
      for i in "${!UNIQUE_INSTALLS[@]}"; do
        echo "    - ${UNIQUE_LABELS[$i]}  via  ${UNIQUE_INSTALLS[$i]}"
      done
      echo ""
      if prompt_yn "Install missing dependencies? [Y/n]" "default_yes"; then
        install_tier1
      else
        echo ""
        echo "  Skipped. Install manually:"
        for inst in "${UNIQUE_INSTALLS[@]}"; do
          echo "    $inst"
        done
        echo ""
        exit 1
      fi
    else
      echo ""
      echo "  Non-interactive mode — cannot prompt for install."
      echo "  Set CROSSAGENT_AUTO_INSTALL=1 to auto-install, or install manually:"
      for inst in "${UNIQUE_INSTALLS[@]}"; do
        echo "    $inst"
      done
      echo ""
      exit 1
    fi
  else
    echo ""
    echo "  Auto-install is only supported on macOS."
    echo "  Please install the missing dependencies manually."
    echo ""
    exit 1
  fi
fi

# ── Section 5: Auth reminder ───────────────────────────────────────────────

if command -v claude >/dev/null 2>&1 || command -v codex >/dev/null 2>&1; then
  HAS_AUTH_REMINDER=false
  for dep in "${MISSING[@]+"${MISSING[@]}"}"; do
    case "$dep" in
      claude|codex) HAS_AUTH_REMINDER=true ;;
    esac
  done
  if $HAS_AUTH_REMINDER; then
    echo ""
    echo "  Note: claude and codex require authentication."
    echo "  Run 'claude' and 'codex' once to complete setup if you haven't already."
  fi
fi

# ── Section 6: Tier 2 — Derived artifact builds ────────────────────────────

echo ""

# --- crossagent binary ---
printf "  %-28s" "crossagent binary"
if [ -x "./crossagent" ]; then
  V="$(./crossagent version 2>&1)"
  echo "✓  $V"
else
  echo "…  building"
  if command -v go >/dev/null 2>&1; then
    if go build -o "$ROOT/crossagent" ./cmd/crossagent; then
      echo "  ✓ crossagent binary built"
    else
      echo "  ✗ go build failed"
      PASS=false
    fi
  else
    echo "  ✗ Go is not available — cannot build crossagent"
    PASS=false
  fi
fi

# --- web/node_modules ---
printf "  %-28s" "web/node_modules"
if [ -d "$ROOT/web/node_modules" ]; then
  echo "✓  installed"
else
  echo "…  installing"
  if command -v npm >/dev/null 2>&1; then
    if (cd "$ROOT/web" && npm install); then
      echo "  ✓ web/node_modules installed"
    else
      echo "  ✗ npm install failed in web/"
      PASS=false
    fi
  else
    echo "  ✗ npm is not available — cannot install web dependencies"
    PASS=false
  fi
fi

# --- node-pty native addon ---
printf "  %-28s" "node-pty native addon"
if node -e "require('$ROOT/web/node_modules/node-pty')" 2>/dev/null; then
  echo "✓  loaded"
else
  if [ -d "$ROOT/web/node_modules" ]; then
    echo "…  rebuilding"
    if (cd "$ROOT/web" && npm rebuild node-pty 2>/dev/null); then
      echo "  ✓ node-pty rebuilt"
    else
      echo "✗  broken — run: cd web && npm rebuild node-pty"
      PASS=false
    fi
  else
    echo "✗  web/node_modules not installed"
    PASS=false
  fi
fi

# --- pty spawn-helper ---
printf "  %-28s" "pty spawn-helper"
SH=$(find "$ROOT/web/node_modules/node-pty/prebuilds" -name spawn-helper 2>/dev/null | head -1)
if [ -z "$SH" ]; then
  echo "⊘  not found (ok if node-pty uses fallback)"
elif [ -x "$SH" ]; then
  echo "✓  executable"
else
  if chmod +x "$SH" 2>/dev/null; then
    echo "✓  fixed (was not executable)"
  else
    echo "✗  not executable — run: chmod +x $SH"
    PASS=false
  fi
fi

# ── Section 7: Final verification ──────────────────────────────────────────

echo ""
if $PASS; then
  echo "  ✓ All checks passed"
else
  echo "  ✗ Some checks failed — fix the issues above before running"
fi
echo ""

$PASS
