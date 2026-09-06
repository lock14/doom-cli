# doom-cli

[![Go Version](https://img.shields.io/github/go-mod/go-version/lock14/doom-cli)](https://go.dev/)
[![CI](https://github.com/lock14/doom-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/lock14/doom-cli/actions/workflows/ci.yml)
[![Security](https://github.com/lock14/doom-cli/actions/workflows/security.yml/badge.svg)](https://github.com/lock14/doom-cli/actions/workflows/security.yml)
![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-blue)
![Engines](https://img.shields.io/badge/engines-DSDA--Doom%20%7C%20UZDoom-red)
![Presets](https://img.shields.io/badge/presets-32%20megawads-orange)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A unified cross-platform CLI tool and curated collection of configurations and launcher presets for classic Doom source ports (**[DSDA-Doom](https://github.com/kraflab/dsda-doom)** and **[UZDoom](https://github.com/UZDoom/uzdoom)**) and the **[DoomRunner](https://github.com/Youda008/DoomRunner)** graphical launcher across Linux, macOS, and Windows.

---

## Supported Source Ports & Launchers

- **`doom` CLI**: Unified, zero-dependency cross-platform CLI tool and interactive terminal launcher with fuzzy search, real-time metadata previews, Steam/GOG auto-discovery, multi-mirror downloading, and configuration management on Linux, macOS, and Windows.
- **[DSDA-Doom](https://github.com/kraflab/dsda-doom)**: Precision speedrunning and demo-accurate source port preconfigured for MBF21, extended HUD, dynamic resolution scaling, and uncapped framerates.
- **[UZDoom](https://github.com/UZDoom/uzdoom)**: Modern ZDoom-based engine configured with a Nightdive "Software-Plus" visual profile, auto-detecting aspect ratio, and broad mod compatibility.
- **[DoomRunner](https://github.com/Youda008/DoomRunner)**: Modern graphical launcher preloaded with 32 community megawad presets mapped to their ideal engines.

---

## Installation & Setup

### 1. Install the `doom` CLI

#### Linux & macOS
```bash
# Clone the repository and build/install the doom binary to ~/.local/bin/doom
git clone https://github.com/lock14/doom-cli.git
cd doom-cli
make install
```
*(Alternatively, install via Go: `go install github.com/lock14/doom-cli/cmd/doom@latest`)*

> [!TIP]
> Ensure `~/.local/bin` is in your `$PATH` (e.g. in your `~/.bashrc` or `~/.zshrc`):
> ```bash
> export PATH="$HOME/.local/bin:$PATH"
> ```

#### Windows
In PowerShell or Windows Terminal:
```powershell
# Clone the repository and build doom.exe
git clone https://github.com/lock14/doom-cli.git
cd doom-cli
go install ./cmd/doom
# Or compile directly to a bin folder in your PATH:
go build -o $HOME/bin/doom.exe ./cmd/doom
```

---

### 2. ⚡ Automated Setup (`doom setup`)

For players who want everything ready to play immediately in a single step:

```bash
doom setup
```

`doom setup` automatically:
1. **Installs Game Engines**: Downloads portable binaries for UZDoom, DSDA-Doom, and DoomRunner into your local bin path.
2. **Deploys Engine Configurations**: Installs optimized configs for UZDoom and DSDA-Doom, auto-detecting your display resolution and native monitor refresh rate, while creating timestamped backups (`.bak.<timestamp>`) of any existing files.
3. **Deploys MIDI SoundFont**: Downloads and installs the curated Roland SC-55 SoundFont (`GeneralUser-GS.sf2`) for FluidSynth MIDI playback.
4. **Extracts Official Game Files**: Scans all Steam and GOG library folders across drives for *Doom + Doom II (2024)*, *Heretic*, and *Hexen*, copying commercial IWADs and the modern `idkfa 2024.wad` soundtrack into your WADs directory.
5. **Downloads Community Megawads**: Fetches and extracts all 20+ free community megawads (*Eviternity I & II*, *Back to Saturn X 1 & 2*, *Ancient Aliens*, *Sunder*, *Sunlust*, *Sigil I & II*, etc.) and DeHackEd patches from idgames/Doomworld mirrors.

---

### 3. Modular Management Commands

If you prefer to perform individual tasks or maintain your setup over time:

#### Content & Assets
- `doom engines install` — Downloads portable engine binaries (`uzdoom`, `dsda-doom`, `doomrunner`).
- `doom soundfont install` — Downloads and deploys the Roland SC-55 SoundFont.
- `doom wads extract` — Auto-discovers and imports official Steam and GOG IWADs.
- `doom wads fetch` — Downloads and extracts all community megawads.
- `doom wads fetch "Eviternity II"` — Downloads a specific megawad by name.

#### Engine Configurations
- `doom config install` — Deploys engine configurations and DoomRunner options with display auto-detection and timestamped backups.
- `doom config diff` — Shows a diff comparing repository config templates against live configs on your system.
- `doom config sync` — Pulls in-game tweaks (keybindings, sensitivity, video settings) from your system back into the repository so you can commit them.

#### Visual Themes & Styling
- `doom themes` / `doom themes list` — Lists available color themes with visual ANSI swatches and indicates active theme.
- `doom themes set <theme>` — Sets your persistent default launcher theme in user configuration.

---

## Directory Layouts

The `doom` CLI automatically respects standard, platform-idiomatic paths:

| Component | Linux (XDG) | macOS | Windows |
| :--- | :--- | :--- | :--- |
| **Binaries** | `~/.local/bin/` | `~/.local/bin/` | `%LOCALAPPDATA%\Programs\Doom\bin\` |
| **WADs & Mods** | `~/.local/share/games/uzdoom/` | `~/Library/Application Support/games/uzdoom/` | `<Drive>:\Doom WADS\` (or `%LOCALAPPDATA%\Doom WADS\`) |
| **UZDoom Config** | `~/.config/uzdoom/autoexec.cfg` | `~/Library/Application Support/uzdoom/autoexec.cfg` | `%APPDATA%\uzdoom\autoexec.cfg` |
| **DSDA-Doom Config** | `~/.local/share/dsda-doom/dsda-doom.cfg` | `~/Library/Application Support/dsda-doom/dsda-doom.cfg` | `%LOCALAPPDATA%\dsda-doom\dsda-doom.cfg` |
| **DoomRunner Options** | `~/.local/share/DoomRunner/options.json` | `~/Library/Application Support/DoomRunner/options.json` | `%LOCALAPPDATA%\DoomRunner\options.json` |
| **CLI Config & Themes** | `~/.config/doom-cli/` | `~/Library/Application Support/doom-cli/` | `%LOCALAPPDATA%\doom-cli\` |
| **SoundFonts** | `~/.local/share/soundfonts/` | `~/Library/Application Support/soundfonts/` | `%LOCALAPPDATA%\soundfonts\` |

*(Custom WAD directories can be passed to any command via `--wads-dir <path>` or set via the `DOOM_WADS_DIR` environment variable).*

---

## Game Files & Content Setup

> [!IMPORTANT]
> **Commercial Game Files Are Not Tracked in Git**
> 
> This repository contains configuration files, launcher presets, and automated fetch tools. You must provide your own legally acquired game files:
> - **Commercial IWADs** (`DOOM.WAD`, `DOOM2.WAD`, `PLUTONIA.WAD`, `TNT.WAD`, `HERETIC.WAD`, `HEXEN.WAD`) can be acquired from digital storefronts:
>   - **Doom + Doom II**: [Steam](https://store.steampowered.com/app/2280/DOOM_DOOM_II/) / [GOG](https://www.gog.com/en/game/doom_doom_ii)
>   - **Heretic + Hexen**: [Steam](https://store.steampowered.com/app/3286930/Heretic__Hexen/) / [GOG](https://www.gog.com/en/game/heretic_hexen)
>   *(Tip: Run `doom wads extract` to auto-discover and import them directly from your Steam/GOG libraries).*
> - **Community Megawads & Expansions** (*Ancient Aliens*, *Eviternity I & II*, *Back to Saturn X*, *Sunlust*, *Sunder*, etc.) can be fetched automatically via `doom wads fetch` or manually downloaded from **[Doomworld / idgames](https://www.doomworld.com/idgames/)**.

---

## Launching & Playing Games

### 1. Interactive Terminal Launcher (`doom play`)

Launch presets from anywhere in your terminal using the interactive fuzzy search launcher:

```bash
doom play
# Or simply:
doom
```

Features:
- Real-time fuzzy filtering across all 32 presets
- Side-by-side preview pane displaying IWAD, required PWADs, DeHackEd patches, and description
- Instant missing file status indicator (`✓ Ready` vs `✗ Missing`)
- Automatic return to launcher with cursor memory on game exit (or use `--once` to exit immediately)
- Curated color themes adhering to the 60-30-10 design principle (`--theme <name>`)
- Fallback numbered menu mode when running in basic or non-TTY terminal environments

#### Direct Launching & Engine Overrides
Launch any preset directly by name with custom engine flags:

```bash
# Launch a specific preset
doom launch "Eviternity II"

# Override default engine (e.g. run MBF mapset in DSDA-Doom)
doom launch "Ancient Aliens" -e dsda-doom

# Launch with custom warp and skill flags
doom launch "Sunlust" -skill 4 -warp 01

# Inspect synthesized launch command without starting the game
doom launch "Alien Vendetta" --dry-run

# List all available presets and their mapped engines
doom presets list
```

### 2. Terminal Themes & Visual Customization

The interactive launcher includes a semantic color system adhering to the 60-30-10 terminal design principle (60% canvas neutral, 30% structural framing, 10% focused accent), guaranteeing high contrast and readability across dark and light terminals:

```bash
# Preview all available themes with live color swatches
doom themes list

# Persistently set your default theme
doom themes set blood

# Temporarily override theme for a single run
doom play --theme cyberpunk
```

#### Built-in Themes

| Theme | Type | Description |
| :--- | :--- | :--- |
| `default` | ANSI-16 | Classic Doom semantic ANSI palette that adapts naturally to your terminal's color scheme |
| `cyberpunk` | TrueColor | Vibrant 24-bit neon palette (Electric Cyan `#00E5FF`, Neon Magenta `#FF2A85`, Dark Slate) |
| `blood` | TrueColor | Gothic Crimson (`#E63946`) & Bone White Nightdive software-plus aesthetic |
| `matrix` | TrueColor | Retro monochrome Phosphor Green (`#39FF14`, `#00FF41`) hacker aesthetic |
| `monochrome` | ANSI | High-contrast Black & White for minimalists or monochrome terminals |

#### Custom JSON Themes

Create custom themes in `<config_dir>/themes/<theme_name>.json`. For example, `~/.config/doom-cli/themes/solarized.json`:

```json
{
  "name": "solarized",
  "type": "dark",
  "description": "Solarized dark palette",
  "brand_fg": "#FFFFFF",
  "brand_bg": "#CB4B16",
  "accent_primary": "#268BD2",
  "accent_secondary": "#2AA198",
  "text_primary": "#93A1A1",
  "text_muted": "#586E75",
  "border": "#073642",
  "border_focus": "#268BD2",
  "cursor_fg": "#002B36",
  "cursor_bg": "#268BD2",
  "tag_uzdoom_fg": "#B58900",
  "tag_uzdoom_bg": "#073642",
  "tag_dsda_fg": "#859900",
  "tag_dsda_bg": "#073642",
  "status_ok": "#859900",
  "status_missing": "#DC322F"
}
```

Theme resolution follows standard precedence:
1. `--theme <name>` CLI flag
2. `DOOM_THEME` environment variable
3. `theme` setting in user configuration (`config.json`)
4. `default` built-in theme

### 3. Graphical Launcher (DoomRunner)

Launch DoomRunner from your application menu or terminal:

```bash
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

## Source Port Packaging (Linux)

When running Doom source ports on Linux, standalone native binaries or AppImages in `~/.local/bin/` are strongly recommended over sandboxed Flatpaks. 

Flatpaks isolate applications from the rest of the filesystem, preventing launchers (like DoomRunner or `doom play`) from discovering game engines or accessing WADs across custom paths without manual permission overrides. Running `doom setup` or `doom engines install` automatically manages standalone binaries in your user path.

---

## Contributing

We welcome contributions! Please review our [Contributing Guidelines](CONTRIBUTING.md) for architectural invariants, development setup, code style, and pull request verification instructions.

## Security

Please report security issues responsibly according to our [Security Policy](SECURITY.md).

## License

Apache 2.0. See [LICENSE](LICENSE).


