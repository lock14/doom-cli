PREFIX ?= $(HOME)
CONFIG_DIR ?= $(PREFIX)/.config
SHARE_DIR ?= $(PREFIX)/.local/share
BIN_DIR ?= $(PREFIX)/.local/bin

.PHONY: all bootstrap install install-configs install-uzdoom install-dsda install-doomrunner \
        install-engines install-engine-uzdoom install-engine-dsda install-engine-doomrunner \
        sync diff help

all: install

help:
	@echo "Doom Configs - Available Makefile targets:"
	@echo "  make bootstrap         - Complete setup: download all engines + install configs"
	@echo "  make install           - Install all configurations with automatic backups"
	@echo "  make install-uzdoom    - Install only UZDoom autoexec.cfg"
	@echo "  make install-dsda      - Install only DSDA-Doom dsda-doom.cfg"
	@echo "  make install-doomrunner- Install only DoomRunner options.json"
	@echo "  make install-engines   - Download latest AppImages (UZDoom, DSDA-Doom, DoomRunner)"
	@echo "  make sync              - Copy active system configs back into repo"
	@echo "  make diff              - Compare repo configs against installed system configs"

# Complete bootstrap: download engines and install configs
bootstrap: install-engines install

install: install-uzdoom install-dsda install-doomrunner
	@echo "All configurations successfully installed!"

# Config installation targets
install-uzdoom:
	@mkdir -p $(CONFIG_DIR)/uzdoom
	@if [ -f $(CONFIG_DIR)/uzdoom/autoexec.cfg ]; then \
		echo "Backing up existing autoexec.cfg..."; \
		cp $(CONFIG_DIR)/uzdoom/autoexec.cfg $(CONFIG_DIR)/uzdoom/autoexec.cfg.bak.$$(date +%Y%m%d%H%M%S); \
	fi
	@echo "Installing uzdoom/autoexec.cfg -> $(CONFIG_DIR)/uzdoom/autoexec.cfg"
	@cp uzdoom/autoexec.cfg $(CONFIG_DIR)/uzdoom/autoexec.cfg

install-dsda:
	@mkdir -p $(SHARE_DIR)/dsda-doom
	@if [ -f $(SHARE_DIR)/dsda-doom/dsda-doom.cfg ]; then \
		echo "Backing up existing dsda-doom.cfg..."; \
		cp $(SHARE_DIR)/dsda-doom/dsda-doom.cfg $(SHARE_DIR)/dsda-doom/dsda-doom.cfg.bak.$$(date +%Y%m%d%H%M%S); \
	fi
	@echo "Installing dsda-doom/dsda-doom.cfg -> $(SHARE_DIR)/dsda-doom/dsda-doom.cfg"
	@cp dsda-doom/dsda-doom.cfg $(SHARE_DIR)/dsda-doom/dsda-doom.cfg

install-doomrunner:
	@mkdir -p $(SHARE_DIR)/DoomRunner
	@if [ -f $(SHARE_DIR)/DoomRunner/options.json ]; then \
		echo "Backing up existing options.json..."; \
		cp $(SHARE_DIR)/DoomRunner/options.json $(SHARE_DIR)/DoomRunner/options.json.bak.$$(date +%Y%m%d%H%M%S); \
	fi
	@echo "Installing DoomRunner/linux/options.json -> $(SHARE_DIR)/DoomRunner/options.json"
	@sed 's|__HOME__|$(PREFIX)|g' DoomRunner/linux/options.json > $(SHARE_DIR)/DoomRunner/options.json

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
	@if [ -f $(CONFIG_DIR)/uzdoom/autoexec.cfg ]; then \
		echo "Syncing $(CONFIG_DIR)/uzdoom/autoexec.cfg -> uzdoom/autoexec.cfg"; \
		cp $(CONFIG_DIR)/uzdoom/autoexec.cfg uzdoom/autoexec.cfg; \
	fi
	@if [ -f $(SHARE_DIR)/dsda-doom/dsda-doom.cfg ]; then \
		echo "Syncing $(SHARE_DIR)/dsda-doom/dsda-doom.cfg -> dsda-doom/dsda-doom.cfg"; \
		cp $(SHARE_DIR)/dsda-doom/dsda-doom.cfg dsda-doom/dsda-doom.cfg; \
	fi
	@if [ -f $(SHARE_DIR)/DoomRunner/options.json ]; then \
		echo "Syncing $(SHARE_DIR)/DoomRunner/options.json -> DoomRunner/linux/options.json"; \
		sed 's|$(PREFIX)|__HOME__|g' $(SHARE_DIR)/DoomRunner/options.json > DoomRunner/linux/options.json; \
	fi

# Show differences between repo configs and installed system configs
diff:
	@echo "=== UZDoom Diff ==="
	@-diff -u uzdoom/autoexec.cfg $(CONFIG_DIR)/uzdoom/autoexec.cfg || true
	@echo "=== DSDA-Doom Diff ==="
	@-diff -u dsda-doom/dsda-doom.cfg $(SHARE_DIR)/dsda-doom/dsda-doom.cfg || true
	@echo "=== DoomRunner Diff ==="
	@-sed 's|__HOME__|$(PREFIX)|g' DoomRunner/linux/options.json | diff -u - $(SHARE_DIR)/DoomRunner/options.json || true
