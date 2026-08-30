#!/usr/bin/env bash
# doom-launch.sh - Interactive & CLI Launcher for Doom Source Ports
# Launches presets from data/presets.json using DSDA-Doom or UZDoom.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Locate presets.json across source repo and standard installed locations
PRESETS_FILE=""
if [ -n "${DOOM_PRESETS_FILE:-}" ] && [ -f "$DOOM_PRESETS_FILE" ]; then
    PRESETS_FILE="$DOOM_PRESETS_FILE"
elif [ -f "$SCRIPT_DIR/../data/presets.json" ]; then
    PRESETS_FILE="$SCRIPT_DIR/../data/presets.json"
elif [ -f "$HOME/.local/share/doom-configs/presets.json" ]; then
    PRESETS_FILE="$HOME/.local/share/doom-configs/presets.json"
elif [ -f "$HOME/Library/Application Support/doom-configs/presets.json" ]; then
    PRESETS_FILE="$HOME/Library/Application Support/doom-configs/presets.json"
fi

if [ -z "$PRESETS_FILE" ] || [ ! -f "$PRESETS_FILE" ]; then
    echo "Error: presets.json data file not found." >&2
    echo "Run 'make install' or './setup.sh' to install configuration and preset data files." >&2
    exit 1
fi

OS="$(uname -s)"
if [ "$OS" = "Darwin" ]; then
    DEFAULT_WADS_DIR="$HOME/Library/Application Support/games/uzdoom"
else
    DEFAULT_WADS_DIR="$HOME/.local/share/games/uzdoom"
fi
WADS_DIR="${WADS_DIR:-$DEFAULT_WADS_DIR}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

usage() {
    echo "Usage: $(basename "$0") [OPTIONS] [PRESET_NAME] [ENGINE_ARGS...]"
    echo ""
    echo "Interactive terminal launcher for Doom presets."
    echo ""
    echo "Options:"
    echo "  --list           List all available presets"
    echo "  --wads-dir <dir> Set custom WADs directory (default: $WADS_DIR)"
    echo "  --dry-run        Print the engine command without executing"
    echo "  --help, -h       Show this help message"
    echo ""
    echo "Examples:"
    echo "  $(basename "$0")                          # Interactive launcher (fzf or numbered menu)"
    echo "  $(basename "$0") \"Eviternity II\"          # Launch specific preset"
    echo "  $(basename "$0") \"Sunlust\" -skill 4 -warp 01  # Pass custom engine arguments"
    exit 0
}

list_presets() {
    echo "=== Available Doom Presets ==="
    python3 -c "
import json
with open('$PRESETS_FILE') as f:
    data = json.load(f)
for i, p in enumerate(data['presets'], 1):
    eng = 'DSDA-Doom' if p['engine'] == 'dsda-doom' else 'UZDoom'
    print(f\"{i:2d}. {p['name']:<30} [{eng}] - {p.get('description', '')}\")
"
}

get_preset_json() {
    local target="$1"
    python3 -c "
import json, sys
with open('$PRESETS_FILE') as f:
    data = json.load(f)
target = sys.argv[1].strip().lower()
matched = [p for p in data['presets'] if p['name'].lower() == target]
if not matched:
    # Try prefix matching
    matched = [p for p in data['presets'] if p['name'].lower().startswith(target)]
if not matched:
    sys.exit(1)
print(json.dumps(matched[0]))
" "$target"
}

launch_preset() {
    local preset_json="$1"
    shift
    local extra_args=("$@")

    local name
    local engine
    local iwad
    local mappacks_str
    name=$(python3 -c "import json, sys; d=json.loads(sys.argv[1]); print(d['name'])" "$preset_json")
    engine=$(python3 -c "import json, sys; d=json.loads(sys.argv[1]); print(d['engine'])" "$preset_json")
    iwad=$(python3 -c "import json, sys; d=json.loads(sys.argv[1]); print(d['iwad'])" "$preset_json")
    mappacks_str=$(python3 -c "import json, sys; d=json.loads(sys.argv[1]); print('###'.join(d.get('mappacks', [])))" "$preset_json")

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
    if [ -n "$mappacks_str" ]; then
        IFS='###' read -ra files <<< "$mappacks_str"
        for f in "${files[@]}"; do
            [ -z "$f" ] && continue
            local fpath="$WADS_DIR/$f"
            if [ ! -f "$fpath" ]; then
                local alt_f
                alt_f=$(find "$WADS_DIR" -maxdepth 1 -type f -iname "$f" 2>/dev/null | head -n 1 || true)
                if [ -n "$alt_f" ] && [ -f "$alt_f" ]; then
                    fpath="$alt_f"
                else
                    echo "Warning: Mappack file '$f' not found in $WADS_DIR."
                    echo "Run 'make fetch-wads' to download community megawads."
                fi
            fi
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
        # UZDoom accepts all files under -file
        local all_files=("${wads[@]}" "${dehs[@]}")
        if [ ${#all_files[@]} -gt 0 ]; then
            cmd+=("-file" "${all_files[@]}")
        fi
    fi

    if [ ${#extra_args[@]} -gt 0 ]; then
        cmd+=("${extra_args[@]}")
    fi

    echo "=========================================="
    echo "Launching Preset: $name"
    echo "Engine: $engine ($engine_bin)"
    echo "IWAD:   $iwad_path"
    if [ ${#wads[@]} -gt 0 ] || [ ${#dehs[@]} -gt 0 ]; then
        echo "Files:  ${wads[*]} ${dehs[*]}"
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
import json, sys, os
from pathlib import Path

with open('$PRESETS_FILE') as f:
    data = json.load(f)

name = sys.argv[1].strip()
wads_dir = Path('$WADS_DIR')

matched = [p for p in data['presets'] if p['name'] == name]
if not matched:
    sys.exit(0)
p = matched[0]

eng = 'DSDA-Doom (MBF21 / Speedrunning)' if p['engine'] == 'dsda-doom' else 'UZDoom (Software-Plus / ZDoom)'
iwad = p['iwad']
iwad_ok = '✓ Found' if (wads_dir / iwad).exists() else '✗ Missing'

print(f'Preset:        {p[\"name\"]}')
print(f'Engine:        {eng}')
print(f'Category:      {p.get(\"category\", \"N/A\")}')
print(f'Compatibility: {p.get(\"compatibility\", \"N/A\")}')
print(f'Description:   {p.get(\"description\", \"N/A\")}')
print(f'IWAD:          {iwad} [{iwad_ok}]')
print('Mappack Files:')
for m in p.get('mappacks', []):
    f_ok = '✓ Found' if (wads_dir / m).exists() else '✗ Missing'
    print(f'  - {m:<25} [{f_ok}]')
" "$preset_name"
}

DRY_RUN=0
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
if command -v fzf >/dev/null 2>&1; then
    PRESET_NAMES=$(python3 -c "
import json
with open('$PRESETS_FILE') as f:
    data = json.load(f)
for p in data['presets']:
    print(p['name'])
")
    CHOICE=$(echo "$PRESET_NAMES" | fzf \
        --prompt="Select Doom Preset > " \
        --height=60% \
        --layout=reverse \
        --border \
        --preview="$0 --preview {}")

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
with open('$PRESETS_FILE') as f:
    data = json.load(f)
idx = int(sys.argv[1]) - 1
if 0 <= idx < len(data['presets']):
    print(json.dumps(data['presets'][idx]))
" "$SELECTION" || true)
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
