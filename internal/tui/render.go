package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/lock14/doom-cli/internal/preset"
)

type keyHelp struct {
	key  string
	desc string
}

func formatKeyHelp(items []keyHelp) string {
	return defaultStyles.FormatKeyHelp(items)
}

func resolveBadgeText(catalog *preset.Catalog, engine string) string {
	if catalog != nil {
		if eng, ok := catalog.FindEngine(engine); ok {
			return eng.BadgeText()
		}
	}
	if curated, err := preset.LoadCuratedCatalog(); err == nil {
		if eng, ok := curated.FindEngine(engine); ok {
			return eng.BadgeText()
		}
	}
	return (preset.EngineConfig{ID: engine}).BadgeText()
}

func formatEngineTagWithCatalog(catalog *preset.Catalog, engine string, styles ThemeStyles) string {
	badge := resolveBadgeText(catalog, engine)
	switch {
	case strings.EqualFold(badge, "DSDA"):
		return styles.TagDSDA.Render("[DSDA]") + "  "
	case strings.EqualFold(badge, "UZDoom"):
		return styles.TagUZDoom.Render("[UZDoom]")
	default:
		runes := []rune(badge)
		if len(runes) > 6 {
			badge = string(runes[:6])
		}
		formatted := fmt.Sprintf("[%-6s]", badge)
		return styles.TagUZDoom.Render(formatted)
	}
}

func formatEngineTag(engine string, styles ThemeStyles) string {
	return formatEngineTagWithCatalog(nil, engine, styles)
}

func (m model) renderListLines(geom layoutGeometry) []string {
	if len(m.filtered) == 0 {
		return []string{"  No presets matching search."}
	}

	startIdx := 0
	if m.cursor >= geom.maxVisible {
		startIdx = m.cursor - geom.maxVisible + 1
	}
	endIdx := startIdx + geom.maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
		startIdx = endIdx - geom.maxVisible
		if startIdx < 0 {
			startIdx = 0
		}
	}

	var listLines []string
	for i := startIdx; i < endIdx; i++ {
		p := m.filtered[i]
		tag := formatEngineTagWithCatalog(m.catalog, p.Engine, m.styles)

		name := p.Name
		runes := []rune(name)
		if len(runes) > 30 {
			name = string(runes[:29]) + "…"
		}
		paddedName := fmt.Sprintf("%-30s", name)

		if i == m.cursor {
			selected := m.styles.CursorBar.Render("▎ ") + m.styles.CursorText.Render(paddedName) + " " + tag
			listLines = append(listLines, selected)
		} else {
			listLines = append(listLines, "  "+paddedName+" "+tag)
		}
	}
	return listLines
}

func resolveEngineDescription(catalog *preset.Catalog, engineKey string) string {
	if catalog != nil {
		if eng, ok := catalog.FindEngine(engineKey); ok {
			if eng.Description != "" {
				return eng.Description
			}
			return eng.EffectiveDisplayName()
		}
	}
	if curated, err := preset.LoadCuratedCatalog(); err == nil {
		if eng, ok := curated.FindEngine(engineKey); ok {
			if eng.Description != "" {
				return eng.Description
			}
			return eng.EffectiveDisplayName()
		}
	}
	return engineKey
}

