PREFIX ?= $(HOME)
UNAME_S := $(shell uname -s)

ifeq ($(UNAME_S),Darwin)
    # macOS standard Application Support paths
    APP_SUPPORT ?= $(PREFIX)/Library/Application Support
    UZDOOM_DIR  ?= $(APP_SUPPORT)/uzdoom
    DSDA_DIR    ?= $(APP_SUPPORT)/dsda-doom
    RUNNER_DIR  ?= $(APP_SUPPORT)/DoomRunner
    WADS_DIR    ?= $(APP_SUPPORT)/games/uzdoom
    SF_DIR      ?= $(APP_SUPPORT)/soundfonts
    DATA_DIR    ?= $(APP_SUPPORT)/doom-configs
else
    # Linux standard XDG paths
    UZDOOM_DIR  ?= $(PREFIX)/.config/uzdoom
    DSDA_DIR    ?= $(PREFIX)/.local/share/dsda-doom
    RUNNER_DIR  ?= $(PREFIX)/.local/share/DoomRunner
    WADS_DIR    ?= $(PREFIX)/.local/share/games/uzdoom
    SF_DIR      ?= $(PREFIX)/.local/share/soundfonts
    DATA_DIR    ?= $(PREFIX)/.local/share/doom-configs
endif

BIN_DIR ?= $(PREFIX)/.local/bin

.PHONY: all build turnkey bootstrap install install-configs install-uzdoom install-dsda install-doomrunner \
        install-data install-launcher install-soundfonts install-engines install-engine-uzdoom \
        install-engine-dsda install-engine-doomrunner build-presets fetch-wads \
        extract-iwads play sync diff help check test

all: build install

build:
	@if command -v go >/dev/null 2>&1; then \
		echo "Building unified doom CLI binary..."; \
		mkdir -p bin && go build -o bin/doom ./cmd/doom; \
	fi

help:
	@echo "Doom Configs - Available Targets ($(UNAME_S))"
	@echo ""
	@echo "  🚀 Quick Start:"
	@echo "    make turnkey            ⚡ 1-step setup: engines, configs, soundfonts, IWADs & WADs"
	@echo "    make bootstrap          Download engine binaries & deploy configs + launcher"
	@echo "    make install            Deploy all configurations & doom-launch CLI"
	@echo "    make play               Launch interactive terminal preset picker (fzf / menu)"
	@echo ""
	@echo "  📦 Content & Assets:"
	@echo "    make fetch-wads         Download 20+ free community megawads into game directory"
	@echo "    make extract-iwads      Auto-discover & copy official IWADs from Steam / GOG"
	@echo "    make install-soundfonts Download & deploy curated Roland SC-55 SoundFont"
	@echo "    make install-engines    Download latest engine binaries (UZDoom, DSDA-Doom, DoomRunner)"
	@echo ""
	@echo "  🔧 Maintenance & Sync:"
	@echo "    make diff               Compare repo configs against installed system configs"
	@echo "    make sync               Sync in-game configuration tweaks back into repository"
	@echo "    make check              Run validation suite (presets, scripts, invariants, tests)"
	@echo "    make build-presets      Recompile data/presets.json into launcher options.json"

# ⚡ Turnkey Setup: All-in-one installation for players who just want everything ready
turnkey: install-engines install-soundfonts install extract-iwads fetch-wads
	@echo ""
	@echo "============================================================"
	@echo "  ✓ Turnkey Doom setup complete!"
	@echo "  Engines, configs, soundfonts, and megawads are ready."
	@echo "  Run 'make play' or 'doom-launch' to start playing!"
	@echo "============================================================"

# Complete bootstrap: download engines and install configs + launcher
bootstrap: install-engines install

install: install-uzdoom install-dsda install-doomrunner install-data install-launcher
	@echo "All configurations, data files, and launcher successfully installed!"
	@if [ ! -f "$(SF_DIR)/GeneralUser-GS.sf2" ]; then \
		echo "  ℹ Tip: Run 'make install-soundfonts' (or 'make turnkey') to download the configured Roland SC-55 SoundFont."; \
	fi

