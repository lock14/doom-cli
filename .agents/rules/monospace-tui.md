---
trigger: glob
globs: internal/tui/**/*.go, cmd/doom/play.go, cmd/doom/themes.go
---

# Monospace TUI design standard and project rules

When designing, implementing, refactoring, or reviewing terminal UI components in `internal/tui/`, `cmd/doom/play.go`,
or `cmd/doom/themes.go`, follow the embedded Monospace Design TUI (`mono-tui v0.2.5`) specification alongside the
project overrides and conventions declared in this file.

## Reference hierarchy

Consult these documents in precedence order (higher overrides lower when rules conflict):

1. **Project overrides and conventions**: The `[OVERRIDE]`, `[WAIVE]`, and `[P#]` entries in this file.
2. **Design and interaction standard**: [standard.md](../skills/mono-tui-design/references/standard.md) for screen
   and workflow archetypes, keyboard ergonomics, focus navigation, elevation, layout responsiveness, and content
   formatting.
3. **Rendering and implementation reference**: [rendering.md](../skills/mono-tui-design/references/rendering.md) for
   Unicode box-drawing tables, ANSI color tiers, terminal resize and scroll mechanics, and defensive rendering
   checklists.
4. **Canonical pattern library**: [patterns.md](../skills/mono-tui-design/references/patterns.md) for annotated ASCII
   wireframe patterns and checklist verification.
5. **Skill workflows**: [mono-tui-design](../skills/mono-tui-design/SKILL.md) for designing and implementing TUI
   features, and [mono-tui-audit](../skills/mono-tui-audit/SKILL.md) for auditing existing TUI views.

## Screen and workflow archetype mapping

The `doom play` interactive launcher (`internal/tui/`) maps to the following `mono-tui` archetypes:

*   **Screen archetype**: `§11.5` fuzzy finder (`model` in `internal/tui/launcher.go`) paired with a responsive
    `Master-Detail` split view (`renderMainContent` in `internal/tui/render.go` with the `Presets` list pane on the
    left and the `Details & Launch Info` preview pane on the right) and an `Expand-to-Focus` full-pane CP437 document
    viewer (`viewingReadme` viewport in `internal/tui/launcher.go` and `renderReadmePane` in `internal/tui/render.go`).
*   **Workflow archetype**: `§12.4` search-act. Typing filters presets in real time across name, engine, category,
    summary, and WAD filenames; highlighting a preset previews its IWAD, PWADs, DeHackEd patches, and documentation
    status; pressing `Enter` launches the selected preset, or pressing `Tab`, `Ctrl+R`, or `i` (when the filter input
    is empty) opens the bundled CP437 README in the full-pane viewer.

## Project overrides and conventions

### [OVERRIDE] §6.1 single-line rounded Lipgloss borders across panes

*   **Original**: Applications MUST NOT use double-line borders for Level 0–2 elements. Applications MUST NOT use single-line borders for Level 3–4 elements.
*   **Project rule**: `renderBoxWithTitle` in `internal/tui/render.go` uses `lipgloss.RoundedBorder()` (`╭╮╰╯` with
    single-line `─│` edges) across all launcher panes (`Presets`, `Details & Launch Info`, and `README Viewer`). Active
    and inactive focus levels are distinguished through theme border and title tokens (`BorderFgActive` and
    `TitleActive` versus `BorderFgMuted` and `TitleMuted`) rather than switching to double-line `╔╗╚╝═║` characters.
*   **Reason**: Lipgloss rounded single-line borders provide a cohesive visual style across modern terminals while
    letting color tokens and header badges communicate active pane focus.

### [OVERRIDE] §6.3 left-aligned panel titles in top border

*   **Original**: Window and panel titles MUST be centered in the top border row:
*   **Project rule**: `renderBoxWithTitle` in `internal/tui/render.go` left-aligns panel titles immediately after the
    top-left border corner (`╭── Presets (18) ──...──╮` and `╭── Details & Launch Info ──...──╮`), truncating long
    titles with `…` when the panel width is narrow.
*   **Reason**: Left-aligned panel titles keep scan lines anchored to the left margin above left-aligned preset rows
    and metadata labels across wide and narrow terminal widths.

### [OVERRIDE] §6.6 rounded corners for interactive launcher panes

*   **Original**: Rounded corner characters (`╭╮╰╯`) MAY be used for cosmetic, non-interactive containers (e.g., decorative cards, informational callouts). Rounded corners MUST NOT be used for interactive windows, dialogs, or panels that participate in the elevation system.
*   **Project rule**: Interactive launcher containers (`Presets`, `Details & Launch Info`, and `README Viewer`) use
    `lipgloss.RoundedBorder()` (`╭╮╰╯`) as their standard frame geometry.