func (m model) renderPreviewLines() ([]string, bool) {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return []string{m.styles.Help.Render("No preset details available.")}, false
	}

	cur := m.filtered[m.cursor]
	readmePath, hasReadme := m.resolveReadme(cur)

	var previewLines []string
	previewLines = append(previewLines,
		fmt.Sprintf("%s%s", m.styles.Label.Render("Preset:        "), m.styles.ValueBold.Render(cur.Name)),
	)
	if cur.IsCustom() {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Type:          "), m.styles.TagUZDoom.Render("[Custom WAD]")),
		)
	} else if cur.Overridden {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Type:          "), m.styles.TagUZDoom.Render("[Overridden]")),
		)
	}
	if cur.Author != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Author:        "), cur.Author),
		)
	}
	if cur.ReleaseDate != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Released:      "), cur.ReleaseDate),
		)
	}
	engStr := resolveEngineDescription(m.catalog, cur.Engine)
	previewLines = append(previewLines,
		fmt.Sprintf("%s%s", m.styles.Label.Render("Engine:        "), engStr),
	)
	if len(cur.Args) > 0 {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Launch Args:   "), strings.Join(cur.Args, " ")),
		)
	}
	if cur.Category != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Category:      "), cur.Category),
		)
	}
	if cur.Compatibility != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Compatibility: "), cur.Compatibility),
		)
	}
	if cur.Description != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Description:   "), cur.Description),
		)
	}

	// Check IWAD
	iwadStatus := m.styles.StatusMissing.Render("[✗ Missing]")
	if _, ok := m.resolveFile(cur.IWAD); ok {
		iwadStatus = m.styles.StatusReady.Render("[✓ Found]")
	}
	previewLines = append(previewLines,
		fmt.Sprintf("%s%s %s", m.styles.Label.Render("IWAD:          "), cur.IWAD, iwadStatus),
	)

	if len(cur.Files) > 0 || hasReadme {
		previewLines = append(previewLines, m.styles.Label.Render("Files:"))
		for _, f := range cur.Files {
			fStatus := m.styles.StatusMissing.Render("[✗ Missing]")
			if _, ok := m.resolveFile(f.Name); ok {
				fStatus = m.styles.StatusReady.Render("[✓ Found]")
			} else if f.Optional {
				fStatus = m.styles.Help.Render("[Optional]")
			}
			previewLines = append(previewLines, fmt.Sprintf("  - %-22s %s", f.Name, fStatus))
		}
		if hasReadme {
			previewLines = append(previewLines,
				fmt.Sprintf("  - %-22s %s", filepath.Base(readmePath), m.styles.TagUZDoom.Render("[✓ Readme]")),
			)
		}
	}

	return previewLines, hasReadme
}

func renderBoxWithTitle(content string, width int, title string, focused bool, styles ...ThemeStyles) string {
	s := defaultStyles
	if len(styles) > 0 {
		s = styles[0]
	}

	var boxStyle lipgloss.Style
	var borderFg lipgloss.Style
	var titleStyle lipgloss.Style

	if focused {
		boxStyle = s.BoxActive
		borderFg = s.BorderFgActive
		titleStyle = s.TitleActive
	} else {
		boxStyle = s.BoxInactive
		borderFg = s.BorderFgInactive
		titleStyle = s.TitleInactive
	}

	rendered := boxStyle.Width(width).Render(content)
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	totalOuterWidth := width + 2
	maxTitleWidth := totalOuterWidth - 7
	if maxTitleWidth < 1 {
		return rendered
	}

	titleW := lipgloss.Width(title)
	displayTitle := title
	if titleW > maxTitleWidth {
		runes := []rune(title)
		if len(runes) > maxTitleWidth {
			displayTitle = string(runes[:maxTitleWidth-1]) + "…"
		}
		titleW = lipgloss.Width(displayTitle)
	}

	remainingDashes := totalOuterWidth - titleW - 6
	if remainingDashes < 0 {
		remainingDashes = 0
	}

	topLine := borderFg.Render("╭── ") +
		titleStyle.Render(displayTitle) +
		borderFg.Render(" ") +
		borderFg.Render(strings.Repeat("─", remainingDashes)) +
		borderFg.Render("╮")

	lines[0] = topLine
	return strings.Join(lines, "\n")
}

func renderCapsule(capStyle, bodyStyle lipgloss.Style, text string) string {
	return capStyle.Render("") + bodyStyle.Render(text) + capStyle.Render("")
}

func renderBrandPill() string {
	return defaultStyles.RenderBrandPill(false)
}

func renderStatsPill(count, total int) string {
	return defaultStyles.RenderStatsPill(count, total, false)
}

func renderScrollPill(percent float64) string {
	return defaultStyles.RenderScrollPill(percent, false)
}

