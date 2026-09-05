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
    SF_DIR="$APP_SUPPORT/soundfonts"
    DEFAULT_WADS_DIR="$APP_SUPPORT/games/uzdoom"
else
    UZDOOM_DIR="$HOME/.config/uzdoom"
    DSDA_DIR="$HOME/.local/share/dsda-doom"
    RUNNER_DIR="$HOME/.local/share/DoomRunner"
    DATA_DIR="$HOME/.local/share/doom-configs"
    SF_DIR="$HOME/.local/share/soundfonts"
    DEFAULT_WADS_DIR="$HOME/.local/share/games/uzdoom"
fi
WADS_DIR="${WADS_DIR:-$DEFAULT_WADS_DIR}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
export WADS_DIR BIN_DIR SF_DIR

FULL_SETUP=0
if [ "${1:-}" = "--full" ] || [ "${1:-}" = "--all" ] || [ "${1:-}" = "--setup" ] || [ "${1:-}" = "--turnkey" ]; then
    FULL_SETUP=1
fi

DETECTED_RES=$("$SCRIPT_DIR/scripts/detect-resolution.sh" 2>/dev/null || echo "1920x1080")
DETECTED_RATE=$("$SCRIPT_DIR/scripts/detect-refresh-rate.sh" 2>/dev/null || echo "60")

echo "Setting up Doom configurations for $OS (Detected: ${DETECTED_RES} @ ${DETECTED_RATE}Hz)..."

echo "Creating directories..."
mkdir -p "$UZDOOM_DIR"
mkdir -p "$DSDA_DIR"
mkdir -p "$RUNNER_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$BIN_DIR"
mkdir -p "$SF_DIR"

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
    sed -e "s|__HOME__/.local/share/games/uzdoom|$WADS_DIR|g" \
        -e "s|__HOME__/.config/uzdoom|$UZDOOM_DIR|g" \
        -e "s|__HOME__/.local/share/dsda-doom|$DSDA_DIR|g" \
        -e "s|__HOME__|$HOME|g" \
        -e "s|__RESOLUTION__|$DETECTED_RES|g" \
        -e "s|__REFRESH_RATE__|$DETECTED_RATE|g" \
        -e "s|__SOUNDFONT__|$SF_DIR/GeneralUser-GS.sf2|g" \
        "$src" > "$dest"
}

if [ "$FULL_SETUP" -eq 1 ]; then
    echo ""
    echo "=== Running Setup Step 1/4: Downloading Port Engines ==="
    "$SCRIPT_DIR/scripts/install-engines.sh" all
    echo ""
    echo "=== Running Setup Step 2/4: Deploying Roland SC-55 SoundFont ==="
    "$SCRIPT_DIR/scripts/install-soundfonts.sh"
    echo ""
    echo "=== Running Setup Step 3/4: Deploying Configurations & Presets ==="
fi

copy_with_backup "$SCRIPT_DIR/uzdoom/autoexec.cfg" "$UZDOOM_DIR/autoexec.cfg"
copy_with_backup "$SCRIPT_DIR/DoomRunner/linux/options.json" "$RUNNER_DIR/options.json"
copy_with_backup "$SCRIPT_DIR/dsda-doom/dsda-doom.cfg" "$DSDA_DIR/dsda-doom.cfg"

echo "Installing presets data -> $DATA_DIR/presets.json..."
cp "$SCRIPT_DIR/data/presets.json" "$DATA_DIR/presets.json"

echo "Installing doom-launch CLI -> $BIN_DIR/doom-launch..."
cp "$SCRIPT_DIR/scripts/doom-launch.sh" "$BIN_DIR/doom-launch"
chmod +x "$BIN_DIR/doom-launch"

if command -v go >/dev/null 2>&1; then
    echo "Compiling and installing native doom CLI -> $BIN_DIR/doom..."
    (cd "$SCRIPT_DIR" && go build -o "$BIN_DIR/doom" ./cmd/doom) 2>/dev/null || true
fi

if [ "$FULL_SETUP" -eq 1 ]; then
    echo ""
    echo "=== Running Setup Step 4/4: Extracting IWADs & Fetching Megawads ==="
    "$SCRIPT_DIR/scripts/extract-iwads.sh"
    "$SCRIPT_DIR/scripts/fetch-wads.sh" all
    echo ""
    echo "============================================================"
    echo "  ✓ Doom setup complete!"
    echo "  Engines, configs, soundfonts, and megawads are ready."
    echo "  Run 'doom play' or 'doom-launch' to start playing!"
    echo "============================================================"
else
    echo "Setup complete! Run 'doom setup' (or 'make setup') for all-in-one setup."
    if [ ! -f "$SF_DIR/GeneralUser-GS.sf2" ]; then
        echo "  ℹ Tip: Run 'make install-soundfonts' or 'doom setup' to download the configured Roland SC-55 SoundFont."
    fi
fi
