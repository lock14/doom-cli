# Repository Design Principles

*   **Declarative Entity Catalog & Single Source of Truth**:
    *   `catalog/*.json` (`engine.<id>.json` and `preset.<id>.json`) is the sole source of truth for all curated engine
        and preset definitions, IWAD mappings, PWAD file lists, DeHackEd orderings, metadata, and download URLs.
    *   **Single Root Embed**: All embedded assets (`catalog/*.json`, `dsda-doom/dsda-doom.cfg`, `uzdoom/autoexec.cfg`)
        are embedded once at the repository root in `embed.go` (`doomcli.FS`). Do not create duplicate asset copies
        under `internal/*/data/`.
    *   **Synchronize & Lint**: Run `doom presets build` to synchronize `catalog/*.json` into `README.md`, and run
        `doom presets lint` to validate strict schema rules, engine references, SHA-256 digests, personal path checks,
        and `README.md` table parity.
    *   **Parity Verification**: Run `make check` (or `go test -v ./internal/preset ./internal/templates`) to verify
        that `README.md` matches `catalog/*.json` and all files comply with path invariants.
*   **Portability & Path Invariants**:
    *   **Never commit personal user paths, display resolutions, or refresh rates**: Never commit absolute personal
        paths like `/home/<username>/`, hardcoded local display resolutions, or fixed monitor refresh rates into
        configuration files, catalog entities, or code.
    *   **Use Standard Placeholder Tokens**:
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
        *   **CLI Config, Themes & User Catalog**: `~/.config/doom-cli/` (Linux) /
            `~/Library/Application Support/doom-cli/` (macOS) / `%LOCALAPPDATA%\doom-cli\` (Windows), with user
            entity files in `<ConfigDir>/catalog.d/*.json`.
*   **Destructive Safety & Backup Policy**:
    *   **Mandatory Atomic Writes & Non-Destructive Backups**: Any configuration or catalog write (`doom config install`,
        `doom setup`, `doom config set`, `doom engines add`, `doom presets add`, `doom presets config`) must use
        `fsutil.WriteFileAtomic` (temp file in same directory + `os.Rename`) and `fsutil.BackupFile` to create
        non-clobbering timestamped backups (`.bak.<timestamp>`, appending `-1`, `-2` on same-second collisions) before
        overwriting any existing user file.
    *   **Preserve Unrelated Configuration**: When adding new engine variables, aliases, or keybindings, avoid
        modifying or removing unrelated settings unless explicitly requested. `config.json` preserves unknown top-level
        keys across round-trips, and `doom presets config` preserves untouched override fields across successive calls.
    *   **Archive Security & Path Containment**: All archive extractions (`archive/zip`), direct file downloads, and
        relative sub-path resolutions must sanitize paths against `..` traversal, drive-letter, and absolute path
        sequences using `fsutil.SafeJoin(root, rel)` before any filesystem read or write.
*   **Doom Engine Selection & Preset Hygiene**:
    *   **DSDA-Doom**: Use for classic vanilla, Boom, MBF, and MBF21 maps where demo accuracy, standard physics,
        and speedrunning precision are desired (e.g. *Alien Vendetta*, *BTSX*, *Sunder*, *Sunlust*, *Legacy of Rust*,
        *Sigil*).
    *   **UZDoom**: Use for mapsets requiring ZDoom/GZDoom features, advanced scripting, high-res texture packs like
        OTEX (*Eviternity I & II*), or Raven Software games (*Heretic*, *Hexen*).
    *   **Two-Layer Entity Catalog & Quarantine**: Built-in entities in `catalog/*.json` serve as the immutable curated
        layer (`LayerCurated`). User engines (`engine.<id>.json`), custom presets (`preset.<id>.json`), and surgical
        overrides (`override.preset.<id>.json`, `override.engine.<id>.json`) live in `<ConfigDir>/catalog.d/`.
        Malformed user entity files are quarantined with a diagnostic warning without breaking valid siblings.
    *   **No Duplicate IWADs**: Never include the base game IWAD (`DOOM.WAD`, `DOOM2.WAD`, etc.) inside `files`.
        The IWAD must only be specified in `iwad`.
    *   **Load Ordering**: Ensure DeHackEd/BEX patches (`.deh`, `.bex`) and resource files are ordered correctly
        relative to map PWADs and optional music wads (`{"name": "idkfa 2024.wad", "optional": true}`).
    *   **Optional Asset Degradation**: Optional files (`"optional": true`, such as `idkfa 2024.wad`) must not prevent
        presets from running when absent. Missing required files must cleanly abort execution with a descriptive error
        before engine invocation.
    *   **Filename Normalization & Alias Tolerance**: Case-insensitive and whitespace/dash/underscore-normalized
        matching across configured WAD roots (`wads_dir` and `wads_dirs`), safe relative sub-paths (`sub/foo.wad`), and
        1-level child directories (for non-IWAD files) ensures maps and patches resolve reliably. Rerelease add-on
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

*   `embed.go`: Root `embed.FS` (`doomcli.FS`) embedding `catalog/*.json`, `dsda-doom/dsda-doom.cfg`, and
    `uzdoom/autoexec.cfg`.
*   `catalog/`: Curated one-entity-per-file JSON catalog (`engine.<id>.json`, `preset.<id>.json`).
*   `cmd/doom/`: Modular Cobra CLI command implementations:
    *   `main.go`: Entrypoint, root command, global flags, and path/catalog helpers.
    *   `play.go`: `doom play` interactive launcher command.
    *   `launch.go`: `doom launch` direct preset execution command.
    *   `setup.go`: `doom setup` automated turnkey workflow command.
    *   `wads.go`: `doom wads` subcommands (`list`, `fetch`, `extract-steam` with `extract` alias).
    *   `engines.go`: `doom engines` subcommands (`list`, `add`, `remove`, `install`).
    *   `presets.go`: `doom presets` subcommands (`list`, `show`, `add`, `config`, `remove`, `build`, `lint`).
    *   `soundfont.go`: `doom soundfont install` subcommand.
    *   `config.go`: `doom config` subcommands (`show`, `get`, `set`, `toggle`, `install`, `diff`, `sync`).
    *   `themes.go`: `doom themes` subcommands (`list`, `set`).
*   `internal/config/`: Platform path resolvers adhering strictly to XDG (Linux), Library (macOS), and AppData
    (Windows), plus settings-only user configuration persistence (`config.json`).
*   `internal/display/`: Native display resolution and refresh rate auto-detection.
*   `internal/engine/`:
    *   `runner.go`: Engine execution argument synthesis and process lifecycle management.
    *   `installer.go`: Declarative source port asset downloading, SHA-256 verification, and portable binary
        installation (Zip Slip hardened).
*   `internal/fsutil/`:
    *   `fsutil.go`: Atomic file writing (`WriteFileAtomic`), non-clobbering timestamped backups (`BackupFile`), and
        path-containment validation (`CleanRel`, `SafeJoin`).
*   `internal/preset/`:
    *   `preset.go`: Declarative entity types, strict JSON decoder, and two-layer catalog loader with quarantine.
    *   `store.go`: User `catalog.d` write operations (`AddEngine`, `RemoveEngine`, `AddPreset`, `ConfigPreset`,
        `ResetPresetOverride`, `RemovePreset`).
    *   `lint.go`: Catalog schema, reference, digest, personal-path, and `README.md` parity linter (`LintCatalog`).
    *   `resolve.go`: Multi-root cached `DirIndex`, case-insensitive/whitespace-tolerant file resolution, safe relative
        sub-paths, 1-level child directory scan, and `KnownAliases` matching.
    *   `decode.go`: Document decoding with DOS Code Page 437 / CP437 box-drawing detection.
    *   `compiler.go`: `README.md` preset table generator and synchronizer (`doom presets build`).
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
    *   `downloader.go`: Multi-mirror SHA-256 verified archive and direct `.wad`/`.pk3` downloader.
    *   `extract.go`: Zip Slip hardened archive extractor and DeHackEd/BEX ordering.
    *   `soundfont.go`: Roland SC-55 SoundFont downloader and installer.

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
*   **Parallel Execution**: Use `t.Parallel()` on unit tests and table-driven subtests that do not mutate the process
    environment (`t.Setenv`) or package-level globals. Never combine `t.Parallel()` with `t.Setenv` or global CLI flags.
*   **Performance Benchmarking**: Maintain `*_bench_test.go` benchmarks for performance-sensitive subsystems (e.g.
    CP437 decoding, file resolution, Steam VDF parsing, and TUI fuzzy search/layout calculation).
*   **Hardened Concurrency & Randomization**: All tests must execute cleanly under `-race` (race detector) and
    `-shuffle=on` (test order randomization).
*   **Filesystem Isolation**: File resolution, deployment, and extraction tests must execute within isolated
    temporary directories (`t.TempDir()` or `os.MkdirTemp`) and clean up completely.
*   **Parity & Path Auditing**: Every test run includes automated validation that:
    1. Curated entities in `catalog/*.json` pass `LintCatalog` and synchronize with `README.md`.
    2. Zero absolute personal user paths (`/home/`, `%USERPROFILE%`) exist in configuration templates, catalog
       entities, or test fixtures.

# Documentation Roles & Boundaries

*   **`README.md` (User-Facing)**: Reserved strictly for players and end-users of the `doom` CLI and source ports.
    Focuses on installation, `doom setup`, gameplay (`doom play`), preset catalogs, directory layouts, and game file
    acquisition. Must **not** contain internal developer commands, agent instructions, or compiler details.
*   **`CONTRIBUTING.md` (Contributor-Facing)**: Dedicated guide for human open-source developers. Details prerequisites,
    Makefile targets, Go code style, testing requirements, and PR guidelines.
*   **`AGENTS.md` (Agent & Architectural Invariants)**: The single canonical, authoritative reference at the repository
    root for autonomous agents (including Claude Code, which natively reads `AGENTS.md`), architecture invariants, path
    tokens, safety rules, and code quality checklists. Keep `AGENTS.md` as a single root file without splitting into
    `.agents/` or creating `CLAUDE.md` symlinks.
*   **Documentation Synchronization**: Whenever presets, engine settings, build targets, directory layouts, or CLI
    mechanics are modified, all relevant documentation (**Go doc comments**, **`README.md`**, **`CONTRIBUTING.md`**,
    **`AGENTS.md`**, and **`llms.txt`**) MUST be updated in the same pull request.

# Pull Request & Workflow Conventions

*   **Accurate & Detailed Commits**: Use imperative commit messages describing the architectural change (e.g.
    `Adopt Go tooling, static analysis, and testing patterns from collections`).
*   **PR Description Synchronization**: Whenever subsequent commits in a PR branch modify, fix, or expand upon the
    original changeset (e.g. CI adjustments, bug fixes, refactoring), the PR description must be proactively updated
    via `gh pr edit` to reflect the latest state of the PR.
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
*   **Persistent Asset Directory Hierarchy**: Users can override and persist all asset and execution directories
    (`wads-dir`, `wads-dirs`, `bin-dir`, `soundfonts-dir`) via `doom config set <key> <val>` in `config.json`. The
    resolution hierarchy strictly respects: CLI flags (`--wads-dir`, `--bin-dir`, `--soundfonts-dir`) > Environment
    variables (`DOOM_WADS_DIR`/`WADS_DIR`, `DOOM_WADS_DIRS`/`WADS_DIRS`, `DOOM_BIN_DIR`/`BIN_DIR`,
    `DOOM_SF_DIR`/`SF_DIR`) > User configuration (`config.json`) > Platform defaults. Engine launch synthesis and
    configuration deployment always adhere to this resolved hierarchy.
*   **Symmetric Two-Layer Entity Catalog**: Keep user preferences (`config.json`) strictly separated from entity
    definitions (`catalog/*.json` and `<ConfigDir>/catalog.d/*.json`). User entities share the identical JSON schema as
    curated repository entities so user presets and engines can be promoted to `catalog/` by copying a single file.

# Pre-Completion Verification Checklist

Before completing any changes, agents MUST run and verify the following commands succeed without errors or warnings:

1. `make format-check` (verify `gofmt -s -l .` reports zero unformatted files).
2. `make tidy-check` (verify `go mod tidy && git diff --exit-code go.mod go.sum`).
3. `make lint` (verify `go vet ./...` and `revive -config revive.toml -set_exit_status` pass with zero warnings/errors).
4. `make check` (runs full suite: formatting check, tidy check, linters, `go test -v -race -shuffle=on ./...`,
   preset parity invariants, and path invariant inspection verifying zero hardcoded personal user paths).
5. `govulncheck ./...` (when installed, verify zero known vulnerabilities).
