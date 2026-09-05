# Doom Configs

[![CI](https://github.com/lock14/doom-configs/actions/workflows/ci.yml/badge.svg)](https://github.com/lock14/doom-configs/actions/workflows/ci.yml)
![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-blue)
![Engines](https://img.shields.io/badge/engines-DSDA--Doom%20%7C%20UZDoom-red)
![Presets](https://img.shields.io/badge/presets-32%20megawads-orange)

A curated collection of configuration files, launcher presets, build targets, and CLI automation tools for playing Doom using modern source ports (**[DSDA-Doom](https://github.com/kraflab/dsda-doom)** and **[UZDoom](https://github.com/UZDoom/uzdoom)**) with both the **[DoomRunner](https://github.com/Youda008/DoomRunner)** graphical launcher and the lightweight **`doom-launch`** terminal launcher across Linux, macOS, and Windows.

---

## Supported Source Ports & Launchers

- **[DSDA-Doom](https://github.com/kraflab/dsda-doom)**: Precision speedrunning and demo-accurate source port preconfigured for MBF21, extended HUD, dynamic resolution scaling, and uncapped framerates.
- **[UZDoom](https://github.com/UZDoom/uzdoom)**: Modern ZDoom-based engine configured with a Nightdive "Software-Plus" visual profile, auto-detecting aspect ratio, and broad mod compatibility.
- **`doom` CLI**: Unified, zero-dependency cross-platform CLI tool and interactive terminal launcher with fuzzy search, real-time metadata previews, Steam/GOG auto-discovery, multi-mirror downloading, and configuration management on Linux, macOS, and Windows.
- **`doom-launch`**: Lightweight POSIX terminal launcher wrapper with fuzzy search (`fzf`) and numbered menu fallback.
- **[DoomRunner](https://github.com/Youda008/DoomRunner)**: Modern graphical launcher preloaded with 32 community megawad presets mapped to their ideal engines.

---

## Installation & Setup

### Linux & macOS

The `Makefile` automatically detects whether you are running **Linux** or **macOS (Darwin)** and maps configuration directories accordingly:
- **Linux Destinations**: `~/.config/uzdoom/`, `~/.local/share/dsda-doom/`, `~/.local/share/DoomRunner/`, `~/.local/share/doom-configs/`, `~/.local/bin/`
- **macOS Destinations**: `~/Library/Application Support/uzdoom/`, `~/Library/Application Support/dsda-doom/`, `~/Library/Application Support/DoomRunner/`, `~/Library/Application Support/doom-configs/`, `~/.local/bin/`

#### Option 1: ⚡ Automated Setup (One-Command Everything)
For players who just want everything ready to play immediately with zero manual steps:
Downloads all engines (`uzdoom`, `dsda-doom`, `doomrunner`), installs all configurations & `doom`, deploys the Roland SC-55 MIDI SoundFont, auto-extracts official Steam/GOG IWADs, and downloads all 20+ free community megawads:

```bash
# Using native doom CLI:
doom setup

# Or using Make / POSIX script:
make setup
# or: ./setup.sh --all
```

> [!TIP]
> Ensure `~/.local/bin` is in your `$PATH` (e.g. in your `~/.bashrc` or `~/.zshrc`):
> ```bash
> export PATH="$HOME/.local/bin:$PATH"
> ```

#### Option 2: Bootstrap (Engines + Configs + Launcher)
To download the engine binaries into `~/.local/bin/` and deploy all configurations and `doom-launch`:

```bash
make bootstrap
```

#### Option 3: Configs Only
To deploy only configuration files (with automatic `.bak` backups of existing configs):

```bash
make install
```

Or install individual components:
- `make install-uzdoom` — Deploys `autoexec.cfg` (auto-detects display refresh rate)
- `make install-dsda` — Deploys `dsda-doom.cfg` (auto-detects display resolution)
- `make install-doomrunner` — Deploys `options.json`
- `make install-launcher` — Installs `doom-launch` to `~/.local/bin/`
- `make install-engines` — Downloads only the engine binaries into `~/.local/bin/`

#### Managing In-Game Changes
- `make diff` — Compare current repository configs against live configs on your system.
- `make sync` — Pull changes made in-game back into the repository so you can commit them.

#### Standalone Shell Script
Alternatively, deploy without `make` using the POSIX setup script:
```bash
./setup.sh
```

---

### Windows

On Windows, you can use the native `doom.exe` CLI for complete automated setup and interactive launching, or deploy DoomRunner via PowerShell:

#### Option 1: Native Automated Setup (`doom.exe`)
```powershell
# Complete 1-step setup (engines, configs, soundfonts, steam IWADs, and megawads)
.\doom.exe setup

# Interactive fuzzy launcher directly in Windows Terminal / PowerShell
.\doom.exe play
```

#### Option 2: DoomRunner Deployment via PowerShell
To install the preconfigured DoomRunner presets on Windows via PowerShell:

```powershell
.\setup.ps1
```

`setup.ps1` automatically detects the drive letter where the setup script is executed from and defaults all engines and WADs to `<Drive>:\Doom WADS`.

If your WADs or engines are on a different drive, pass the `-BaseDrive` parameter (or `-WadsDir` for a custom folder):
```powershell
.\setup.ps1 -BaseDrive "D:"
# Or customize full path:
.\setup.ps1 -WadsDir "D:\Games\Doom WADS"
```

This deploys `DoomRunner/windows/options.json` to `%LOCALAPPDATA%\DoomRunner\options.json` and `%APPDATA%\DoomRunner\options.json`, creating timestamped backups beforehand. If Go is available, it also compiles `doom.exe` into `%LOCALAPPDATA%\Programs\Doom\bin\`.

---

## Game Files & Content Setup

> [!IMPORTANT]
> **Commercial Game Files Are Not Tracked in Git**
> 
> This repository contains configuration files, launcher presets, and automated fetch scripts. You must provide your own legally acquired game files:
> - **Commercial IWADs** (`DOOM.WAD`, `DOOM2.WAD`, `PLUTONIA.WAD`, `TNT.WAD`, `HERETIC.WAD`, `HEXEN.WAD`) can be acquired from digital storefronts:
>   - **Doom + Doom II**: [Steam](https://store.steampowered.com/app/2280/DOOM_DOOM_II/) / [GOG](https://www.gog.com/en/game/doom_doom_ii)
>   - **Heretic + Hexen**: [Steam](https://store.steampowered.com/app/3286930/Heretic__Hexen/) / [GOG](https://www.gog.com/en/game/heretic_hexen)
>   *(Tip: Use `make extract-iwads` to auto-import them from Steam/GOG).*
> - **Community Megawads & Expansions** (*Ancient Aliens*, *Eviternity I & II*, *Back to Saturn X*, *Sunlust*, *Sunder*, etc.) can be fetched automatically via `make fetch-wads` or manually downloaded from **[Doomworld / idgames](https://www.doomworld.com/idgames/)**.

Place your game files (`.wad`, `.deh`, `.pk3`) in the standard directory:
- **Linux Default**: `~/.local/share/games/uzdoom/`
- **macOS Default**: `~/Library/Application Support/games/uzdoom/`
- **Windows Default**: `<Drive>:\Doom WADS\` (defaults to the execution drive, e.g. `C:\Doom WADS\` or `E:\Doom WADS\`, or customized via `-BaseDrive`)

### 1. Steam & GOG IWAD Auto-Extractor
If you own *Doom + Doom II (2024)* or *Heretic / Hexen* on Steam or GOG, you can automatically discover and copy official IWADs and the modern `idkfa 2024.wad` soundtrack into your WADs folder:

```bash
make extract-iwads
```

### 2. Community Megawad Auto-Downloader
Download and extract all 20+ free community megawads and DeHackEd patches (*Eviternity I & II*, *BTSX 1 & 2*, *Ancient Aliens*, *Sunder*, *Sunlust*, *Sigil I & II*, etc.) directly from idgames / Doomworld mirrors:

```bash
# Download all community megawads
make fetch-wads

# Or download a specific megawad via script
./scripts/fetch-wads.sh "Eviternity II"
```

### 3. Curated Roland SC-55 MIDI SoundFont
Download and deploy the balanced `GeneralUser-GS.sf2` SoundFont for high-definition FluidSynth MIDI playback:

```bash
make install-soundfonts
```

---

## Launching & Playing Games

### 1. Interactive Terminal Launcher (`doom` / `doom play`)

Launch presets from anywhere in your terminal using the native `doom` CLI (or `doom-launch` / `make play`):

```bash
# Interactive fuzzy search & live preview (Linux, macOS, AND Windows!)
doom play
# Or simply:
doom

# Or launch via Makefile / POSIX script
make play
# or: doom-launch
```

#### Direct CLI Launching & Custom Engine Flags
You can launch presets directly by name and pass additional engine arguments on the fly:

```bash
# Launch a specific preset
doom launch "Eviternity II"

# Override default engine (e.g. run MBF mapset in DSDA-Doom)
doom launch "Ancient Aliens" -e dsda-doom

# Launch with custom skill and warp flags
doom launch "Sunlust" -skill 4 -warp 01

# Inspect synthesized launch command without starting the game
doom launch "Alien Vendetta" --dry-run

# List all available presets and their mapped engines
doom presets list
```

### 2. Graphical Launcher (DoomRunner)

Launch DoomRunner from your application menu or terminal:

```bash
# Start the graphical launcher
doomrunner
```

DoomRunner will open with all 32 presets pre-configured with their matching engines, IWADs, load orders, and DeHackEd patches. Simply select a preset and click **Launch**.

---

## Preconfigured Presets

All presets are declaratively managed in [`data/presets.json`](data/presets.json) and mapped to their optimal engine:

| Megawad / Expansion | Engine | Compatibility / Details |
| :--- | :--- | :--- |
| **Alien Vendetta** | DSDA-Doom | Classic Boom megawad + DEH patch |
| **Ancient Aliens** | UZDoom | MBF / Complevel 11 with custom color palette |
| **Back to Saturn X: Episode 1** | DSDA-Doom | Vanilla/Boom compatible with custom soundtrack & palettes |
| **Back to Saturn X: Episode 2** | DSDA-Doom | Vanilla/Boom compatible with custom soundtrack & palettes |
| **Deathless** | DSDA-Doom | Modern Ultimate Doom episode replacement |
| **Doom** | DSDA-Doom | Classic IWAD with modern MIDI audio |
| **Doom II** | DSDA-Doom | Classic IWAD with modern MIDI audio |
| **Doom Zero** | DSDA-Doom | Anniversary megawad + DEH modifications |
| **Doom: The Way ID Did** | DSDA-Doom | Classic vanilla-style homage megawad |
| **Doom II: The Way ID Did** | DSDA-Doom | Classic vanilla-style homage megawad |
| **Eviternity** | UZDoom | OTEX texture pack & custom monsters |
| **Eviternity II** | UZDoom | OTEX texture pack, advanced MBF21 & custom monsters |
| **Going Down Turbo** | DSDA-Doom | Fast-paced, compact map pack |
| **Heretic** | UZDoom | Raven Software classic fantasy shooter |
| **Hexen** | UZDoom | Raven Software hub-based fantasy shooter |
| **Legacy of Rust** | DSDA-Doom | Official ID24 standard episode + weapons & monsters |
| **Master Levels** | DSDA-Doom | Full 20-level classic collection |
| **Nostalgia** | DSDA-Doom | Vanilla-compatible megawad |
| **No End In Sight** | DSDA-Doom | Classic 4-episode expansion for Ultimate Doom |
| **No Rest for the Living** | DSDA-Doom | Official 9-level expansion for Doom II |
| **Sigil** | DSDA-Doom | John Romero's unofficial 5th episode for Ultimate Doom |
| **Sigil II** | DSDA-Doom | John Romero's unofficial 6th episode for Ultimate Doom |
| **Scythe** | DSDA-Doom | Iconic fast-paced speedrunning megawad |
| **Scythe 2** | DSDA-Doom | Erik Alm's legendary sequel with custom monsters |
| **Speed of Doom** | DSDA-Doom | 33-level intense challenge megawad |
| **Sunder** | DSDA-Doom | Monumental architectural slaughter megawad |
| **Sunlust** | DSDA-Doom | 32-level visual and gameplay masterpiece |
| **The Plutonia Experiment** | DSDA-Doom | Classic Final Doom IWAD with modern soundtrack |
| **Plutonia 2** | DSDA-Doom | Community sequel to The Plutonia Experiment |
| **TNT: Evilution** | DSDA-Doom | Classic Final Doom IWAD with modern soundtrack |
| **TNT: Revilution** | DSDA-Doom | Community sequel to TNT: Evilution |
| **Valiant** | UZDoom | MBF-compatible with custom weapons & dehacked |

---

## Engine Profiles & Customizations

### UZDoom ("Software-Plus" Profile)
Configured in [`uzdoom/autoexec.cfg`](uzdoom/autoexec.cfg) to reproduce the crisp Nightdive Remaster aesthetic:
- **Software Sector Lighting**: `gl_lightmode 0` with light stepping (`gl_bandedsw 1`).
- **Tonemapping**: 256-color palette tonemapping (`gl_tonemap 3`).
- **Nearest-Neighbor Filtering**: Crisp, unblurred pixels (`gl_texture_filter 0`) with 16x anisotropic filtering (`gl_texture_filter_aniso 16`).
- **Remaster HUD & Crosshair**: Minimalist widescreen alternate HUD and classic Nightdive green crosshair (`#00FF00`).
- **Quality of Life**: High-definition 48kHz audio sampling, FluidSynth MIDI backend, and `F5`/`F9` fast save & load.

### DSDA-Doom (Speedrunning & Demo Accuracy)
Configured in [`dsda-doom/dsda-doom.cfg`](dsda-doom/dsda-doom.cfg):
- **Compatibility**: Default `complevel 21` (MBF21 standard).
- **Video & Display**: OpenGL mode with integer scaling, VSync frame pacing, and uncapped high-refresh support (no artificial 60 FPS cap).
- **Extended HUD (exHUD)**: In-game level splits, secret counters, and completion times.
- **Built-in Capture**: Ready-to-use `ffmpeg` video recording commands.

---

## Makefile Quick Reference

| Category | Target | Description |
| :--- | :--- | :--- |
| **🚀 Quick Start** | `make setup` | ⚡ **1-Step Setup:** Downloads engines, configs, SoundFont, IWADs & megawads |
| | `make bootstrap` | Downloads engine binaries & deploys configs + launcher |
| | `make install` | Deploys all configuration files & `doom-launch` CLI with backups |
| | `make play` | Opens interactive terminal preset launcher (`fzf` or numbered menu) |
| **📦 Content & Assets** | `make fetch-wads` | Auto-downloads and unpacks 20+ free community megawads |
| | `make extract-iwads` | Auto-discovers and copies official IWADs from Steam / GOG |
| | `make install-soundfonts` | Deploys curated GeneralUser GS SoundFont for FluidSynth |
| | `make install-engines` | Downloads latest binaries (UZDoom, DSDA-Doom, DoomRunner) to `~/.local/bin/` |
| **🔧 Maintenance** | `make diff` | Shows diff between repository configs and live system configs |
| | `make sync` | Syncs in-game configuration tweaks back into the git repository |
| | `make check` | Runs full validation suite (presets, scripts, invariants, tests) |
| | `make build-presets` | Compiles `data/presets.json` into launcher `options.json` files |

---

## Installation Recommendations (Linux)

When installing Doom tools on Linux, using **AppImages** or native binaries is strongly recommended over Flatpaks. 

Flatpaks run inside a sandbox that isolates them from the rest of the filesystem, preventing launchers (like DoomRunner or `doom-launch`) from easily discovering game engines (like UZDoom or DSDA-Doom) or accessing WADs across custom paths without manual permission overrides.

- Run `make bootstrap` or `make install-engines` to place portable AppImages in `~/.local/bin/`.
- Alternatively, download binaries directly from each project's GitHub Releases page.
