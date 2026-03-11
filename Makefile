PREFIX ?= /usr/local

.PHONY: install uninstall install-ui start check

install:
	@chmod +x crossagent
	@mkdir -p "$(PREFIX)/bin" 2>/dev/null || true
	@if ln -sf "$(CURDIR)/crossagent" "$(PREFIX)/bin/crossagent" 2>/dev/null; then \
		echo "✓ Installed crossagent to $(PREFIX)/bin/crossagent"; \
	elif sudo ln -sf "$(CURDIR)/crossagent" "$(PREFIX)/bin/crossagent"; then \
		echo "✓ Installed crossagent to $(PREFIX)/bin/crossagent (via sudo)"; \
	else \
		echo "✗ Permission denied. Try: make install PREFIX=\$$HOME/.local"; \
		echo "  (ensure ~/.local/bin is in your PATH)"; \
		exit 1; \
	fi

install-ui:
	@cd web && npm install
	@echo "✓ Web UI dependencies installed"

uninstall:
	@rm -f "$(PREFIX)/bin/crossagent" 2>/dev/null || \
		sudo rm -f "$(PREFIX)/bin/crossagent" 2>/dev/null || true
	@echo "✓ Removed crossagent from $(PREFIX)/bin"

check:
	@echo ""
	@echo "  Crossagent — Preflight Checks"
	@echo "  ─────────────────────────────────────────"
	@echo ""
	@PASS=true; \
	printf "  %-28s" "bash (3.2+)"; \
	if bash --version >/dev/null 2>&1; then \
		V=$$(bash -c 'echo $${BASH_VERSION}'); \
		echo "✓  $$V"; \
	else \
		echo "✗  not found"; PASS=false; \
	fi; \
	printf "  %-28s" "node (18+)"; \
	if command -v node >/dev/null 2>&1; then \
		V=$$(node --version); \
		MAJOR=$$(echo "$$V" | sed 's/^v//' | cut -d. -f1); \
		if [ "$$MAJOR" -ge 18 ] 2>/dev/null; then \
			echo "✓  $$V"; \
		else \
			echo "✗  $$V (need v18+)"; PASS=false; \
		fi; \
	else \
		echo "✗  not found"; PASS=false; \
	fi; \
	printf "  %-28s" "npm"; \
	if command -v npm >/dev/null 2>&1; then \
		echo "✓  $$(npm --version)"; \
	else \
		echo "✗  not found"; PASS=false; \
	fi; \
	printf "  %-28s" "claude (Claude Code CLI)"; \
	if command -v claude >/dev/null 2>&1; then \
		echo "✓  found"; \
	else \
		echo "✗  not found — install: https://docs.anthropic.com/en/docs/claude-code"; PASS=false; \
	fi; \
	printf "  %-28s" "codex (Codex CLI)"; \
	if command -v codex >/dev/null 2>&1; then \
		echo "✓  found"; \
	else \
		echo "✗  not found — install: https://github.com/openai/codex"; PASS=false; \
	fi; \
	printf "  %-28s" "crossagent CLI"; \
	if [ -x "$(CURDIR)/crossagent" ]; then \
		echo "✓  $(CURDIR)/crossagent"; \
	else \
		echo "✗  not executable"; PASS=false; \
	fi; \
	printf "  %-28s" "crossagent --version"; \
	if "$(CURDIR)/crossagent" version >/dev/null 2>&1; then \
		echo "✓  $$("$(CURDIR)/crossagent" version 2>&1)"; \
	else \
		echo "✗  failed to run"; PASS=false; \
	fi; \
	printf "  %-28s" "web/node_modules"; \
	if [ -d "$(CURDIR)/web/node_modules" ]; then \
		echo "✓  installed"; \
	else \
		echo "✗  missing — run: make install-ui"; PASS=false; \
	fi; \
	printf "  %-28s" "node-pty native addon"; \
	if node -e "require('$(CURDIR)/web/node_modules/node-pty')" 2>/dev/null; then \
		echo "✓  loaded"; \
	else \
		echo "✗  broken — run: cd web && npm rebuild node-pty"; PASS=false; \
	fi; \
	printf "  %-28s" "pty spawn-helper"; \
	SH=$$(find "$(CURDIR)/web/node_modules/node-pty/prebuilds" -name spawn-helper 2>/dev/null | head -1); \
	if [ -z "$$SH" ]; then \
		echo "⊘  not found (ok if node-pty uses fallback)"; \
	elif [ -x "$$SH" ]; then \
		echo "✓  executable"; \
	else \
		chmod +x "$$SH" 2>/dev/null && echo "✓  fixed (was not executable)" || \
		(echo "✗  not executable — run: chmod +x $$SH"; PASS=false); \
	fi; \
	echo ""; \
	if $$PASS; then \
		echo "  ✓ All checks passed"; \
	else \
		echo "  ✗ Some checks failed — fix the issues above before running"; \
	fi; \
	echo ""

start: check
	@echo "  Starting Crossagent Web UI..."
	@echo ""
	@cd web && node server.js
