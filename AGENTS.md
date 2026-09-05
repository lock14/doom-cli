# Agent Guidelines for doom-configs

This repository contains configuration files, presets, build targets, and a unified CLI tool (`doom`) for classic Doom source ports (**DSDA-Doom**, **UZDoom**) and launchers (**DoomRunner**, **`doom` CLI**) across Linux, macOS, and Windows.

Any agent modifying this repository must follow these core principles.

---

## 1. Declarative Presets & Single Source of Truth

- **`data/presets.json` is the Single Source of Truth**: All preset definitions, engine assignments, IWAD mappings, PWAD file lists, DeHackEd orderings, metadata, and download URLs are defined declaratively in [`data/presets.json`](data/presets.json).
- **Never Manually Edit Generated Preset JSONs**: Do not manually edit `DoomRunner/linux/options.json` or `DoomRunner/windows/options.json`. Run `doom presets build` to compile `data/presets.json` into both launcher options files and update `README.md`.
- **Parity Verification**: Run `make check` (or `go test -v ./internal/preset`) to verify that launcher options and `README.md` match `data/presets.json` and comply with all path invariants.

---

## 2. Portability & Path Invariants

- **Never Commit Personal User Paths, Resolutions, or Refresh Rates**: Never commit absolute personal paths like `/home/<username>/`, hardcoded local display resolutions, or fixed monitor refresh rates into configuration files, presets, or code.
- **Use `__HOME__`, `__RESOLUTION__`, `__REFRESH_RATE__`, and `__SOUNDFONT__` Placeholders**:
  - In `DoomRunner/linux/options.json` and `data/presets.json`, all user home paths must use the `__HOME__` placeholder.
  - In `dsda-doom/dsda-doom.cfg`, `screen_resolution` must use `__RESOLUTION__` and `snd_soundfont` must use `__SOUNDFONT__`.
  - In `uzdoom/autoexec.cfg`, `fluid_patchset` must use `__SOUNDFONT__` and `vid_maxfps` must use `__REFRESH_RATE__`.
  - Deployment tooling (`doom config install`, `doom setup`) dynamically substitutes `__HOME__`, `__RESOLUTION__` (via native display detection), `__REFRESH_RATE__`, and `__SOUNDFONT__` at installation time.
  - On macOS and Windows, deployment tooling dynamically remaps paths to standard platform directories so DoomRunner and engines work seamlessly across platforms.
