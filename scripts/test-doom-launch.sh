#!/usr/bin/env bash
# test-doom-launch.sh - Comprehensive CLI & Functionality Test Suite for doom-launch
# Tests all flags, option combinations, engine overrides, error handling, and interactive modes.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "============================================================"
echo " Starting Comprehensive doom-launch CLI Test Suite          "
echo "============================================================"

# Create isolated test sandbox
SANDBOX=$(mktemp -d "${TMPDIR:-/tmp}/doom_launch_test.XXXXXX")
trap 'rm -rf "$SANDBOX"' EXIT INT TERM

BIN_DIR="$SANDBOX/bin"
WADS_DIR="$SANDBOX/wads"
CUSTOM_WADS_DIR="$SANDBOX/custom_wads"
DATA_DIR="$SANDBOX/data"

mkdir -p "$BIN_DIR" "$WADS_DIR" "$CUSTOM_WADS_DIR" "$DATA_DIR"

# Copy presets data
cp "$ROOT_DIR/data/presets.json" "$DATA_DIR/presets.json"

# Create mock engine binaries
cat << 'EOF' > "$BIN_DIR/dsda-doom"
#!/bin/sh
echo "MOCK_DSDA_DOOM: $@"
EOF
chmod +x "$BIN_DIR/dsda-doom"

cat << 'EOF' > "$BIN_DIR/uzdoom"
#!/bin/sh
echo "MOCK_UZDOOM: $@"
EOF
chmod +x "$BIN_DIR/uzdoom"

# Create mock IWADs and PWADs in primary WADS_DIR
touch "$WADS_DIR/DOOM.WAD" "$WADS_DIR/DOOM2.WAD" "$WADS_DIR/PLUTONIA.WAD" "$WADS_DIR/TNT.WAD" "$WADS_DIR/HERETIC.WAD" "$WADS_DIR/HEXEN.WAD"
touch "$WADS_DIR/aaliens_v1_2.wad" "$WADS_DIR/deathless.wad" "$WADS_DIR/AV.WAD" "$WADS_DIR/AV.DEH" "$WADS_DIR/sunlust.wad"

# Create mock IWADs and PWADs in CUSTOM_WADS_DIR
touch "$CUSTOM_WADS_DIR/DOOM2.WAD" "$CUSTOM_WADS_DIR/custom_map.wad" "$CUSTOM_WADS_DIR/sunlust.wad"

DOOM_LAUNCH="$ROOT_DIR/scripts/doom-launch.sh"

run_launch() {
    HOME="$SANDBOX" \
    BIN_DIR="$BIN_DIR" \
    WADS_DIR="$WADS_DIR" \
    DOOM_PRESETS_FILE="$DATA_DIR/presets.json" \
    "$DOOM_LAUNCH" "$@"
}

TOTAL_TESTS=0
PASSED_TESTS=0

assert_cmd() {
    local desc="$1"
    local cmd="$2"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if eval "$cmd"; then
        echo "  ✓ $desc"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "  ✗ FAIL: $desc"
        exit 1
    fi
}

# -----------------------------------------------------------------------------
echo ""
echo "Section 1: Help and Information Options"
# -----------------------------------------------------------------------------
assert_cmd "--help displays usage and returns 0" \
    'run_launch --help | grep -q "Usage: doom-launch"'

assert_cmd "-h shorthand displays usage and returns 0" \
    'run_launch -h | grep -q "Usage: doom-launch"'

assert_cmd "--list returns all 32 presets with engines" \
    "test \$(run_launch --list | grep -E '^\s*[0-9]+\.' | wc -l) -eq 32"

assert_cmd "--list formats DSDA-Doom and UZDoom labels correctly" \
    'run_launch --list | grep -q "\[DSDA-Doom\]" && run_launch --list | grep -q "\[UZDoom\]"'

# -----------------------------------------------------------------------------
echo ""
echo "Section 2: Preview Panel Generation"
# -----------------------------------------------------------------------------
assert_cmd "--preview generates structured metadata block" \
    'run_launch --preview "Ancient Aliens" | grep -q "Preset:\s*Ancient Aliens"'

assert_cmd "--preview detects existing files with [✓ Found]" \
    'run_launch --preview "Ancient Aliens" | grep -q "DOOM2.WAD \[✓ Found\]"'

