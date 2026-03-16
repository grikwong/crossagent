BINARY := crossagent
PREFIX ?= /usr/local

.PHONY: build test install uninstall install-ui check start clean

build:
	go build -o $(BINARY) ./cmd/crossagent

test: build
	go test ./...
	bash test/integration_test.sh ./$(BINARY)

install: build
	@mkdir -p "$(PREFIX)/bin" 2>/dev/null || true
	@cp $(BINARY) "$(PREFIX)/bin/$(BINARY)" 2>/dev/null && \
		echo "✓ Installed $(BINARY) to $(PREFIX)/bin/$(BINARY)" || \
	(sudo cp $(BINARY) "$(PREFIX)/bin/$(BINARY)" && \
		echo "✓ Installed $(BINARY) to $(PREFIX)/bin/$(BINARY) (via sudo)")

uninstall:
	@rm -f "$(PREFIX)/bin/$(BINARY)" 2>/dev/null || \
		sudo rm -f "$(PREFIX)/bin/$(BINARY)" 2>/dev/null || true
	@echo "✓ Removed $(BINARY) from $(PREFIX)/bin"

install-ui:
	@cd web && npm install
	@echo "✓ Web UI dependencies installed"

check:
	@CROSSAGENT_ROOT="$(CURDIR)" bash scripts/preflight.sh

start: check
	@echo "  Starting Crossagent Web UI..."
	@echo ""
	@cd web && node server.js

clean:
	@rm -f $(BINARY)
	@echo "✓ Removed $(BINARY)"
