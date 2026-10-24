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

func formatEngineTagWithCatalog(catalog *preset.Catalog, engine string, styles ThemeStyles) string {
	badge := catalog.EngineBadge(engine)
	argsStyle := preset.GuessArgsStyle(engine, "")
	if catalog != nil {
		if eng, ok := catalog.FindEngine(engine); ok {
			argsStyle = eng.EffectiveArgsStyle()
		}
	}
	tagStyle := styles.TagUZDoom
	if argsStyle == "boom" {
		tagStyle = styles.TagDSDA
	}
	switch {
	case strings.EqualFold(badge, "DSDA"):
		return tagStyle.Render("[DSDA]") + "  "
	case strings.EqualFold(badge, "UZDoom"):
		return tagStyle.Render("[UZDoom]")
	default:
		formatted := "[" + padOrTruncateName(badge, 6) + "]"
		return tagStyle.Render(formatted)
	}
}

func formatEngineTag(engine string, styles ThemeStyles) string {
	return formatEngineTagWithCatalog(nil, engine, styles)
}

func padOrTruncateName(name string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(name) > width {
		name = truncateANSI(name, width)
	}
	pad := width - lipgloss.Width(name)
	if pad > 0 {
		return name + strings.Repeat(" ", pad)
	}
	return name
}

func (m model) renderListLines(geom layoutGeometry) []string {
	if len(m.filtered) == 0 {
		return []string{"  No presets matching search."}
	}

	maxVisible := geom.maxVisible
	if maxVisible < 1 {
		maxVisible = 1
	}
	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
		startIdx = endIdx - maxVisible
		if startIdx < 0 {
			startIdx = 0
		}
	}

	contentWidth := geom.leftWidth - 2
	if !geom.sideBySide {
		contentWidth = geom.boxWidth - 2
	}
	nameWidth := contentWidth - 12
	if nameWidth > 48 {
		nameWidth = 48
	}
	if nameWidth < 12 {
		nameWidth = 12
	}

	var listLines []string
	for i := startIdx; i < endIdx; i++ {
		p := m.filtered[i]
		tag := formatEngineTagWithCatalog(m.catalog, p.Engine, m.styles)
		paddedName := padOrTruncateName(p.Name, nameWidth)

		if i == m.cursor {
			selected := m.styles.CursorBar.Render("▎ ") + m.styles.CursorText.Render(paddedName) + " " + tag
			listLines = append(listLines, selected)
		} else {
			listLines = append(listLines, "  "+paddedName+" "+tag)
		}
	}
	return listLines
}

type previewFileEntry struct {
	name   string
	status string
}

func (m model) buildFileEntries(cur preset.Preset, readmePath string, hasReadme bool) []previewFileEntry {
	var entries []previewFileEntry
	for _, f := range cur.Files {
		fStatus := m.styles.StatusMissing.Render("[✗ Missing]")
		if _, ok := m.resolveFile(f.Name); ok {
			fStatus = m.styles.StatusReady.Render("[✓ Found]")
		} else if f.Optional {
			fStatus = m.styles.Help.Render("[Optional]")
		}
		entries = append(entries, previewFileEntry{name: f.Name, status: fStatus})
	}
	if hasReadme {
		entries = append(entries, previewFileEntry{
			name:   filepath.Base(readmePath),
			status: m.styles.TagUZDoom.Render("[✓ Readme]"),
		})
	}
	return entries
}

func formatSingleFileLine(entry previewFileEntry, detailsWidth int) string {
	maxNameW := detailsWidth - 16
	if maxNameW < 8 {
		maxNameW = 8
	}
	name := entry.name
	if lipgloss.Width(name) > maxNameW {
		name = truncateANSI(name, maxNameW)
	}
	padW := 22
	if padW > maxNameW {
		padW = maxNameW
	}
	if pad := padW - lipgloss.Width(name); pad > 0 {
		name += strings.Repeat(" ", pad)
	}
	return fmt.Sprintf("  - %s %s", name, entry.status)
}

func formatCompactFileCol(entry previewFileEntry, colNameW int, padStatus bool) string {
	name := padOrTruncateName(entry.name, colNameW)
	status := entry.status
	if padStatus {
		if pad := 11 - lipgloss.Width(status); pad > 0 {
			status += strings.Repeat(" ", pad)
		}
	}
	return name + " " + status
}

