---
trigger: glob
description: "Curated WAD and engine catalog rules, complevels, aliases, load ordering, and soundtrack hygiene."
globs: "catalog/*.json, internal/catalog/**/*.go, internal/repolint/**/*.go, internal/engine/runner.go, internal/wad/**/*.go, internal/steam/**/*.go"
---

# Curated catalog and WAD hygiene rules

Follow these rules whenever adding, modifying, linting, extracting, or launching curated presets and engines in
`catalog/*.json`, `internal/catalog/`, `internal/repolint/`, `internal/engine/runner.go`, `internal/wad/`, or
`internal/steam/`. See [AGENTS.md](../../AGENTS.md) for repository-wide invariants.

## Engine selection and compatibility levels

*   **DSDA-Doom default**: Assign `dsda-doom` to all classic vanilla (`complevel` `2`, `3`, `4`),
    limit-removing (`2`, `3`), Boom (`9`), MBF (`11`), MBF21 (`21`), and ID24 (`21`) Doom presets where demo accuracy,
    authentic engine physics, custom 256-color software palettes, and speedrunning precision are desired (including
    *Alien Vendetta*, *Ancient Aliens*, *AUGER;ZENITH*, *BTSX*, *Eviternity II*, *Going Down Turbo*, *Legacy of Rust*,
    *Overboard*, *Rowdy Rudy II*, *Sigil*, *Sunder*, *Sunlust*, and *Valiant*). OTEX is a standard-resolution, 8-bit
    paletted texture pack and runs natively in DSDA-Doom.
*   **UZDoom exceptions**: Reserve `uzdoom` strictly for mapsets requiring ZScript or UDMF scripting
    (*MyHouse.pk3*), pre-`UMAPINFO` mapsets whose chapter selection menus, episode pistol starts, and `ACS` weather
    effects rely on ZDoom `ZMAPINFO` and `ACS` (*Eviternity*, which still declares `"complevel": 11` for `-e dsda-doom`
    or `-e woof` overrides), and Raven Software or Rogue Entertainment games (*Heretic*, *Hexen*, *Strife*).
*   **Explicit Boom compatibility levels**: Because `configs/dsda-doom/dsda-doom.cfg` sets
    `default_compatibility_level 21`, every `dsda-doom` preset (and any `uzdoom` preset playable on Boom ports such as
    `eviternity`) must declare an explicit `"complevel"` integer (`2` for Doom v1.9 and Doom II, `3` for Ultimate Doom
    and Sigil, `4` for Final Doom TNT and Plutonia, `9` for Boom, `11` for MBF, and `21` for MBF21 and ID24) so older
    mapsets lacking a `COMPLVL` lump never run under MBF21 physics by accident.

## Three-tier declarative launch configuration

*   **Typed domain fields over flag-stripping heuristics**: Separate preset launch configuration into three orthogonal
    fields rather than embedding port-specific flags in universal slices and scraping strings at runtime:
    1. `complevel` (`*int` `0..17` or `21`; `ComplevelUnset = -1` in `OverrideSet.Complevel`): Synthesized automatically
       as `-complevel <N>` by `buildEngineArgs` (`internal/engine/runner.go`) for any `args-style: "boom"` engine
       unless `-complevel` is already present in `engine-args`, `args`, or CLI extra arguments, and omitted naturally on
       `args-style: "zdoom"`.
    2. `engine-args` (`map[string][]string` keyed by canonical engine ID, plus `OverrideSet.EngineArgs`,
       `Override.AppendEngineArgs`, and repeatable `--engine-args <engine>=<flags>`): Port-specific launch flags
       normalized through `ResolveEngineAlias` and validated against registered engines by `LintCatalog`.
    3. `args` (`[]string`, plus `append-args` and `--args`): Universal engine-agnostic flags (`-skill`, `-warp`,
       `-fast`, `-nomonsters`, `-respawn`).

## IWAD separation, game classification, and edition policy

*   **No duplicate IWADs and DRY game inference**: Never place the base game IWAD (`DOOM.WAD`, `DOOM2.WAD`, `TNT.WAD`,
    `PLUTONIA.WAD`, `HERETIC.WAD`, `HEXEN.WAD`, `STRIFE1.WAD`) inside `files`; specify it only in `iwad`.
    `catalog.InferGameFromIWAD` infers `"doom"` (`catalog.CoreGame`) for Doom-family IWADs so all Doom presets omit
    `"game"` (preserving single-file `cp` promotion from `<ConfigDir>/catalog.d/`), whereas curated non-Doom presets
    (`heretic`, `hexen`, `strife`) must explicitly declare `"game"`, validated by `LintCatalog`.
