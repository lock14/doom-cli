---
name: mono-tui-audit
description: Audit doom-cli terminal UI code against the Monospace Design TUI (mono-tui v0.2.5) specification and project rules in .agents/rules/monospace-tui.md. Use when reviewing TUI changes, checking layout and rendering compliance, or diagnosing visual and keyboard bugs in internal/tui/.
---

# Monospace TUI audit skill

Use this skill when auditing or reviewing `internal/tui/`, `cmd/doom/play.go`, or `cmd/doom/themes.go` against the
Monospace Design TUI (`mono-tui v0.2.5`) specification and `doom-cli`'s project rules.

## Audit reference order

1. [monospace-tui.md](../../rules/monospace-tui.md): Load all `[OVERRIDE]`, `[WAIVE]`, and `[P#]` entries first.
   Never flag an intentional project override or waiver as a violation; instead, verify the code follows the stated
   "Project rule" entry.
2. [standard.md](../mono-tui-design/references/standard.md): Check keyboard navigation, focus indicators, layout
   responsiveness, and content formatting rules that are not overridden.
3. [rendering.md](../mono-tui-design/references/rendering.md): Check `§R6` defensive rendering checklists (box
   alignment, ANSI reset discipline, scroll bounds, resize safety, and color contrast).
4. [patterns.md](../mono-tui-design/references/patterns.md): Compare against `Master-Detail`, `Expand-to-Focus`,
   `Responsive Breakpoints`, and `Full-Screen Application Shell` checklists.
5. [AGENTS.md](../../../AGENTS.md): Verify repository-wide glyph hygiene, 120-column line limits, and test coverage.

## Step-by-step audit checklist

1. **Glyph and badge hygiene**:
   * Confirm zero emojis or Unicode Private Use Area (PUA) characters appear when `m.nerdFonts` is `false` (`[P1]`).
   * Confirm `renderCapsule` uses rectangular brackets or spaces by default and only emits `U+E0B6` and `U+E0B4` when
     `m.nerdFonts` is `true`.
2. **Container framing and focus clarity**:
   * Confirm panes render through `renderBoxWithTitle` with `lipgloss.RoundedBorder()`, left-aligned top-border titles,
     and active focus distinguished via `BorderFgActive` and `TitleActive` (`[OVERRIDE] §6.1`, `§6.3`, and `§6.6`).
   * Confirm no drop shadows (`░`) or modal scrims are rendered on the `Expand-to-Focus` `README Viewer` pane (`[WAIVE]
     §6.4` and `§6.5`).
3. **Theme and color token discipline**:
   * Confirm all styles in `internal/tui/render.go` derive from `ThemeStyles` (`internal/tui/theme.go`) and work across
     all 10 built-in palettes in `internal/tui/builtin_themes.go` (`[OVERRIDE] §R3.2`, including `classic` ANSI-16 and
     `monochrome`).
4. **Width math, truncation, and resize safety**:
   * Confirm `calculateLayout` (`internal/tui/layout.go`) clamps widths and heights to non-negative values down to tiny
     terminal dimensions (`width < 45` or `height < 12`, `[P2]` and `§R6`).
   * Confirm `truncateANSI` measures visible cell width and appends `…` (`U+2026`, `[OVERRIDE] §R1.7`) plus `\x1b[0m`
     when truncating.
5. **Keyboard bindings and footer parity**:
   * Confirm every active keybinding in `Update()` (`internal/tui/launcher.go`) for both launcher mode and
     `README Viewer` mode is accurately reflected in `renderFooter` (`internal/tui/render.go`, `[OVERRIDE] §2.2` and
     `§2.5`).
6. **UI thread performance**:
   * Confirm `View()` and `renderDetailsContent()` use `m.dirIndex` (`*preset.DirIndex`) and `m.readmeCache` with zero
     uncached filesystem directory scans (`[P3]`).
7. **Override freshness**:
   * Run `go test -v ./internal/preset -run TestMonospaceTUIOverridesNotStale` to verify every `- **Original**:` quote
     in `.agents/rules/monospace-tui.md` still matches `standard.md` or `rendering.md` verbatim.

## Audit report format

Group findings by severity:

* **Blocking violations**: Rules broken in `monospace-tui.md`, `standard.md`, or `rendering.md` with file path, line
  range, section reference, and concrete fix.
* **Non-blocking polish**: Minor layout, readability, or test coverage improvements prefixed with `Nit:`.
* **Verified compliant areas**: Summary of audited checks that passed.
