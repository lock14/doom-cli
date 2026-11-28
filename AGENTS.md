# Repository guidelines for doom-cli

## Commands and workflow

*   **Verify before completion**: Run `make format-check`, `make tidy-check`, `make lint`, `make check` (`go vet`,
    `revive`, `go test -v -race -shuffle=on ./...`, parity, and personal-path checks), and `govulncheck ./...` (when
    installed).
*   **Catalog and template maintenance**: Run `go run ./cmd/doom wads build` to regenerate the `README.md` preset table
    after editing `catalog/*.json`, and `go run ./cmd/doom wads lint` to validate catalog schemas, digests,
    documentation, frontmatter, and package boundaries (`doom wads build` and `doom engines cfg sync` are hidden
    maintainer commands with `Hidden: true`).
*   **PR and learning hygiene**: Follow imperative Chris Beams commit rules, verify `git diff` introduces zero personal
    paths, complete `.github/PULL_REQUEST_TEMPLATE.md`, update PR descriptions via `gh pr edit` on follow-up commits,
    synchronize `README.md`, `CONTRIBUTING.md`, `AGENTS.md`, `.agents/`, `skills/`, and `llms.txt` in the same PR, and
    immediately encode user corrections into `AGENTS.md` or the relevant `.agents/rules/*.md` file.

## Scoped domain rules and skills

Keep `AGENTS.md` as the single root entry point (do not create `.agents/AGENTS.md` or `CLAUDE.md` symlinks). Read the
corresponding domain file before modifying its area:

*   **Catalog and WAD curation**: [catalog-curation.md](.agents/rules/catalog-curation.md) covers `dsda-doom` vs
    `uzdoom` engine pairing, the explicit `complevel` matrix, edition and taxonomy rules, canonical archive filenames vs
    `files[].aliases`, DeHackEd ordering, and `idkfa 2024.wad` soundtrack hygiene.
*   **Markdown and documentation**: [documentation-style.md](.agents/rules/documentation-style.md) covers typography
    (one format per span, sentence-case headings, zero decorative emojis, zero `---` dividers, zero ASCII arrows or `/`
    alternatives in prose), Diátaxis player-only `README.md` boundaries, and commit and PR formatting.
*   **Terminal UI**: [monospace-tui.md](.agents/rules/monospace-tui.md),
    [mono-tui-design](.agents/skills/mono-tui-design/SKILL.md), and
    [mono-tui-audit](.agents/skills/mono-tui-audit/SKILL.md) cover `mono-tui v0.2.5` overrides, `80x24` and `90`-column
    responsive geometry, emoji- and PUA-free default glyphs (`nerd-fonts` opt-in), and footer key parity.

## Architecture and schema invariants

*   **Two-layer catalog and baseline isolation**: `catalog/*.json` (`LayerCurated`, embedded via root `embed.go`) and
    `<ConfigDir>/catalog.d/*.json` (`LayerUser` entities and `override.*` files) share an identical schema for
    single-file `cp` promotion, `<ConfigDir>/portcfg/{dsda-doom.cfg,autoexec.cfg}` layers portable user keybinds and
    CVARs onto curated `configs/` templates (`doom engines cfg save` and `reset`, stripped during maintainer `sync`),
    and `config.json` stores CLI settings only. `LoadCurated`, `LoadLayered`, and
    `MergeUserDir` must deep-copy all slices and maps and reset `loadEmbeddedCurated` baselines before merging
    `catalog.d` so reloads are idempotent and never mutate cached arrays. Quarantine malformed `catalog.d` files without
    breaking siblings, and block shadowing curated engine IDs or aliases or deleting `LayerCurated` entities.
*   **Typed domain fields over string-scraping heuristics**: Never overload untyped flag slices and scrape, strip, or
    rewrite CLI strings across engine families at runtime. Model cross-engine concepts as typed fields (`complevel`
    `*int` `0..17` or `21`, synthesized as `-complevel <N>` on `args-style: "boom"` and omitted naturally on `"zdoom"`),
    scope port-specific flags in `engine-args` keyed by canonical engine ID (`ResolveEngineAlias`), and keep `args`
    universal. In overrides (`OverrideSet`, `OverridePatchRequest`, `PresetConfigOptions`), preserve tri-state semantics
    distinguishing unchanged (`nil`), cleared or unset (`ComplevelUnset = -1` or empty slice or map pointer), and
    replaced values.
*   **Strict kebab-case and zero legacy shims**: Use hyphenated `kebab-case` across all `doom-cli`-owned JSON files
    (`config.json`, `catalog`, `catalog.d`, `themes`), CLI flags, and `doom config` keys. Set `DisallowUnknownFields()`
    and reject trailing tokens on entity and theme decoders (`config.json` preserves unknown top-level keys while
    deduplicating case variants on save); only structs decoding external APIs (such as GitHub Releases in
    `internal/engine/installer.go`) mirror upstream wire casing. Never introduce deprecated flag aliases, env fallbacks,
    shorthand key shims, or single-root wrapper functions; make clean breaks and update callers and tests directly.
