---
trigger: glob
globs: internal/tui/**/*.go, cmd/doom/wads.go, cmd/doom/engines.go, cmd/doom/config.go
---

# Monospace TUI design standard and project rules

When designing, implementing, refactoring, or reviewing terminal UI components in `internal/tui/`, `cmd/doom/wads.go`,
`cmd/doom/engines.go`, or `cmd/doom/config.go`, follow the embedded Monospace Design TUI (`mono-tui v0.2.5`)
specification alongside the project overrides and conventions declared in this file.

## Reference hierarchy

Consult these documents in precedence order (higher overrides lower when rules conflict):

1. **Project overrides and conventions**: The `[OVERRIDE]`, `[WAIVE]`, and `[P#]` entries in this file.
2. **Design and interaction standard**: [standard.md](../skills/mono-tui-design/references/standard.md) for screen
   and workflow archetypes, keyboard ergonomics, focus navigation, elevation, layout responsiveness, and content
   formatting.
3. **Rendering and implementation reference**: [rendering.md](../skills/mono-tui-design/references/rendering.md) for
   Unicode box-drawing tables, layout geometry, ANSI color tiers, component measurements, and color capability
   degradation.
4. **Canonical pattern library**: [patterns.md](../skills/mono-tui-design/references/patterns.md) for reusable
   interaction patterns (`Footer Command Bar`, `Focused Surface`, `Master-Detail`, `Expand-to-Focus`, and
   `Search/Select Set`) and audit checklists.
5. **Skill workflows**: [mono-tui-design](../skills/mono-tui-design/SKILL.md) for designing and implementing TUI
   features, and [mono-tui-audit](../skills/mono-tui-audit/SKILL.md) for auditing existing TUI views.

## Screen and workflow archetype mapping

The `doom wads`, `doom engines`, and `doom config` interactive TUI views (`internal/tui/`) map to the following
`mono-tui` archetypes:

*   **Screen archetype**: `§11.5` fuzzy finder (`model` in `internal/tui/launcher.go`) paired with a responsive
    `Master-Detail` split view (`renderPanels` in `internal/tui/render.go` with the `Presets` list pane on the left
    and the `Preset Details` preview pane on the right) and `Expand-to-Focus` full-pane views for the CP437 document
    viewer (`viewingReadme` in `internal/tui/render.go`), the `§11.2` settings form for launch options and per-preset
    configuration (`viewingLaunchOpts` in `internal/tui/options.go`), the 3-tab configuration manager
    (`viewingConfigMgr` in `internal/tui/config_view.go`), and contextual help (`viewingHelp` in
    `internal/tui/render.go`).
*   **Workflow archetype**: `§12.4` search-act and `§12.2` inspect-configure. Typing filters presets in real time across
    `Name`, `Engine`, `Category`, `Compatibility`, and installation readiness (`ready`, `installed`, `missing`);
    highlighting a preset previews its IWAD, PWADs, DeHackEd patches, and documentation status; pressing `Enter`
    launches the selected preset, `Ctrl+O` or `F4` opens launch options, `Ctrl+G` or `F2` opens the 3-tab configuration
    manager, `Ctrl+F` fetches or extracts missing WADs in place, `Tab` or `Ctrl+R` opens the bundled CP437 README, and
    `F1` opens contextual help.

## Project overrides and conventions

### [OVERRIDE] §6.1 single-line rounded Lipgloss borders across panes

*   **Original**: Applications MUST NOT use double-line borders for Level 0–2 elements. Applications MUST NOT use single-line borders for Level 3–4 elements.
*   **Project rule**: Non-overlapping Level 1 split and expanded launcher panes (`Presets`, `Preset Details`, and
    `README Viewer`) use single-line `─│` edges in compliance with `§6.2` and `§5.5`, while `renderBoxWithTitle` in
    `internal/tui/render.go` overrides the sharp `┌┐└┘` corner set by rendering `lipgloss.RoundedBorder()` (`╭╮╰╯`).
    Active and inactive focus levels are distinguished through theme border and title tokens (`BorderFgActive` and bold
    `TitleActive` versus `BorderFgInactive` and normal-weight `TitleInactive`, with `.Faint(true)` `SGR 2` in
    `MonochromeTheme`) rather than double-line `╔╗╚╝═║` characters (`§5.5` and `§9.6`).
