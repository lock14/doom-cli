PREFIX ?= $(HOME)
UNAME_S := $(shell uname -s)

ifeq ($(UNAME_S),Darwin)
    # macOS standard Application Support paths
    APP_SUPPORT ?= $(PREFIX)/Library/Application Support
    UZDOOM_DIR  ?= $(APP_SUPPORT)/uzdoom
    DSDA_DIR    ?= $(APP_SUPPORT)/dsda-doom
    RUNNER_DIR  ?= $(APP_SUPPORT)/DoomRunner
else
    # Linux standard XDG paths
    UZDOOM_DIR  ?= $(PREFIX)/.config/uzdoom
    DSDA_DIR    ?= $(PREFIX)/.local/share/dsda-doom
    RUNNER_DIR  ?= $(PREFIX)/.local/share/DoomRunner
endif

BIN_DIR ?= $(PREFIX)/.local/bin

.PHONY: all bootstrap install install-configs install-uzdoom install-dsda install-doomrunner \
        install-engines install-engine-uzdoom install-engine-dsda install-engine-doomrunner \
        sync diff help check test

all: install

help:
	@echo "Doom Configs - Available Makefile targets ($(UNAME_S)):"
	@echo "  make bootstrap         - Complete setup: download all engines + install configs"
	@echo "  make install           - Install all configurations with automatic backups"
	@echo "  make install-uzdoom    - Install only UZDoom autoexec.cfg"
	@echo "  make install-dsda      - Install only DSDA-Doom dsda-doom.cfg"
	@echo "  make install-doomrunner- Install only DoomRunner options.json"
	@echo "  make install-engines   - Download latest binaries (UZDoom, DSDA-Doom, DoomRunner)"
	@echo "  make sync              - Copy active system configs back into repo"
	@echo "  make diff              - Compare repo configs against installed system configs"
	@echo "  make check             - Run validation suite (syntax, invariants, dry install)"

# Complete bootstrap: download engines and install configs
bootstrap: install-engines install

install: install-uzdoom install-dsda install-doomrunner
	@echo "All configurations successfully installed!"

# Config installation targets
install-uzdoom:
	@mkdir -p "$(UZDOOM_DIR)"
	@if [ -f "$(UZDOOM_DIR)/autoexec.cfg" ]; then \
		echo "Backing up existing autoexec.cfg..."; \
		cp "$(UZDOOM_DIR)/autoexec.cfg" "$(UZDOOM_DIR)/autoexec.cfg.bak.$$(date +%Y%m%d%H%M%S)"; \
	fi
	@echo "Installing uzdoom/autoexec.cfg -> $(UZDOOM_DIR)/autoexec.cfg"
	@cp uzdoom/autoexec.cfg "$(UZDOOM_DIR)/autoexec.cfg"

install-dsda:
	@mkdir -p "$(DSDA_DIR)"
	@if [ -f "$(DSDA_DIR)/dsda-doom.cfg" ]; then \
		echo "Backing up existing dsda-doom.cfg..."; \
		cp "$(DSDA_DIR)/dsda-doom.cfg" "$(DSDA_DIR)/dsda-doom.cfg.bak.$$(date +%Y%m%d%H%M%S)"; \
	fi
	@echo "Installing dsda-doom/dsda-doom.cfg -> $(DSDA_DIR)/dsda-doom.cfg"
	@cp dsda-doom/dsda-doom.cfg "$(DSDA_DIR)/dsda-doom.cfg"

install-doomrunner:
	@mkdir -p "$(RUNNER_DIR)"
	@if [ -f "$(RUNNER_DIR)/options.json" ]; then \
		echo "Backing up existing options.json..."; \
		cp "$(RUNNER_DIR)/options.json" "$(RUNNER_DIR)/options.json.bak.$$(date +%Y%m%d%H%M%S)"; \
	fi
	@echo "Installing DoomRunner/linux/options.json -> $(RUNNER_DIR)/options.json"
	@sed 's|__HOME__|$(PREFIX)|g' DoomRunner/linux/options.json > "$(RUNNER_DIR)/options.json"

# Engine & Launcher download targets
install-engines:
	@BIN_DIR="$(BIN_DIR)" ./scripts/install-engines.sh all

install-engine-uzdoom:
	@BIN_DIR="$(BIN_DIR)" ./scripts/install-engines.sh uzdoom

