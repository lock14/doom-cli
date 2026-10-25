# Repository design principles

*   **Declarative entity catalog and single source of truth**:
    *   `catalog/*.json` (`engine.<id>.json` and `preset.<id>.json`) is the sole source of truth for all curated engine
        and preset definitions, IWAD mappings, PWAD file lists, DeHackEd orderings, metadata, and download URLs.
    *   **Single root embed**: All embedded assets (`catalog/*.json`, `dsda-doom/dsda-doom.cfg`, `uzdoom/autoexec.cfg`)
        are embedded once at the repository root in `embed.go` (`doomcli.FS`). Do not create duplicate asset copies
        under `internal/*/data/`.
    *   **Synchronize and lint**: Run `doom wads build` to synchronize `catalog/*.json` into `README.md`, and run
        `doom wads lint` to validate strict schema rules, engine references, SHA-256 digests, personal path checks,
        and `README.md` table parity.
    *   **Parity verification**: Run `make check` (or `go test -v ./internal/preset ./internal/templates`) to verify
        that `README.md` matches `catalog/*.json` and all files comply with path invariants.
*   **Portability and path invariants**:
    *   **Never commit personal user paths, display resolutions, or refresh rates**: Never commit absolute personal
        paths like `/home/<username>/`, hardcoded local display resolutions, or fixed monitor refresh rates into
        configuration files, catalog entities, or code.
    *   **Use standard placeholder tokens**:
        *   In `dsda-doom/dsda-doom.cfg`, `screen_resolution` must use `__RESOLUTION__` and `snd_soundfont` must
            use `__SOUNDFONT__`.
        *   In `uzdoom/autoexec.cfg`, `fluid_patchset` must use `__SOUNDFONT__` and `vid_maxfps` must use
            `__REFRESH_RATE__`.
        *   Deployment tooling (`doom engines cfg install`, `doom engines install`, `doom setup`) dynamically
            substitutes `__RESOLUTION__` (via native display detection), `__REFRESH_RATE__`, and `__SOUNDFONT__` at
            installation time.
    *   **Respect platform standard directories**:
        *   **UZDoom configuration**: `~/.config/uzdoom/autoexec.cfg` on Linux,
            `~/Library/Application Support/uzdoom/` on macOS, and
            `%USERPROFILE%\Games\Doom\bin\autoexec.cfg` on Windows.
        *   **DSDA-Doom configuration**: `~/.local/share/dsda-doom/dsda-doom.cfg` on Linux,
            `~/Library/Application Support/dsda-doom/` on macOS, and
            `%USERPROFILE%\Games\Doom\bin\dsda-doom.cfg` on Windows.
        *   **Binaries**: `~/.local/bin/` on Linux and macOS, and `%USERPROFILE%\Games\Doom\bin\` on Windows.
        *   **WADs directory**: `~/.local/share/games/doom/` on Linux,
            `~/Library/Application Support/games/doom/` on macOS, and `%USERPROFILE%\Games\Doom\wads\` on Windows.
        *   **SoundFonts**: `~/.local/share/soundfonts/` on Linux,
            `~/Library/Application Support/soundfonts/` on macOS, and `%USERPROFILE%\Games\Doom\soundfonts\` on Windows.
        *   **CLI configuration, themes, and user catalog**: `~/.config/doom-cli/` on Linux,
            `~/Library/Application Support/doom-cli/` on macOS, and `%LOCALAPPDATA%\doom-cli\` on Windows (overridable
            via `DOOM_CONFIG_DIR`), with user entity files in `<ConfigDir>/catalog.d/*.json`.
*   **Destructive safety and backup policy**:
    *   **Mandatory atomic writes and non-destructive backups**: Any configuration or catalog write
        (`doom engines cfg install`, `doom engines install`, `doom setup`, `doom config set`, `doom engines add`,
        `doom engines set`, `doom wads add`, `doom wads set`, and `Ctrl+S` in the TUI `Launch Options` and
        `Configuration Manager` views) must write atomically and create non-clobbering timestamped backups
        (`.bak.<timestamp>`, appending `-1` and `-2` on same-second collisions and retaining at most
        `MaxBackupsPerFile = 3` per file) before overwriting any existing user file. `fsutil.WriteFileAtomic`,
        `fsutil.WriteReaderAtomic`, `fsutil.CopyFileAtomic`, and `fsutil.DownloadVerified` share
        `writeReaderTempAndRename`, which streams to a same-directory `.doom-tmp-*` file, applies `Chmod`, `Sync`, and
        `Close`, and runs any validation callback (including SHA-256 verification) before calling `fsutil.BackupFile`
        and `os.Rename`, so failed or corrupt writes never create spurious `.bak.*` files. When deleting user entities
        or resetting overrides (`doom engines remove`, `doom engines reset`, `doom wads remove`, `doom wads reset`, and
        `Ctrl+R` in the TUI `Launch Options` and `Configuration Manager` views), clean up `.bak.*` files via
        `fsutil.RemoveBackups`. `doom engines cfg sync` (`templates.SyncConfigs`) also calls `fsutil.RemoveBackups` on
        repository template files so `.bak.*` files never dirty the Git working tree.
    *   **Preserve unrelated configuration**: When adding new engine variables, aliases, or keybindings, avoid
        modifying or removing unrelated settings unless explicitly requested. `config.json` preserves unknown top-level
        keys across round-trips, and `doom wads set` and `doom engines set` preserve untouched override fields
        across successive calls.
    *   **Verified HTTP downloads and archive security**: All outbound HTTP downloads (`engine.Installer`,
        `wad.Downloader`, and `wad.InstallSoundFontURL`) must use `fsutil.DownloadVerified` and `fsutil.UserAgent`
        (`"doom-cli/2.0"`). All archive extractions (`archive/zip`), direct file downloads, and relative sub-path
        resolutions must sanitize paths against `..` traversal, drive-letter, and absolute path sequences using
        `fsutil.SafeJoin(root, rel)` before any filesystem read or write.
*   **Doom engine selection and preset hygiene**:
    *   **DSDA-Doom**: Use for classic vanilla, Boom, MBF, and MBF21 maps where demo accuracy, standard physics,
        and speedrunning precision are desired (such as *Alien Vendetta*, *BTSX*, *Sunder*, *Sunlust*,
        *Legacy of Rust*, and *Sigil*).
    *   **UZDoom**: Use for mapsets requiring ZDoom or GZDoom features, advanced scripting, high-resolution texture
        packs like OTEX (*Eviternity I & II*), or Raven Software games (*Heretic*, *Hexen*).
    *   **Two-layer entity catalog and quarantine**: Built-in entities in `catalog/*.json` serve as the immutable
        curated layer (`LayerCurated`). User engines (`engine.<id>.json`), custom presets (`preset.<id>.json`), and
        surgical overrides (`override.preset.<id>.json`, `override.engine.<id>.json`) live in
        `<ConfigDir>/catalog.d/`. Malformed user entity files are quarantined with a diagnostic warning without
        breaking valid siblings.
    *   **No duplicate IWADs**: Never include the base game IWAD (`DOOM.WAD`, `DOOM2.WAD`) inside `files`.
        The IWAD must only be specified in `iwad`.
    *   **Load ordering**: Ensure DeHackEd and BEX patches (`.deh`, `.bex`) and resource files are ordered correctly
        relative to map PWADs and optional music WADs (`{"name": "idkfa 2024.wad", "optional": true}`).
    *   **Optional asset degradation**: Optional files (`"optional": true`, such as `idkfa 2024.wad`) must not prevent
        presets from running when absent. Missing required files must cleanly abort execution with a descriptive error
        before engine invocation.
    *   **Unified engine launch synthesis and zero-I/O preview parity**: `PrepareLaunch` and `FormatLaunchPreview` share
        `resolveEngineConfig` and `buildEngineArgs` so argument ordering (`PrefixArgs`, `-iwad`, `-file`, `-deh`,
        `DefaultArgs`, preset `Args`, `ExtraArgs`), `LoadMapsAfterMods` sorting, and missing optional file omission
        (when `idx != nil`) never diverge between live execution, `doom wads show`, and the TUI `Launch Options`
        command preview.
    *   **Four-tier preset readiness and acquisition predicates**: `DirIndex.IsPresetInstalled` checks the base IWAD
        plus required files for launch readiness and search indexing; `DirIndex.ArePresetFilesInstalled` checks
        required files only for `wad.Downloader` (reusing a single `DirIndex` across `DownloadAll` and rebuilding only
        when an archive downloads); `DirIndex.NeedsSteamDiscovery` checks for no download sources, a missing IWAD, or
        missing optional files to gate Steam and GOG extraction during `doom wads fetch <name>` and TUI `Ctrl+F`; and
        `areAllPresetFilesPresent` checks the IWAD plus all required and optional files so TUI `Ctrl+F` runs when only
        optional assets like `idkfa 2024.wad` are missing, with all `FileSpec` lookups routed through
        `DirIndex.ResolveFileSpec`.
    *   **Filename normalization and declarative catalog aliases**: Case-insensitive and whitespace-, dash-, and
        underscore-normalized matching (`preset.NormalizeFilename`) across configured WAD roots (`wads_dir` and
        `wads_dirs`), safe relative sub-paths (`sub/foo.wad`), 1-level child directories (for non-IWAD files), and
        Steam and GOG extraction ensures maps and patches resolve without redundant case or punctuation entries. When
        upstream archives or rerelease add-ons use a genuinely distinct filename stem (such as `sigil.wad` for
        `SIGIL_V1_23.wad`, `sigil2.wad` or `sigil_ii.wad` for `SIGIL_II_V1_0.WAD`, and `sunder.wad` for
        `Sunder 2512.wad`), declare `"aliases"` directly on the `FileSpec` in `catalog/preset.<id>.json`. Both
        `preset.DirIndex` and `steam.DefaultTargetPatterns()` derive their mappings automatically from the catalog
        rather than maintaining separate hardcoded filename tables.
    *   **Embedded DeHackEd extraction**: When a preset declares both a `.wad` and `.deh` file and its direct `.wad`
        download source embeds a `DEHACKED` lump (such as `DoomZero.wad`), `wad.Downloader` extracts the embedded
        `DEHACKED` lump atomically into the expected `.deh` file.
    *   **Remaster WAD precedence and soundtrack synthesis**: During Steam and GOG discovery,
        `steam.DiscoverAndExtract` ranks `rerelease/` candidate paths ahead of legacy 4:3 `base/` paths for all
        extracted files (`DOOM.WAD`, `DOOM2.WAD`, `TNT.WAD`, `PLUTONIA.WAD`, `extras.wad`) so Nightdive's
        widescreen-enabled IWADs and remaster assets take precedence when both exist. If no standalone `idkfa 2024.wad`
        is present on disk, `steam.SynthesizeIDKFAWAD` automatically extracts the `H_` prefixed Ogg Vorbis lumps from
        the *DOOM + DOOM II (2024)* `extras.wad` resource file, maps their lump names to `D_` (including reused Doom I
        and Doom II map music slots) for classic source port compatibility, and writes a valid `PWAD` atomically to
        `<WadsDir>/idkfa 2024.wad`.
    *   **Visual aesthetic and frame pacing**: Maintain UZDoom's curated Nightdive "Software-Plus" visual profile
        (software light mode `gl_lightmode 0`, banded stepping `gl_bandedsw 1`, palette tonemapping `gl_tonemap 3`,
        and nearest-neighbor texture sampling with 16x anisotropic filtering). In DSDA-Doom, maintain classic software
        lightmaps (`render_doom_lightmaps 1`), banded sector fading (`gl_fade_mode 1`), and textured widescreen status
        bar borders (`sts_solid_bg_color 0`), alongside `dsda_fps_limit 0`, `render_vsync 1`, and
        `uncapped_framerate 1` so high-refresh monitors pace frames smoothly to native monitor refresh rates.
*   **TUI styling and font glyph hygiene**:
    *   **Monospace TUI specification and overrides**: Terminal UI work in `internal/tui/`, `cmd/doom/wads.go`,
        `cmd/doom/engines.go`, and `cmd/doom/config.go` is governed by `.agents/rules/monospace-tui.md` and the embedded
        Monospace Design TUI (`mono-tui v0.2.5`) reference library under `.agents/skills/mono-tui-design/` and
        `.agents/skills/mono-tui-audit/`.
    *   **Clean typography over emojis**: Emojis render with fixed vendor bitmaps and colors regardless of
        ANSI foreground escapes. They clash with light backgrounds (such as monochrome white pills). Use clean
        typography (`DOOM`) instead of emojis for branded badges.
    *   **Universal fallback by default**: Avoid Unicode Private Use Area (PUA) glyphs (such as ``, ``, ``, ``)
        in standard rendering. Default to universal rectangular badges and plain text prompts (`Filter: `,
        `README Viewer`) that render flawlessly in 100% of standard system fonts. Keep Powerlevel10k rounded
        capsule ends opt-in via `nerd_fonts: true` (`doom config set nerd-fonts on`).

# Package structure and architecture conventions

The repository features a unified, zero-dependency Go CLI (`doom`) that compiles to a single static binary on
Linux, macOS, and Windows:

*   `embed.go`: Root `embed.FS` (`doomcli.FS`) embedding `catalog/*.json`, `dsda-doom/dsda-doom.cfg`, and
    `uzdoom/autoexec.cfg`.
*   `catalog/`: Curated one-entity-per-file JSON catalog (`engine.<id>.json`, `preset.<id>.json`).
*   `cmd/doom/`: Modular Cobra CLI command implementations:
    *   `main.go`: Entrypoint, root direct game launcher (`doom [wad] [flags]`), and path and catalog
        helpers.
    *   `wads.go`: `doom wads` interactive TUI browser and subcommands (`list`, `show`, `fetch`, `scan`, `add`, `set`,
        `reset`, `remove`, `lint`, and hidden `build`).
    *   `engines.go`: `doom engines` interactive TUI engine manager and subcommands (`list`, `install`, `add`, `set`,
        `reset`, `remove`, and `cfg` with `install`, `diff`, and hidden `sync`).
    *   `config.go`: `doom config` interactive TUI settings editor and subcommands (`show`, `get`, `set`, `themes`).
    *   `setup.go`: `doom setup` automated turnkey workflow command.
*   `internal/config/`:
    *   `paths.go`: Platform path resolvers (`ResolveFor`, `WadRoots`, `ParsePathList`) adhering strictly to XDG
        (Linux), Library (macOS), and AppData (Windows).
    *   `settings.go`: Settings-only user configuration persistence (`config.json` via `AppConfig`), explicit
        `nerd_fonts` state tracking, and unknown top-level key preservation.
*   `internal/display/`: Native display resolution and refresh rate auto-detection.
*   `internal/engine/`:
    *   `runner.go`: Engine execution argument synthesis (`PrepareLaunch`, including `.deh` and `.bex` ordering), pure
        zero-I/O command preview formatting (`FormatLaunchPreview`), and process lifecycle management.
    *   `installer.go`: Declarative source port asset downloading, SHA-256 verification, and portable binary
        installation (Zip Slip hardened).
*   `internal/fsutil/`:
    *   `fsutil.go`: Atomic file writing (`WriteFileAtomic`, `WriteReaderAtomic`, `CopyFileAtomic`), SHA-256 verified
        HTTP downloading (`DownloadVerified`), non-clobbering timestamped backups (`BackupFile`, retaining at most
        `MaxBackupsPerFile = 3` per file, and `RemoveBackups`), and path-containment validation (`CleanRel`,
        `SafeJoin`).
*   `internal/preset/`:
    *   `preset.go`: Declarative entity types, strict JSON decoder, two-layer catalog loader with quarantine
        (`LoadLayeredCatalog`, `MergeUserDir`, `LoadPresetBaseAndOverride`), and query normalization
        (`NormalizePresetQuery`).
    *   `store.go`: User `catalog.d` write operations (`AddEngine`, `ConfigEngine`, `ResetEngineOverride`,
        `RemoveEngine`, `AddPreset`, `ConfigPreset`, `ResetPresetOverride`, `RemovePreset`).
    *   `lint.go`: Catalog schema, reference, digest, personal-path, `README.md` parity, and documentation typography,
        link, anchor, and JSON code block linter (`LintCatalog`, `lintDocumentation`).
    *   `resolve.go`: Multi-root cached `DirIndex`, case-insensitive and whitespace-tolerant file resolution, safe
        relative sub-paths, 1-level child directory scan, catalog alias matching (`ResolveFileSpec`), preset readiness
        checking (`IsPresetInstalled`, `ArePresetFilesInstalled`, `NeedsSteamDiscovery`), and version-aware README
        resolution (`ResolveReadme`, `IsReadmeSuffix`).
    *   `decode.go`: Document decoding with DOS Code Page 437 (CP437) box-drawing detection.
    *   `compiler.go`: `README.md` preset table generator and synchronizer (`doom wads build`).
*   `internal/steam/`: Multi-library `libraryfolders.vdf` parsing, catalog-driven target synthesis
    (`TargetPatternsForCatalog`), game file extraction, and `idkfa 2024.wad` synthesis from `extras.wad`
    (`SynthesizeIDKFAWAD`).
*   `internal/templates/`: Embedded engine configuration templates and timestamped backup deployer.
*   `internal/tui/`: Interactive Bubble Tea fuzzy launcher:
    *   `launcher.go`: State machine, update loop, in-TUI asset acquisition (`Ctrl+F`), and message routing.
    *   `options.go`: Unified launch options and per-preset configuration view (`Ctrl+O` or `F4`).
    *   `config_view.go`: 3-tab in-TUI configuration manager (`Ctrl+G` or `F2`).
    *   `layout.go`: Dynamic responsive layout geometry and dimension clamping.
    *   `render.go`: UI view rendering, badges, status lines, contextual help (`F1`), and key help formatting.
    *   `menu.go`: Fallback numbered terminal menu for basic or non-TTY environments.
    *   `theme.go`: Theme loader, custom JSON theme parser, and style builders.
    *   `builtin_themes.go`: Curated 60-30-10 color palettes.
*   `internal/wad/`:
    *   `downloader.go`: Multi-mirror SHA-256 verified archive and direct `.wad` and `.pk3` downloader.
    *   `extract.go`: Zip Slip hardened archive extractor, embedded `DEHACKED` lump extractor (`ExtractWADLump`), and
        ranked README (`.txt`) extractor with `<primaryStem>.txt` fallback normalization.
    *   `soundfont.go`: Roland SC-55 SoundFont downloader and installer.

# Naming and code style conventions

*   **Formatting**: Always format code using `gofmt -s -w .` (simplifying slice and composite literal syntax).
*   **Maximum line length (120 columns)**: Enforce a strict 120-character maximum line length limit across the
    repository via `.editorconfig` and Revive's `line-length-limit` rule in `revive.toml`. Long strings, command
    invocations, and error messages must be wrapped cleanly.
*   **Receiver naming**: Use short (1 to 2 letters), mnemonic receiver names consistently across methods on a type.
    Never use `this` or `self`.
*   **Avoid identifier shadowing**: Never shadow Go built-in identifiers (`min`, `max`, `len`, `cap`, `new`,
    `clear`, `copy`, `close`, `delete`). Use descriptive identifiers such as `maxVal`, `count`, or `limit`.
*   **Documentation comments**: All exported packages, types, interfaces, constants, and functions must have
    comprehensive Go doc comments starting with the symbol name and adhering to standard Go and `revive` conventions.
*   **Static analysis and linting**: Enforce clean linting via `revive -config revive.toml -formatter friendly ./...`
    and `go vet ./...`.

# Testing conventions

*   **Table-driven tests**: Table-driven tests are standard for unit testing. Each test case struct must include a
    descriptive `name` string field.
*   **Parallel execution**: Use `t.Parallel()` on unit tests and table-driven subtests that do not mutate the process
    environment (`t.Setenv`) or package-level globals. Never combine `t.Parallel()` with `t.Setenv` or global CLI flags.
*   **Performance benchmarking**: Maintain `*_bench_test.go` benchmarks for performance-sensitive subsystems (such as
    CP437 decoding, file resolution, Steam VDF parsing, and TUI fuzzy search and layout calculation).
*   **Hardened concurrency and randomization**: All tests must execute cleanly under `-race` (race detector) and
    `-shuffle=on` (test order randomization).
*   **Filesystem isolation**: File resolution, deployment, and extraction tests must execute within isolated
    temporary directories (`t.TempDir()` or `os.MkdirTemp`) and clean up completely.
*   **Parity and path auditing**: Every test run includes automated validation that:
    1. Curated entities in `catalog/*.json` pass `LintCatalog` and synchronize with `README.md`.
    2. Zero absolute personal user paths exist in configuration templates, catalog entities, documentation, or test
       fixtures.
    3. Documentation entity and theme JSON code blocks decode cleanly (`TestDocumentationEntityJSONBlocks` and
       `TestReadmeCustomThemeJSONBlock`), `mono-tui v0.2.5` verbatim override quotes in `.agents/rules/monospace-tui.md`
       remain fresh (`TestMonospaceTUIOverridesNotStale`), and built-in themes satisfy WCAG AA contrast thresholds
       (`TestBuiltinThemesWCAGContrast`).

# Technical writing and typography conventions

*   **Typography and formatting restraint**:
    *   **One format per span**: Never combine bold, italics, code font, or hyperlinks on the same text (no bolded
        code spans, no bolded links, and no bold-italics).
    *   **Strict role separation**:
        *   **Code font**: Use backticks strictly for literal code, CLI commands, flags, file paths, configuration
            keys, and keyboard shortcuts. Keep sentence punctuation outside backticks.
        *   **Bold**: Use bold (`**...**`) only for UI labels and run-in list headings followed by a colon (`:`).
            Never use a dash after a run-in heading, and never use bold for mid-sentence emphasis or whole clauses.
        *   **Italics**: Use italics (`_..._` or `*...*`) only when introducing a term on first use or for titles of
            standalone works (such as game and megawad titles in prose).
    *   **Sentence-case headings and table headers**: Use sentence case for all document titles, section headings, and
        table column headers (capitalizing only the first word and proper nouns like `CLI`, `TUI`, `WAD`, `WADs`,
        `IWAD`, `PWAD`, `UZDoom`, `DSDA-Doom`, `Linux`, `macOS`, `Windows`, `JSON`, `ANSI`, `Nerd Fonts`, `README`,
        `Go`, `Makefile`, and `Git`), with no trailing punctuation.
    *   **No decorative symbols or arrows in prose**: Omit decorative emojis, GitHub emoji shortcodes, LaTeX math
        expressions, ASCII arrows, and inequality signs as prose connectors outside code blocks and diagrams. Write
        plain English verbs ("maps to", "takes precedence over") instead.
    *   **No horizontal section dividers**: Do not place horizontal rules (`---`) between sections; Markdown headings
        provide structural hierarchy.
    *   **Clean punctuation and list discipline**: Do not use italic parenthetical asides for notes, tips, or
        alternatives; state important details as normal sentences or table entries. Do not use slashes (`/`) in prose
        to join alternatives (write "or" or "and" instead), reserving `/` for file paths, URLs, and literal commands
        or keys.
*   **Surface-specific standards**:
    *   **READMEs**: Follow Diátaxis structure in `README.md` (concise summary, quick start, task guides, reference
        tables, and troubleshooting). Apply the dual representation rule so interactive keybindings, CLI flags, and
        environment variables appear both conceptually in prose and tabularly in reference tables. Document only
        behavior verified against the current codebase (zero aspirational documentation).
    *   **Pull request descriptions**: Follow the Google CL Description style with a standalone imperative title, a
        summary that explains "why" before "what" grouped by subsystem, and a lean checklist without decorative emojis.
    *   **Issues**: Structure bug reports around four factual sections (environment, numbered steps to reproduce,
        expected behavior, and actual behavior with terminal logs) and frame feature proposals around the workflow
        problem before proposing a CLI or schema design.
    *   **Comments**: Start every exported Go doc comment with the symbol name and state its contract. Keep
        implementation comments focused on "why" a non-obvious constraint exists rather than "what" the code does, and
        prefix non-blocking review polish suggestions with `Nit:`.
    *   **Commit messages**: Follow Chris Beams' seven rules: imperative subject line of 50 characters or fewer (hard
        cap at 72) with no trailing period, a blank line, and a 72-character wrapped plain-text body explaining what
        and why.

# Documentation roles and boundaries

*   **User-facing documentation**: `README.md` is reserved strictly for players and end-users of the `doom` CLI and
    source ports. Focuses on installation, `doom setup`, gameplay (`doom` and `doom wads`), preset catalogs, directory
    layouts, and game file acquisition. Must not contain internal developer commands, agent instructions, or compiler
    details. Repository-maintainer subcommands (`doom wads build`, `doom engines cfg sync`) must set `Hidden: true` on
    their Cobra commands so player-facing `--help` output matches `README.md`.
*   **Contributor-facing documentation**: `CONTRIBUTING.md` is the dedicated guide for human open-source developers.
    Details prerequisites, Makefile targets, Go code style, testing requirements, and pull request guidelines.
*   **Agent and architectural invariants**: `AGENTS.md` is the single canonical, authoritative reference at the
    repository root for autonomous agents (including Claude Code, which natively reads `AGENTS.md`), architecture
    invariants, path tokens, safety rules, and code quality checklists. Keep `AGENTS.md` at the repository root
    without moving it into `.agents/AGENTS.md` or creating `CLAUDE.md` symlinks; scoped conditional rules and agent
    skills live in `.agents/rules/` and `.agents/skills/`.
*   **Documentation synchronization**: Whenever presets, engine settings, build targets, directory layouts, or CLI
    mechanics are modified, all relevant documentation (Go doc comments, `README.md`, `CONTRIBUTING.md`, `AGENTS.md`,
    and `llms.txt`) must be updated in the same pull request.

# Pull request and workflow conventions

*   **Accurate and detailed commits**: Use imperative commit messages describing the architectural change (such as
    `Adopt Go tooling, static analysis, and testing patterns from collections`).
*   **Pull request description synchronization**: Whenever subsequent commits in a pull request branch modify, fix, or
    expand upon the original changeset (such as CI adjustments, bug fixes, or refactoring), the pull request
    description must be proactively updated via `gh pr edit` to reflect the latest state.
*   **Path invariant verification**: Verify `git diff` contains zero hardcoded personal user paths before committing.
*   **Pull request template completion**: Ensure the pull request template checklist is completely satisfied.

# Continuous learning and principle encoding

*   **Persist user corrections**: Whenever an agent is corrected, redirected, or receives feedback on repository
    conventions, it must immediately encode the underlying principle into `AGENTS.md` before concluding the task.
    This ensures all future agent sessions automatically inherit the correction.
*   **Windows engine configuration locality**: On Windows, source ports (DSDA-Doom, UZDoom) operate in portable
    mode and read configurations relative to their program directory (`$PROGDIR` or `BinDir`), not Unix-style AppData
    subdirectories. Deploying `dsda-doom.cfg` and `autoexec.cfg` directly to `BinDir` (defaulting to
    `%USERPROFILE%\Games\Doom\bin\`) ensures both launcher-mediated and direct executable invocations load the
    intended configurations.
*   **Five-pillar CLI dispatch, TUI entry modes, and root argument parsing**: The CLI is organized around five primary
    commands (`doom [wad] [flags]`, `doom wads`, `doom engines`, `doom config`, and `doom setup`). `doom wads`,
    `doom engines`, and `doom config` enter `RunInteractiveLauncherWithOptions` using `InitialModeWads`,
    `InitialModeEngines`, and `InitialModeConfig` on interactive terminals and fall back to `RunNumberedMenu`,
    `runEnginesList`, and `runConfigShow` on non-TTY streams. Pressing `Enter` in the TUI enforces an in-TUI
    missing-files guard (`!m.isPresetReady(cur)`) or exits for a single-shot launch, where `runWadsTUI` reloads disk
    state via `loadAppState()` and forwards `selected.EngineOverridden` to `PrepareLaunch`. All 12 non-positional
    commands enforce `Args: cobra.NoArgs`, while `UnknownFlags: true` is restricted strictly to root
    `doom [wad] [flags]`, where `parseRootInvocation` joins unquoted multi-word WAD names (`doom Doom II`) and forwards
    native Doom single-dash flags (`-episode`, `-extratics`, `-endoom`, `-height`, `-viddump`) and `+` console
    commands.
*   **Persistent asset directory hierarchy and two-tier settings precedence**: Users can override and persist all asset
    and execution directories (`wads-dir`, `wads-dirs`, `bin-dir`, `soundfonts-dir`) and UI preferences (`theme`,
    `nerd-fonts`) via `doom config set <key> <val>` in `config.json` or via the in-TUI `Ctrl+G` configuration manager.
    Settings resolve in a strict two-tier hierarchy (`config` in `config.json` takes precedence over platform `default`
    values), with `DOOM_CONFIG_DIR` as the sole application environment variable and zero global persistent flags or
    per-setting `DOOM_*` environment variables. Subcommand-specific flags (`--engine` or `-e` and `--dry-run` on root
    `doom`; `--engine` or `-e` on `doom wads add` and `doom wads set`; `--force` on `doom engines install`,
    `doom wads fetch`, `doom wads scan`, and `doom setup`) are scoped locally to their respective Cobra commands.
    Engine launch synthesis, configuration deployment, `doom config show`, and `doom config get` always adhere to this
    resolved hierarchy.
*   **Symmetric two-layer entity catalog, re-merge idempotency, and query normalization**: Keep user preferences
    (`config.json`) strictly separated from entity definitions (`catalog/*.json` and `<ConfigDir>/catalog.d/*.json`).
    User entities share the identical JSON schema as curated repository entities so user presets and engines can be
    promoted to `catalog/` by copying a single file. `MergeUserDir` resets curated entities to their
    `loadEmbeddedCurated()` baselines before applying `catalog.d` so repeated in-TUI reloads and override resets are
    idempotent, `LoadPresetBaseAndOverride` loads unmerged base presets and `exclude_files` overrides for
    `Launch Options`, `NormalizePresetQuery` normalizes `.wad` and `.pk3` extensions and Roman numerals across `Find`
    and `FindExact`, `RemovePreset` and `RemoveEngine` protect `LayerCurated` entities from deletion, `LintCatalog`
    gates repository-only template, `README.md`, and documentation checks behind `hasDiskCatalog`, and each
    configuration diff, deploy, or TUI `Configuration Manager` session probes `display.DetectDisplay()` once and passes
    the result to `RenderAutoexecWithRate` and `RenderDSDADoomWithRes`.
*   **Engine installer idempotency, multi-root WAD discovery, and setup failure reporting**: `doom setup` and
    `doom engines install` must check for existing executable binaries in `BinDir` and existing `.cfg` files on disk
    and skip redundant asset downloads or config overwrites unless `--force` is specified. Both `wad.NewDownloader` and
    `steam.DiscoverAndExtract` check all configured `WadRoots` (`WadsDir` and secondary read-only `WadsDirs`) before
    downloading, extracting, or synthesizing files into primary `WadsDir`. `steam.GetSearchRoots` and
    `steam.DiscoverAndExtract` resolve symlinks via `filepath.EvalSymlinks` so Linux Steam symlinks (`~/.steam/steam`,
    `~/.steam/root`) are deduplicated and traversed by `filepath.WalkDir`, traversing up to depth 15 to discover game
    files inside Steam Proton prefixes (`compatdata/<id>/pfx/...`). `doom setup` emits step warnings to `stderr`
    (`cmdErr(cmd)`) and exits non-zero if any step fails.
*   **Catalog-driven WAD aliases and Steam target synthesis**: Never duplicate WAD filenames or alias tables across
    `internal/preset/resolve.go` and `internal/steam/discover.go`, and never add aliases that only differ by case,
    spaces, dashes, or underscores (which `preset.NormalizeFilename` matches automatically and `LintCatalog` rejects).
    Declare true alternate filename stems once in `files[].aliases` inside `catalog/preset.<id>.json`;
    `preset.DirIndex` and `steam.DefaultTargetPatterns()` synthesize their lookup tables directly from the catalog.
*   **Canonical upstream archive filenames and README hygiene**:
    *   `files[].name` in `catalog/preset.<id>.json` must match the exact canonical filename inside the upstream `.zip`
        archive (`Eviternity II.wad`, `Sunder 2512.wad`, `SODfinal.wad`, `aaliens.wad`, `scythe.wad`, `TNTR.WAD`).
        Reserve `files[].aliases` strictly for genuine external distribution or Steam and GOG rerelease filename stems
        (`sigil.wad`, `sigil2.wad`, `sigil_ii.wad`, `sunder.wad`).
    *   During `.zip` extraction (`internal/wad/extract.go`), `orderReadmeCandidates` ranks `.txt` files by stem, alias,
        and preset name (skipping `license.txt`), and normalizes generic fallback manuals (such as `readme.txt`) to
        `<primaryWadStem>.txt` so archives never clobber each other in `WadsDir`. `DirIndex.ResolveReadme` matches
        versioned suffixes via `IsReadmeSuffix` (`readme*`, `v<digit>*`, `<digit>.<digit>*`) without matching sequel
        numbers (`2`) or borrowing base IWAD `.txt` files for PWAD presets.
*   **Zero backwards-compatibility shims or legacy aliases**: Never introduce or retain deprecated flag aliases,
    unprefixed environment variable fallbacks, shorthand config or theme key aliases, hidden v1 flags, or single-root
    wrapper functions. Make clean breaks and update callers and tests directly.

# Pre-completion verification checklist

Before completing any changes, agents must run and verify the following commands succeed without errors or warnings:

1. `make format-check` (verify `gofmt -s -l .` reports zero unformatted files).
2. `make tidy-check` (verify `go mod tidy && git diff --exit-code go.mod go.sum`).
3. `make lint` (verify `go vet ./...` and `revive -config revive.toml -set_exit_status` pass with zero warnings or
   errors).
4. `make check` (runs full suite: formatting check, tidy check, linters, `go test -v -race -shuffle=on ./...`,
   preset parity invariants, and path invariant inspection verifying zero hardcoded personal user paths).
5. `govulncheck ./...` (when installed, verify zero known vulnerabilities).
