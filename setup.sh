#!/bin/bash
set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Setting up Doom configurations..."

echo "Creating directories..."
mkdir -p "$HOME/.config/uzdoom"
mkdir -p "$HOME/.local/share/DoomRunner"
mkdir -p "$HOME/.local/share/dsda-doom"

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

copy_with_backup "$SCRIPT_DIR/uzdoom/autoexec.cfg" "$HOME/.config/uzdoom/autoexec.cfg"
copy_with_backup "$SCRIPT_DIR/DoomRunner/linux/options.json" "$HOME/.local/share/DoomRunner/options.json"
copy_with_backup "$SCRIPT_DIR/dsda-doom/dsda-doom.cfg" "$HOME/.local/share/dsda-doom/dsda-doom.cfg"

echo "Setup complete!"
