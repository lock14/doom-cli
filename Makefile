BIN ?= bin/doom
PREFIX ?= $(HOME)/.local/bin

.PHONY: all build install test check clean help

all: build

help:
	@echo "doom-configs developer targets:"
	@echo "  make build    - Compile bin/doom static binary"
	@echo "  make install  - Install binary to $(PREFIX)/doom"
	@echo "  make test     - Run full Go unit test suite"
	@echo "  make check    - Run unit tests and path invariant audit"
	@echo "  make clean    - Remove build artifacts"
	@echo ""
	@echo "For Doom management and playing, use the CLI:"
	@echo "  doom setup    - Full automated setup (engines, configs, soundfonts, IWADs, WADs)"
	@echo "  doom play     - Interactive terminal launcher with fuzzy search and preview"
	@echo "  doom --help   - View all available commands and flags"

build:
	@mkdir -p bin
	go build -o $(BIN) ./cmd/doom

install: build
	@mkdir -p $(PREFIX)
	cp $(BIN) $(PREFIX)/doom
	@echo "✓ Successfully installed doom to $(PREFIX)/doom"

test:
	go test -v ./...

check: test
	@echo "Auditing path invariants..."
	@! grep -rE '/home/[a-zA-Z0-9_-]+' internal/ data/ cmd/
	@echo "✓ All checks and tests passed!"

clean:
	rm -rf bin/