install-engine-dsda:
	@BIN_DIR="$(BIN_DIR)" ./scripts/install-engines.sh dsda

install-engine-doomrunner:
	@BIN_DIR="$(BIN_DIR)" ./scripts/install-engines.sh doomrunner

# Copy live changes made in-game back into this repository
sync:
	@if [ -f "$(UZDOOM_DIR)/autoexec.cfg" ]; then \
		echo "Syncing $(UZDOOM_DIR)/autoexec.cfg -> uzdoom/autoexec.cfg"; \
		cp "$(UZDOOM_DIR)/autoexec.cfg" uzdoom/autoexec.cfg; \
	fi
	@if [ -f "$(DSDA_DIR)/dsda-doom.cfg" ]; then \
		echo "Syncing $(DSDA_DIR)/dsda-doom.cfg -> dsda-doom/dsda-doom.cfg"; \
		cp "$(DSDA_DIR)/dsda-doom.cfg" dsda-doom/dsda-doom.cfg; \
	fi
	@if [ -f "$(RUNNER_DIR)/options.json" ]; then \
		echo "Syncing $(RUNNER_DIR)/options.json -> DoomRunner/linux/options.json"; \
		sed 's|$(PREFIX)|__HOME__|g' "$(RUNNER_DIR)/options.json" > DoomRunner/linux/options.json; \
	fi

# Show differences between repo configs and installed system configs
diff:
	@echo "=== UZDoom Diff ==="
	@-diff -u uzdoom/autoexec.cfg "$(UZDOOM_DIR)/autoexec.cfg" || true
	@echo "=== DSDA-Doom Diff ==="
	@-diff -u dsda-doom/dsda-doom.cfg "$(DSDA_DIR)/dsda-doom.cfg" || true
	@echo "=== DoomRunner Diff ==="
	@-sed 's|__HOME__|$(PREFIX)|g' DoomRunner/linux/options.json | diff -u - "$(RUNNER_DIR)/options.json" || true

# Test & Validation targets
check: test

test:
	@echo "=== Validating Shell Scripts ==="
	@bash -n setup.sh
	@bash -n scripts/install-engines.sh
	@echo "=== Validating JSON Files ==="
	@jq . DoomRunner/linux/options.json > /dev/null
	@jq . DoomRunner/windows/options.json > /dev/null
	@echo "=== Checking Path Invariants ==="
	@if grep -E '/home/[a-zA-Z0-9_-]+' DoomRunner/linux/options.json; then \
		echo "Error: Hardcoded personal path found in DoomRunner/linux/options.json"; \
		exit 1; \
	fi
	@echo "=== Testing Isolated Installation for $(UNAME_S) ==="
	@TEST_DIR=$$(mktemp -d); \
	trap 'rm -rf "$$TEST_DIR"' EXIT; \
	$(MAKE) PREFIX="$$TEST_DIR" install > /dev/null && \
	if [ "$(UNAME_S)" = "Darwin" ]; then \
		test -f "$$TEST_DIR/Library/Application Support/uzdoom/autoexec.cfg" && \
		test -f "$$TEST_DIR/Library/Application Support/dsda-doom/dsda-doom.cfg" && \
		test -f "$$TEST_DIR/Library/Application Support/DoomRunner/options.json" && \
		! grep -q '__HOME__' "$$TEST_DIR/Library/Application Support/DoomRunner/options.json"; \
	else \
		test -f "$$TEST_DIR/.config/uzdoom/autoexec.cfg" && \
		test -f "$$TEST_DIR/.local/share/dsda-doom/dsda-doom.cfg" && \
		test -f "$$TEST_DIR/.local/share/DoomRunner/options.json" && \
		! grep -q '__HOME__' "$$TEST_DIR/.local/share/DoomRunner/options.json"; \
	fi && \
	$(MAKE) PREFIX="$$TEST_DIR" install > /dev/null && \
	if [ "$(UNAME_S)" = "Darwin" ]; then \
		ls "$$TEST_DIR/Library/Application Support/uzdoom/" | grep -q 'autoexec.cfg.bak.'; \
	else \
		ls "$$TEST_DIR/.config/uzdoom/" | grep -q 'autoexec.cfg.bak.'; \
	fi && \
	echo "All validation checks passed successfully for $(UNAME_S)!"