assert_cmd "--preview detects missing files with [✗ Missing]" \
    'run_launch --preview "Eviternity II" | grep -q "eviternityii.wad\s*\[✗ Missing\]"'

assert_cmd "--preview on unknown preset exits cleanly without error" \
    'run_launch --preview "NonExistentPreset" >/dev/null'

# -----------------------------------------------------------------------------
echo ""
echo "Section 3: Dry-Run Preset Synthesis & Name Matching"
# -----------------------------------------------------------------------------
assert_cmd "Exact name match launches correctly" \
    'run_launch --dry-run "Ancient Aliens" | grep -q "Launching Preset: Ancient Aliens"'

assert_cmd "Case-insensitive name match works (ancient aliens)" \
    'run_launch --dry-run "ancient aliens" | grep -q "Launching Preset: Ancient Aliens"'

assert_cmd "Prefix name match works (Alien Vend)" \
    'run_launch --dry-run "Alien Vend" | grep -q "Launching Preset: Alien Vendetta"'

# -----------------------------------------------------------------------------
echo ""
echo "Section 4: Engine Overrides (--engine and -e)"
# -----------------------------------------------------------------------------
assert_cmd "Default engine for Ancient Aliens is uzdoom" \
    'run_launch --dry-run "Ancient Aliens" | grep -q "Engine: uzdoom"'

assert_cmd "-e dsda-doom overrides engine from uzdoom to dsda-doom" \
    'run_launch --dry-run "Ancient Aliens" -e dsda-doom | grep -q "Engine: dsda-doom"'

assert_cmd "-e dsda alias normalizes to dsda-doom" \
    'run_launch --dry-run "Ancient Aliens" -e dsda | grep -q "Engine: dsda-doom"'

assert_cmd "--engine uzdoom overrides engine from dsda-doom to uzdoom" \
    'run_launch --dry-run "Sunlust" --engine uzdoom | grep -q "Engine: uzdoom"'

assert_cmd "-e gzdoom alias normalizes to uzdoom" \
    'run_launch --dry-run "Sunlust" -e gzdoom | grep -q "Engine: uzdoom"'

assert_cmd "Engine flag position independence (before preset name)" \
    'run_launch -e dsda-doom "Ancient Aliens" --dry-run | grep -q "Engine: dsda-doom"'

assert_cmd "Engine flag position independence (interleaved with args)" \
    'run_launch "Ancient Aliens" --dry-run -e dsda-doom -skill 4 | grep -q "Engine: dsda-doom"'

# -----------------------------------------------------------------------------
echo ""
echo "Section 5: DeHackEd (.deh) and File Flag Separation"
# -----------------------------------------------------------------------------
assert_cmd "DSDA-Doom correctly places .deh under -deh and .wad under -file" \
    'run_launch --dry-run "Alien Vendetta" | grep -q "Command:.*dsda-doom.*-iwad.*DOOM2.WAD.*-file.*AV.WAD.*-deh.*AV.DEH"'

assert_cmd "UZDoom receives all files under -file" \
    'run_launch --dry-run "Alien Vendetta" -e uzdoom | grep -q "Command:.*uzdoom.*-iwad.*DOOM2.WAD.*-file.*AV.WAD.*AV.DEH"'

# -----------------------------------------------------------------------------
echo ""
echo "Section 6: Custom Engine Flags and WADs Directory Overrides"
# -----------------------------------------------------------------------------
assert_cmd "Extra engine arguments are forwarded correctly" \
    'run_launch --dry-run "Sunlust" -skill 4 -warp 01 -nomonsters | grep -q "Command:.*-skill 4 -warp 01 -nomonsters"'

# Inject temporary additional_args into a preset in sandboxed presets.json
python3 -c "
import json
with open('$DATA_DIR/presets.json', 'r') as f:
    d = json.load(f)
for p in d['presets']:
    if p['name'] == 'Sunlust':
        p['additional_args'] = '-complevel 21'
with open('$DATA_DIR/presets.json', 'w') as f:
    json.dump(d, f)
"
assert_cmd "Preset additional_args from presets.json are included in launch command" \
    'run_launch --dry-run "Sunlust" | grep -q "Command:.*-complevel 21"'

