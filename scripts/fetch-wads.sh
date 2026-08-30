#!/usr/bin/env bash
# fetch-wads.sh - Community Megawad Downloader
# Downloads and installs free/legal community Doom megawads into the WAD directory.

set -e

OS="$(uname -s)"
if [ "$OS" = "Darwin" ]; then
    DEFAULT_DATA_DIR="$HOME/Library/Application Support/doom-configs"
    DEFAULT_WADS_DIR="$HOME/Library/Application Support/games/uzdoom"
else
    XDG_DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}"
    DEFAULT_DATA_DIR="$XDG_DATA_DIR/doom-configs"
    DEFAULT_WADS_DIR="$XDG_DATA_DIR/games/uzdoom"
fi

PRESETS_FILE="${DOOM_PRESETS_FILE:-$DEFAULT_DATA_DIR/presets.json}"

if [ ! -f "$PRESETS_FILE" ]; then
    # Fallback only if run directly from within the source repository tree
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -f "$SCRIPT_DIR/../data/presets.json" ]; then
        PRESETS_FILE="$SCRIPT_DIR/../data/presets.json"
    else
        echo "Error: Presets data file not found at: $PRESETS_FILE" >&2
        echo "Run 'make install' or './setup.sh' to install doom-configs." >&2
        exit 1
    fi
fi

WADS_DIR="${WADS_DIR:-$DEFAULT_WADS_DIR}"
FORCE=0

usage() {
    echo "Usage: $0 [OPTIONS] [PRESET_NAME|all]"
    echo ""
    echo "Options:"
    echo "  --list           List all available downloadable community megawads"
    echo "  --force          Re-download and overwrite existing WAD files"
    echo "  --dir <path>     Custom destination directory (default: $WADS_DIR)"
    echo "  --help           Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 all                    # Download all community megawads"
    echo "  $0 \"Eviternity II\"        # Download only Eviternity II"
    echo "  $0 --list                 # List available downloads"
    exit 0
}

list_wads() {
    echo "=== Available Downloadable Community Megawads ==="
    python3 -c "
import json
with open('$PRESETS_FILE') as f:
    data = json.load(f)
for p in data['presets']:
    urls = p.get('download_urls', [])
    if urls:
        files = ', '.join(p.get('mappacks', []))
        print(f\"  - {p['name']:<30} [{files}]\")
"
}

download_preset() {
    local name="$1"
    
    # Query preset details from presets.json
    local info
    info=$(python3 -c "
import json, sys
with open('$PRESETS_FILE') as f:
    data = json.load(f)
matched = [p for p in data['presets'] if p['name'].lower() == sys.argv[1].lower()]
if not matched:
    sys.exit(1)
p = matched[0]
urls = '|'.join(p.get('download_urls', []))
files = '|'.join(p.get('mappacks', []))
print(f\"{p['name']}###{urls}###{files}\")
" "$name" || true)

    if [ -z "$info" ]; then
        echo "Error: Preset '$name' not found or has no download sources."
        return 1
    fi

    local preset_name
    local urls_str
    local files_str
    preset_name=$(echo "$info" | awk -F'###' '{print $1}')
    urls_str=$(echo "$info" | awk -F'###' '{print $2}')
    files_str=$(echo "$info" | awk -F'###' '{print $3}')

    if [ -z "$urls_str" ]; then
        echo "Note: '$preset_name' is an official commercial release; download must be provided by user."
        return 0
    fi

    # Check if all files already exist
    local missing=0
    IFS='|' read -ra expected_files <<< "$files_str"
    for f in "${expected_files[@]}"; do
        if [ ! -f "$WADS_DIR/$f" ]; then
            missing=1
            break
        fi
    done

    if [ "$missing" -eq 0 ] && [ "$FORCE" -eq 0 ]; then
        echo "✓ [$preset_name] All required files already exist in $WADS_DIR. (Use --force to re-download)"
        return 0
    fi

    echo ">>> Downloading: $preset_name"
    mkdir -p "$WADS_DIR"

    local tmp_dir
    tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/wad_dl.XXXXXX")
    local success=0

    IFS='|' read -ra url_list <<< "$urls_str"
    for url in "${url_list[@]}"; do
        [ -z "$url" ] && continue
        echo "    Trying: $url"
        local archive="$tmp_dir/archive.zip"
        rm -f "$archive"

        if command -v curl >/dev/null 2>&1; then
            if curl -f -sSL -o "$archive" "$url"; then
                success=1
            fi
        elif command -v wget >/dev/null 2>&1; then
            if wget -q -O "$archive" "$url"; then
                success=1
            fi
        fi

        if [ "$success" -eq 1 ] && [ -f "$archive" ]; then
            local extract_dir="$tmp_dir/extracted"
            mkdir -p "$extract_dir"
            if unzip -q -o "$archive" -d "$extract_dir" 2>/dev/null; then
                # Search case-insensitively and move expected files
                for exp in "${expected_files[@]}"; do
                    local found
                    found=$(find "$extract_dir" -type f -iname "$exp" | head -n 1)
                    if [ -n "$found" ]; then
                        cp "$found" "$WADS_DIR/$exp"
                        echo "    Installed: $exp -> $WADS_DIR/$exp"
                    else
                        # In case files are nested or have slight extension variation
                        echo "    Warning: Could not find exact match for $exp in archive."
                    fi
                done
                break
            fi
        fi
        success=0
    done

    rm -rf "$tmp_dir"

    if [ "$success" -eq 0 ]; then
        echo "    Error: Failed to download $preset_name from available mirrors."
        return 1
    fi
    echo ""
}

# Parse options
TARGET=""
while [ $# -gt 0 ]; do
    case "$1" in
        --list)
            list_wads
            exit 0
            ;;
        --force)
            FORCE=1
            shift
            ;;
        --dir)
            WADS_DIR="$2"
            shift 2
            ;;
        --help|-h)
            usage
            ;;
        *)
            TARGET="$1"
            shift
            ;;
    esac
done

if [ -z "$TARGET" ]; then
    TARGET="all"
fi

mkdir -p "$WADS_DIR"
echo "=== Doom Community Megawad Downloader ==="
echo "Target directory: $WADS_DIR"
echo ""

if [ "$TARGET" = "all" ]; then
    python3 -c "
import json
with open('$PRESETS_FILE') as f:
    data = json.load(f)
for p in data['presets']:
    if p.get('download_urls'):
        print(p['name'])
" | while read -r name; do
        download_preset "$name" || true
    done
    echo "All community megawads processed!"
else
    download_preset "$TARGET"
fi