# Config installation targets
install-uzdoom:
	@mkdir -p "$(UZDOOM_DIR)"
	@if [ -f "$(UZDOOM_DIR)/autoexec.cfg" ]; then \
		echo "Backing up existing autoexec.cfg..."; \
		cp "$(UZDOOM_DIR)/autoexec.cfg" "$(UZDOOM_DIR)/autoexec.cfg.bak.$$(date +%Y%m%d%H%M%S)"; \
	fi
	@RATE=$$("./scripts/detect-refresh-rate.sh" 2>/dev/null || echo "60"); \
	echo "Installing uzdoom/autoexec.cfg -> $(UZDOOM_DIR)/autoexec.cfg (Refresh Rate: $${RATE}Hz)"; \
	sed -e "s|__SOUNDFONT__|$(SF_DIR)/GeneralUser-GS.sf2|g" -e "s|__REFRESH_RATE__|$$RATE|g" uzdoom/autoexec.cfg > "$(UZDOOM_DIR)/autoexec.cfg"

install-dsda:
	@mkdir -p "$(DSDA_DIR)"
	@if [ -f "$(DSDA_DIR)/dsda-doom.cfg" ]; then \
		echo "Backing up existing dsda-doom.cfg..."; \
		cp "$(DSDA_DIR)/dsda-doom.cfg" "$(DSDA_DIR)/dsda-doom.cfg.bak.$$(date +%Y%m%d%H%M%S)"; \
	fi
	@RES=$$("./scripts/detect-resolution.sh" 2>/dev/null || echo "1920x1080"); \
	echo "Installing dsda-doom/dsda-doom.cfg -> $(DSDA_DIR)/dsda-doom.cfg (Resolution: $$RES)"; \
	sed -e "s|__RESOLUTION__|$$RES|g" -e "s|__SOUNDFONT__|$(SF_DIR)/GeneralUser-GS.sf2|g" dsda-doom/dsda-doom.cfg > "$(DSDA_DIR)/dsda-doom.cfg"

install-doomrunner:
	@mkdir -p "$(RUNNER_DIR)"
	@if [ -f "$(RUNNER_DIR)/options.json" ]; then \
		echo "Backing up existing options.json..."; \
		cp "$(RUNNER_DIR)/options.json" "$(RUNNER_DIR)/options.json.bak.$$(date +%Y%m%d%H%M%S)"; \
	fi
	@echo "Installing DoomRunner/linux/options.json -> $(RUNNER_DIR)/options.json"
	@sed -e 's|__HOME__/.local/share/games/uzdoom|$(WADS_DIR)|g' \
	     -e 's|__HOME__/.config/uzdoom|$(UZDOOM_DIR)|g' \
	     -e 's|__HOME__/.local/share/dsda-doom|$(DSDA_DIR)|g' \
	     -e 's|__HOME__|$(PREFIX)|g' DoomRunner/linux/options.json > "$(RUNNER_DIR)/options.json"

install-data:
	@mkdir -p "$(DATA_DIR)"
	@echo "Installing data/presets.json -> $(DATA_DIR)/presets.json"
	@cp data/presets.json "$(DATA_DIR)/presets.json"

install-launcher:
	@mkdir -p "$(BIN_DIR)"
	@if command -v go >/dev/null 2>&1; then \
		echo "Compiling and installing native doom CLI -> $(BIN_DIR)/doom"; \
		go build -o "$(BIN_DIR)/doom" ./cmd/doom; \
	fi
	@echo "Installing scripts/doom-launch.sh -> $(BIN_DIR)/doom-launch"
	@cp scripts/doom-launch.sh "$(BIN_DIR)/doom-launch"
	@chmod +x "$(BIN_DIR)/doom-launch"

# Interactive CLI & Automation targets
play:
	@BIN_DIR="$(BIN_DIR)" WADS_DIR="$(WADS_DIR)" ./scripts/doom-launch.sh

