#!/usr/bin/env bash
# doom-launch.sh - Interactive & CLI Launcher for Doom Source Ports
# Launches presets using DSDA-Doom or UZDoom from installed configuration.

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

# Locate presets.json from standard installed location or override
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
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

# Export environment so child processes (such as fzf --preview) inherit overrides
export WADS_DIR PRESETS_FILE BIN_DIR

if ! command -v python3 >/dev/null 2>&1; then
    echo "Error: python3 is required by doom-launch but was not found in PATH." >&2
    exit 1
fi

# Resolve absolute self path for fzf subshell preview execution
SELF_BIN="$(command -v "$0" 2>/dev/null || true)"
if [ -z "$SELF_BIN" ] || [ ! -x "$SELF_BIN" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    SELF_BIN="$SCRIPT_DIR/$(basename "$0")"
fi

usage() {
    echo "Usage: $(basename "$0") [OPTIONS] [PRESET_NAME] [ENGINE_ARGS...]"
    echo ""
    echo "Interactive terminal launcher for Doom presets."
    echo ""
    echo "Options:"
    echo "  --list                 List all available presets"
    echo "  --engine, -e <engine>  Override engine (dsda-doom or uzdoom)"
    echo "  --wads-dir <dir>       Set custom WADs directory (default: $WADS_DIR)"
    echo "  --dry-run              Print the engine command without executing"
    echo "  --help, -h             Show this help message"
    echo ""
    echo "Examples:"
    echo "  $(basename "$0")                                  # Interactive launcher (fzf or numbered menu)"
    echo "  $(basename "$0") \"Eviternity II\"                  # Launch specific preset"
    echo "  $(basename "$0") \"Ancient Aliens\" -e dsda-doom    # Override engine"
    echo "  $(basename "$0") \"Sunlust\" -skill 4 -warp 01      # Pass custom engine arguments"
    exit 0
}

list_presets() {
    echo "=== Available Doom Presets ==="
    python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
for i, p in enumerate(data['presets'], 1):
    eng = 'DSDA-Doom' if p['engine'] == 'dsda-doom' else 'UZDoom'
    print(f\"{i:2d}. {p['name']:<30} [{eng}] - {p.get('description', '')}\")
" "$PRESETS_FILE"
}

get_preset_json() {
    local target="$1"
    python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
target = sys.argv[2].strip().lower()
matched = [p for p in data['presets'] if p['name'].lower() == target]
if not matched:
    # Try prefix matching
    matched = [p for p in data['presets'] if p['name'].lower().startswith(target)]
if not matched:
    sys.exit(1)
print(json.dumps(matched[0]))
" "$PRESETS_FILE" "$target"
}

launch_preset() {
    local preset_json="$1"
    shift
    local extra_args=("$@")

    local name engine iwad mappacks_str preset_args
    IFS=$'\t' read -r name engine iwad mappacks_str preset_args < <(python3 -c "
import json, sys
d = json.loads(sys.argv[1])
print(f\"{d['name']}\t{d['engine']}\t{d['iwad']}\t{'###'.join(d.get('mappacks', []))}\t{d.get('additional_args', '')}\")
" "$preset_json")

    # Apply engine override if requested via --engine / -e
    if [ -n "$ENGINE_OVERRIDE" ]; then
        case "$(echo "$ENGINE_OVERRIDE" | tr '[:upper:]' '[:lower:]')" in
            dsda|dsda-doom|dsdadoom)
                engine="dsda-doom"
                ;;
            uzdoom|gzdoom|zdoom)
                engine="uzdoom"
                ;;
            *)
                engine="$ENGINE_OVERRIDE"
                ;;
        esac
    fi

    # Find engine executable
    local engine_bin=""
    if [ -x "$BIN_DIR/$engine" ]; then
        engine_bin="$BIN_DIR/$engine"
    elif command -v "$engine" >/dev/null 2>&1; then
        engine_bin="$(command -v "$engine")"
    else
        echo "Error: Engine binary '$engine' not found in $BIN_DIR or system PATH."
        echo "Run 'make install-engines' or 'make bootstrap' to install it."
        return 1
    fi

    # Check IWAD
    local iwad_path="$WADS_DIR/$iwad"
    if [ ! -f "$iwad_path" ]; then
        # Check case variation
        local alt_iwad
        alt_iwad=$(find "$WADS_DIR" -maxdepth 1 -type f -iname "$iwad" 2>/dev/null | head -n 1 || true)
        if [ -z "$alt_iwad" ] || [ ! -f "$alt_iwad" ]; then
            case "$(echo "$iwad" | tr '[:upper:]' '[:lower:]')" in
                "doom.wad")
                    alt_iwad=$(find "$WADS_DIR" -maxdepth 1 -type f -iname "doom1.wad" 2>/dev/null | head -n 1 || true)
                    ;;
            esac
        fi
        if [ -n "$alt_iwad" ] && [ -f "$alt_iwad" ]; then
            iwad_path="$alt_iwad"
        else
            echo "Error: Base IWAD '$iwad' not found in $WADS_DIR."
            echo "Run 'make extract-iwads' if you own the game on Steam/GOG."
            return 1
        fi
    fi

    # Build file arguments
    local wads=()
    local dehs=()
    local ordered_files=()
    if [ -n "$mappacks_str" ]; then
        IFS='###' read -ra files <<< "$mappacks_str"
        for f in "${files[@]}"; do
            [ -z "$f" ] && continue
            local fpath="$WADS_DIR/$f"
            if [ ! -f "$fpath" ]; then
                local alt_f
                # 1. Exact case-insensitive match
                alt_f=$(find "$WADS_DIR" -maxdepth 1 -type f -iname "$f" 2>/dev/null | head -n 1 || true)
                # 2. Normalized match (stripping spaces, dashes, underscores)
                if [ -z "$alt_f" ] || [ ! -f "$alt_f" ]; then
                    local f_norm
                    f_norm=$(echo "$f" | tr '[:upper:]' '[:lower:]' | tr -d ' _-')
                    while IFS= read -r cand; do
                        [ -z "$cand" ] && continue
                        local cand_norm
                        cand_norm=$(basename "$cand" | tr '[:upper:]' '[:lower:]' | tr -d ' _-')
                        if [ "$f_norm" = "$cand_norm" ]; then
                            alt_f="$cand"
                            break
                        fi
                    done < <(find "$WADS_DIR" -maxdepth 1 -type f 2>/dev/null || true)
                fi
                # 3. Known aliases (e.g. gdturbo.wad <-> gd.wad)
                if [ -z "$alt_f" ] || [ ! -f "$alt_f" ]; then
                    case "$(echo "$f" | tr '[:upper:]' '[:lower:]')" in
                        "gdturbo.wad")
                            alt_f=$(find "$WADS_DIR" -maxdepth 1 -type f -iname "gd.wad" 2>/dev/null | head -n 1 || true)
                            ;;
                        "gd.wad")
                            alt_f=$(find "$WADS_DIR" -maxdepth 1 -type f -iname "gdturbo.wad" 2>/dev/null | head -n 1 || true)
                            ;;
                    esac
                fi
                if [ -n "$alt_f" ] && [ -f "$alt_f" ]; then
                    fpath="$alt_f"
                else
                    case "$(echo "$f" | tr '[:upper:]' '[:lower:]')" in
                        "idkfa 2024.wad")
                            echo "  ℹ Optional soundtrack '$f' not found in $WADS_DIR; launching with default MIDI."
                            continue
                            ;;
                        *)
                            echo "Error: Required mappack file '$f' not found in $WADS_DIR." >&2
                            echo "Run 'make fetch-wads' or './scripts/fetch-wads.sh \"$name\"' to download it." >&2
                            return 1
                            ;;
                    esac
                fi
            fi
            ordered_files+=("$fpath")
            if [[ "$f" =~ \.deh$|\.DEH$ ]]; then
                dehs+=("$fpath")
            else
                wads+=("$fpath")
            fi
        done
    fi

    local cmd=("$engine_bin" "-iwad" "$iwad_path")

    if [ "$engine" = "dsda-doom" ]; then
        if [ ${#wads[@]} -gt 0 ]; then
            cmd+=("-file" "${wads[@]}")
        fi
        if [ ${#dehs[@]} -gt 0 ]; then
            cmd+=("-deh" "${dehs[@]}")
        fi
    else
        # UZDoom accepts all files under -file while maintaining exact declared order
        if [ ${#ordered_files[@]} -gt 0 ]; then
            cmd+=("-file" "${ordered_files[@]}")
        fi
    fi

    if [ -n "$preset_args" ]; then
        read -ra preset_args_array <<< "$preset_args"
        cmd+=("${preset_args_array[@]}")
    fi

    if [ ${#extra_args[@]} -gt 0 ]; then
        cmd+=("${extra_args[@]}")
    fi

    echo "=========================================="
    echo "Launching Preset: $name"
    echo "Engine: $engine ($engine_bin)"
    echo "IWAD:   $iwad_path"
    if [ ${#ordered_files[@]} -gt 0 ]; then
        echo "Files:  ${ordered_files[*]}"
    fi
    echo "=========================================="
    echo "Command: ${cmd[*]}"
    echo ""

    if [ "$DRY_RUN" -eq 1 ]; then
        return 0
    fi

    exec "${cmd[@]}"
}

# Preview helper for fzf
generate_preview() {
    local preset_name="$1"
    python3 -c "
import json, re, sys
from pathlib import Path

presets_file = sys.argv[1]
wads_dir = Path(sys.argv[2])
name = sys.argv[3].strip()

with open(presets_file) as f:
    data = json.load(f)

matched = [p for p in data['presets'] if p['name'] == name]
if not matched:
    sys.exit(0)
p = matched[0]

def check_file(target):
    if (wads_dir / target).exists():
        return True
    t_norm = re.sub(r'[\s_\-]', '', target).lower()
    if wads_dir.is_dir():
        for item in wads_dir.iterdir():
            if item.is_file():
                if item.name.lower() == target.lower():
                    return True
                if re.sub(r'[\s_\-]', '', item.name).lower() == t_norm:
                    return True
                if target.lower() == 'gdturbo.wad' and item.name.lower() == 'gd.wad':
                    return True
                if target.lower() == 'doom.wad' and item.name.lower() == 'doom1.wad':
                    return True
    return False

eng = 'DSDA-Doom (MBF21 / Speedrunning)' if p['engine'] == 'dsda-doom' else 'UZDoom (Software-Plus / ZDoom)'
iwad = p['iwad']
iwad_ok = '✓ Found' if check_file(iwad) else '✗ Missing'

print(f'Preset:        {p[\"name\"]}')
print(f'Engine:        {eng}')
print(f'Category:      {p.get(\"category\", \"N/A\")}')
print(f'Compatibility: {p.get(\"compatibility\", \"N/A\")}')
print(f'Description:   {p.get(\"description\", \"N/A\")}')
print(f'IWAD:          {iwad} [{iwad_ok}]')
print('Mappack Files:')
for m in p.get('mappacks', []):
    f_ok = '✓ Found' if check_file(m) else '✗ Missing'
    print(f'  - {m:<25} [{f_ok}]')
" "$PRESETS_FILE" "$WADS_DIR" "$preset_name"
}

DRY_RUN=0
ENGINE_OVERRIDE=""
SELECTED_TARGET=""
EXTRA_ARGS=()

while [ $# -gt 0 ]; do
    case "$1" in
        --preview)
            # Internal helper for fzf preview
            generate_preview "$2"
            exit 0
            ;;
        --list)
            list_presets
            exit 0
            ;;
        --dry-run)
            DRY_RUN=1
            shift
            ;;
        --engine|-e)
            ENGINE_OVERRIDE="$2"
            shift 2
            ;;
        --wads-dir)
            WADS_DIR="$2"
            shift 2
            ;;
        --help|-h)
            usage
            ;;
        -*)
            EXTRA_ARGS+=("$1")
            shift
            ;;
        *)
            if [ -z "$SELECTED_TARGET" ]; then
                SELECTED_TARGET="$1"
            else
                EXTRA_ARGS+=("$1")
            fi
            shift
            ;;
    esac
done

if [ -n "$SELECTED_TARGET" ]; then
    preset_json=$(get_preset_json "$SELECTED_TARGET" || true)
    if [ -z "$preset_json" ]; then
        echo "Error: Preset '$SELECTED_TARGET' not found."
        echo "Run '$(basename "$0") --list' to see available presets."
        exit 1
    fi
    launch_preset "$preset_json" "${EXTRA_ARGS[@]}"
    exit 0
fi

# Interactive Selection
if [ -t 0 ] && command -v fzf >/dev/null 2>&1; then
    PRESET_NAMES=$(python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
for p in data['presets']:
    print(p['name'])
" "$PRESETS_FILE")
    CHOICE=$(echo "$PRESET_NAMES" | fzf \
        --prompt="Select Doom Preset > " \
        --height=60% \
        --layout=reverse \
        --border \
        --preview="'$SELF_BIN' --preview {}")

    if [ -n "$CHOICE" ]; then
        preset_json=$(get_preset_json "$CHOICE")
        launch_preset "$preset_json" "${EXTRA_ARGS[@]}"
    fi
else
    # Fallback numbered terminal menu
    echo "======================================================"
    echo "               DOOM PRESET LAUNCHER                   "
    echo "======================================================"
    list_presets
    echo ""
    read -r -p "Enter preset number to launch (or 'q' to quit): " SELECTION
    if [[ "$SELECTION" =~ ^[0-9]+$ ]]; then
        preset_json=$(python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
idx = int(sys.argv[2]) - 1
if 0 <= idx < len(data['presets']):
    print(json.dumps(data['presets'][idx]))
" "$PRESETS_FILE" "$SELECTION" || true)
        if [ -n "$preset_json" ]; then
            launch_preset "$preset_json" "${EXTRA_ARGS[@]}"
        else
            echo "Invalid selection."
            exit 1
        fi
    else
        echo "Exiting."
        exit 0
    fi
fi