*   **Reason**: Keeps container framing consistent with the Lipgloss component ecosystem used throughout `internal/tui/`.

### [OVERRIDE] §2.2 direct common-key bindings and always-active fuzzy search input

*   **Original**: All single-letter key bindings MUST be case-insensitive — `r` and `R` trigger the same action, `d` and `D` trigger the same action, etc. Applications MUST bind both cases but MUST display only the lowercase form in the footer key strip.
*   **Project rule**: In the primary launcher view (`!m.viewingReadme`), the fuzzy search `textinput.Model` is focused
    by default (`§11.5`) so printable characters immediately filter presets; single-letter shortcuts `q` (quit) and `i`
    (inspect README) trigger only when the filter input is empty, while modifier and special keys (`↑`, `↓`, `Ctrl+P`,
    `Ctrl+N`, `Ctrl+K`, `Ctrl+J`, `PgUp`, `PgDn`, `Ctrl+U`, `Ctrl+D`, `Home`, `End`, `Tab`, `Ctrl+R`, `Enter`, `Esc`,
    and `Ctrl+C`) work regardless of filter text. Inside the `README Viewer` (`m.viewingReadme`), Vim and pager keys
    are bound deliberately by case (`j` and `k`, `d` and `u`, `f` and `b`, `Space`, `g` for top, `G` for bottom, and
    `q`, `Esc`, `Tab`, or `Ctrl+R` to close).
*   **Reason**: Matches `fzf` and Telescope ergonomics in the fuzzy finder while preserving standard `less` and Vim
    navigation (`g` versus `G`) in the document reader.

### [OVERRIDE] §R3.2 curated 60-30-10 semantic theme palettes and custom JSON themes

*   **Original**: | Primary | 75 (light blue) | 17 (dark blue) | fg #5fafff, bg #00005f |
*   **Project rule**: All TUI colors are governed by `Theme` and `ThemeStyles` in `internal/tui/theme.go` and
    `internal/tui/builtin_themes.go`. The CLI ships 10 built-in 60-30-10 palettes (`classic` ANSI-16 default, `blood`,
    `toxic`, `inferno`, `frost`, `plasma`, `heretic`, `amber`, `sigil`, and `monochrome`) and loads custom user themes
    from `<ConfigDir>/themes/<name>.json`. Never hardcode raw ANSI color numbers or hex strings in `render.go` or
    `launcher.go`.
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

*   **Project rule**: Default all badges (` DOOM `, `[DSDA]`, `[UZDoom]`, `[Ready]`, `[Missing IWAD]`) and prompts
    (`Filter: `, `README Viewer: `) to universal ASCII and standard Unicode glyphs (`•`, `✓`, `✗`, `…`) with zero
    Unicode Private Use Area (PUA) characters or emojis. Rounded Powerlevel10k capsule caps (`U+E0B6` and `U+E0B4`) and
    Nerd Font icons in `renderCapsule` (`internal/tui/render.go`) must only render when `--nerd-fonts` or
    `nerd_fonts: true` is active (`m.nerdFonts`).
*   **Reason**: Guarantees zero tofu (`□` or `?`) in 100% of out-of-the-box system fonts while supporting opt-in
    powerline styling. See [AGENTS.md](../../AGENTS.md).

### [P2] ANSI-safe width math and responsive layout clamping

*   **Project rule**: All layout geometry (`calculateLayout` in `internal/tui/layout.go`) and line clipping
    (`truncateANSI` in `internal/tui/render.go`) must measure visible cell widths via `lipgloss.Width` / `x/ansi`,
    clamp pane dimensions down to small terminals without negative padding panics, and terminate truncated lines with
    `\x1b[0m` so styling never bleeds into adjacent borders.
*   **Reason**: Prevents right-border drift, UTF-8 byte-slicing corruption, and resize panics across narrow or rapidly
    resized terminal windows (`§7.2` and `§R4.2`).

### [P3] cached multi-root directory index on the UI thread

*   **Project rule**: `model` (`internal/tui/launcher.go`) must resolve IWAD, PWAD, and README paths exclusively
    through the pre-constructed `*preset.DirIndex` (`m.dirIndex`) and cached README map (`m.readmeCache`). Never perform
    uncached `os.ReadDir` walks inside `View()` or per-keypress `Update()` cycles.
*   **Reason**: Keeps fuzzy filtering and scrolling responsive on large multi-directory WAD collections.

### [P4] non-TTY and basic terminal numbered menu fallback

*   **Project rule**: When standard input or output is not an interactive terminal or when the fallback menu is
    invoked, `RunFallbackMenu` in `internal/tui/menu.go` provides a clean numbered selection prompt with graceful
    `EOF` and interrupt handling.
*   **Reason**: Ensures `doom play` remains usable in basic terminals, serial consoles, and non-fullscreen environments.
