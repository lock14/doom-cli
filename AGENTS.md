# Repository Design Principles

*   **Declarative Presets & Single Source of Truth**:
    *   `data/presets.json` is the sole source of truth for all preset definitions, engine assignments, IWAD mappings, PWAD file lists, DeHackEd orderings, metadata, and download URLs.
    *   **Never manually edit generated launcher files**: Do not edit `DoomRunner/linux/options.json` or `DoomRunner/windows/options.json` directly. Run `doom presets build` to compile `data/presets.json` into both launcher options files and update `README.md`.
    *   **Parity Verification**: Run `make check` (or `go test -v ./internal/preset`) to verify that launcher options and `README.md` match `data/presets.json` and comply with all path invariants.
*   **Portability & Path Invariants**:
    *   **Never commit personal user paths, display resolutions, or refresh rates**: Never commit absolute personal paths like `/home/<username>/`, hardcoded local display resolutions, or fixed monitor refresh rates into configuration files, presets, or code.
    *   **Use Standard Placeholder Tokens**:
        *   In `DoomRunner/linux/options.json` and `data/presets.json`, all user home paths must use the `__HOME__` placeholder.
        *   In `dsda-doom/dsda-doom.cfg`, `screen_resolution` must use `__RESOLUTION__` and `snd_soundfont` must use `__SOUNDFONT__`.
        *   In `uzdoom/autoexec.cfg`, `fluid_patchset` must use `__SOUNDFONT__` and `vid_maxfps` must use `__REFRESH_RATE__`.
        *   Deployment tooling (`doom config install`, `doom setup`) dynamically substitutes `__HOME__`, `__RESOLUTION__` (via native display detection), `__REFRESH_RATE__`, and `__SOUNDFONT__` at installation time.
    *   **Respect Platform Standard Directories**:
        *   **UZDoom Config**: `~/.config/uzdoom/autoexec.cfg` (Linux) / `~/Library/Application Support/uzdoom/` (macOS) / `%APPDATA%\uzdoom\` (Windows)
        *   **DSDA-Doom Config**: `~/.local/share/dsda-doom/dsda-doom.cfg` (Linux) / `~/Library/Application Support/dsda-doom/` (macOS) / `%LOCALAPPDATA%\dsda-doom\` (Windows)
        *   **DoomRunner Options**: `~/.local/share/DoomRunner/options.json` (Linux) / `~/Library/Application Support/DoomRunner/` (macOS) / `%LOCALAPPDATA%\DoomRunner\` (Windows)
        *   **Binaries**: `~/.local/bin/` (Linux/macOS) / `%LOCALAPPDATA%\Programs\Doom\bin\` (Windows)
        *   **WADs Directory**: `~/.local/share/games/uzdoom/` (Linux) / `~/Library/Application Support/games/uzdoom/` (macOS) / `<Drive>:\Doom WADS\` (Windows)
        *   **SoundFonts**: `~/.local/share/soundfonts/` (Linux) / `~/Library/Application Support/soundfonts/` (macOS) / `%LOCALAPPDATA%\soundfonts\` (Windows)
        *   **CLI Config & Themes**: `~/.config/doom-cli/` (Linux) / `~/Library/Application Support/doom-cli/` (macOS) / `%LOCALAPPDATA%\doom-cli\` (Windows)
*   **Destructive Safety & Backup Policy**:
    *   **Mandatory Non-Destructive Backups**: Any configuration deployment target or command (`doom config install`, `doom setup`) must create timestamped backups (`.bak.<timestamp>`) before overwriting or modifying any existing user configuration file.
    *   **Preserve Unrelated Configuration**: When adding new engine variables, aliases, or keybindings, avoid modifying or removing unrelated settings unless explicitly requested.
*   **Doom Engine Selection & Preset Hygiene**:
    *   **DSDA-Doom**: Use for classic vanilla, Boom, MBF, and MBF21 maps where demo accuracy, standard physics, and speedrunning precision are desired (e.g. *Alien Vendetta*, *BTSX*, *Sunder*, *Sunlust*, *Legacy of Rust*, *Sigil*).
    *   **UZDoom**: Use for mapsets requiring ZDoom/GZDoom features, advanced scripting, high-res texture packs like OTEX (*Eviternity I & II*), or Raven Software games (*Heretic*, *Hexen*).
    *   **No Duplicate IWADs**: Never include the base game IWAD (`DOOM.WAD`, `DOOM2.WAD`, etc.) inside `mappacks`. The IWAD must only be specified in `iwad` / `selected_IWAD`.
    *   **Load Ordering**: Ensure DeHackEd patches (`.deh`) and resource files are ordered correctly relative to map PWADs and music wads (`idkfa 2024.wad`).
    *   **Optional Asset Degradation**: Optional soundtrack enhancements like `idkfa 2024.wad` must not prevent base games from running when absent (falling back cleanly to standard MIDI). Missing required map files must cleanly abort execution with a descriptive error before engine invocation.
    *   **Filename Normalization & Alias Tolerance**: Case-insensitive and whitespace/dash/underscore-normalized matching ensures maps and DeHackEd patches resolve reliably regardless of user file naming. Rerelease add-on aliases (`gdturbo.wad` -> `gd.wad`, `doomzero.wad`/`DOOMZERO.DEH`) must be discovered by `doom wads extract`.
    *   **Visual Aesthetic & Frame Pacing**: Maintain UZDoom's curated Nightdive "Software-Plus" visual profile (software light mode `gl_lightmode 0`, banded stepping `gl_bandedsw 1`, palette tonemapping `gl_tonemap 3`, and nearest-neighbor texture sampling with 16x anisotropic filtering). In DSDA-Doom, keep `dsda_fps_limit 0` with `render_vsync 1` and `uncapped_framerate 1` so high-refresh monitors pace frames smoothly to native monitor refresh rates.

# Package Structure & Architecture Conventions

The repository features a unified, zero-dependency Go CLI (`doom`) that compiles to a single static binary on Linux, macOS, and Windows:

*   `cmd/doom/`: Cobra CLI commands (`setup`, `play`, `launch`, `wads`, `engines`, `soundfont`, `config`, `presets`, `themes`).
*   `internal/config/`: Platform path resolvers adhering strictly to XDG (Linux), Library (macOS), and AppData (Windows), plus user config persistence (`config.json`).
*   `internal/display/`: Native display resolution and refresh rate detection.
*   `internal/engine/`: Source port asset downloading and argument synthesis/execution.
*   `internal/preset/`: Embedded presets catalog loader and options.json/README compiler.
*   `internal/steam/`: Multi-library `libraryfolders.vdf` parsing and game file extraction.
*   `internal/templates/`: Embedded engine configuration templates and backup deployer.
*   `internal/tui/`: Interactive Bubble Tea fuzzy launcher, preview pane, and customizable themes engine.
*   `internal/wad/`: Multi-mirror archive downloader, `archive/zip` extractor, and SoundFont installer.
*   **Data Synchronization**: Whenever presets or config templates are updated, synchronize both the repository templates and the embedded data in `internal/preset/data/` and `internal/templates/data/`.

# Naming & Code Style Conventions

*   **Formatting**: Always format code using `gofmt -s -w .` (simplifying slice and composite literal syntax).
*   **Maximum Line Length (120 Columns)**: Enforce a strict 120-character maximum line length limit across the repository via `.editorconfig` and Revive's `line-length-limit` rule in `revive.toml`. Long strings, command invocations, and error messages must be wrapped cleanly.
*   **Receiver Naming**: Use short (1-2 letters), mnemonic receiver names consistently across methods on a type. Never use `this` or `self`.
*   **Avoid Identifier Shadowing**: Never shadow Go built-in identifiers (`min`, `max`, `len`, `cap`, `new`, `clear`, `copy`, `close`, `delete`). Use descriptive identifiers such as `maxVal`, `count`, or `limit`.
*   **Documentation Comments**: All exported packages, types, interfaces, constants, and functions must have comprehensive Go doc comments starting with the symbol name and adhering to standard Go and `revive` conventions.
*   **Static Analysis & Linting**: Enforce clean linting via `revive -config revive.toml -formatter friendly ./...` and `go vet ./...`.

# Testing Conventions

*   **Table-Driven Tests**: Table-driven tests are standard for unit testing. Each test case struct must include a descriptive `name` string field.
*   **Hardened Concurrency & Randomization**: All tests must execute cleanly under `-race` (race detector) and `-shuffle=on` (test order randomization).
*   **Filesystem Isolation**: File resolution, deployment, and extraction tests must execute within isolated temporary directories (`t.TempDir()` or `os.MkdirTemp`) and clean up completely.
*   **Parity & Path Auditing**: Every test run includes automated validation that:
    1. Launcher JSON files match `data/presets.json` exactly.
    2. Zero absolute personal user paths (`/home/`, `%USERPROFILE%`) exist in configuration files, presets, or test fixtures.

# Documentation Roles & Boundaries

*   **`README.md` (User-Facing)**: Reserved strictly for players and end-users of the `doom` CLI and source ports. Focuses on installation, `doom setup`, gameplay (`doom play`), preset catalogs, directory layouts, and game file acquisition. Must **not** contain internal developer commands, agent instructions, or compiler details.
*   **`CONTRIBUTING.md` (Contributor-Facing)**: Dedicated guide for human open-source developers. Details prerequisites, Makefile targets, Go code style, testing requirements, and PR guidelines.
*   **`AGENTS.md` (Agent & Architectural Invariants)**: The canonical, authoritative reference for autonomous agents, architecture invariants, path tokens, safety rules, and code quality checklists.
*   **Documentation Synchronization**: Whenever presets, engine settings, build targets, directory layouts, or CLI mechanics are modified, all relevant documentation (**Go doc comments**, **`README.md`**, **`CONTRIBUTING.md`**, and **`AGENTS.md`**) MUST be updated in the same pull request.

# Pull Request & Workflow Conventions

*   **Accurate & Detailed Commits**: Use imperative commit messages describing the architectural change (e.g. `Adopt Go tooling, static analysis, and testing patterns from collections`).
*   **Path Invariant Verification**: Verify `git diff` contains zero hardcoded personal user paths before committing.
*   **PR Template Completion**: Ensure the PR template checklist is completely satisfied.

# Continuous Learning & Principle Encoding

*   **Persist User Corrections**: Whenever an agent is corrected, redirected, or receives feedback on repository conventions, it **must immediately encode the underlying principle into `AGENTS.md`** before concluding the task. This ensures all future agent sessions automatically inherit the correction.

# Pre-Completion Verification Checklist

Before completing any changes, agents MUST run and verify the following commands succeed without errors or warnings:

1. `make format-check` (verify `gofmt -s -l .` reports zero unformatted files).
2. `make tidy-check` (verify `go mod tidy && git diff --exit-code go.mod go.sum`).
3. `make lint` (verify `go vet ./...` and `revive -config revive.toml` pass with zero warnings).
4. `make check` (runs full suite: formatting check, tidy check, linters, `go test -v -race -shuffle=on ./...`, preset parity invariants, and path invariant inspection verifying zero hardcoded personal user paths).