func (m model) packTwoColumnFiles(
	fileEntries []previewFileEntry,
	colNameW int,
	availLines int,
	hasReadme bool,
) []string {
	var rows []string
	for i := 0; i < len(fileEntries); i += 2 {
		prefix := "               "
		if i == 0 {
			prefix = m.styles.Label.Render("Files:         ")
		}
		first := formatCompactFileCol(fileEntries[i], colNameW, i+1 < len(fileEntries))
		if i+1 < len(fileEntries) {
			second := formatCompactFileCol(fileEntries[i+1], colNameW, false)
			rows = append(rows, prefix+first+"  "+second)
		} else {
			rows = append(rows, prefix+first)
		}
	}

	if availLines >= 1 && len(rows) > availLines {
		rows = rows[:availLines]
		lastRowIdx := availLines - 1
		shownBeforeLast := lastRowIdx * 2
		remaining := len(fileEntries) - shownBeforeLast
		prefix := "               "
		if lastRowIdx == 0 {
			prefix = m.styles.Label.Render("Files:         ")
		}
		if hasReadme && len(fileEntries) > 0 {
			hidden := remaining - 1
			moreText := padOrTruncateName(fmt.Sprintf("(+%d more files)", hidden), colNameW+12)
			first := m.styles.Help.Render(moreText)
			second := formatCompactFileCol(fileEntries[len(fileEntries)-1], colNameW, false)
			rows[lastRowIdx] = prefix + first + "  " + second
		} else {
			first := formatCompactFileCol(fileEntries[shownBeforeLast], colNameW, true)
			hidden := remaining - 1
			second := m.styles.Help.Render(fmt.Sprintf("(+%d more files)", hidden))
			rows[lastRowIdx] = prefix + first + "  " + second
		}
	}

	return rows
}

func (m model) renderPreviewLines(geom layoutGeometry) ([]string, bool) {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return []string{m.styles.Help.Render("No preset details available.")}, false
	}

	cur := m.filtered[m.cursor]
	readmePath, hasReadme := m.resolveReadme(cur)
	engStr := m.catalog.EngineDisplayName(cur.Engine)
	if eng, ok := m.catalog.FindEngine(cur.Engine); ok && eng.Description != "" {
		engStr = eng.Description
	}

	iwadStatus := m.styles.StatusMissing.Render("[✗ Missing]")
	if _, ok := m.resolveFile(cur.IWAD); ok {
		iwadStatus = m.styles.StatusReady.Render("[✓ Found]")
	}
	fileEntries := m.buildFileEntries(cur, readmePath, hasReadme)

	detailsWidth := geom.rightWidth - 2
	if !geom.sideBySide {
		detailsWidth = geom.boxWidth - 2
	}

	var previewLines []string
	presetDisplay := m.styles.ValueBold.Render(cur.Name)
	if m.statusMsg != "" {
		statusStyle := m.styles.StatusReady
		if strings.HasPrefix(m.statusMsg, "✗") {
			statusStyle = m.styles.StatusMissing
		}
		presetDisplay += "  " + statusStyle.Render("["+m.statusMsg+"]")
	}
	previewLines = append(previewLines,
		fmt.Sprintf("%s%s", m.styles.Label.Render("Preset:        "), presetDisplay),
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
	previewLines = append(previewLines,
		fmt.Sprintf("%s%s %s", m.styles.Label.Render("IWAD:          "), cur.IWAD, iwadStatus),
	)

	if len(fileEntries) > 0 {
		previewLines = append(previewLines, m.styles.Label.Render("Files:"))
		for _, entry := range fileEntries {
			previewLines = append(previewLines, formatSingleFileLine(entry, detailsWidth))
		}
	}

	maxLines := geom.interiorHeight
	if !geom.sideBySide {
		maxLines = geom.detailsHeight - 2
	}
	if maxLines > 0 && len(previewLines) > maxLines {
		return m.condensePreviewLines(
			cur, engStr, iwadStatus, fileEntries, detailsWidth, maxLines, hasReadme,
		), hasReadme
	}

	return previewLines, hasReadme
}

