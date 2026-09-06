# Repository Design Principles

*   **Declarative Presets & Single Source of Truth**:
    *   `data/presets.json` is the sole source of truth for all preset definitions, engine assignments, IWAD mappings,
        PWAD file lists, DeHackEd orderings, metadata, and download URLs.
    *   **Synchronize Documentation**: Run `doom presets build` to synchronize `data/presets.json` into `README.md`.
    *   **Parity Verification**: Run `make check` (or `go test -v ./internal/preset`) to verify that `README.md`
        matches `data/presets.json` and complies with all path invariants.
*   **Portability & Path Invariants**:
    *   **Never commit personal user paths, display resolutions, or refresh rates**: Never commit absolute personal
        paths like `/home/<username>/`, hardcoded local display resolutions, or fixed monitor refresh rates into
        configuration files, presets, or code.
    *   **Use Standard Placeholder Tokens**:
        *   In `data/presets.json`, all user home paths must use the `__HOME__` placeholder.
        *   In `dsda-doom/dsda-doom.cfg`, `screen_resolution` must use `__RESOLUTION__` and `snd_soundfont` must
            use `__SOUNDFONT__`.
        *   In `uzdoom/autoexec.cfg`, `fluid_patchset` must use `__SOUNDFONT__` and `vid_maxfps` must use
            `__REFRESH_RATE__`.
        *   Deployment tooling (`doom config install`, `doom setup`) dynamically substitutes `__HOME__`,
            `__RESOLUTION__` (via native display detection), `__REFRESH_RATE__`, and `__SOUNDFONT__` at
            installation time.
    *   **Respect Platform Standard Directories**:
        *   **UZDoom Config**: `~/.config/uzdoom/autoexec.cfg` (Linux) /
            `~/Library/Application Support/uzdoom/` (macOS) /
            `%USERPROFILE%\Games\Doom\bin\autoexec.cfg` (Windows)
        *   **DSDA-Doom Config**: `~/.local/share/dsda-doom/dsda-doom.cfg` (Linux) /
            `~/Library/Application Support/dsda-doom/` (macOS) /
            `%USERPROFILE%\Games\Doom\bin\dsda-doom.cfg` (Windows)
        *   **Binaries**: `~/.local/bin/` (Linux/macOS) / `%USERPROFILE%\Games\Doom\bin\` (Windows)
        *   **WADs Directory**: `~/.local/share/games/uzdoom/` (Linux) /
            `~/Library/Application Support/games/uzdoom/` (macOS) / `%USERPROFILE%\Games\Doom\wads\` (Windows)
        *   **SoundFonts**: `~/.local/share/soundfonts/` (Linux) /
            `~/Library/Application Support/soundfonts/` (macOS) / `%USERPROFILE%\Games\Doom\soundfonts\` (Windows)
        *   **CLI Config & Themes**: `~/.config/doom-cli/` (Linux) /
            `~/Library/Application Support/doom-cli/` (macOS) / `%LOCALAPPDATA%\doom-cli\` (Windows)
*   **Destructive Safety & Backup Policy**:
    *   **Mandatory Non-Destructive Backups**: Any configuration deployment target or command (`doom config install`,
        `doom setup`) must create timestamped backups (`.bak.<timestamp>`) before overwriting or modifying any
        existing user configuration file.
    *   **Preserve Unrelated Configuration**: When adding new engine variables, aliases, or keybindings, avoid
        modifying or removing unrelated settings unless explicitly requested.
    *   **Archive Security & Zip Slip Hardening**: All archive extractions (`archive/zip`) must sanitize entry
        paths against `..` traversal sequences and verify
        `strings.HasPrefix(cleanDest, cleanTargetDir + string(filepath.Separator))` before any filesystem write.
*   **Doom Engine Selection & Preset Hygiene**:
    *   **DSDA-Doom**: Use for classic vanilla, Boom, MBF, and MBF21 maps where demo accuracy, standard physics,
        and speedrunning precision are desired (e.g. *Alien Vendetta*, *BTSX*, *Sunder*, *Sunlust*, *Legacy of Rust*,
        *Sigil*).
    *   **UZDoom**: Use for mapsets requiring ZDoom/GZDoom features, advanced scripting, high-res texture packs like
        OTEX (*Eviternity I & II*), or Raven Software games (*Heretic*, *Hexen*).
    *   **User Extensibility & Layered Catalogs**: Users can register custom engines (`doom engines add`), custom
        WAD presets (`doom presets add`), or override launch options (`doom presets config`). Built-in presets in
        `data/presets.json` serve as the immutable curated defaults. User configurations layer cleanly without
        mutating repo defaults.
    *   **No Duplicate IWADs**: Never include the base game IWAD (`DOOM.WAD`, `DOOM2.WAD`, etc.) inside `mappacks`.
        The IWAD must only be specified in `iwad`.
    *   **Load Ordering**: Ensure DeHackEd patches (`.deh`) and resource files are ordered correctly relative to
        map PWADs and music wads (`idkfa 2024.wad`).
    *   **Optional Asset Degradation**: Optional soundtrack enhancements like `idkfa 2024.wad` must not prevent
        base games from running when absent (falling back cleanly to standard MIDI). Missing required map files must
        cleanly abort execution with a descriptive error before engine invocation.
    *   **Filename Normalization & Alias Tolerance**: Case-insensitive and whitespace/dash/underscore-normalized
        matching ensures maps and DeHackEd patches resolve reliably regardless of user file naming. Rerelease add-on
        aliases (`gdturbo.wad` -> `gd.wad`, `doomzero.wad`/`DOOMZERO.DEH`) must be discovered by `doom wads extract`.
    *   **Visual Aesthetic & Frame Pacing**: Maintain UZDoom's curated Nightdive "Software-Plus" visual profile
        (software light mode `gl_lightmode 0`, banded stepping `gl_bandedsw 1`, palette tonemapping `gl_tonemap 3`,
        and nearest-neighbor texture sampling with 16x anisotropic filtering). In DSDA-Doom, keep `dsda_fps_limit 0`
        with `render_vsync 1` and `uncapped_framerate 1` so high-refresh monitors pace frames smoothly to native
        monitor refresh rates.
*   **TUI Styling & Font Glyph Hygiene**:
    *   **Clean Typography Over Emojis**: Emojis (e.g. `💀`) render with fixed vendor bitmaps/colors regardless of
        ANSI foreground escapes. They clash with light backgrounds (such as monochrome white pills). Use clean
        typography (`DOOM`) instead of emojis for branded badges.
    *   **Universal Fallback by Default**: Avoid Unicode Private Use Area (PUA) glyphs (such as ``, ``, ``, ``)
        in standard rendering. Default to universal rectangular badges and plain text prompts (`Filter: `,
        `README Viewer`) that render flawlessly in 100% of standard system fonts. Keep Powerlevel10k rounded
        capsule ends opt-in via `--nerd-fonts` / `nerd_fonts: true`.

# Package Structure & Architecture Conventions

The repository features a unified, zero-dependency Go CLI (`doom`) that compiles to a single static binary on
Linux, macOS, and Windows:

*   `cmd/doom/`: Modular Cobra CLI command implementations:
    *   `main.go`: Entrypoint, root command, global flags, and path helpers.
    *   `play.go`: `doom play` interactive launcher command.
    *   `launch.go`: `doom launch` direct preset execution command.
    *   `setup.go`: `doom setup` automated turnkey workflow command.
    *   `wads.go`: `doom wads` subcommands (`list`, `fetch`, `extract-steam` with `extract` alias).
    *   `engines.go`: `doom engines` subcommands (`list`, `add`, `remove`, `install`).
    *   `presets.go`: `doom presets` subcommands (`list`, `show`, `add`, `config`, `remove`, `build`).
    *   `soundfont.go`: `doom soundfont install` subcommand.
    *   `config.go`: `doom config` subcommands (`show`, `get`, `set`, `toggle`, `install`, `diff`, `sync`).
    *   `themes.go`: `doom themes` subcommands (`list`, `set`).
*   `internal/config/`: Platform path resolvers adhering strictly to XDG (Linux), Library (macOS), and AppData
    (Windows), plus user configuration persistence (`config.json`).
*   `internal/display/`: Native display resolution and refresh rate auto-detection.
*   `internal/engine/`:
    *   `runner.go`: Engine execution argument synthesis and process lifecycle management.
    *   `installer.go`: Source port asset downloading and portable binary installation (Zip Slip hardened).
*   `internal/preset/`:
    *   `preset.go`: Declarative preset catalog, catalog layering, and embedded presets loader.
    *   `resolve.go`: Case-insensitive, whitespace-tolerant file resolution with alias matching.
    *   `decode.go`: Document decoding with DOS Code Page 437 / CP437 box-drawing detection.
    *   `compiler.go`: README table synchronizer (`doom presets build`).
*   `internal/steam/`: Multi-library `libraryfolders.vdf` parsing and game file extraction.
*   `internal/templates/`: Embedded engine configuration templates and timestamped backup deployer.
*   `internal/tui/`: Interactive Bubble Tea fuzzy launcher:
    *   `launcher.go`: State machine, update loop, and message routing.
    *   `layout.go`: Dynamic responsive layout geometry and dimension clamping.
    *   `render.go`: UI view rendering, badges, status lines, and key help formatting.
    *   `menu.go`: Fallback numbered terminal menu for basic or non-TTY environments.
    *   `theme.go`: Theme loader, custom JSON theme parser, and style builders.
    *   `builtin_themes.go`: Curated 60-30-10 color palettes.
*   `internal/wad/`:
    *   `downloader.go`: Multi-mirror archive downloader for community megawads.
    *   `extract.go`: Zip Slip hardened archive extractor and DeHackEd ordering.
    *   `soundfont.go`: Roland SC-55 SoundFont downloader and installer.
*   **Data Synchronization**: Whenever presets or config templates are updated, synchronize both the repository
    templates and the embedded data in `internal/preset/data/` and `internal/templates/data/`.

# Naming & Code Style Conventions

*   **Formatting**: Always format code using `gofmt -s -w .` (simplifying slice and composite literal syntax).
*   **Maximum Line Length (120 Columns)**: Enforce a strict 120-character maximum line length limit across the
    repository via `.editorconfig` and Revive's `line-length-limit` rule in `revive.toml`. Long strings, command
    invocations, and error messages must be wrapped cleanly.
*   **Receiver Naming**: Use short (1-2 letters), mnemonic receiver names consistently across methods on a type.
    Never use `this` or `self`.
*   **Avoid Identifier Shadowing**: Never shadow Go built-in identifiers (`min`, `max`, `len`, `cap`, `new`,
    `clear`, `copy`, `close`, `delete`). Use descriptive identifiers such as `maxVal`, `count`, or `limit`.
*   **Documentation Comments**: All exported packages, types, interfaces, constants, and functions must have
    comprehensive Go doc comments starting with the symbol name and adhering to standard Go and `revive` conventions.
*   **Static Analysis & Linting**: Enforce clean linting via `revive -config revive.toml -formatter friendly ./...`
    and `go vet ./...`.

# Testing Conventions

*   **Table-Driven Tests**: Table-driven tests are standard for unit testing. Each test case struct must include a
    descriptive `name` string field.
*   **Hardened Concurrency & Randomization**: All tests must execute cleanly under `-race` (race detector) and
    `-shuffle=on` (test order randomization).
*   **Filesystem Isolation**: File resolution, deployment, and extraction tests must execute within isolated
    temporary directories (`t.TempDir()` or `os.MkdirTemp`) and clean up completely.
*   **Parity & Path Auditing**: Every test run includes automated validation that:
    1. Curated presets match `data/presets.json` and synchronize with `README.md`.
    2. Zero absolute personal user paths (`/home/`, `%USERPROFILE%`) exist in configuration files, presets, or
       test fixtures.

# Documentation Roles & Boundaries

*   **`README.md` (User-Facing)**: Reserved strictly for players and end-users of the `doom` CLI and source ports.
    Focuses on installation, `doom setup`, gameplay (`doom play`), preset catalogs, directory layouts, and game file
    acquisition. Must **not** contain internal developer commands, agent instructions, or compiler details.
*   **`CONTRIBUTING.md` (Contributor-Facing)**: Dedicated guide for human open-source developers. Details prerequisites,
    Makefile targets, Go code style, testing requirements, and PR guidelines.
*   **`AGENTS.md` (Agent & Architectural Invariants)**: The canonical, authoritative reference for autonomous agents,
    architecture invariants, path tokens, safety rules, and code quality checklists.
*   **Documentation Synchronization**: Whenever presets, engine settings, build targets, directory layouts, or CLI
    mechanics are modified, all relevant documentation (**Go doc comments**, **`README.md`**, **`CONTRIBUTING.md`**,
    and **`AGENTS.md`**) MUST be updated in the same pull request.

# Pull Request & Workflow Conventions

*   **Accurate & Detailed Commits**: Use imperative commit messages describing the architectural change (e.g.
    `Adopt Go tooling, static analysis, and testing patterns from collections`).
*   **Path Invariant Verification**: Verify `git diff` contains zero hardcoded personal user paths before committing.
*   **PR Template Completion**: Ensure the PR template checklist is completely satisfied.

# Continuous Learning & Principle Encoding

*   **Persist User Corrections**: Whenever an agent is corrected, redirected, or receives feedback on repository
    conventions, it **must immediately encode the underlying principle into `AGENTS.md`** before concluding the task.
    This ensures all future agent sessions automatically inherit the correction.
*   **Windows Engine Configuration Locality**: On Windows, source ports (DSDA-Doom, UZDoom) operate in portable
    mode and read configurations relative to their program directory (`$PROGDIR` / `BinDir`), NOT Unix-style AppData
    subdirectories. Deploying `dsda-doom.cfg` and `autoexec.cfg` directly to `BinDir` (defaulting to
    `%USERPROFILE%\Games\Doom\bin\`) ensures both launcher-mediated and direct executable invocations load the
    intended configs.
*   **Persistent Asset Directory Hierarchy**: Users can override and persist all three asset and execution directories
    (`wads-dir`, `bin-dir`, `soundfonts-dir`) via `doom config set <key> <val>` in `config.json`. The resolution
    hierarchy strictly respects: CLI flags (`--wads-dir`, `--bin-dir`, `--soundfonts-dir`) > Environment variables
    (`DOOM_WADS_DIR`/`WADS_DIR`, `DOOM_BIN_DIR`/`BIN_DIR`, `DOOM_SF_DIR`/`SF_DIR`) > User configuration (`config.json`)
    > Platform defaults. Engine launch synthesis and configuration deployment always adhere to this resolved hierarchy.

# Pre-Completion Verification Checklist

Before completing any changes, agents MUST run and verify the following commands succeed without errors or warnings:

1. `make format-check` (verify `gofmt -s -l .` reports zero unformatted files).
2. `make tidy-check` (verify `go mod tidy && git diff --exit-code go.mod go.sum`).
3. `make lint` (verify `go vet ./...` and `revive -config revive.toml` pass with zero warnings).
4. `make check` (runs full suite: formatting check, tidy check, linters, `go test -v -race -shuffle=on ./...`,
   preset parity invariants, and path invariant inspection verifying zero hardcoded personal user paths).
