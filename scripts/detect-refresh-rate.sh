#!/usr/bin/env bash
# detect-refresh-rate.sh - Monitor Refresh Rate Auto-Detector
# Detects the primary monitor refresh rate (in Hz) on Linux and macOS, with fallback to 60.

set -e

OS="$(uname -s)"
DETECTED_RATE=""

if [ "$OS" = "Darwin" ]; then
    if command -v system_profiler >/dev/null 2>&1; then
        DETECTED_RATE=$(system_profiler SPDisplaysDataType 2>/dev/null | grep -E -i 'refresh rate|hertz|@ [0-9]+' | head -n 1 | grep -o -E '[0-9]+' | head -n 1 || true)
    fi
else
    # Try xrandr on X11 / Wayland (Xwayland)
    if command -v xrandr >/dev/null 2>&1; then
        DETECTED_RATE=$(xrandr 2>/dev/null | grep -E '\*' | head -n 1 | awk '{for(i=1;i<=NF;i++) if ($i ~ /\*/) {gsub(/[*+]/, "", $i); print int($i + 0.5)}}' || true)
    fi

    # Try wlr-randr on Wayland
    if [ -z "$DETECTED_RATE" ] && command -v wlr-randr >/dev/null 2>&1; then
        DETECTED_RATE=$(wlr-randr 2>/dev/null | grep -E 'current' | head -n 1 | grep -o -E '[0-9]+(\.[0-9]+)? Hz' | awk '{print int($1 + 0.5)}' || true)
    fi

    # Try kscreen-doctor on KDE Plasma Wayland
    if [ -z "$DETECTED_RATE" ] && command -v kscreen-doctor >/dev/null 2>&1; then
        DETECTED_RATE=$(kscreen-doctor -o 2>/dev/null | grep -E -o '@[0-9]+' | head -n 1 | tr -d '@' || true)
    fi
fi

# Fallback default if refresh rate could not be determined (headless / CI / virtual)
if [ -z "$DETECTED_RATE" ] || [ "$DETECTED_RATE" -le 0 ] 2>/dev/null; then
    DETECTED_RATE="60"
fi

echo "$DETECTED_RATE"