fetch-wads:
	@WADS_DIR="$(WADS_DIR)" ./scripts/fetch-wads.sh all

extract-iwads:
	@WADS_DIR="$(WADS_DIR)" ./scripts/extract-iwads.sh

install-soundfonts:
	@SF_DIR="$(SF_DIR)" ./scripts/install-soundfonts.sh

build-presets:
	@python3 scripts/build-presets.py --build --update-readme

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
		sed -E -e 's/fluid_patchset[[:space:]]+".*"/fluid_patchset "__SOUNDFONT__"/g' -e 's/vid_maxfps[[:space:]]+[0-9]+/vid_maxfps __REFRESH_RATE__/g' "$(UZDOOM_DIR)/autoexec.cfg" > uzdoom/autoexec.cfg; \
	fi
	@if [ -f "$(DSDA_DIR)/dsda-doom.cfg" ]; then \
		echo "Syncing $(DSDA_DIR)/dsda-doom.cfg -> dsda-doom/dsda-doom.cfg"; \
		sed -E -e 's/screen_resolution[[:space:]]+"[0-9]+x[0-9]+"/screen_resolution               "__RESOLUTION__"/g' -e 's/snd_soundfont[[:space:]]+".*"/snd_soundfont                   "__SOUNDFONT__"/g' "$(DSDA_DIR)/dsda-doom.cfg" > dsda-doom/dsda-doom.cfg; \
	fi
	@if [ -f "$(RUNNER_DIR)/options.json" ]; then \
		echo "Syncing $(RUNNER_DIR)/options.json -> DoomRunner/linux/options.json"; \
		sed -e 's|$(WADS_DIR)|__HOME__/.local/share/games/uzdoom|g' \
		    -e 's|$(UZDOOM_DIR)|__HOME__/.config/uzdoom|g' \
		    -e 's|$(DSDA_DIR)|__HOME__/.local/share/dsda-doom|g' \
		    -e 's|$(PREFIX)|__HOME__|g' "$(RUNNER_DIR)/options.json" > DoomRunner/linux/options.json; \
	fi

# Show differences between repo configs and installed system configs
diff:
	@echo "=== UZDoom Diff ==="
	@-RATE=$$("./scripts/detect-refresh-rate.sh" 2>/dev/null || echo "60"); \
	sed -e "s|__SOUNDFONT__|$(SF_DIR)/GeneralUser-GS.sf2|g" -e "s|__REFRESH_RATE__|$$RATE|g" uzdoom/autoexec.cfg | diff -u - "$(UZDOOM_DIR)/autoexec.cfg" || true
	@echo "=== DSDA-Doom Diff ==="
	@-RES=$$("./scripts/detect-resolution.sh" 2>/dev/null || echo "1920x1080"); \
	sed -e "s|__RESOLUTION__|$$RES|g" -e "s|__SOUNDFONT__|$(SF_DIR)/GeneralUser-GS.sf2|g" dsda-doom/dsda-doom.cfg | diff -u - "$(DSDA_DIR)/dsda-doom.cfg" || true
	@echo "=== DoomRunner Diff ==="
	@-sed -e 's|__HOME__/.local/share/games/uzdoom|$(WADS_DIR)|g' \
	      -e 's|__HOME__/.config/uzdoom|$(UZDOOM_DIR)|g' \
	      -e 's|__HOME__/.local/share/dsda-doom|$(DSDA_DIR)|g' \
	      -e 's|__HOME__|$(PREFIX)|g' DoomRunner/linux/options.json | diff -u - "$(RUNNER_DIR)/options.json" || true

# Test & Validation targets
check: test

