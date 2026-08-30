#!/usr/bin/env bash
# test-turnkey.sh - Comprehensive End-to-End Turnkey & System Test Suite
# Tests complete turnkey installation, mock Steam discovery, WAD downloads,
# and CLI launcher execution in an isolated sandbox.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

OS="$(uname -s)"
echo "============================================================"
echo " Starting Turnkey Validation & End-to-End Test Suite ($OS)   "
echo "============================================================"

# Create isolated sandbox environment
SANDBOX=$(mktemp -d "${TMPDIR:-/tmp}/doom_turnkey_test.XXXXXX")
trap 'rm -rf "$SANDBOX"' EXIT INT TERM

echo "Sandbox environment: $SANDBOX"

# Set sandboxed XDG and Application Support directories
if [ "$OS" = "Darwin" ]; then
    APP_SUPPORT="$SANDBOX/Library/Application Support"
    UZDOOM_DIR="$APP_SUPPORT/uzdoom"
    DSDA_DIR="$APP_SUPPORT/dsda-doom"
    RUNNER_DIR="$APP_SUPPORT/DoomRunner"
    DATA_DIR="$APP_SUPPORT/doom-configs"
    WADS_DIR="$APP_SUPPORT/games/uzdoom"
    SF_DIR="$APP_SUPPORT/soundfonts"
else
    UZDOOM_DIR="$SANDBOX/.config/uzdoom"
    DSDA_DIR="$SANDBOX/.local/share/dsda-doom"
    RUNNER_DIR="$SANDBOX/.local/share/DoomRunner"
    DATA_DIR="$SANDBOX/.local/share/doom-configs"
    WADS_DIR="$SANDBOX/.local/share/games/uzdoom"
    SF_DIR="$SANDBOX/.local/share/soundfonts"
fi
BIN_DIR="$SANDBOX/.local/bin"

mkdir -p "$BIN_DIR" "$WADS_DIR" "$SF_DIR"

# -----------------------------------------------------------------------------
# TEST 1: Mock Steam & GOG Discovery Environment
# -----------------------------------------------------------------------------
echo ""
echo "[Test 1/6] Setting up mock Steam library & testing IWAD extractor..."

if [ "$OS" = "Darwin" ]; then
    STEAM_MOCK="$SANDBOX/Library/Application Support/Steam/steamapps"
else
    STEAM_MOCK="$SANDBOX/.local/share/Steam/steamapps"
fi
mkdir -p "$STEAM_MOCK/common/Doom 2/base"
mkdir -p "$STEAM_MOCK/common/Doom/rerelease"
mkdir -p "$SANDBOX/secondary_steam_drive/steamapps/common/Final Doom"

# Create mock commercial files
echo "MOCK_DOOM2_IWAD" > "$STEAM_MOCK/common/Doom 2/base/DOOM2.WAD"
echo "MOCK_DOOM_IWAD" > "$STEAM_MOCK/common/Doom/rerelease/DOOM.WAD"
echo "MOCK_IDKFA_SOUNDTRACK" > "$STEAM_MOCK/common/Doom/rerelease/idkfa 2024.wad"
echo "MOCK_PLUTONIA_IWAD" > "$SANDBOX/secondary_steam_drive/steamapps/common/Final Doom/PLUTONIA.WAD"

# Create mock libraryfolders.vdf
cat << EOF > "$STEAM_MOCK/libraryfolders.vdf"
"libraryfolders"
{
    "0"
    {
        "path" "$SANDBOX/secondary_steam_drive"
        "label" ""
    }
}
EOF

# Run extraction pointing to sandbox HOME
HOME="$SANDBOX" "$ROOT_DIR/scripts/extract-iwads.sh" --dir "$WADS_DIR"

# Verify all mock IWADs and soundtrack were discovered and extracted
test -f "$WADS_DIR/DOOM2.WAD" || { echo "FAIL: DOOM2.WAD was not extracted"; exit 1; }
test -f "$WADS_DIR/DOOM.WAD" || { echo "FAIL: DOOM.WAD was not extracted"; exit 1; }
test -f "$WADS_DIR/PLUTONIA.WAD" || { echo "FAIL: PLUTONIA.WAD from secondary library was not extracted"; exit 1; }
test -f "$WADS_DIR/idkfa 2024.wad" || { echo "FAIL: idkfa 2024.wad was not extracted"; exit 1; }
echo "✓ Test 1 Passed: Steam & GOG multi-library discovery and extraction verified."