*   **Reason**: Single-line borders on non-overlapping Level 1 split panels already comply with `§6.2` and `§5.5`; this
    override specifically covers using `lipgloss.RoundedBorder()` (`╭╮╰╯`) instead of sharp `┌┐└┘` corners across
    interactive Level 1 launcher panes while color tokens, bold title weight, and `SGR 2` dim attributes communicate
    active pane focus.

### [OVERRIDE] §6.3 left-aligned panel titles in top border

*   **Original**: Window and panel titles MUST be centered in the top border row:
*   **Project rule**: `renderBoxWithTitle` in `internal/tui/render.go` left-aligns panel titles immediately after the
    top-left border corner (`╭── Presets (32) ──...──╮` and `╭── Preset Details ──...──╮`), truncating long titles
    with `…` when the panel width is narrow.
*   **Reason**: Left-aligned panel titles keep scan lines anchored to the left margin above left-aligned preset rows
    and metadata labels across wide and narrow terminal widths.

### [OVERRIDE] §6.6 rounded corners for interactive launcher panes

*   **Original**: Rounded corner characters (`╭╮╰╯`) MAY be used for cosmetic, non-interactive containers (e.g., decorative cards, informational callouts). Rounded corners MUST NOT be used for interactive windows, dialogs, or panels that participate in the elevation system.
*   **Project rule**: Interactive Level 1 launcher panes (`Presets`, `Preset Details`, and `README Viewer`) use
    `lipgloss.RoundedBorder()` (`╭╮╰╯`) instead of sharp `┌┐└┘` corners (`§6.2`) as their standard frame geometry.
*   **Reason**: Single-line borders on non-overlapping Level 1 split panels already comply with `§6.2` and `§5.5`; this
    override specifically covers using `lipgloss.RoundedBorder()` (`╭╮╰╯`) instead of sharp `┌┐└┘` corners across
    interactive Level 1 launcher panes to keep framing consistent with the Lipgloss component ecosystem used throughout
    `internal/tui/`.

### [OVERRIDE] §1.6 90-column side-by-side split breakpoint

*   **Original**: | Compact | 40–79 | Region A collapses; Region C hidden or stacked below B; footer reduces to single row |
*   **Project rule**: `computeLayout` in `internal/tui/layout.go` switches from the stacked vertical layout to the
    side-by-side split layout (`Presets` on the left, `Preset Details` on the right) at `sideBySideMinWidth = 90`
    columns (`m.width >= 90`) rather than `80` columns. When stacked (`m.width < 90`, including `80×24`),
    `renderPreviewLines` (`internal/tui/render.go`) condenses metadata rows so `IWAD:` and `Files:` remain visible
    (`§1.5`).
*   **Reason**: The left `Presets` pane requires `44` columns plus `4` border columns and a `2`-column gutter (`50`
    columns total), so stacking below `90` columns gives `Preset Details` a full `78`-column width at `80×24` instead
    of cramping it into `30` columns.

### [OVERRIDE] §7.2 and §8.1 left quarter-block cursor bar and bold highlight for list selection

*   **Original**: - Focused text MUST use reverse video (SGR 7).
*   **Project rule**: `renderListLines` in `internal/tui/render.go` indicates the active preset row using a `2`-column
    left quarter-block cursor bar (`▎ `, `U+258E` via `CursorBar`) paired with bold accent typography (`CursorText`,
    `SGR 1`) rather than full-row reverse video (`SGR 7`) or `[▸ item ◂]` bracket markers.
*   **Reason**: Preserves clean horizontal scan lines and right-aligned `[DSDA]` and `[UZDoom]` engine badges without
    full-row background blocks overpowering the `60-30-10` palette while maintaining a non-color glyph indicator
    (`▎ `) for accessibility (`§5.3` and `§9.6`).

### [OVERRIDE] §4.4 and §R4.2 frameless filter prompt without underscore fill

*   **Original**: - Empty entry fields MUST display fill characters (underscores `_` or dots `·`) to indicate their width.
*   **Project rule**: `renderHeader` in `internal/tui/render.go` and `initialModel` in
    `internal/tui/launcher.go` render the search bar as a frameless `  Filter: ` prompt followed by `m.input`
    (`Type to search presets...` placeholder) without `[...]` brackets or `_` and `·` fill characters.
