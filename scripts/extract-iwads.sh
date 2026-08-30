#!/usr/bin/env bash
# extract-iwads.sh - Steam & GOG IWAD Auto-Discovery and Extractor
# Locates legally purchased Doom/Heretic/Hexen game files on local machine and deploys them to WADs folder.

set -e

OS="$(uname -s)"
if [ "$OS" = "Darwin" ]; then
    DEFAULT_WADS_DIR="$HOME/Library/Application Support/games/uzdoom"
else
    DEFAULT_WADS_DIR="$HOME/.local/share/games/uzdoom"
fi

WADS_DIR="${WADS_DIR:-$DEFAULT_WADS_DIR}"
FORCE=0

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --dir <path>     Custom destination directory (default: $WADS_DIR)"
    echo "  --force          Overwrite existing files in destination directory"
    echo "  --help           Show this help message"
    exit 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        --dir)
            WADS_DIR="$2"
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

mkdir -p "$WADS_DIR"

echo "=== Doom IWAD & Commercial Expansion Extractor ==="
echo "Target WADs directory: $WADS_DIR"
echo ""

# Discover search roots
SEARCH_ROOTS=()

if [ "$OS" = "Darwin" ]; then
    SEARCH_ROOTS+=(
        "$HOME/Library/Application Support/Steam"
        "$HOME/Library/Application Support/GOG.com"
        "/Applications"
    )
else
    SEARCH_ROOTS+=(
        "$HOME/.local/share/Steam"
        "$HOME/.steam/steam"
        "$HOME/.steam/root"
        "$HOME/.var/app/com.valvesoftware.Steam/data/Steam"
        "$HOME/GOG Games"
        "$HOME/.local/share/bottles"
        "$HOME/.wine"
    )
fi

# Extract custom Steam library folders if libraryfolders.vdf exists
for steam_root in "${SEARCH_ROOTS[@]}"; do
    vdf="$steam_root/steamapps/libraryfolders.vdf"
    if [ -f "$vdf" ]; then
        while IFS= read -r path; do
            if [ -n "$path" ] && [ -d "$path" ]; then
                SEARCH_ROOTS+=("$path")
            fi
        done < <(grep -E '^\s*"path"' "$vdf" | cut -d '"' -f 4 || true)
    fi
done

# Filter search roots to existing directories without duplicates
EXISTING_ROOTS=()
declare -A SEEN_ROOTS
for r in "${SEARCH_ROOTS[@]}"; do
    if [ -d "$r" ] && [ -z "${SEEN_ROOTS["$r"]:-}" ]; then
        EXISTING_ROOTS+=("$r")
        SEEN_ROOTS["$r"]=1
    fi
done

if [ ${#EXISTING_ROOTS[@]} -eq 0 ]; then
    echo "No Steam or GOG directories found on this system."
    exit 0
fi

echo "Searching for official game files across:"
for r in "${EXISTING_ROOTS[@]}"; do
    echo "  - $r"
done
echo ""

# Targets to search and their normalized destination names
TARGET_PATTERNS=(
    "DOOM.WAD:DOOM.WAD"
    "DOOM1.WAD:DOOM.WAD"
    "DOOM2.WAD:DOOM2.WAD"
    "PLUTONIA.WAD:PLUTONIA.WAD"
    "TNT.WAD:TNT.WAD"
    "HERETIC.WAD:HERETIC.WAD"
    "HEXEN.WAD:HEXEN.WAD"
    "HEXDD.WAD:HEXDD.WAD"
    "NERVE.WAD:NERVE.WAD"
    "MASTERLEVELS.WAD:MASTERLEVELS.WAD"
    "idkfa 2024.wad:idkfa 2024.wad"
    "idkfa_2024.wad:idkfa 2024.wad"
    "id24res.wad:id24res.wad"
    "id1-res.wad:id1-res.wad"
    "id1-weap.wad:id1-weap.wad"
    "id1.wad:id1.wad"
    "extras.wad:extras.wad"
    "sigil.wad:SIGIL_V1_23.wad"
    "sigil_ii.wad:SIGIL_II_V1_0.WAD"
)

FOUND_COUNT=0

for item in "${TARGET_PATTERNS[@]}"; do
    pattern="${item%%:*}"
    dest_name="${item##*:}"
    dest_file="$WADS_DIR/$dest_name"

    if [ -f "$dest_file" ] && [ "$FORCE" -eq 0 ]; then
        continue
    fi

    for root in "${EXISTING_ROOTS[@]}"; do
        # Search case-insensitively, limited depth for speed
        match=$(find "$root" -maxdepth 6 -type f -iname "$pattern" 2>/dev/null | head -n 1 || true)
        if [ -n "$match" ] && [ -f "$match" ]; then
            cp "$match" "$dest_file"
            echo "✓ Found & Installed: $dest_name"
            echo "    Source: $match"
            FOUND_COUNT=$((FOUND_COUNT + 1))
            break
        fi
    done
done

echo ""
if [ "$FOUND_COUNT" -gt 0 ]; then
    echo "Extracted $FOUND_COUNT game file(s) into $WADS_DIR"
else
    echo "No new game files found to extract. (Existing files were preserved)"
fi