func renderHeaderBar(leftPart, rightPart string, width int) string {
	leftW := lipgloss.Width(leftPart)
	rightW := lipgloss.Width(rightPart)

	if leftW+rightW < width {
		gap := width - leftW - rightW
		return leftPart + strings.Repeat(" ", gap) + rightPart + "\n\n"
	}
	return leftPart + "\n\n"
}

func (m model) renderHeader() string {
	leftPart := m.styles.RenderBrandPill(m.nerdFonts) + m.styles.FilterPrompt.Render("  Filter: ") + m.input.View()
	rightPart := m.styles.RenderStatsPill(len(m.filtered), len(m.catalog.Presets), m.nerdFonts)
	return renderHeaderBar(leftPart, rightPart, m.width)
}

func (m model) renderPanels(geom layoutGeometry, listLines, previewLines []string) string {
	leftTitle := fmt.Sprintf("Presets (%d)", len(m.filtered))
	rightTitle := "Preset Details"

	if geom.sideBySide {
		leftText := formatBoxContent(listLines, geom.leftWidth-2, geom.interiorHeight)
		rightText := formatBoxContent(previewLines, geom.rightWidth-2, geom.interiorHeight)
		leftBox := renderBoxWithTitle(leftText, geom.leftWidth, leftTitle, true, m.styles)
		rightBox := renderBoxWithTitle(rightText, geom.rightWidth, rightTitle, true, m.styles)
		gutterStr := strings.Repeat(" ", geom.gutter)
		return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, gutterStr, rightBox)
	}

	leftText := formatBoxContent(listLines, geom.boxWidth-2, geom.listHeight-2)
	rightText := formatBoxContent(previewLines, geom.boxWidth-2, geom.detailsHeight-2)
	leftBox := renderBoxWithTitle(leftText, geom.boxWidth, leftTitle, true, m.styles)
	rightBox := renderBoxWithTitle(rightText, geom.boxWidth, rightTitle, true, m.styles)
	return leftBox + "\n" + rightBox
}

func (m model) renderReadmeHeader() string {
	leftPart := m.styles.RenderBrandPill(m.nerdFonts) + m.styles.FilterPrompt.Render("  README Viewer")
	rightPart := m.styles.RenderScrollPill(m.viewport.ScrollPercent(), m.nerdFonts)
	return renderHeaderBar(leftPart, rightPart, m.width)
}

func (m model) renderReadmeView() string {
	boxWidth, _, _ := calculateReadmeDimensions(m.width, m.height)
	header := m.renderReadmeHeader()
	box := renderBoxWithTitle(m.viewport.View(), boxWidth, m.readmeTitle, true, m.styles)
	footer := m.renderReadmeFooter()
	return "\n" + header + box + footer + "\n"
}

func (m model) renderReadmeFooter() string {
	return m.styles.FormatKeyHelp([]keyHelp{
		{"↑/↓/PgUp/PgDn", "Scroll"},
		{"Enter", "Launch"},
		{"Tab/Esc/q", "Back"},
	})
}

func (m model) renderFooter(hasReadme bool) string {
	items := []keyHelp{
		{"↑/↓", "Navigate"},
	}
	if hasReadme {
		items = append(items, keyHelp{"Tab", "View README"})
	}
	items = append(items,
		keyHelp{"Enter", "Launch"},
		keyHelp{"Esc/Ctrl+C", "Quit"},
	)
	return m.styles.FormatKeyHelp(items)
}

func formatBoxContent(lines []string, width, height int) string {
	var out []string
	for i := 0; i < height; i++ {
		if i < len(lines) {
			out = append(out, truncateANSI(lines[i], width))
		} else {
			out = append(out, "")
		}
	}
	return strings.Join(out, "\n")
}

func truncateANSI(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}

	target := maxWidth - 1
	var b strings.Builder
	b.Grow(len(s))
	visWidth := 0
	hasANSI := false

	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			hasANSI = true
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			b.WriteString(s[i:j])
			i = j
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		rw := lipgloss.Width(string(r))
		if visWidth+rw > target {
			break
		}
		b.WriteRune(r)
		visWidth += rw
		i += size
	}

	if hasANSI {
		b.WriteString("\x1b[0m")
	}
	b.WriteString("…")
	return b.String()
}