*   **Reason**: Matches the header bar conventions of modern terminal fuzzy finders (`§11.5`) and avoids visual noise in
    the top brand and stats bar.

### [OVERRIDE] §1.4 and §R4.11 bullet-separated footer key strip

*   **Original**: Format: `F1 Help  F5 Refresh  / Filter  q Quit` — key name followed by action label, separated by 2+ spaces. (CUA §1 function key area, Norton Commander F-key bar)
*   **Project rule**: `ThemeStyles.FormatKeyHelp` in `internal/tui/theme.go` joins `keyHelp` pairs (`renderFooter` and
    `renderReadmeFooter` in `internal/tui/render.go`) using a muted bullet separator (`"  •  "`, `U+2022` via `Bullet`)
    rather than whitespace alone.
*   **Reason**: Clearly delineates compound key labels (such as `↑/↓/PgUp/PgDn Navigate` and `Esc/Ctrl+C Quit`) from
    adjacent action descriptions across all terminal widths.

### [OVERRIDE] §2.2 direct common-key bindings and always-active fuzzy search input

*   **Original**: All single-letter key bindings MUST be case-insensitive — `r` and `R` trigger the same action, `d` and `D` trigger the same action, etc. Applications MUST bind both cases but MUST display only the lowercase form in the footer key strip.
*   **Project rule**: In the primary launcher view (`!m.viewingReadme`), the fuzzy search `textinput.Model` (`m.input`)
    is focused by default (`§11.5`) so all printable characters immediately filter presets; modifier and special keys
    (`↑`, `↓`, `Ctrl+P`, `Ctrl+N`, `Ctrl+K`, `Ctrl+J`, `PgUp`, `PgDn`, `Ctrl+U`, `Ctrl+D`, `Tab`, `Ctrl+R`, `Enter`,
    `Esc`, and `Ctrl+C`, with `Home` and `End` moving the filter text cursor) work regardless of filter text. Inside
    the `README Viewer` (`m.viewingReadme`), Vim and pager keys are bound deliberately by case (`j` and `k`, `d` and
    `u`, `f` and `b`, `Space`, `g` and `Home` for top, `G` and `End` for bottom, and `q`, `Q`, `Esc`, `Tab`, or
    `Ctrl+R` to close).
*   **Reason**: Matches `fzf` and Telescope ergonomics in the fuzzy finder while preserving standard `less` and Vim
    navigation (`g` versus `G`) in the document reader.

### [OVERRIDE] §R3.2 curated 60-30-10 semantic theme palettes and custom JSON themes

*   **Original**: | Primary | 75 (light blue) | 17 (dark blue) | fg #5fafff, bg #00005f |
*   **Project rule**: All TUI colors are governed by `Theme` and `ThemeStyles` in `internal/tui/theme.go` and
    `internal/tui/builtin_themes.go`. The CLI ships 10 built-in 60-30-10 palettes (`classic` ANSI-16 default, `blood`,
    `toxic`, `inferno`, `frost`, `plasma`, `heretic`, `amber`, `sigil`, and `monochrome`) and loads custom user themes
    from `<ConfigDir>/themes/<name>.json`. TrueColor text tokens adhere to `§9.5` WCAG AA contrast thresholds while 30%
    structural border tokens (`BorderActive` and `BorderInactive`) use intentional atmospheric contrast, and
    `MonochromeTheme.StatusMissing` uses bold reverse video (`SGR 1;7`, `§R3.4`). Never hardcode raw ANSI color numbers
    or hex strings in `render.go` or `launcher.go`.
*   **Reason**: Preserves Doom-inspired episode and source-port palettes alongside an ANSI-16 default (`classic`) and
    high-contrast accessibility (`monochrome`).

### [OVERRIDE] §R1.7 single-character horizontal ellipsis for truncation

*   **Original**: | `⋯` | MIDLINE HORIZONTAL ELLIPSIS | U+22EF | Truncation indicator |
*   **Project rule**: `truncateANSI` and `renderBoxWithTitle` in `internal/tui/render.go` use the standard horizontal
    ellipsis `…` (`U+2026`, 1 display column) for single-line text and border title truncation.
