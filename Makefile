BIN ?= bin/doom
PREFIX ?= $(HOME)/.local/bin

.PHONY: all build install test check lint format tidy clean help format-check tidy-check

all: build

help:
	@echo "doom-cli developer targets:"
	@echo "  make build    - Compile bin/doom static binary"
	@echo "  make install  - Install binary to $(PREFIX)/doom"
	@echo "  make test     - Run full Go test suite with -race and -shuffle=on"
	@echo "  make lint     - Run go vet and revive static analysis"
	@echo "  make format   - Run gofmt -s to format all Go source files"
	@echo "  make tidy     - Tidy and verify go.mod / go.sum"
	@echo "  make check    - Run formatting, module hygiene, lint, tests, and path audit"
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

format:
	@echo "Formatting Go source files..."
	@gofmt -s -w .

tidy:
	@echo "Verifying module hygiene..."
	@go mod verify
	@go mod download

lint:
	@echo "Running go vet..."
	@go vet ./...
	@if command -v revive >/dev/null 2>&1; then \
		echo "Running revive..."; \
		revive -config revive.toml -formatter friendly ./...; \
	fi

test:
	go test -v -race -shuffle=on ./...

check: format-check tidy-check lint test
	@echo "Auditing path invariants..."
	@! grep -rE '/home/[a-zA-Z0-9_-]+' internal/ data/ cmd/
	@echo "✓ All checks, linters, and tests passed!"

format-check:
	@echo "Checking formatting..."
	@if [ -n "$$(gofmt -s -l .)" ]; then \
		echo "The following files are not formatted properly:"; \
		gofmt -s -l .; \
		echo "Run 'make format' or 'gofmt -s -w .' locally."; \
		exit 1; \
	fi

tidy-check:
	@echo "Checking go mod tidy..."
	@go mod verify
	@go mod tidy
	@git diff --exit-code go.mod go.sum

clean:
	rm -rf bin/
