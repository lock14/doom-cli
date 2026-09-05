#!/usr/bin/env bash
set -e

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$BIN_DIR"

OS="$(uname -s)"
ARCH="$(uname -m)"

echo "=== Doom Engines & Launcher Installer ==="
echo "Platform: $OS ($ARCH)"
echo "Target directory: $BIN_DIR"
echo ""

# Helper to fetch latest asset URL from GitHub Releases API with fallback
get_latest_github_url() {
    local repo="$1"
    local pattern="$2"
    local fallback_url="$3"
    local url=""

    if command -v curl >/dev/null 2>&1; then
        local release_json
        release_json=$(curl -sL "https://api.github.com/repos/${repo}/releases/latest" 2>/dev/null || true)
        if [ -n "$release_json" ]; then
            url=$(python3 -c "
import json, re, sys
try:
    data = json.loads(sys.argv[1])
    pattern = re.compile(sys.argv[2], re.IGNORECASE)
    for asset in data.get('assets', []):
        name = asset.get('name', '')
        if pattern.search(name) and not name.endswith('.zsync') and not name.endswith('.asc') and not name.endswith('.sig'):
            print(asset.get('browser_download_url', ''))
            break
except Exception:
    pass
" "$release_json" "$pattern" 2>/dev/null || true)
        fi
    fi

    if [ -z "$url" ]; then
        url="$fallback_url"
    fi
    echo "$url"
}

download_binary() {
    local name="$1"
    local url="$2"
    local dest="$BIN_DIR/$name"

    if [ -z "$url" ]; then
        echo "Error: Could not resolve download URL for $name."
        return 1
    fi

    echo "Downloading $name from:"
    echo "  $url"

    local tmp_file
    tmp_file=$(mktemp "${TMPDIR:-/tmp}/doom_dl.XXXXXX")
    trap 'rm -f "$tmp_file"' EXIT INT TERM

    if command -v curl >/dev/null 2>&1; then
        curl -f -L --progress-bar -o "$tmp_file" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --show-progress -O "$tmp_file" "$url"
    else
        echo "Error: Neither curl nor wget was found."
        rm -f "$tmp_file"
        return 1
    fi

    if [[ "$url" == *.zip ]]; then
        if ! command -v unzip >/dev/null 2>&1; then
            echo "Error: 'unzip' utility is required to extract zip archives but was not found in PATH." >&2
            echo "Please install 'unzip' (e.g. 'sudo apt install unzip' or 'brew install unzip')." >&2
            rm -f "$tmp_file"
            return 1
        fi
        local extract_dir
        extract_dir=$(mktemp -d "${TMPDIR:-/tmp}/doom_zip.XXXXXX")
        trap 'rm -rf "$extract_dir" "$tmp_file"' EXIT INT TERM
        unzip -q -o "$tmp_file" -d "$extract_dir"
        local bin_match
        bin_match=$(find "$extract_dir" -type f -perm -111 -name "$name*" | head -n 1)
        if [ -n "$bin_match" ]; then
            cp "$bin_match" "$dest"
            local bin_parent
            bin_parent=$(dirname "$bin_match")
            # Also copy companion resource files and libraries if present
            find "$bin_parent" -maxdepth 1 -name "*.wad" -exec cp {} "$BIN_DIR/" \; 2>/dev/null || true
            find "$bin_parent" -maxdepth 1 -type d -name "libs*" -exec cp -r {} "$BIN_DIR/" \; 2>/dev/null || true
        else
            cp -r "$extract_dir"/* "$BIN_DIR/"
        fi
        rm -rf "$extract_dir" "$tmp_file"
        trap - EXIT INT TERM
    elif [[ "$url" == *.dmg ]] && [ "$OS" = "Darwin" ]; then
        local mount_point
        mount_point=$(mktemp -d "${TMPDIR:-/tmp}/doom_dmg.XXXXXX")
        trap 'hdiutil detach "$mount_point" -quiet 2>/dev/null || true; rm -rf "$mount_point" "$tmp_file"' EXIT INT TERM
        hdiutil attach "$tmp_file" -mountpoint "$mount_point" -nobrowse -quiet
        local app_match
        app_match=$(find "$mount_point" -maxdepth 2 -name "*.app" | head -n 1)
        if [ -n "$app_match" ]; then
            local apps_dir="$HOME/Applications"
            mkdir -p "$apps_dir"
            local app_dest
            app_dest="$apps_dir/$(basename "$app_match")"
            rm -rf "$app_dest"
            cp -R "$app_match" "$apps_dir/"
            local app_bin
            app_bin=$(find "$app_dest/Contents/MacOS" -type f -perm -111 | head -n 1)
            if [ -n "$app_bin" ]; then
                # Create a wrapper launcher in BIN_DIR pointing into the installed .app bundle
                cat << EOF > "$dest"
#!/bin/sh
exec "$app_bin" "\$@"
EOF
            fi
        fi
        hdiutil detach "$mount_point" -quiet || true
        rm -rf "$mount_point" "$tmp_file"
        trap - EXIT INT TERM
    else
        mv "$tmp_file" "$dest"
        trap - EXIT INT TERM
    fi

    chmod +x "$dest" 2>/dev/null || true
    echo "Successfully installed $name to $dest"
    echo ""
}

install_uzdoom() {
    local repo="UZDoom/UZDoom"
    local pattern
    local fallback
    if [ "$OS" = "Darwin" ]; then
        if [ "$ARCH" = "arm64" ]; then
            pattern=".*(macos|mac|darwin).*uzdoom.*arm64.*(\.dmg|\.zip)"
            fallback="https://github.com/UZDoom/UZDoom/releases/download/5.0.0/MacOS-UZDoom-Release-ARM64.dmg"
        else
            pattern=".*(macos|mac|darwin).*uzdoom.*(x86_64|x64|intel).*(\.dmg|\.zip)"
            fallback="https://github.com/UZDoom/UZDoom/releases/download/5.0.0/MacOS-UZDoom-Release-ARM64.dmg"
        fi
    else
        pattern="linux.*uzdoom.*\.appimage"
        fallback="https://github.com/UZDoom/UZDoom/releases/download/5.0.0/Linux-UZDoom-Release-x86_64.AppImage"
    fi
    local url
    url=$(get_latest_github_url "$repo" "$pattern" "$fallback")
    download_binary "uzdoom" "$url"
}

install_dsda() {
    local repo="kraflab/dsda-doom"
    local pattern
    local fallback
    if [ "$OS" = "Darwin" ]; then
        if [ "$ARCH" = "arm64" ]; then
            pattern="dsda-doom-.*-mac-arm64\.zip"
            fallback="https://github.com/kraflab/dsda-doom/releases/download/v0.29.4/dsda-doom-0.29.4-mac-arm64.zip"
        else
            pattern="dsda-doom-.*-mac-x86_64\.zip"
            fallback="https://github.com/kraflab/dsda-doom/releases/download/v0.29.4/dsda-doom-0.29.4-mac-x86_64.zip"
        fi
    else
        pattern="dsda-doom-.*-linux-x86_64\.appimage"
        fallback="https://github.com/kraflab/dsda-doom/releases/download/v0.29.4/dsda-doom-0.29.4-linux-x86_64.appimage"
    fi
    local url
    url=$(get_latest_github_url "$repo" "$pattern" "$fallback")
    download_binary "dsda-doom" "$url"
}

install_doomrunner() {
    local repo="Youda008/DoomRunner"
    local pattern
    local fallback
    if [ "$OS" = "Darwin" ]; then
        if [ "$ARCH" = "arm64" ]; then
            pattern="DoomRunner-.*-MacOS-arm64\.dmg"
            fallback="https://github.com/Youda008/DoomRunner/releases/download/v1.9.2/DoomRunner-1.9.2-MacOS-arm64.dmg"
        else
            pattern="DoomRunner-.*-MacOS-x86_64\.dmg"
            fallback="https://github.com/Youda008/DoomRunner/releases/download/v1.9.2/DoomRunner-1.9.2-MacOS-x86_64.dmg"
        fi
    else
        pattern="DoomRunner-.*-Linux-x86_64\.AppImage"
        fallback="https://github.com/Youda008/DoomRunner/releases/download/v1.9.2/DoomRunner-1.9.2-Linux-x86_64.AppImage"
    fi
    local url
    url=$(get_latest_github_url "$repo" "$pattern" "$fallback")
    download_binary "doomrunner" "$url"
}

TARGET="${1:-all}"

case "$TARGET" in
    uzdoom)
        install_uzdoom
        ;;
    dsda|dsda-doom)
        install_dsda
        ;;
    doomrunner)
        install_doomrunner
        ;;
    all)
        install_uzdoom
        install_dsda
        install_doomrunner
        ;;
    *)
        echo "Usage: $0 [all|uzdoom|dsda|doomrunner]"
        exit 1
        ;;
esac

echo "Done! Make sure '$BIN_DIR' is in your system PATH."