*   **Reason**: `U+2026` has universal 1-column glyph metrics across standard monospace terminal fonts on Linux, macOS,
    and Windows.

### [WAIVE] §6.4 drop shadows on README viewer pane

*   **Original**: Shadows at Levels 2–4 MUST be rendered as a 2-column × 1-row offset below and to the right of the bordered element.
*   **Project rule**: Do not render `░` (`U+2591`) drop shadows around launcher panes or the `README Viewer` view.
*   **Reason**: The `README Viewer` uses the `Expand-to-Focus` pattern (`patterns.md`), replacing the split panes inside
    the main content region rather than floating a windowed dialog over background text.

### [WAIVE] §6.5 background scrims behind README viewer pane

*   **Original**: Level 4 (modal overlay) MUST dim all background content behind the modal. The scrim MUST apply the dim attribute (SGR 2) to all non-modal cells while preserving their character content.
*   **Project rule**: Do not composite a dimmed SGR 2 background scrim behind the `README Viewer`.
*   **Reason**: Because `README Viewer` expands to fill the content frame (`Expand-to-Focus`), there is no partially
    occluded background text underneath the reader.

### [P1] universal rectangular badges by default and opt-in Nerd Fonts capsules

*   **Project rule**: Default all badges (` DOOM `, `[DSDA]`, `[UZDoom]`, `[✓ Found]`, `[✗ Missing]`, `[Optional]`,
    `[✓ Readme]`) and prompts (`  Filter: `, `  README Viewer`) to universal ASCII and standard Unicode glyphs (`•`,
    `✓`, `✗`, `…`) with zero Unicode Private Use Area (PUA) characters or emojis. `formatEngineTagWithCatalog`
    (`internal/tui/render.go`) formats custom engine badges (`EngineConfig.BadgeText()`) as 8-column bracketed tags
    (`"[" + padOrTruncateName(badge, 6) + "]"`) to align with `[DSDA]  ` and `[UZDoom]`, styling `boom` engines with
    `TagDSDA` and `zdoom` engines with `TagUZDoom`. `RenderBrandPill`, `RenderStatsPill`, and `RenderScrollPill`
    (`internal/tui/theme.go`) must only call `renderCapsule` (`internal/tui/render.go`) with rounded Powerlevel10k
    capsule caps (`U+E0B6` and `U+E0B4`) when `--nerd-fonts`, `DOOM_NERD_FONTS`, or `nerd_fonts: true` is active
    (`m.nerdFonts`).
*   **Reason**: Guarantees zero tofu (`□` or `?`) in 100% of out-of-the-box system fonts while supporting opt-in
    powerline styling. See [AGENTS.md](../../AGENTS.md).

### [P2] ANSI-safe width math and responsive layout clamping

*   **Project rule**: All layout geometry (`computeLayout` in `internal/tui/layout.go`), list row formatting
    (`padOrTruncateName` in `internal/tui/render.go`), and line clipping (`truncateANSI` in `internal/tui/render.go`)
    must measure visible cell widths via `lipgloss.Width`, clamp pane dimensions down to small terminals without
    negative padding panics, and terminate truncated lines with `\x1b[0m` so styling never bleeds into adjacent borders.
*   **Reason**: Prevents right-border drift, UTF-8 byte-slicing corruption, and resize panics across narrow or rapidly
    resized terminal windows (`§1.5` and `§1.6`).

### [P3] cached multi-root directory index on the UI thread

*   **Project rule**: `model` (`internal/tui/launcher.go`) must resolve IWAD, PWAD, and README paths exclusively
    through the pre-constructed `*preset.DirIndex` (`m.dirIndex`, including cached relative subdirectory entries) and
    cached README map (`m.readmeCache`). Never perform uncached `os.ReadDir` walks inside `View()` or per-keypress
    `Update()` cycles.
*   **Reason**: Keeps fuzzy filtering and scrolling responsive on large multi-directory WAD collections.

### [P4] non-TTY and basic terminal numbered menu fallback

*   **Project rule**: When standard input or output is not an interactive terminal or when the fallback menu is
    invoked, `RunNumberedMenu` in `internal/tui/menu.go` provides a clean numbered selection prompt with graceful
    `EOF` and interrupt handling.
*   **Reason**: Ensures `doom wads` remains usable in basic terminals, serial consoles, and non-fullscreen environments.
