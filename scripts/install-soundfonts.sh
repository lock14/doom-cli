#!/usr/bin/env bash
# install-soundfonts.sh - High Quality MIDI SoundFont Downloader
# Downloads and deploys curated General MIDI / Roland SC-55 compatible SoundFonts for FluidSynth.

set -e

OS="$(uname -s)"
if [ "$OS" = "Darwin" ]; then
    DEFAULT_SF_DIR="$HOME/Library/Application Support/soundfonts"
else
    DEFAULT_SF_DIR="$HOME/.local/share/soundfonts"
fi

SF_DIR="${SF_DIR:-$DEFAULT_SF_DIR}"
FORCE=0

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --dir <path>     Custom destination directory (default: $SF_DIR)"
    echo "  --force          Re-download and overwrite existing SoundFont"
    echo "  --help           Show this help message"
    exit 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        --dir)
            SF_DIR="$2"
            shift 2
            ;;
        --force)
            FORCE=1
            shift
            ;;
        --help|-h)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

mkdir -p "$SF_DIR"

echo "=== Doom SoundFont Installer ==="
echo "Target directory: $SF_DIR"
echo ""

SF_FILE="$SF_DIR/GeneralUser-GS.sf2"
SF_URL="https://raw.githubusercontent.com/mrbumpy409/GeneralUser-GS/main/GeneralUser-GS.sf2"

if [ -f "$SF_FILE" ] && [ "$FORCE" -eq 0 ]; then
    echo "✓ SoundFont already installed: $SF_FILE"
    echo "  (Use --force to re-download)"
    exit 0
fi

echo "Downloading GeneralUser-GS SoundFont (Roland SC-55 Balanced GM)..."
TMP_DL=$(mktemp "${TMPDIR:-/tmp}/soundfont.XXXXXX")
trap 'rm -f "$TMP_DL"' EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
    curl -L --progress-bar -o "$TMP_DL" "$SF_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -q --show-progress -O "$TMP_DL" "$SF_URL"
else
    echo "Error: Neither curl nor wget found."
    exit 1
fi

mv "$TMP_DL" "$SF_FILE"
trap - EXIT INT TERM
chmod 644 "$SF_FILE"

echo ""
echo "✓ Successfully installed GeneralUser-GS.sf2 to:"
echo "  $SF_FILE"
echo ""
echo "To use with DSDA-Doom or UZDoom, ensure your config or launch command points to this SoundFont:"
echo "  dsda-doom.cfg : snd_soundfont \"$SF_FILE\""
echo "  autoexec.cfg  : snd_soundfont \"$SF_FILE\""
