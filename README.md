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
- **`doom-launch`**: Lightweight, lightning-fast terminal UI / CLI launcher with interactive fuzzy search (`fzf`), real-time metadata previews, and direct CLI launching.
- **[DoomRunner](https://github.com/Youda008/DoomRunner)**: Modern graphical launcher preloaded with 32 community megawad presets mapped to their ideal engines.

---

## Installation & Setup

### Linux & macOS

The `Makefile` automatically detects whether you are running **Linux** or **macOS (Darwin)** and maps configuration directories accordingly:
- **Linux Destinations**: `~/.config/uzdoom/`, `~/.local/share/dsda-doom/`, `~/.local/share/DoomRunner/`, `~/.local/share/doom-configs/`, `~/.local/bin/`
- **macOS Destinations**: `~/Library/Application Support/uzdoom/`, `~/Library/Application Support/dsda-doom/`, `~/Library/Application Support/DoomRunner/`, `~/Library/Application Support/doom-configs/`, `~/.local/bin/`

#### Option 1: ⚡ Turnkey Setup (One-Command Everything)
For players who just want everything ready to play immediately with zero manual steps:
Downloads all engines (`uzdoom`, `dsda-doom`, `doomrunner`), installs all configurations & `doom-launch`, deploys the Roland SC-55 MIDI SoundFont, auto-extracts official Steam/GOG IWADs, and downloads all 20+ free community megawads:

```bash
make turnkey
```

*(Or without `make`: `./setup.sh --turnkey`)*

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
- `make install-uzdoom` — Deploys `autoexec.cfg`
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

To install the preconfigured DoomRunner presets on Windows via PowerShell:

```powershell
.\setup.ps1
```

If your WADs or engines are on a drive other than `E:`, pass the `-BaseDrive` parameter:
```powershell
.\setup.ps1 -BaseDrive "D:"
```

This deploys `DoomRunner/windows/options.json` to `%LOCALAPPDATA%\DoomRunner\options.json` and `%APPDATA%\DoomRunner\options.json`, creating timestamped backups beforehand.

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
- **Windows Default**: `E:\Doom WADS\` (or your customized `-BaseDrive` path)

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

### 1. Interactive Terminal Launcher (`doom-launch`)

Launch presets from anywhere in your terminal using `doom-launch` (or `make play`):

```bash
# Interactive fuzzy-finder menu (requires fzf) or numbered menu
doom-launch

# Or launch via Makefile
make play
```

#### Direct CLI Launching & Custom Engine Flags
You can launch presets directly by name and pass additional engine arguments on the fly:

```bash
# Launch a specific preset
doom-launch "Eviternity II"

# Override default engine (e.g. run MBF mapset in DSDA-Doom)
doom-launch "Ancient Aliens" -e dsda-doom

# Launch with custom skill and warp flags
doom-launch "Sunlust" -skill 4 -warp 01

# Inspect synthesized launch command without starting the game
doom-launch --dry-run "Alien Vendetta"

# List all available presets and their mapped engines
doom-launch --list
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
| **Doom / Doom II** | DSDA-Doom | Classic IWADs with modern MIDI audio (`idkfa 2024.wad`) |
| **Alien Vendetta** | DSDA-Doom | Classic Boom megawad + DEH patch |
| **Ancient Aliens** | UZDoom | MBF / Complevel 11 with custom color palette |
| **Back to Saturn X (Ep 1 & 2)** | DSDA-Doom | Vanilla/Boom compatible with custom soundtrack & palettes |
| **Deathless** | DSDA-Doom | Modern Ultimate Doom episode replacement |
| **Doom Zero** | DSDA-Doom | Anniversary megawad + DEH modifications |
| **Doom / Doom II: The Way ID Did** | DSDA-Doom | Classic vanilla-style homage megawads |
| **Eviternity I & II** | UZDoom | OTEX texture pack, advanced MBF21 & custom monsters |
| **Going Down Turbo** | DSDA-Doom | Fast-paced, compact map pack |
| **Heretic / Hexen** | UZDoom | Raven Software classics |
| **Legacy of Rust** | DSDA-Doom | Official ID24 standard episode + weapons & monsters |
| **Master Levels for Doom II** | DSDA-Doom | Full 20-level classic collection |
| **No End In Sight / NRFTL** | DSDA-Doom | Classic episode expansions |
| **Nostalgia** | DSDA-Doom | Vanilla-compatible megawad |
| **Plutonia 2 / TNT: Revilution** | DSDA-Doom | Community sequels to Final Doom |
| **Scythe 1 & 2 / Speed of Doom** | DSDA-Doom | Iconic speedrunning & challenge megawads |
| **Sigil I & II** | DSDA-Doom | John Romero's unofficial 5th & 6th episodes |
| **Sunder / Sunlust** | DSDA-Doom | Benchmark slaughter & challenge mapsets |
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
- **Video & Display**: OpenGL mode with integer scaling, 60 FPS limiter, and uncapped framerate.
- **Extended HUD (exHUD)**: In-game level splits, secret counters, and completion times.
- **Built-in Capture**: Ready-to-use `ffmpeg` video recording commands.

---

## Makefile Quick Reference

| Category | Target | Description |
| :--- | :--- | :--- |
| **🚀 Quick Start** | `make turnkey` | ⚡ **1-Step Setup:** Downloads engines, configs, SoundFont, IWADs & megawads |
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