*   **Curated edition policy and taxonomy metadata**: Every `LayerCurated` preset in `catalog/preset.<id>.json` must
    declare `author`, `release-date`, `category`, `genre`, `difficulty`, `quality`, `links` (with `PrimaryURL()`
    rendered as a clickable title in [README.md](../../README.md)), and a unique positive `sort` integer spaced by 10s.
    When an author releases a sanctioned compatibility update or rebalanced edition (such as *Going Down Turbo*
    (`gdturbo.wad`) for `mouldy`'s 2024 remaster of *Going Down*, or `Veddge`'s April 2025 update of *MyHouse.pk3* for
    GZDoom 4.14 and UZDoom), curate the definitive edition rather than keeping broken older mirrors, and route shorthand
    queries (`doom gd`, `doom going-down`) to the canonical preset via `NormalizePresetQuery`.

## Filenames, aliases, load ordering, and soundtrack hygiene

*   **Canonical archive filenames and single-source aliases**: `files[].name` must match the exact canonical filename
    inside the upstream `.zip` archive (`Eviternity II.wad`, `Sunder 2512.wad`, `SODfinal.wad`, `aaliens.wad`,
    `scythe.wad`, `TNTR.WAD`). Reserve `files[].aliases` strictly for genuine external distribution or Steam and GOG
    rerelease filename stems (`sigil.wad`, `sigil2.wad`, `sigil_ii.wad`, `sunder.wad`), never for case, space, dash, or
    underscore variants handled automatically by `catalog.NormalizeFilename`. Both `catalog.DirIndex` and
    `steam.DefaultTargetPatterns()` derive lookup tables directly from `files[].aliases`.
*   **DeHackEd extraction, load ordering, and README normalization**: Order `.deh` and `.bex` patches after map `.wad`
    files (`runner.go` separates `.deh` and `.bex` into `-deh` on Boom ports while preserving relative order). When a
    preset declares both a `.wad` and `.deh` file and its direct `.wad` download embeds a `DEHACKED` lump (such as
    `DoomZero.wad`), `wad.Downloader` extracts the lump atomically via `internal/wadfmt`. During `.zip` extraction,
    `orderReadmeCandidates` ranks companion `.txt` manuals by stem, alias, and preset name (skipping `license.txt`) and
    renames generic `readme.txt` fallbacks to `<primaryWadStem>.txt` so manuals never collide in `wads-dir`;
    `DirIndex.ResolveReadme` matches versioned suffixes via `IsReadmeSuffix` without matching sequel numbers (`2`) or
    borrowing base IWAD `.txt` files for PWAD presets.
*   **Soundtrack and subset WAD hygiene**: Attach optional `{"name": "idkfa 2024.wad", "optional": true}` only to
    presets that rely on stock Doom or Doom II `D_*` MIDI slots without a bespoke replacement soundtrack (`doom`,
    `doom-ii`, `tnt-evilution`, `the-plutonia-experiment`, `master-levels`, `no-rest-for-the-living`, `deathless`,
    `doom-the-way-id-did`, `doom-ii-the-way-id-did`, `doom-zero`, `nostalgia`). Never attach `idkfa 2024.wad` to
    expansions or megawads that bundle original custom music (such as `legacy-of-rust`, `sigil`, `ancient-aliens`, or
    `eviternity-ii`), where loading `idkfa 2024.wad` after the map PWAD overwrites custom soundtrack lumps. Never list
    modder-only standalone subset WADs (`id1-res.wad`, `id1-weap.wad`) alongside full runtime archives (`id24res.wad`
    and `id1.wad`), and never add `extras.wad` to preset `files`: `steam.TargetPatternsForCatalog` automatically
    includes `extras.wad` whenever `idkfa 2024.wad` is referenced, ranking Nightdive `rerelease/` paths ahead of legacy
    `base/` paths and synthesizing `<WadsDir>/idkfa 2024.wad` via `steam.SynthesizeIDKFAWAD`.

## Verified sources and synchronization workflow

*   **HTTPS URLs and SHA-256 digests**: Every `sources[].url` must use `https://` and end in `.zip`, `.wad`, or `.pk3`
    (preferring full `.zip` releases that bundle `.txt` manuals as the primary mirror, with Doomworld web pages placed
    in `links[]` using `"rel": "idgames"`). Every `LayerCurated` source must declare a verified lowercase 64-character
    hex `sha256` digest (`sha256` remains optional on `LayerUser` presets in `catalog.d/`).
*   **Build and lint parity**: After editing any file in `catalog/`, run `go run ./cmd/doom wads build` to regenerate
    the preset table in [README.md](../../README.md) and `go run ./cmd/doom wads lint` to verify schema, engine
    reference, digest, and table parity invariants.