func (m model) condensePreviewLines(
	cur preset.Preset,
	engStr string,
	iwadStatus string,
	fileEntries []previewFileEntry,
	detailsWidth int,
	maxLines int,
	hasReadme bool,
) []string {
	presetVal := m.styles.ValueBold.Render(cur.Name)
	if cur.IsCustom() {
		presetVal += " " + m.styles.TagUZDoom.Render("[Custom WAD]")
	} else if cur.Overridden {
		presetVal += " " + m.styles.TagUZDoom.Render("[Overridden]")
	}
	if m.statusMsg != "" {
		statusStyle := m.styles.StatusReady
		if strings.HasPrefix(m.statusMsg, "✗") {
			statusStyle = m.styles.StatusMissing
		}
		presetVal += " " + statusStyle.Render("["+m.statusMsg+"]")
	}

	headLines := []string{
		fmt.Sprintf("%s%s", m.styles.Label.Render("Preset:        "), presetVal),
		fmt.Sprintf("%s%s", m.styles.Label.Render("Engine:        "), engStr),
	}
	iwadLine := fmt.Sprintf("%s%s %s", m.styles.Label.Render("IWAD:          "), cur.IWAD, iwadStatus)

	var fileBlock []string
	if len(fileEntries) > 0 {
		fileBlock = append(fileBlock, m.styles.Label.Render("Files:"))
		for _, entry := range fileEntries {
			fileBlock = append(fileBlock, formatSingleFileLine(entry, detailsWidth))
		}
		availFileLines := maxLines - len(headLines) - 1
		if len(headLines)+1+len(fileBlock) > maxLines {
			colNameW := (detailsWidth-17)/2 - 12
			if colNameW > 17 {
				colNameW = 17
			}
			if colNameW < 8 {
				colNameW = 8
			}
			fileBlock = m.packTwoColumnFiles(fileEntries, colNameW, availFileLines, hasReadme)
		}
	}

	var metaCandidates []string
	if len(cur.Args) > 0 {
		metaCandidates = append(metaCandidates,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Launch Args:   "), strings.Join(cur.Args, " ")),
		)
	}
	switch {
	case cur.Author != "" && cur.ReleaseDate != "":
		metaCandidates = append(metaCandidates,
			fmt.Sprintf("%s%s (%s)", m.styles.Label.Render("Author:        "), cur.Author, cur.ReleaseDate),
		)
	case cur.Author != "":
		metaCandidates = append(metaCandidates,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Author:        "), cur.Author),
		)
	case cur.ReleaseDate != "":
		metaCandidates = append(metaCandidates,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Released:      "), cur.ReleaseDate),
		)
	}
	switch {
	case cur.Category != "" && cur.Compatibility != "":
		metaCandidates = append(metaCandidates,
			fmt.Sprintf("%s%s (%s)", m.styles.Label.Render("Category:      "), cur.Category, cur.Compatibility),
		)
	case cur.Category != "":
		metaCandidates = append(metaCandidates,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Category:      "), cur.Category),
		)
	case cur.Compatibility != "":
		metaCandidates = append(metaCandidates,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Compatibility: "), cur.Compatibility),
		)
	}
	if cur.Description != "" {
		metaCandidates = append(metaCandidates,
			fmt.Sprintf("%s%s", m.styles.Label.Render("Description:   "), cur.Description),
		)
	}

	budget := maxLines - len(headLines) - 1 - len(fileBlock)
	if budget < 0 {
		budget = 0
	}
	if budget > len(metaCandidates) {
		budget = len(metaCandidates)
	}

	out := make([]string, 0, len(headLines)+budget+1+len(fileBlock))
	out = append(out, headLines...)
	out = append(out, metaCandidates[:budget]...)
	out = append(out, iwadLine)
	out = append(out, fileBlock...)
	return out
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

	displayTitle := truncateANSI(title, maxTitleWidth)
	titleW := lipgloss.Width(displayTitle)

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
	if borderFg.GetFaint() && len(lines) > 1 {
		rawVert := lipgloss.NewStyle().Foreground(borderFg.GetForeground()).Render("│")
		faintVert := borderFg.Render("│")
		for i := 1; i < len(lines)-1; i++ {
			if strings.HasPrefix(lines[i], rawVert) && strings.HasSuffix(lines[i], rawVert) {
				lines[i] = faintVert + lines[i][len(rawVert):len(lines[i])-len(rawVert)] + faintVert
			}
		}
		lines[len(lines)-1] = borderFg.Render("╰" + strings.Repeat("─", width) + "╯")
	}
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
	totalPresets := 0
	if m.catalog != nil {
		totalPresets = len(m.catalog.Presets)
	}
	rightPart := m.styles.RenderStatsPill(len(m.filtered), totalPresets, m.nerdFonts)
	return renderHeaderBar(leftPart, rightPart, m.width)
}

