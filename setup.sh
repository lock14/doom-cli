#!/bin/bash
set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OS="$(uname -s)"
if [ "$OS" = "Darwin" ]; then
    APP_SUPPORT="$HOME/Library/Application Support"
    UZDOOM_DIR="$APP_SUPPORT/uzdoom"
    DSDA_DIR="$APP_SUPPORT/dsda-doom"
    RUNNER_DIR="$APP_SUPPORT/DoomRunner"
    DATA_DIR="$APP_SUPPORT/doom-configs"
else
    UZDOOM_DIR="$HOME/.config/uzdoom"
    DSDA_DIR="$HOME/.local/share/dsda-doom"
    RUNNER_DIR="$HOME/.local/share/DoomRunner"
    DATA_DIR="$HOME/.local/share/doom-configs"
fi
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

TURNKEY=0
if [ "${1:-}" = "--turnkey" ]; then
    TURNKEY=1
fi

DETECTED_RES=$("$SCRIPT_DIR/scripts/detect-resolution.sh" 2>/dev/null || echo "1920x1080")

echo "Setting up Doom configurations for $OS (Detected Resolution: $DETECTED_RES)..."

echo "Creating directories..."
mkdir -p "$UZDOOM_DIR"
mkdir -p "$DSDA_DIR"
mkdir -p "$RUNNER_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$BIN_DIR"

copy_with_backup() {
    local src="$1"
    local dest="$2"

    if [ -f "$dest" ]; then
        local timestamp
        timestamp=$(date +%Y%m%d%H%M%S)
        echo "Backing up existing $(basename "$dest") to ${dest}.bak.${timestamp}"
        cp "$dest" "${dest}.bak.${timestamp}"
    fi

    echo "Installing $(basename "$dest")..."
    sed -e "s|__HOME__|$HOME|g" -e "s|__RESOLUTION__|$DETECTED_RES|g" "$src" > "$dest"
}

copy_with_backup "$SCRIPT_DIR/uzdoom/autoexec.cfg" "$UZDOOM_DIR/autoexec.cfg"
copy_with_backup "$SCRIPT_DIR/DoomRunner/linux/options.json" "$RUNNER_DIR/options.json"
copy_with_backup "$SCRIPT_DIR/dsda-doom/dsda-doom.cfg" "$DSDA_DIR/dsda-doom.cfg"

echo "Installing presets data -> $DATA_DIR/presets.json..."
cp "$SCRIPT_DIR/data/presets.json" "$DATA_DIR/presets.json"

echo "Installing doom-launch CLI -> $BIN_DIR/doom-launch..."
cp "$SCRIPT_DIR/scripts/doom-launch.sh" "$BIN_DIR/doom-launch"
chmod +x "$BIN_DIR/doom-launch"

if [ "$TURNKEY" -eq 1 ]; then
    echo ""
    echo "=== Running Turnkey Additions ==="
    "$SCRIPT_DIR/scripts/install-engines.sh" all
    "$SCRIPT_DIR/scripts/install-soundfonts.sh"
    "$SCRIPT_DIR/scripts/extract-iwads.sh"
    "$SCRIPT_DIR/scripts/fetch-wads.sh" all
    echo ""
    echo "============================================================"
    echo "  ✓ Turnkey Doom setup complete!"
    echo "  Engines, configs, soundfonts, and megawads are ready."
    echo "  Run 'doom-launch' to start playing!"
    echo "============================================================"
else
    echo "Setup complete! Run 'make turnkey' or './setup.sh --turnkey' for all-in-one setup."
fi