*   **Two-tier settings, disk-as-state, and lookup safety**: Settings resolve strictly as `config.json` over platform
    defaults, with `DOOM_CONFIG_DIR` as the sole app environment variable and zero global persistent flags or
    per-setting `DOOM_*` variables. Never store installed-game state in `config.json`: derive optional games from IWADs
    on disk via `InstalledOptionalGames`, default `doom setup` to 4 core steps (`--wads none`), require an explicit
    `<name[,name...]|all>` target on `doom wads fetch`, skip presets missing their base IWAD during bulk `DownloadAll`,
    and default `doom wads`, `doom wads list`, and `RunNumberedMenu` to `LibraryPresets` (`--all` and TUI `F3` or
    `Ctrl+T` toggle full catalog, auto-falling back when 0 presets are installed). Read and launch commands use
    `Catalog.Find`, whereas mutating or destructive commands (`set`, `reset`, `remove`, `uninstall`) must use
    `Catalog.FindExact` or `FindEngine` without substring guessing.
*   **Five-pillar CLI and unified launch synthesis**: Organize commands around `doom [wad] [flags]`, `doom wads`,
    `doom engines`, `doom config`, and `doom setup` (plus environment helpers `doom completion` and `doom skill`, which
    embeds player-only [SKILL.md](skills/doom-cli/SKILL.md) separately from contributor `.agents/skills/`), restricting
    `DisableFlagParsing: true` + `UnknownFlags: true` strictly to root `doom` (zero `reflect` on unexported Cobra or
    pflag fields), enforcing `cobra.NoArgs` on non-positional commands and `exactArgsWithHint` on positional
    subcommands, and wiring `signal.NotifyContext` into `ExecuteContext`. `PrepareLaunch` (`filepath.Abs`
    canonicalization) and pure zero-I/O `FormatLaunchPreview` must share `resolveEngineConfig` and `buildEngineArgs`;
    route file readiness checks through `DirIndex.ResolveFileSpec`.

## Safety, portability, and code boundaries

*   **Portability and Windows locality**: Never commit personal paths, resolutions, or refresh rates; use
    `__RESOLUTION__`, `__REFRESH_RATE__`, and `__SOUNDFONT__` tokens in `configs/`, and preserve UZDoom's Nightdive
    "Software-Plus" profile (`F5`, `F6`, `F9` only, without movement or weapon binds) and DSDA-Doom's software lightmaps
    and WASD Profile 0 defaults. On Windows, deploy `dsda-doom.cfg` and `autoexec.cfg` directly to `BinDir` (`$PROGDIR`
    portable locality). Bound OS display probes to `1500ms` (`display.DetectDisplay()`), probing once per diff, deploy,
    or TUI session. Skip existing binaries and `.cfg` files unless `--force` is passed, check all `WadRoots` (`WadsDir`
    and read-only `WadsDirs`) before downloading or walking game libraries (narrowing Steam roots to `steamapps/common`
    and supporting `--from` custom directories), and resolve Steam symlinks up to depth 15 with cycle guards.
*   **Atomic writes, verified downloads, and OS confinement**: All config and catalog writes must use `fsutil`
    post-validation atomic rename (`writeReaderTempAndRename`) with bounded backups (`MaxBackupsPerFile = 3`, cleaned up
    via `RemoveBackups`), preserving untouched fields across partial updates. `wad.UninstallPreset` deletes only
    preset-exclusive files in primary `WadsDir`, protecting base IWADs, shared assets (`idkfa 2024.wad`, `VOICES.WAD`),
    symlinks escaping `WadsDir`, and secondary `WadsDirs`. Route all outbound HTTP downloads through
    `fsutil.DownloadVerifiedContext` (mandatory curated `sha256`, HTTPS downgrade rejection, bounded reads), sanitize
    paths via `fsutil.CleanRel` and `fsutil.SafeJoin`, and confine engine binary and `.app` extraction via `os.OpenRoot`
    with post-extraction symlink-chain verification.
*   **Package boundaries, Go discipline, and CI security**: Sibling runtime packages under `internal/` must never import
    maintainer-only `internal/repolint`. Use singular domain package names (`portcfg`, `wadfmt`, `fsutil`), centralize
    binary `IWAD` and `PWAD` lump parsing and streaming building in `internal/wadfmt` (`MaxLumps = 65536`,
    `io.ReaderAt`), split `.go` and co-located `*_test.go` files before roughly `900` to `1,000` lines, enforce the
    strict 120-column line limit (`revive.toml`) without shadowing Go built-ins, never combine `t.Parallel()` with
    `t.Setenv`, and bind untrusted `${{ github.event.* }}` values through `env:` in `.github/workflows/*.yml`.
