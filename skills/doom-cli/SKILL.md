---
name: doom-cli
description: >-
  Install, configure, fetch, customize, and launch classic Doom, Heretic, Hexen, Strife, and community megawads
  with the doom CLI (doom-cli, DSDA-Doom, and UZDoom). Use when the user asks to set up Doom, scan Steam or GOG
  IWADs, browse or download megawads, add custom WADs or source ports, configure complevels or launch flags, or
  troubleshoot doom-cli. Do not use for Doom Emacs.
---

# Doom CLI skill

Operate the `doom` CLI (`doom-cli`) to install portable source ports (`dsda-doom` and `uzdoom`), import commercial IWADs
from Steam and GOG, fetch SHA-256 verified community megawads, manage custom presets and overrides, and launch classic
Doom and id Tech 1 games across Linux, macOS, and Windows.

## Non-interactive agent rules

*   **Verify binary availability**: Check `command -v doom` first. If `doom` is missing from `PATH`, check
    `~/.local/bin/doom` on Linux and macOS, or `%USERPROFILE%\Games\Doom\bin\doom.exe` and Go `bin` on Windows.
*   **Avoid blocking game windows**: Bare `doom` and `doom <wad>` launch a graphical SDL or Vulkan game window that
    blocks until the player quits. Always pass `--dry-run` (`doom <wad> --dry-run`) or run `doom wads show <name>` when
    verifying setups or previewing flags, and omit `--dry-run` only when the user explicitly asks to launch and play.
*   **Avoid interactive TUI views**: Bare `doom wads` opens a full-screen Bubble Tea TUI on a TTY or waits on `stdin` in
    a numbered menu on non-TTY streams, and bare `doom engines` and `doom config` open TUI editors when attached to a
    PTY. Always run explicit non-interactive subcommands (`doom wads list`, `doom wads list --all`,
    `doom wads show <name>`, `doom engines list`, `doom engines cfg diff`, `doom config show`, `doom config get [key]`).
*   **Mutate through CLI subcommands**: Never hand-edit `config.json` or `<config-dir>/catalog.d/*.json` unless
    repairing a quarantined file. Use `doom config set`, `doom wads add`, `doom wads set`, `doom wads reset`,
    `doom wads remove`, `doom engines add`, `doom engines set`, `doom engines reset`, and `doom engines remove` so
    `doom-cli` enforces `kebab-case` schema validation, tri-state override merging, and atomic `.bak.<timestamp>`
    backups. Run `doom wads lint` to validate catalog entities after any manual edit.

## Setup, discovery, and preset management

```bash
# Run 4-step core setup (configs, engines, SC-55 SoundFont, and Steam or GOG Doom IWADs)
doom setup

# Include community megawads and optional non-Doom id Tech 1 games during setup
doom setup --wads eviternity-ii,ancient-aliens --games heretic,hexen

# List installed presets vs all 37 curated presets (including uninstalled WADs and optional games)
doom wads list
doom wads list --all

# Inspect a preset's resolved IWAD, PWADs, complevel, launch command preview, and file status
doom wads show eviternity-ii

# Import Steam, GOG, Xbox, and Epic IWADs (or scan a custom directory) and fetch community megawads
doom wads scan --games all
doom wads scan --from /mnt/backup/doom
doom wads fetch eviternity-ii,sunlust
doom wads fetch all
```

*   **Library filtering** (`--all`): `doom wads list` hides uninstalled curated megawads and unowned optional games
    (`heretic`, `hexen`, `strife`) by default. Pass `--all` whenever browsing the full catalog to recommend or fetch new
    presets.
*   **Optional game scoping** (`--games`): `doom setup`, `doom wads scan`, and `doom wads fetch all` default to `doom`
    plus optional games whose base IWAD is already installed on disk. Pass `--games heretic,hexen,strife` or
    `--games all` to discover and extract newly purchased Raven Software or Rogue Entertainment titles.
*   **Custom scan directories** (`--from`): `doom setup`, `doom wads scan`, and `doom wads fetch` auto-discover Steam
    (`steamapps/common` and Proton `compatdata`), GOG, GOG Galaxy, Xbox (`XboxGames`), Epic, Heroic, Lutris, Bottles,
    CrossOver, Whisky, and Linux system WAD folders. Pass `--from <dir[,dir...]>` to import commercial IWADs from an
    external drive, network share, or custom archive folder.
*   **Uninstalling files vs removing presets**: `doom wads uninstall <id[,id...]>` deletes downloaded PWAD, PK3,
    DeHackEd, and `.txt` files from primary `wads-dir` while protecting base IWADs, shared `idkfa 2024.wad` and
    `VOICES.WAD`, and secondary read-only `wads-dirs`. `doom wads remove <id>` deletes a custom
    `catalog.d/preset.<id>.json` definition (curated presets cannot be removed), and `doom wads reset <id>` clears
    persistent overrides in `catalog.d/override.preset.<id>.json`.