func (m model) renderPanels(geom layoutGeometry, listLines, previewLines []string) string {
	leftTitle := fmt.Sprintf("Presets (%d)", len(m.filtered))
	rightTitle := "Preset Details"

	if geom.sideBySide {
		leftText := formatBoxContent(listLines, geom.leftWidth-2, geom.interiorHeight)
		rightText := formatBoxContent(previewLines, geom.rightWidth-2, geom.interiorHeight)
		leftBox := renderBoxWithTitle(leftText, geom.leftWidth, leftTitle, true, m.styles)
		rightBox := renderBoxWithTitle(rightText, geom.rightWidth, rightTitle, false, m.styles)
		gutterStr := strings.Repeat(" ", geom.gutter)
		return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, gutterStr, rightBox)
	}

	leftText := formatBoxContent(listLines, geom.boxWidth-2, geom.listHeight-2)
	rightText := formatBoxContent(previewLines, geom.boxWidth-2, geom.detailsHeight-2)
	leftBox := renderBoxWithTitle(leftText, geom.boxWidth, leftTitle, true, m.styles)
	rightBox := renderBoxWithTitle(rightText, geom.boxWidth, rightTitle, false, m.styles)
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
	footer := m.styles.FormatKeyHelp([]keyHelp{
		{"↑/↓/PgUp/PgDn/g/G", "Scroll"},
		{"Enter", "Launch"},
		{"Tab/Esc/q", "Back"},
	})
	if m.width > 0 {
		return "\n\n" + truncateANSI(strings.TrimPrefix(footer, "\n\n"), m.width)
	}
	return footer
}

func (m model) renderFooter(hasReadme bool) string {
	var items []keyHelp
	if m.width > 90 {
		items = []keyHelp{
			{"↑/↓", "Navigate"},
		}
		if hasReadme {
			items = append(items, keyHelp{"Tab", "README"})
		}
		items = append(items,
			keyHelp{"Ctrl+O", "Opts"},
			keyHelp{"Ctrl+G", "Cfg"},
			keyHelp{"Ctrl+F", "Fetch"},
			keyHelp{"F1", "Help"},
			keyHelp{"Enter", "Launch"},
			keyHelp{"Esc", "Quit"},
		)
	} else {
		items = []keyHelp{
			{"↑/↓/PgUp/PgDn", "Navigate"},
		}
		if hasReadme {
			items = append(items, keyHelp{"Tab", "View README"})
		}
		items = append(items,
			keyHelp{"Enter", "Launch"},
			keyHelp{"Esc/Ctrl+C", "Quit"},
		)
	}
	footer := m.styles.FormatKeyHelp(items)
	if m.width > 0 {
		return "\n\n" + truncateANSI(strings.TrimPrefix(footer, "\n\n"), m.width)
	}
	return footer
}

func (m model) renderHelpHeader() string {
	leftPart := m.styles.RenderBrandPill(m.nerdFonts) +
		m.styles.FilterPrompt.Render("  Contextual Help")
	rightPart := m.styles.RenderScrollPill(m.helpViewport.ScrollPercent(), m.nerdFonts)
	return renderHeaderBar(leftPart, rightPart, m.width)
}

func (m model) renderHelpFooter() string {
	footer := m.styles.FormatKeyHelp([]keyHelp{
		{"↑/↓/PgUp/PgDn/g/G", "Scroll"},
		{"F1/Esc/q", "Back"},
	})
	if m.width > 0 {
		return "\n\n" + truncateANSI(strings.TrimPrefix(footer, "\n\n"), m.width)
	}
	return footer
}

func (m model) renderHelpView() string {
	boxWidth, _, _ := calculateReadmeDimensions(m.width, m.height)
	header := m.renderHelpHeader()
	box := renderBoxWithTitle(
		m.helpViewport.View(),
		boxWidth,
		"Keybindings & Features",
		true,
		m.styles,
	)
	footer := m.renderHelpFooter()
	return "\n" + header + box + footer + "\n"
}