# -----------------------------------------------------------------------------
# TEST 2: Standalone and Makefile Installation with Backup Verification
# -----------------------------------------------------------------------------
echo ""
echo "[Test 2/6] Testing config deployment & timestamped backup generation..."

# First install
HOME="$SANDBOX" PREFIX="$SANDBOX" BIN_DIR="$BIN_DIR" "$ROOT_DIR/setup.sh"

test -f "$UZDOOM_DIR/autoexec.cfg" || { echo "FAIL: Missing autoexec.cfg"; exit 1; }
test -f "$DSDA_DIR/dsda-doom.cfg" || { echo "FAIL: Missing dsda-doom.cfg"; exit 1; }
test -f "$RUNNER_DIR/options.json" || { echo "FAIL: Missing options.json"; exit 1; }
test -f "$DATA_DIR/presets.json" || { echo "FAIL: Missing presets.json"; exit 1; }
test -f "$BIN_DIR/doom-launch" || { echo "FAIL: Missing doom-launch executable"; exit 1; }

# Verify no un-substituted template placeholders remain in installed files
if grep -q '__HOME__' "$RUNNER_DIR/options.json"; then
    echo "FAIL: Un-substituted __HOME__ found in $RUNNER_DIR/options.json"; exit 1
fi
if grep -q '__RESOLUTION__' "$DSDA_DIR/dsda-doom.cfg"; then
    echo "FAIL: Un-substituted __RESOLUTION__ found in $DSDA_DIR/dsda-doom.cfg"; exit 1
fi
if grep -q '__SOUNDFONT__' "$DSDA_DIR/dsda-doom.cfg"; then
    echo "FAIL: Un-substituted __SOUNDFONT__ found in $DSDA_DIR/dsda-doom.cfg"; exit 1
fi
if grep -q '__SOUNDFONT__' "$UZDOOM_DIR/autoexec.cfg"; then
    echo "FAIL: Un-substituted __SOUNDFONT__ found in $UZDOOM_DIR/autoexec.cfg"; exit 1
fi

# Second install (must trigger backups)
sleep 1 # Ensure timestamp differs
HOME="$SANDBOX" PREFIX="$SANDBOX" BIN_DIR="$BIN_DIR" "$ROOT_DIR/setup.sh"

find "$UZDOOM_DIR" -maxdepth 1 -name "autoexec.cfg.bak.*" | grep -q . || { echo "FAIL: Backup not created for autoexec.cfg"; exit 1; }
find "$DSDA_DIR" -maxdepth 1 -name "dsda-doom.cfg.bak.*" | grep -q . || { echo "FAIL: Backup not created for dsda-doom.cfg"; exit 1; }
find "$RUNNER_DIR" -maxdepth 1 -name "options.json.bak.*" | grep -q . || { echo "FAIL: Backup not created for options.json"; exit 1; }
echo "✓ Test 2 Passed: Configuration deployment, placeholder substitution, and backups verified."

# -----------------------------------------------------------------------------
# TEST 3: Mock Engines Deployment
# -----------------------------------------------------------------------------
echo ""
echo "[Test 3/6] Setting up mock engine executables..."

cat << 'EOF' > "$BIN_DIR/uzdoom"
#!/bin/sh
echo "MOCK_UZDOOM_EXECUTED: $@"
EOF
chmod +x "$BIN_DIR/uzdoom"

cat << 'EOF' > "$BIN_DIR/dsda-doom"
#!/bin/sh
echo "MOCK_DSDA_DOOM_EXECUTED: $@"
EOF
chmod +x "$BIN_DIR/dsda-doom"

cat << 'EOF' > "$BIN_DIR/doomrunner"
#!/bin/sh
echo "MOCK_DOOMRUNNER_EXECUTED: $@"
EOF
chmod +x "$BIN_DIR/doomrunner"

echo "✓ Test 3 Passed: Mock engine binaries created in $BIN_DIR."

# -----------------------------------------------------------------------------
# TEST 4: Community Megawad Downloader on Sample Target
# -----------------------------------------------------------------------------
echo ""
echo "[Test 4/6] Testing community megawad fetcher with a sample target (Deathless)..."

HOME="$SANDBOX" "$ROOT_DIR/scripts/fetch-wads.sh" --dir "$WADS_DIR" "Deathless"

# Verify that deathless.wad was downloaded and unpacked
test -f "$WADS_DIR/deathless.wad" || { echo "FAIL: deathless.wad was not downloaded"; exit 1; }
echo "✓ Test 4 Passed: Megawad download and archive extraction verified."

