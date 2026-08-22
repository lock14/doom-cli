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
else
    UZDOOM_DIR="$HOME/.config/uzdoom"
    DSDA_DIR="$HOME/.local/share/dsda-doom"
    RUNNER_DIR="$HOME/.local/share/DoomRunner"
fi

echo "Setting up Doom configurations for $OS..."

echo "Creating directories..."
mkdir -p "$UZDOOM_DIR"
mkdir -p "$DSDA_DIR"
mkdir -p "$RUNNER_DIR"

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
    sed "s|__HOME__|$HOME|g" "$src" > "$dest"
}

copy_with_backup "$SCRIPT_DIR/uzdoom/autoexec.cfg" "$UZDOOM_DIR/autoexec.cfg"
copy_with_backup "$SCRIPT_DIR/DoomRunner/linux/options.json" "$RUNNER_DIR/options.json"
copy_with_backup "$SCRIPT_DIR/dsda-doom/dsda-doom.cfg" "$DSDA_DIR/dsda-doom.cfg"

echo "Setup complete!"
