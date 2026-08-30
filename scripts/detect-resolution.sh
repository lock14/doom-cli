#!/usr/bin/env bash
# detect-resolution.sh - Display Resolution Auto-Detector
# Detects the primary monitor resolution on Linux and macOS, with fallback to 1920x1080.

set -e

OS="$(uname -s)"
DETECTED_RES=""

if [ "$OS" = "Darwin" ]; then
    if command -v system_profiler >/dev/null 2>&1; then
        DETECTED_RES=$(system_profiler SPDisplaysDataType 2>/dev/null | grep -E 'Resolution:' | head -n 1 | awk '{print $2 "x" $4}' || true)
    fi
else
    # Try xrandr on X11 / Wayland (Xwayland)
    if command -v xrandr >/dev/null 2>&1; then
        DETECTED_RES=$(xrandr 2>/dev/null | grep -E '\*' | head -n 1 | awk '{print $1}' || true)
    fi

    # Try wlr-randr on Wayland
    if [ -z "$DETECTED_RES" ] && command -v wlr-randr >/dev/null 2>&1; then
        DETECTED_RES=$(wlr-randr 2>/dev/null | grep -E 'current' | head -n 1 | awk '{print $1}' || true)
    fi

    # Try sysfs DRM modes on Linux
    if [ -z "$DETECTED_RES" ]; then
        for mode_file in /sys/class/drm/*/modes; do
            if [ -f "$mode_file" ]; then
                DETECTED_RES=$(head -n 1 "$mode_file" 2>/dev/null || true)
                [ -n "$DETECTED_RES" ] && break
            fi
        done
    fi
fi

# Fallback default if resolution could not be determined (headless / CI)
if [ -z "$DETECTED_RES" ]; then
    DETECTED_RES="1920x1080"
fi

echo "$DETECTED_RES"
