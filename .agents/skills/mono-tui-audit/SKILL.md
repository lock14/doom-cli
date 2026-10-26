---
name: mono-tui-audit
description: Audit doom-cli terminal UI code against the Monospace Design TUI (mono-tui v0.2.5) specification and project rules in .agents/rules/monospace-tui.md. Use when reviewing TUI changes, checking layout and rendering compliance, or diagnosing visual and keyboard bugs in internal/tui/.
---

# Monospace TUI audit skill

Use this skill when auditing or reviewing `internal/tui/`, `cmd/doom/wads.go`, `cmd/doom/engines.go`, or
`cmd/doom/config.go` against the Monospace Design TUI (`mono-tui v0.2.5`) specification and `doom-cli`'s project rules.

## Audit reference order

1. [monospace-tui.md](../../rules/monospace-tui.md): Load all `[OVERRIDE]`, `[WAIVE]`, and `[P#]` entries first.
   Never flag an intentional project override or waiver as a violation; instead, verify the code follows the stated
   "Project rule" entry.
2. [standard.md](../mono-tui-design/references/standard.md): Check keyboard navigation, focus indicators, layout
   responsiveness, and content formatting rules that are not overridden.
3. [rendering.md](../mono-tui-design/references/rendering.md): Check `§R1` to `§R4` and `§R7` rendering rules (box
   alignment, SGR reset discipline, component measurements, and color capability degradation).
4. [patterns.md](../mono-tui-design/references/patterns.md): Compare against `Footer Command Bar`, `Focused Surface`,
   `Master-Detail`, `Expand-to-Focus`, and `Search/Select Set` checklists.
5. [AGENTS.md](../../../AGENTS.md): Verify repository-wide glyph hygiene, 120-column line limits, and test coverage.

## Step-by-step audit checklist

1. **Glyph and badge hygiene**:
   * Confirm zero emojis or Unicode Private Use Area (PUA) characters appear when `m.nerdFonts` is `false` (`[P1]`).
   * Confirm `RenderBrandPill`, `RenderStatsPill`, and `RenderScrollPill` (`internal/tui/theme.go`) as well as the
     `Launch Options` and `Configuration Manager` header pills (`renderLaunchOptionsView` in `internal/tui/options.go`
     and `renderConfigManagerView` in `internal/tui/config_view.go`) use rectangular badges by default and only call
     `renderCapsule` (`internal/tui/render.go`) with `U+E0B6` and `U+E0B4` when `m.nerdFonts` is `true`.
2. **Container framing, focus clarity, and form cursors**:
   * Confirm panes render through `renderBoxWithTitle` with `lipgloss.RoundedBorder()`, left-aligned top-border titles,
     and active focus distinguished via `BorderFgActive` and bold `TitleActive` versus `BorderFgInactive` and
     normal-weight `TitleInactive` across all four top, left, right, and bottom edges (`[OVERRIDE] §6.1`, `§6.3`, and
     `§6.6`, plus `§5.5` and `§9.6`).
   * Confirm active list and form rows in `renderListLines`, `renderLaunchOptionsView`, and `renderConfigManagerView`
     use the `2`-column `▎ ` (`CursorBar`) and bold `CursorText` or `ValueBold` cursor (`[OVERRIDE] §7.2 and §8.1`), and
     that inapplicable `(Boom)` fields (`-solonet` and `-complevel`) are dimmed via `.Faint(true)` (`SGR 2`) when a
     ZDoom-family engine is selected.
   * Confirm no drop shadows (`░`) or modal scrims are rendered on `Expand-to-Focus` full-pane views (`README Viewer`,
     `Launch Options — <Name>`, `Doom CLI & Engine Configuration`, `Configuration Diff`, and `Keybindings & Features`,
     `[WAIVE] §6.4` and `§6.5`).
3. **Theme and color token discipline**:
   * Confirm all styles in `internal/tui/render.go`, `internal/tui/options.go`, and `internal/tui/config_view.go` derive
     from `ThemeStyles` (`internal/tui/theme.go`) and work across all 10 built-in palettes in
     `internal/tui/builtin_themes.go` (`[OVERRIDE] §R3.2`, including `classic` ANSI-16 and `monochrome`).
4. **Width math, truncation, and resize safety**:
   * Confirm `computeLayout` (`internal/tui/layout.go`, with `availHeight := m.height - 6` and
     `sideBySideMinWidth = 90`), `renderFooter` (using the same `m.width >= sideBySideMinWidth` breakpoint), and
     `renderPreviewLines` (`internal/tui/render.go`) preserve `IWAD:` and `Files:` visibility and exact 24-row vertical
     parity at `80×24` (`§1.5` and `[OVERRIDE] §1.6`), and clamp widths and heights to non-negative values down to tiny
     terminal dimensions (`width < 45` or `height < 12`, `[P2]`).
   * Confirm `padOrTruncateName`, `truncateANSI`, and all six footer renderers (`renderFooter`, `renderReadmeFooter`,
     `renderHelpFooter`, `renderLaunchOptionsFooter`, `renderConfigManagerFooter`, and the inline `viewingDiff` footer)
     measure visible cell width via `lipgloss.Width` and append `…` (`U+2026`, `[OVERRIDE] §R1.7`) plus `\x1b[0m` when
     truncating.
5. **Keyboard bindings and footer parity**:
   * Confirm every active keybinding in `Update()` (`internal/tui/launcher.go`, `internal/tui/options.go`, and
     `internal/tui/config_view.go`) across launcher mode, `Launch Options` (including `!isTextField` gating on `Ctrl+K`,
     `Ctrl+J`, `←`, `→`, and `Space`), `Configuration Manager` (including unconditional `Ctrl+K` and `Ctrl+J` navigation
     and live preview rollback on `Esc`), and all three viewport overlays (`m.viewingReadme`, `s.viewingDiff`, and
     `m.viewingHelp`) is accurately reflected in its corresponding footer key strip (`[OVERRIDE] §2.2`, `§1.4`, `§2.3`,
     and `§R4.11`).
6. **UI thread performance**:
   * Confirm `View()`, `renderPreviewLines`, `renderLaunchOptionsView`, `renderConfigManagerView`, and
     `refreshPortConfigStatus` use `m.dirIndex` (`*preset.DirIndex`), `m.readmeCache`, pure zero-I/O
     `engine.FormatLaunchPreview`, and cached `s.detectedRes` and `s.detectedRate` (`templates.RenderAutoexecWithRate`
     and `templates.RenderDSDADoomWithRes`) with zero uncached filesystem directory scans or display-detection
     subprocess spawns (`[P3]`).
7. **Override freshness**:
   * Run `go test -v ./internal/preset -run TestMonospaceTUIOverridesNotStale` to verify every `### [OVERRIDE]` and
     `### [WAIVE]` section in `.agents/rules/monospace-tui.md` includes a verbatim `- **Original**:` quote from
     `standard.md`, `rendering.md`, or `patterns.md`.

## Audit report format

Group findings by severity:

* **Blocking violations**: Rules broken in `monospace-tui.md`, `standard.md`, or `rendering.md` with file path, line
  range, section reference, and concrete fix.
* **Non-blocking polish**: Minor layout, readability, or test coverage improvements prefixed with `Nit:`.
* **Verified compliant areas**: Summary of audited checks that passed.