# Restore presets.json
cp "$ROOT_DIR/data/presets.json" "$DATA_DIR/presets.json"

assert_cmd "--wads-dir overrides search path for IWADs and PWADs" \
    "run_launch --dry-run --wads-dir '$CUSTOM_WADS_DIR' 'Sunlust' | grep -q 'IWAD:\s*$CUSTOM_WADS_DIR/DOOM2.WAD'"

# -----------------------------------------------------------------------------
echo ""
echo "Section 7: Interactive Numbered Menu (stdin Simulation)"
# -----------------------------------------------------------------------------
assert_cmd "Interactive numbered menu launches chosen preset on valid number input" \
    'echo "1" | run_launch --dry-run | grep -q "Launching Preset:"'

assert_cmd "Interactive numbered menu exits cleanly on '\''q'\''" \
    'echo "q" | run_launch >/dev/null'

assert_cmd "Interactive numbered menu rejects invalid selection" \
    '! echo "999" | run_launch --dry-run 2>/dev/null'

# -----------------------------------------------------------------------------
echo ""
echo "Section 8: Error Handling & Missing Requirements"
# -----------------------------------------------------------------------------
assert_cmd "Non-existent preset exits with error code 1" \
    '! run_launch "NoSuchDoomMapsetXYZ" 2>/dev/null'

assert_cmd "Missing engine binary exits with error code 1" \
    "env -i HOME='$SANDBOX' DOOM_PRESETS_FILE='$DATA_DIR/presets.json' WADS_DIR='$WADS_DIR' PATH='$SANDBOX/empty_bin' BIN_DIR='$SANDBOX/empty_bin' '$DOOM_LAUNCH' --dry-run 'Ancient Aliens' 2>/dev/null; test \$? -ne 0"

assert_cmd "Missing base IWAD exits with error code 1" \
    "env WADS_DIR='$SANDBOX/empty_wads' run_launch --dry-run 'Ancient Aliens' 2>/dev/null; test \$? -ne 0"

mkdir -p "$SANDBOX/missing_pwad" && touch "$SANDBOX/missing_pwad/DOOM2.WAD"
assert_cmd "Missing required mappack file exits with error code 1" \
    "env WADS_DIR='$SANDBOX/missing_pwad' run_launch --dry-run 'Ancient Aliens' 2>/dev/null; test \$? -ne 0"

assert_cmd "Missing optional idkfa 2024.wad launches cleanly with default MIDI" \
    "run_launch --dry-run 'Doom' | grep '^Command:' | grep -q 'dsda-doom.*-iwad.*DOOM.WAD' && ! (run_launch --dry-run 'Doom' | grep '^Command:' | grep -q 'idkfa 2024.wad')"

# -----------------------------------------------------------------------------
echo ""
echo "Section 9: Normalized Filename Matching & Archive Aliases"
# -----------------------------------------------------------------------------
touch "$WADS_DIR/Eviternity II.wad"
assert_cmd "Preview detects spaced file Eviternity II.wad for eviternityii.wad" \
    'run_launch --preview "Eviternity II" | grep -q "eviternityii.wad.*\[✓ Found\]"'

assert_cmd "Dry-run resolves Eviternity II.wad with space tolerance" \
    'run_launch --dry-run "Eviternity II" | grep -q "Eviternity II\.wad"'

touch "$WADS_DIR/gd.wad"
assert_cmd "Preview detects gd.wad alias for Going Down Turbo" \
    'run_launch --preview "Going Down Turbo" | grep -q "gdturbo.wad.*\[✓ Found\]"'

assert_cmd "Dry-run resolves gd.wad alias for Going Down Turbo" \
    'run_launch --dry-run "Going Down Turbo" | grep -q "gd\.wad"'

mkdir -p "$SANDBOX/doom1_sandbox" && touch "$SANDBOX/doom1_sandbox/doom1.wad"
assert_cmd "IWAD discovery resolves doom1.wad fallback for DOOM.WAD" \
    "WADS_DIR='$SANDBOX/doom1_sandbox' run_launch --dry-run 'Doom' | grep -q 'IWAD:.*doom1\.wad'"

echo ""
echo "============================================================"
echo "  ✓ ALL $TOTAL_TESTS doom-launch CLI TESTS PASSED (100%)!    "
echo "============================================================"
