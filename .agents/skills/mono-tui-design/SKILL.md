---
name: mono-tui-design
description: Design and build terminal user interfaces for doom-cli following the Monospace Design TUI (mono-tui v0.2.5) specification and project rules. Use when adding, modifying, or styling TUI views, layouts, themes, or keybindings in internal/tui/ or cmd/doom/.
---

# Monospace TUI design skill

Use this skill when designing, implementing, or refactoring terminal UI views, layouts, themes, or keyboard interactions
in `internal/tui/`, `cmd/doom/wads.go`, `cmd/doom/engines.go`, or `cmd/doom/config.go`.

## Reference reading order

Always resolve rules in this order (higher overrides lower):

1. [monospace-tui.md](../../rules/monospace-tui.md): Project-specific `[OVERRIDE]`, `[WAIVE]`, and `[P#]` rules for
   `doom-cli`.
2. [standard.md](references/standard.md): The Monospace Design TUI (`mono-tui v0.2.5`) interaction, layout, keyboard,
   and archetype specification.
3. [rendering.md](references/rendering.md): Character encodings, layout geometry, ANSI color palettes, component
   measurements, and color capability degradation (`§R1` to `§R4` and `§R7`).
4. [patterns.md](references/patterns.md): Canonical interaction patterns and audit checklists (`Footer Command Bar`,
   `Focused Surface`, `Master-Detail`, `Expand-to-Focus`, and `Search/Select Set`).
5. [AGENTS.md](../../../AGENTS.md): Repository-wide architecture, glyph hygiene, testing, and code style invariants.

## Design and implementation workflow

1. **Identify the screen and workflow archetype**:
   * Confirm how the change fits into `doom wads`'s `§11.5` fuzzy finder and `Master-Detail` split view (`Presets` list
     on the left, `Preset Details` preview on the right in `renderPanels`) or `Expand-to-Focus` full-pane
     `README Viewer` (`renderReadmeView`, `§12.4` search-act workflow).
2. **Apply project overrides and glyph hygiene**:
   * Frame panes via `renderBoxWithTitle` (`internal/tui/render.go`) using `lipgloss.RoundedBorder()`, left-aligned
     titles, and `focused = true` on active panes (`Presets` and `README Viewer`) versus `focused = false` on inactive
     panes (`Preset Details`, `[OVERRIDE] §6.1`, `§6.3`, `§6.6`).
   * Keep default rendering 100% free of emojis and Unicode Private Use Area (PUA) glyphs (`[P1]`). Gate Nerd Font
     powerline capsule caps (`U+E0B6` and `U+E0B4`) in `RenderBrandPill`, `RenderStatsPill`, and `RenderScrollPill`
     (`internal/tui/theme.go`) strictly behind `m.nerdFonts` (`--nerd-fonts`, `DOOM_NERD_FONTS`, or `nerd_fonts: true`).
   * Use semantic `ThemeStyles` tokens from `internal/tui/theme.go` and `internal/tui/builtin_themes.go`
     (`[OVERRIDE] §R3.2`). Never hardcode raw ANSI codes or hex colors in view renderers.
3. **Preserve keyboard ergonomics**:
   * In the primary launcher view (`!m.viewingReadme`), all printable characters feed the live fuzzy search input
     (`m.input`) per `[OVERRIDE] §2.2`.
   * Keep modifier and navigation bindings (`↑`, `↓`, `Ctrl+P`, `Ctrl+N`, `Ctrl+K`, `Ctrl+J`, `PgUp`, `PgDn`, `Ctrl+U`,
     `Ctrl+D`, `Tab`, `Ctrl+R`, `Enter`, `Esc`, and `Ctrl+C`, with `Home` and `End` moving the filter text cursor)
     working regardless of filter state, and bind `g`, `Home`, `G`, `End`, `q`, `Q`, `Esc`, `Tab`, and `Ctrl+R` inside
     `m.viewingReadme`.
   * Keep the footer key strips (`renderFooter` and `renderReadmeFooter` in `internal/tui/render.go`) synchronized with
     active bindings for the current mode (`!m.viewingReadme` versus `m.viewingReadme`).
4. **Enforce defensive rendering and responsive geometry**:
   * Update `computeLayout` (`internal/tui/layout.go`) and `renderPreviewLines` (`internal/tui/render.go`) when pane
     heights or margins change (`[P2]` and `[P3]`), clamping dimensions so small terminals (`80×24` or smaller resize
     events) never trigger negative width or height panics and never clip `IWAD:` or `Files:` rows.
   * Measure strings with `lipgloss.Width` (`padOrTruncateName`) and truncate with `truncateANSI` (`…`,
     `[OVERRIDE] §R1.7`).
   * Query WAD and README availability through `m.dirIndex` (`*preset.DirIndex`) and `m.readmeCache` rather than
     performing uncached filesystem scans inside `View()`.
5. **Verify with tests and benchmarks**:
   * Add or update table-driven unit tests in `internal/tui/launcher_test.go` and benchmarks in
     `internal/tui/launcher_bench_test.go`.
   * Run `make check` to validate formatting, `revive` linting, race-enabled tests, and documentation invariants.
