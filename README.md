# Doom Configs

A curated set of configuration files and presets for playing Doom using modern source ports (**[DSDA-Doom](https://github.com/kraflab/dsda-doom)** and **[UZDoom](https://github.com/UZDoom/uzdoom)**) with the **[DoomRunner](https://github.com/Youda008/DoomRunner)** graphical launcher across Linux and Windows.

---

## Supported Ports & Launchers

- **[DSDA-Doom](https://github.com/kraflab/dsda-doom)**: Precision speedrunning and demo-accurate source port preconfigured for MBF21, extended HUD, and uncapped framerates.
- **[UZDoom](https://github.com/UZDoom/uzdoom)**: Modern ZDoom-based engine configured with a Nightdive "Software-Plus" visual aesthetic and broad mod compatibility.
- **[DoomRunner](https://github.com/Youda008/DoomRunner)**: Graphical launcher preloaded with 20+ community megawad presets mapped to their ideal engines.

---

### Linux & macOS

The `Makefile` automatically detects whether you are running **Linux** or **macOS (Darwin)** and maps configuration directories accordingly:
- **Linux Destinations**: `~/.config/uzdoom/`, `~/.local/share/dsda-doom/`, `~/.local/share/DoomRunner/`
- **macOS Destinations**: `~/Library/Application Support/uzdoom/`, `~/Library/Application Support/dsda-doom/`, `~/Library/Application Support/DoomRunner/`

#### Option 1: Full Bootstrap (Engines + Configs)
To automatically download the latest official binaries/AppImages (`uzdoom`, `dsda-doom`, `doomrunner`) into `~/.local/bin/` and deploy all configurations:

```bash
make bootstrap
```

> [!TIP]
> Ensure `~/.local/bin` is in your `$PATH` (e.g. in your `~/.bashrc` or `~/.zshrc`):
> ```bash
> export PATH="$HOME/.local/bin:$PATH"
> ```

#### Option 2: Configs Only
To deploy only configuration files (with automatic `.bak` backups of existing configs):

```bash
make install
```

Or install individual components:
- `make install-uzdoom` — Deploys `autoexec.cfg`
- `make install-dsda` — Deploys `dsda-doom.cfg`
- `make install-doomrunner` — Deploys `options.json`
- `make install-engines` — Downloads only the binaries into `~/.local/bin/`

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

## Makefile Quick Reference

| Target | Description |
| :--- | :--- |
| `make bootstrap` | **Full setup:** Downloads engine AppImages + installs all configs |
| `make install` | Deploys all configuration files with automatic backups |
| `make install-engines` | Downloads AppImages (UZDoom, DSDA-Doom, DoomRunner) to `~/.local/bin/` |
| `make install-uzdoom` | Installs only `uzdoom/autoexec.cfg` |
| `make install-dsda` | Installs only `dsda-doom/dsda-doom.cfg` |
| `make install-doomrunner` | Installs only `DoomRunner/linux/options.json` |
| `make diff` | Shows diff between repo configs and live system configs |
| `make sync` | Syncs live system configs back into the git repository |
| `make check` | Runs validation suite (syntax, JSON validation, invariants, test install) |

---

## WAD & File Locations

> [!IMPORTANT]
> **Game Files (WADs) Are Not Included**
> 
> This repository contains configuration files, launcher presets, and bootstrap scripts. You must provide your own legally acquired game files:
> - **Commercial IWADs** (`DOOM.WAD`, `DOOM2.WAD`, `PLUTONIA.WAD`, `TNT.WAD`, `HERETIC.WAD`, `HEXEN.WAD`) can be acquired from digital storefronts:
>   - **Doom + Doom II**: [Steam](https://store.steampowered.com/app/2280/DOOM_DOOM_II/) / [GOG](https://www.gog.com/en/game/doom_doom_ii)
>   - **Heretic + Hexen**: [Steam](https://store.steampowered.com/app/3286930/Heretic__Hexen/) / [GOG](https://www.gog.com/en/game/heretic_hexen)
> - **Community Megawads & Expansions** (*Ancient Aliens*, *Eviternity I & II*, *Back to Saturn X*, *Sunlust*, *Sunder*, etc.) are free community creations downloadable from **[Doomworld / idgames](https://www.doomworld.com/idgames/)** or their respective release threads.

Place your game files (`.wad`, `.deh`, `.pk3`) in the standard directory expected by DoomRunner:

- **Linux Default**: `~/.local/share/games/uzdoom/`
- **macOS Default**: `~/Library/Application Support/games/uzdoom/`
- **Windows Default**: `E:\Doom WADS\` (or your customized `-BaseDrive` path)

### Recommended Core Files
- **IWADs**: `DOOM.WAD`, `DOOM2.WAD`, `PLUTONIA.WAD`, `TNT.WAD`, `HERETIC.WAD`, `HEXEN.WAD`
- **Music Packs**: `idkfa 2024.wad` (modern soundtrack expansion)

---

## Preconfigured Presets

DoomRunner comes pre-populated with presets configured for the best matching engine:

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

## Installation Recommendations (Linux)

When installing Doom tools on Linux, using **AppImages** or native binaries is strongly recommended over Flatpaks. 

Flatpaks run inside a sandbox that isolates them from the rest of the filesystem, preventing launchers (like DoomRunner) from easily discovering game engines (like UZDoom or DSDA-Doom) or accessing WADs across custom paths without manual permission overrides.

- Run `make bootstrap` or `make install-engines` to place portable AppImages in `~/.local/bin/`.
- Alternatively, download binaries directly from each project's GitHub Releases page.