## Custom presets, overrides, and source-port hygiene

*   **Exact-match lookup on mutations**: Read and launch commands (`doom <wad>`, `doom wads show`, `doom wads fetch`)
    accept shorthand queries (`av`, `btsx1`, `gd`, `doom 2`), whereas mutating and destructive commands (`set`, `reset`,
    `remove`, `uninstall`) require exact canonical IDs, names, or primary WAD stems from `doom wads list --all` or
    `doom engines list`.
*   **Engine pairing**: Pair classic vanilla, limit-removing, Boom, MBF, MBF21, ID24, and OTEX Doom mapsets with
    `dsda-doom`. Reserve `uzdoom` for ZScript, DECORATE, PK3, or UDMF mods (such as *MyHouse.pk3*), ZDoom `ZMAPINFO` and
    `ACS` mapsets (*Eviternity*), and *Heretic*, *Hexen*, and *Strife*. When registering custom engines via
    `doom engines add`, pass `--style boom` for ports expecting separate `-file` and `-deh` flags (`woof`,
    `crispy-doom`) and `--style zdoom` for `-file`-only ports (`gzdoom`).
*   **Explicit Boom compatibility levels**: Because `dsda-doom.cfg` sets `default_compatibility_level 21`, always pass
    an explicit `--complevel` when registering a custom `dsda-doom` preset that lacks an embedded `COMPLVL` lump:

| Complevel | Target standard and base IWADs |
| :--- | :--- |
| `2` | Doom v1.9 and Doom II (`DOOM2.WAD` vanilla and limit-removing mapsets) |
| `3` | The Ultimate Doom (`DOOM.WAD` 4-episode and *Sigil* mapsets) |
| `4` | Final Doom (`TNT.WAD` and `PLUTONIA.WAD` vanilla mapsets) |
| `9` | Boom 2.02 (`aaliens`, `sunlust`, `speed-of-doom`, and Boom megawads) |
| `11` | MBF (`valiant` and MBF mapsets) |
| `21` | MBF21 and ID24 (`eviternity-ii`, `overboard`, `legacy-of-rust`) |

*   **IWAD separation, DeHackEd order, and three-tier flags**:
    *   Specify the base IWAD (`DOOM.WAD`, `DOOM2.WAD`, `TNT.WAD`, `PLUTONIA.WAD`, `HERETIC.WAD`, `HEXEN.WAD`,
        `STRIFE1.WAD`) only in `--iwad`, never inside `--files`.
    *   Order `.deh` and `.bex` patches after map `.wad` files (`--files "mymap.wad,mymap.deh"`).
    *   Keep launch flags separated across three tiers rather than placing port-specific flags in universal `--args`:
        use `--complevel <N>` for Boom compatibility (`default` or `none` on `doom wads set`), repeatable
        `--engine-args <engine>=<flags>` for port-specific options (`--engine-args dsda-doom="-pistolstart -solonet"`),
        and `--args "<flags>"` for universal flags (`-skill 4`, `-warp 15`, `-fast`, `-nomonsters`, `-respawn`).
    *   Enable or disable optional WADs such as `idkfa 2024.wad` per preset via `--include-file "idkfa 2024.wad"` or
        `--exclude-file "idkfa 2024.wad"`.

```bash
# Register a custom Boom megawad with explicit complevel and DeHackEd ordering, then preview launch
doom wads add "Vanguard" -e dsda-doom --iwad DOOM2.WAD --files "vanguard.wad,vanguard.deh" --complevel 9
doom vanguard --dry-run

# Persist pistol-start and Ultra-Violence on Sunlust without breaking -e uzdoom overrides
doom wads set sunlust --engine-args dsda-doom="-pistolstart" --args "-skill 4"
doom sunlust --dry-run
```

## Settings and troubleshooting

*   **CLI settings and environment**: Manage persistent settings via `doom config show`, `doom config get [key]`, and
    `doom config set <key> <value>` for keys `theme`, `nerd-fonts`, `wads-dir`, `wads-dirs` (comma-separated read-only
    extra WAD directories), `bin-dir`, and `soundfonts-dir`. Capture personal in-game keybinds and tweaks into
    `<config-dir>/portcfg/` via `doom engines cfg save [engine|all]` (or clear them with `doom engines cfg reset`) so
    they survive `doom engines cfg install` and `--force` upgrades and stay portable in dotfiles. `DOOM_CONFIG_DIR` is
    the sole application environment variable; do not invent `DOOM_*` directory variables or global `--config` flags.
*   **Common fixes**: Resolve missing commercial IWADs with `doom wads scan`, `doom wads scan --from <dir>`, or
    `doom config set wads-dir <path>`; resolve missing engine binaries or silent FluidSynth MIDI with
    `doom engines install`; re-detect monitor resolution and refresh rate in port configurations with
    `doom engines cfg install`.