- **Respect Standard Directory Structures**:
  - **UZDoom Config (Linux)**: `~/.config/uzdoom/autoexec.cfg`
  - **UZDoom Config (macOS)**: `~/Library/Application Support/uzdoom/autoexec.cfg`
  - **DSDA-Doom Config (Linux)**: `~/.local/share/dsda-doom/dsda-doom.cfg`
  - **DSDA-Doom Config (macOS)**: `~/Library/Application Support/dsda-doom/dsda-doom.cfg`
  - **DoomRunner (Linux)**: `~/.local/share/DoomRunner/options.json`
  - **DoomRunner (macOS)**: `~/Library/Application Support/DoomRunner/options.json`
  - **DoomRunner (Windows)**: `%LOCALAPPDATA%\DoomRunner\options.json` & `%APPDATA%\DoomRunner\options.json`
  - **Linux / macOS Binaries**: `~/.local/bin/` (`uzdoom`, `dsda-doom`, `doomrunner`, `doom`)
  - **Linux WADs Directory**: `~/.local/share/games/uzdoom/`
  - **macOS WADs Directory**: `~/Library/Application Support/games/uzdoom/`
  - **Windows WADs Directory**: `<BaseDrive>\Doom WADS\` (defaults to the drive where `doom.exe` is executed from or `%LOCALAPPDATA%\Doom WADS\`, configurable via `--wads-dir` or `DOOM_WADS_DIR`)
  - **SoundFonts Directory**: `~/.local/share/soundfonts/` (Linux) / `~/Library/Application Support/soundfonts/` (macOS) / `%LOCALAPPDATA%\soundfonts\` (Windows)

---

## 3. Destructive Safety & Backup Policy

- **Mandatory Non-Destructive Backups**: Any configuration deployment target or command (`doom config install`, `doom setup`) must create timestamped backups (`.bak.<timestamp>`) before overwriting or modifying any existing user configuration file.
- **Preserve Unrelated Configuration**: When adding new engine variables, aliases, or keybindings, avoid modifying or removing unrelated settings unless explicitly requested.

---

## 4. Doom Engine Selection & Preset Hygiene

- **Engine Mapping Rules**:
  - **DSDA-Doom**: Use for classic vanilla, Boom, MBF, and MBF21 maps where demo accuracy, standard physics, and speedrunning precision are desired (e.g., *Alien Vendetta*, *BTSX*, *Sunder*, *Sunlust*, *Legacy of Rust*, *Sigil*).
  - **UZDoom**: Use for mapsets requiring ZDoom/GZDoom features, advanced scripting, high-res texture packs like OTEX (*Eviternity I & II*), or Raven Software games (*Heretic*, *Hexen*).
- **Preset Hygiene in DoomRunner & `doom`**:
  - **No Duplicate IWADs**: Never include the base game IWAD (`DOOM.WAD`, `DOOM2.WAD`, `PLUTONIA.WAD`, `TNT.WAD`) inside `mappacks`. The IWAD must only be specified in `iwad` / `selected_IWAD`.
  - **Proper Load Ordering**: Ensure DeHackEd patches (`.deh`) and resource files are ordered correctly relative to map PWADs and music wads (`idkfa 2024.wad`). In UZDoom, preserve the declared order across all files under `-file`.
  - **Optional Asset Graceful Degradation**: Optional soundtrack enhancements like `idkfa 2024.wad` must not prevent base games from running when absent (falling back to standard MIDI) and must not be treated as downloadable community megawad files by `doom wads fetch`. Missing required map files must cleanly abort execution with an error code before engine invocation.
  - **Filename Normalization & Alias Tolerance**: idgames archives and user directories frequently vary in case, spacing, and roman numerals (e.g., `Eviternity II.wad` in idgames vs `eviternityii.wad` in presets, or `gd.wad` vs `gdturbo.wad`). Both `doom wads fetch` and `doom launch` (and the `internal/wad` package) apply case-insensitive and whitespace/dash/underscore-normalized matching so maps and DeHackEd patches resolve reliably regardless of user file naming. Known commercial rerelease add-on aliases (`gdturbo.wad` mapping to `gd.wad`, `doomzero.wad`/`DOOMZERO.DEH`) must also be discovered and extracted by `doom wads extract` (`internal/steam`).
- **Visual Aesthetic & Display Refresh Intent**:
  - Maintain UZDoom’s curated Nightdive "Software-Plus" visual profile (software light mode `gl_lightmode 0`, banded stepping `gl_bandedsw 1`, palette tonemapping `gl_tonemap 3`, and nearest-neighbor texture sampling with 16x anisotropic filtering) unless explicitly instructed otherwise.
  - In DSDA-Doom, keep `dsda_fps_limit 0` with `render_vsync 1` and `uncapped_framerate 1` so high-refresh monitors (144Hz, 240Hz+) pace frames smoothly to native monitor refresh rates rather than being artificially throttled.

---

## 5. Cross-Platform Parity & Tooling Maintenance

- **Cross-Platform Consistency**: When adding or updating presets in `data/presets.json`, ensure changes compile cleanly across Linux (`DoomRunner/linux/options.json`) and Windows (`DoomRunner/windows/options.json`).
- **Unified Go Architecture (`cmd/doom` & `internal/`)**:
  - The repository features a unified, zero-dependency Go CLI (`doom`) that compiles to a single static binary on Linux, macOS, and Windows.
  - Package structure:
    - `cmd/doom/`: Cobra CLI commands (`play`, `launch`, `setup`, `wads`, `engines`, `soundfont`, `config`, `presets`).
    - `internal/config/`: Platform path resolvers adhering strictly to XDG (Linux), `~/Library/Application Support/` (macOS), and `%LOCALAPPDATA%`/`%APPDATA%` (Windows with execution drive auto-mapping).
    - `internal/display/`: Native display resolution and refresh rate detection.
    - `internal/engine/`: Source port asset downloading and argument synthesis/execution.
    - `internal/preset/`: Embedded presets catalog loader and options.json/README compiler.
    - `internal/steam/`: Multi-library `libraryfolders.vdf` parsing and game file extraction.
    - `internal/templates/`: Embedded engine configuration templates and backup deployer.
    - `internal/tui/`: Interactive Bubble Tea fuzzy launcher and preview pane.
    - `internal/wad/`: Multi-mirror archive downloader, `archive/zip` extractor, and SoundFont installer.
  - Whenever presets or config templates are updated, synchronize both the repository templates and the embedded data in `internal/preset/data/` and `internal/templates/data/`.
- **Maintain Automation & Sync Targets**:
  - Every config file and CLI feature tracked in the repository must be integrated into:
    1. `make build` & `make install` (compiling and deploying the static binary)
    2. `doom config sync` (to pull in-game tweaks from the system back into the repo)
    3. `doom config diff` (to inspect differences between repo and active system configs)
    4. `make check` (validation suite including `go test ./...` and path invariant audit)
    5. `README.md` (documentation, commands, and presets table)
- **CLI Subcommand Naming & Tone**: Prefer clean, idiomatic imperative verbs (`setup`, `play`, `launch`, `install`, `diff`, `sync`, `fetch`) over corporate jargon or adjectives (use `setup` rather than `turnkey`).

- **Go Code Style & Quality Standards**:
  - **Formatting**: Always format code using `gofmt -s -w .` (simplifying slice and composite literal syntax).
  - **Receiver Naming**: Use short (1-2 letters), mnemonic receiver names consistently across methods on a type. Never use `this` or `self`.
  - **Avoid Identifier Shadowing**: Never shadow Go built-in identifiers (`min`, `max`, `len`, `cap`, `new`, `clear`, `copy`, `close`, `delete`). Use descriptive identifiers such as `maxVal`, `count`, or `limit`.
  - **Documentation Comments**: All exported packages, types, interfaces, constants, and functions must have Go doc comments starting with the symbol name and adhering to standard Go and `revive` conventions.
  - **Static Analysis & Linting**: Enforce clean linting via `revive -config revive.toml -formatter friendly ./...` and `go vet ./...`.
  - **Hardened Testing**: All tests must execute cleanly under `-race` (race detector) and `-shuffle=on` (test order randomization).

---

## 6. Documentation Boundaries & Mandatory Updates

- **Mandatory Documentation Synchronization**:
  - **Every agent change must update documentation**: Whenever presets, engine settings, build targets, directory layouts, or CLI mechanics are modified, the relevant documentation **must** be updated within the same pull request/commit.
- **`README.md` is for Users**:
  - `README.md` must focus purely on user-facing concerns: installation commands, directory setup, presets catalog, engine features, CLI launcher usage, and player-facing tools.
  - Keep `README.md` clear, accessible, and free of internal agent mechanics or maintenance minutiae.
- **`AGENTS.md` is for Agents & Contributors**:
  - Repository principles, single-source-of-truth compiler mechanics, path invariant enforcement, internal build architecture, and agent guidelines belong in `AGENTS.md`.
  - Never leak prompt engineering directives, agent-specific meta-instructions, or internal maintenance rules into `README.md`.

---

## 7. Continuous Learning & Principle Encoding

- **Persist User Corrections**:
  - Whenever an agent is corrected, redirected, or receives feedback on repository conventions, it **must immediately encode the underlying principle into `AGENTS.md`** before concluding the task.
  - This ensures all future agent sessions and contributors automatically inherit the correction.

---

## 8. Verification Checklist for Agents

Before completing any changes:
1. Run the local verification sequence:
   - `make format-check` (verify `gofmt -s -l .` reports zero unformatted files).
   - `make tidy-check` (verify `go mod tidy && git diff --exit-code go.mod go.sum`).
   - `make lint` (verify `go vet ./...` and `revive -config revive.toml` pass with zero warnings).
   - `make check` (runs full suite: formatting check, tidy check, linters, `go test -v -race -shuffle=on ./...`, preset parity invariants, and path invariant inspection verifying zero hardcoded personal user paths).
2. Verify `git diff` contains no hardcoded usernames or personal paths.
3. Run `make help` to verify `Makefile` syntax.
4. **Update Documentation**:
   - Update `README.md` if user-facing behavior changed (presets, commands, defaults, directories).
   - Update `AGENTS.md` if repository principles, build architecture, or agent rules changed.
   - **Encode Corrections**: If the user or a reviewer provided a correction, ensure the principle is codified in `AGENTS.md`.
5. Verify `README.md` tables (presets catalog, Makefile reference) accurately reflect the new state.