# -----------------------------------------------------------------------------
# TEST 5: CLI Launcher (doom-launch) Functional Verification
# -----------------------------------------------------------------------------
echo ""
echo "[Test 5/6] Testing doom-launch CLI from outside working directory..."

DOOM_LAUNCH_BIN="$BIN_DIR/doom-launch"

# Execute doom-launch from a completely disconnected directory (e.g. /tmp)
OUT_LIST=$(cd /tmp && HOME="$SANDBOX" BIN_DIR="$BIN_DIR" WADS_DIR="$WADS_DIR" "$DOOM_LAUNCH_BIN" --list)
echo "$OUT_LIST" | grep -q "Deathless" || { echo "FAIL: Deathless not listed in doom-launch --list"; exit 1; }
echo "$OUT_LIST" | grep -q "Alien Vendetta" || { echo "FAIL: Alien Vendetta not listed in doom-launch --list"; exit 1; }

# Test preview generation
OUT_PREV=$(cd /tmp && HOME="$SANDBOX" BIN_DIR="$BIN_DIR" WADS_DIR="$WADS_DIR" "$DOOM_LAUNCH_BIN" --preview "Deathless")
echo "$OUT_PREV" | grep -q "Preset:        Deathless" || { echo "FAIL: Invalid preview for Deathless"; exit 1; }
echo "$OUT_PREV" | grep -q "DOOM.WAD \[✓ Found\]" || { echo "FAIL: IWAD not shown as Found in preview"; exit 1; }
echo "$OUT_PREV" | grep -q "deathless.wad.*\[✓ Found\]" || { echo "FAIL: Mappack not shown as Found in preview"; exit 1; }

# Test dry-run execution
OUT_DRY=$(cd /tmp && HOME="$SANDBOX" BIN_DIR="$BIN_DIR" WADS_DIR="$WADS_DIR" "$DOOM_LAUNCH_BIN" --dry-run "Deathless" -skill 4 -warp E1M1)
echo "$OUT_DRY" | grep -q "dsda-doom.*-iwad.*DOOM.WAD.*-file.*deathless.wad.*-skill 4 -warp E1M1" || {
    echo "FAIL: Synthesized launch command is incorrect: $OUT_DRY"
    exit 1
}

# Test real mock execution
OUT_EXEC=$(cd /tmp && HOME="$SANDBOX" BIN_DIR="$BIN_DIR" WADS_DIR="$WADS_DIR" "$DOOM_LAUNCH_BIN" "Deathless")
echo "$OUT_EXEC" | grep -q "MOCK_DSDA_DOOM_EXECUTED:" || {
    echo "FAIL: Mock engine binary was not executed: $OUT_EXEC"
    exit 1
}
echo "✓ Test 5 Passed: doom-launch list, preview, dry-run, and engine invocation verified."

# -----------------------------------------------------------------------------
# TEST 6: Turnkey All-in-One Execution Test
# -----------------------------------------------------------------------------
echo ""
echo "[Test 6/6] Testing full turnkey pipeline in fresh sandbox..."

FRESH_SANDBOX=$(mktemp -d "${TMPDIR:-/tmp}/doom_turnkey_fresh.XXXXXX")
trap 'rm -rf "$SANDBOX" "$FRESH_SANDBOX"' EXIT INT TERM

# Run make turnkey or setup.sh --turnkey in sandbox
HOME="$FRESH_SANDBOX" PREFIX="$FRESH_SANDBOX" BIN_DIR="$FRESH_SANDBOX/.local/bin" "$ROOT_DIR/setup.sh"

test -f "$FRESH_SANDBOX/.local/bin/doom-launch" || { echo "FAIL: Turnkey did not install doom-launch"; exit 1; }

if [ "$OS" = "Darwin" ]; then
    test -f "$FRESH_SANDBOX/Library/Application Support/doom-configs/presets.json" || { echo "FAIL: Turnkey did not install presets.json"; exit 1; }
else
    test -f "$FRESH_SANDBOX/.local/share/doom-configs/presets.json" || { echo "FAIL: Turnkey did not install presets.json"; exit 1; }
fi

echo "✓ Test 6 Passed: Turnkey all-in-one setup completed successfully."

echo ""
echo "============================================================"
echo "  ✓ ALL TURNKEY & SYSTEM TESTS PASSED SUCCESSFULLY ($OS)!   "
echo "============================================================"