func (m model) buildHelpContent(fromMode int) string {
	h := m.styles.TitleActive.Render
	k := m.styles.KeyHelpKey.Render

	browserTitle := "Preset Browser (Default View)"
	if fromMode == helpModeBrowser {
		browserTitle += " — Active View"
	}
	browserSection := []string{
		h(browserTitle),
		fmt.Sprintf("  %-22s  Move selection up or down", k("↑/↓, Ctrl+P/Ctrl+N")),
		fmt.Sprintf("  %-22s  Page up or down", k("PgUp/PgDn, Ctrl+U/D")),
		fmt.Sprintf("  %-22s  Move filter text cursor to start or end", k("Home/End")),
		fmt.Sprintf("  %-22s  Open README Viewer for selected preset", k("Tab, Ctrl+R")),
		fmt.Sprintf("  %-22s  Open Launch Options & Preset Config", k("Ctrl+O, F4")),
		fmt.Sprintf("  %-22s  Open 3-Tab Config Manager", k("Ctrl+G, F2")),
		fmt.Sprintf("  %-22s  Fetch or extract missing WADs for selected preset", k("Ctrl+F")),
		fmt.Sprintf("  %-22s  Open Contextual Help", k("F1")),
		fmt.Sprintf("  %-22s  Launch selected preset", k("Enter")),
		fmt.Sprintf("  %-22s  Quit launcher", k("Esc, Ctrl+C")),
		"",
		h("Fuzzy Search Tokens"),
		"  Filter by title, engine (dsda-doom, uzdoom), category (Doom II, Heretic),",
		"  compatibility (MBF21, Boom, Vanilla, ID24), or readiness (ready, installed, missing).",
	}

	launchTitle := "Launch Options & Preset Config (Ctrl+O / F4)"
	if fromMode == helpModeLaunchOptions {
		launchTitle += " — Active View"
	}
	launchSection := []string{
		h(launchTitle),
		fmt.Sprintf("  %-22s  Move between option fields", k("↑/↓, Tab/Shift+Tab")),
		fmt.Sprintf("  %-22s  Cycle spin selector values (Engine, Skill, Complevel)", k("←/→")),
		fmt.Sprintf("  %-22s  Toggle checkboxes (-pistolstart, -fast, Optional WADs)", k("Space")),
		fmt.Sprintf("  %-22s  Ephemeral one-shot launch with current options", k("Enter")),
		fmt.Sprintf("  %-22s  Save persistent preset override to catalog.d", k("Ctrl+S")),
		fmt.Sprintf("  %-22s  Reset persistent preset override to catalog defaults", k("Ctrl+R")),
		fmt.Sprintf("  %-22s  Return to Preset Browser without launching", k("Esc")),
	}

	cfgTitle := "3-Tab Config Manager (Ctrl+G / F2)"
	if fromMode == helpModeConfigManager {
		cfgTitle += " — Active View"
	}
	cfgSection := []string{
		h(cfgTitle),
		fmt.Sprintf("  %-22s  Switch between tabs (1. CLI, 2. Engines, 3. Ports)", k("1/2/3, [/]")),
		fmt.Sprintf("  %-22s  Move between settings fields", k("↑/↓, Tab/Shift+Tab")),
		fmt.Sprintf("  %-22s  Cycle spin selectors or action buttons", k("←/→")),
		fmt.Sprintf("  %-22s  Toggle checkboxes or trigger Port Config action", k("Space, Enter")),
		fmt.Sprintf("  %-22s  Save active tab changes atomically with backup", k("Ctrl+S")),
		fmt.Sprintf("  %-22s  Reset selected engine override (Tab 2)", k("Ctrl+R")),
		fmt.Sprintf("  %-22s  Close Config Manager and revert unsaved previews", k("Esc")),
	}

	readmeTitle := "README Viewer (Tab / Ctrl+R)"
	if fromMode == helpModeReadme {
		readmeTitle += " — Active View"
	}
	readmeSection := []string{
		h(readmeTitle),
		fmt.Sprintf("  %-22s  Scroll document up or down", k("↑/↓, PgUp/PgDn")),
		fmt.Sprintf("  %-22s  Jump to top or bottom of document", k("g/G, Home/End")),
		fmt.Sprintf("  %-22s  Launch selected preset", k("Enter")),
		fmt.Sprintf("  %-22s  Return to Preset Browser", k("Esc, q, Tab, Ctrl+R")),
	}

	var sections [][]string
	switch fromMode {
	case helpModeLaunchOptions:
		sections = [][]string{launchSection, browserSection, cfgSection, readmeSection}
	case helpModeConfigManager:
		sections = [][]string{cfgSection, browserSection, launchSection, readmeSection}
	case helpModeReadme:
		sections = [][]string{readmeSection, browserSection, launchSection, cfgSection}
	default:
		sections = [][]string{browserSection, launchSection, cfgSection, readmeSection}
	}

	var lines []string
	for i, sec := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sec...)
	}
	return strings.Join(lines, "\n")
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
