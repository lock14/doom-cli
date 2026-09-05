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

if ! command -v python3 >/dev/null 2>&1; then
    echo "Error: python3 is required by fetch-wads but was not found in PATH." >&2
    exit 1
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
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
for p in data['presets']:
    urls = p.get('download_urls', [])
    if urls:
        files = ', '.join([m for m in p.get('mappacks', []) if m.lower() != 'idkfa 2024.wad'])
        print(f\"  - {p['name']:<30} [{files}]\")
" "$PRESETS_FILE"
}

download_preset() {
    local name="$1"
    local urls_str="${2:-}"
    local files_str="${3:-}"
    
    if [ -z "$urls_str" ] || [ -z "$files_str" ]; then
        # Query preset details from presets.json
        local info
        info=$(python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
matched = [p for p in data['presets'] if p['name'].lower() == sys.argv[2].lower()]
if not matched:
    sys.exit(1)
p = matched[0]
urls = '|'.join(p.get('download_urls', []))
files = '|'.join([m for m in p.get('mappacks', []) if m.lower() != 'idkfa 2024.wad'])
print(f\"{p['name']}###{urls}###{files}\")
" "$PRESETS_FILE" "$name" || true)

        if [ -z "$info" ]; then
            echo "Error: Preset '$name' not found or has no download sources."
            return 1
        fi

        urls_str=$(echo "$info" | awk -F'###' '{print $2}')
        files_str=$(echo "$info" | awk -F'###' '{print $3}')
    fi

    if [ -z "$urls_str" ]; then
        echo "Note: '$name' is an official commercial release; download must be provided by user."
        return 0
    fi

    # Check if all files already exist
    local missing=0
    IFS='|' read -ra expected_files <<< "$files_str"
    for f in "${expected_files[@]}"; do
        if [ ! -f "$WADS_DIR/$f" ]; then
            local alt
            alt=$(find "$WADS_DIR" -maxdepth 1 -type f -iname "$f" 2>/dev/null | head -n 1 || true)
            if [ -z "$alt" ] || [ ! -f "$alt" ]; then
                local f_norm
                f_norm=$(echo "$f" | tr '[:upper:]' '[:lower:]' | tr -d ' _-')
                while IFS= read -r cand; do
                    [ -z "$cand" ] && continue
                    local cand_norm
                    cand_norm=$(basename "$cand" | tr '[:upper:]' '[:lower:]' | tr -d ' _-')
                    if [ "$f_norm" = "$cand_norm" ]; then
                        alt="$cand"
                        break
                    fi
                done < <(find "$WADS_DIR" -maxdepth 1 -type f 2>/dev/null || true)
            fi
            if [ -z "$alt" ] || [ ! -f "$alt" ]; then
                case "$(echo "$f" | tr '[:upper:]' '[:lower:]')" in
                    "gdturbo.wad")
                        alt=$(find "$WADS_DIR" -maxdepth 1 -type f -iname "gd.wad" 2>/dev/null | head -n 1 || true)
                        ;;
                esac
            fi
            if [ -z "$alt" ] || [ ! -f "$alt" ]; then
                missing=1
                break
            fi
        fi
    done

    if [ "$missing" -eq 0 ] && [ "$FORCE" -eq 0 ]; then
        echo "✓ [$name] All required files already exist in $WADS_DIR. (Use --force to re-download)"
        return 0
    fi

    echo ">>> Downloading: $name"
    mkdir -p "$WADS_DIR"

    local tmp_dir
    tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/wad_dl.XXXXXX")
    local success=0

    IFS='|' read -ra url_list <<< "$urls_str"
    for url in "${url_list[@]}"; do
        [ -z "$url" ] && continue
        # Skip informational web page URLs that do not point to downloadable archives
        if [[ ! "$url" =~ \.(zip|wad|pk3|7z)$ ]]; then
            continue
        fi
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
                local all_found=1
                # Search case-insensitively and move expected files
                for exp in "${expected_files[@]}"; do
                    local found
                    # 1. Exact case-insensitive match
                    found=$(find "$extract_dir" -type f -iname "$exp" | head -n 1 || true)
                    # 2. Normalized match (ignoring spaces, dashes, underscores, and case)
                    if [ -z "$found" ] || [ ! -f "$found" ]; then
                        local exp_norm
                        exp_norm=$(echo "$exp" | tr '[:upper:]' '[:lower:]' | tr -d ' _-')
                        while IFS= read -r cand; do
                            [ -z "$cand" ] && continue
                            local cand_norm
                            cand_norm=$(basename "$cand" | tr '[:upper:]' '[:lower:]' | tr -d ' _-')
                            if [ "$exp_norm" = "$cand_norm" ]; then
                                found="$cand"
                                break
                            fi
                        done < <(find "$extract_dir" -type f 2>/dev/null || true)
                    fi
                    # 3. Known archive aliases (e.g. gd.zip containing gd.wad for gdturbo.wad)
                    if [ -z "$found" ] || [ ! -f "$found" ]; then
                        case "$(echo "$exp" | tr '[:upper:]' '[:lower:]')" in
                            "gdturbo.wad")
                                found=$(find "$extract_dir" -type f -iname "gd.wad" | head -n 1 || true)
                                ;;
                        esac
                    fi
                    # 4. Fallback: single .wad or .deh in archive if respective file was requested
                    if [ -z "$found" ] || [ ! -f "$found" ]; then
                        if [[ "$exp" =~ \.wad$|\.WAD$ ]]; then
                            local wad_matches
                            wad_matches=$(find "$extract_dir" -type f -iname "*.wad")
                            if [ "$(echo "$wad_matches" | grep -c . || true)" -eq 1 ]; then
                                found="$wad_matches"
                            fi
                        elif [[ "$exp" =~ \.deh$|\.DEH$ ]]; then
                            local deh_matches
                            deh_matches=$(find "$extract_dir" -type f -iname "*.deh")
                            if [ "$(echo "$deh_matches" | grep -c . || true)" -eq 1 ]; then
                                found="$deh_matches"
                            fi
                        fi
                    fi

                    if [ -n "$found" ] && [ -f "$found" ]; then
                        cp "$found" "$WADS_DIR/$exp"
                        echo "    Installed: $(basename "$found") -> $WADS_DIR/$exp"
                    else
                        echo "    Warning: Could not find match for $exp in archive."
                        all_found=0
                    fi
                done
                if [ "$all_found" -eq 1 ]; then
                    success=1
                    break
                else
                    success=0
                fi
            fi
        fi
        success=0
    done

    rm -rf "$tmp_dir"

    if [ "$success" -eq 0 ]; then
        echo "    Error: Failed to download $name from available mirrors."
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

if ! command -v unzip >/dev/null 2>&1; then
    echo "Error: 'unzip' utility is required to extract downloaded archives but was not found in PATH." >&2
    echo "Please install 'unzip' (e.g. 'sudo apt install unzip' or 'brew install unzip')." >&2
    exit 1
fi

mkdir -p "$WADS_DIR"
echo "=== Doom Community Megawad Downloader ==="
echo "Target directory: $WADS_DIR"
echo ""

if [ "$TARGET" = "all" ]; then
    while IFS=$'\t' read -r name urls files; do
        [ -z "$name" ] && continue
        download_preset "$name" "$urls" "$files" || true
    done < <(python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
for p in data['presets']:
    if p.get('download_urls'):
        urls = '|'.join(p.get('download_urls', []))
        files = '|'.join([m for m in p.get('mappacks', []) if m.lower() != 'idkfa 2024.wad'])
        print(f\"{p['name']}\t{urls}\t{files}\")
" "$PRESETS_FILE")
    echo "All community megawads processed!"
else
    download_preset "$TARGET"
fi
