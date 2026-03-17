BINARY := crossagent
PREFIX ?= /usr/local

.PHONY: build test install uninstall check start clean

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

check:
	@CROSSAGENT_ROOT="$(CURDIR)" bash scripts/preflight.sh

start: build
	@./$(BINARY) serve

clean:
	@rm -f $(BINARY)
	@echo "✓ Removed $(BINARY)"