test:
	@if command -v go >/dev/null 2>&1; then \
		echo "=== Running Go Test Suite ==="; \
		go test -v ./... || exit 1; \
	fi
	@echo "=== Validating Shell Scripts ==="
	@bash -n setup.sh
	@bash -n scripts/install-engines.sh
	@bash -n scripts/detect-resolution.sh
	@bash -n scripts/detect-refresh-rate.sh
	@bash -n scripts/fetch-wads.sh
	@bash -n scripts/extract-iwads.sh
	@bash -n scripts/install-soundfonts.sh
	@bash -n scripts/doom-launch.sh
	@bash -n scripts/test-turnkey.sh
	@bash -n scripts/test-doom-launch.sh
	@if command -v shellcheck >/dev/null 2>&1; then \
		echo "Running ShellCheck..."; \
		shellcheck setup.sh scripts/*.sh; \
	fi
	@echo "=== Validating Declarative Presets & Parity ==="
	@python3 scripts/build-presets.py --check
	@echo "=== Validating JSON Files ==="
	@jq . data/presets.json > /dev/null
	@jq . DoomRunner/linux/options.json > /dev/null
	@jq . DoomRunner/windows/options.json > /dev/null
	@echo "=== Checking Path Invariants ==="
	@if grep -E '/home/[a-zA-Z0-9_-]+' DoomRunner/linux/options.json data/presets.json; then \
		echo "Error: Hardcoded personal path found in options.json or presets.json"; \
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
		test -f "$$TEST_DIR/Library/Application Support/doom-configs/presets.json" && \
		test -f "$$TEST_DIR/.local/bin/doom-launch" && \
		! grep -q '__RESOLUTION__' "$$TEST_DIR/Library/Application Support/dsda-doom/dsda-doom.cfg" && \
		! grep -q '__SOUNDFONT__' "$$TEST_DIR/Library/Application Support/dsda-doom/dsda-doom.cfg" && \
		! grep -q '__SOUNDFONT__' "$$TEST_DIR/Library/Application Support/uzdoom/autoexec.cfg" && \
		! grep -q '__REFRESH_RATE__' "$$TEST_DIR/Library/Application Support/uzdoom/autoexec.cfg" && \
		! grep -q '__HOME__' "$$TEST_DIR/Library/Application Support/DoomRunner/options.json" && \
		! grep -q '\.local/share/games/uzdoom' "$$TEST_DIR/Library/Application Support/DoomRunner/options.json"; \
	else \
		test -f "$$TEST_DIR/.config/uzdoom/autoexec.cfg" && \
		test -f "$$TEST_DIR/.local/share/dsda-doom/dsda-doom.cfg" && \
		test -f "$$TEST_DIR/.local/share/DoomRunner/options.json" && \
		test -f "$$TEST_DIR/.local/share/doom-configs/presets.json" && \
		test -f "$$TEST_DIR/.local/bin/doom-launch" && \
		! grep -q '__RESOLUTION__' "$$TEST_DIR/.local/share/dsda-doom/dsda-doom.cfg" && \
		! grep -q '__SOUNDFONT__' "$$TEST_DIR/.local/share/dsda-doom/dsda-doom.cfg" && \
		! grep -q '__SOUNDFONT__' "$$TEST_DIR/.config/uzdoom/autoexec.cfg" && \
		! grep -q '__REFRESH_RATE__' "$$TEST_DIR/.config/uzdoom/autoexec.cfg" && \
		! grep -q '__HOME__' "$$TEST_DIR/.local/share/DoomRunner/options.json"; \
	fi && \
	$(MAKE) PREFIX="$$TEST_DIR" install > /dev/null && \
	if [ "$(UNAME_S)" = "Darwin" ]; then \
		ls "$$TEST_DIR/Library/Application Support/uzdoom/" | grep -q 'autoexec.cfg.bak.'; \
	else \
		ls "$$TEST_DIR/.config/uzdoom/" | grep -q 'autoexec.cfg.bak.'; \
	fi && \
	echo "=== Running doom-launch CLI Test Suite ===" && \
	./scripts/test-doom-launch.sh && \
	echo "=== Running End-to-End Turnkey & System Test Suite ===" && \
	./scripts/test-turnkey.sh && \
	echo "All validation checks passed successfully for $(UNAME_S)!"
