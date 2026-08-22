#!/usr/bin/env bash
set -e

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$BIN_DIR"

echo "=== Doom Engines & Launcher Installer ==="
echo "Target directory: $BIN_DIR"
echo ""

# Helper to fetch latest asset URL from GitHub Releases API with fallback
get_latest_github_url() {
    local repo="$1"
    local pattern="$2"
    local fallback_url="$3"
    local url=""

    if command -v curl >/dev/null 2>&1; then
        url=$(curl -sL "https://api.github.com/repos/${repo}/releases/latest" | \
              grep "browser_download_url" | \
              grep -E "${pattern}" | \
              head -n 1 | \
              cut -d '"' -f 4 || true)
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

    if command -v curl >/dev/null 2>&1; then
        curl -L --progress-bar -o "$tmp_file" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --show-progress -O "$tmp_file" "$url"
    else
        echo "Error: Neither curl nor wget was found."
        rm -f "$tmp_file"
        return 1
    fi

    mv "$tmp_file" "$dest"
    chmod +x "$dest"
    echo "Successfully installed $name to $dest"
    echo ""
}

install_uzdoom() {
    local repo="UZDoom/UZDoom"
    local pattern="Linux-UZDoom-.*\.AppImage"
    local fallback="https://github.com/UZDoom/UZDoom/releases/download/4.14.3/Linux-UZDoom-4.14.3.AppImage"
    local url
    url=$(get_latest_github_url "$repo" "$pattern" "$fallback")
    download_binary "uzdoom" "$url"
}

install_dsda() {
    local repo="kraflab/dsda-doom"
    local pattern="dsda-doom-.*-linux-x86_64\.appimage"
    local fallback="https://github.com/kraflab/dsda-doom/releases/download/v0.29.4/dsda-doom-0.29.4-linux-x86_64.appimage"
    local url
    url=$(get_latest_github_url "$repo" "$pattern" "$fallback")
    download_binary "dsda-doom" "$url"
}

install_doomrunner() {
    local repo="Youda008/DoomRunner"
    local pattern="DoomRunner-.*-Linux-x86_64\.AppImage"
    local fallback="https://github.com/Youda008/DoomRunner/releases/download/v1.9.2/DoomRunner-1.9.2-Linux-x86_64.AppImage"
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
