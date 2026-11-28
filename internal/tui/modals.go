package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
		{"F1", "Help"},
		{"Enter", "Launch"},
		{"Tab/Esc/q", "Back"},
	})
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
	row := func(keys, desc string) string {
		pad := 24 - lipgloss.Width(keys)
		if pad < 2 {
			pad = 2
		}
		return "  " + k(keys) + strings.Repeat(" ", pad) + desc
	}

	browserTitle := "Preset Browser (Default View)"
	if fromMode == helpModeBrowser {
		browserTitle += " — Active View"
	}
	browserSection := []string{
		h(browserTitle),
		row("↑/↓, Ctrl+P/Ctrl+N", "Move selection up or down"),
		row("PgUp/PgDn, Ctrl+U/D", "Page up or down"),
		row("Home/End", "Move filter text cursor to start or end"),
		row("Tab, Ctrl+R", "Open README Viewer for selected preset"),
		row("Ctrl+O, F4", "Open Launch Options & Preset Config"),
		row("Ctrl+G, F2", "Open 3-Tab Config Manager"),
		row("F3, Ctrl+T", "Toggle Installed library and full Catalog"),
		row("Ctrl+F", "Fetch or extract missing WADs asynchronously"),
		row("F1, Alt+?", "Open Contextual Help"),
		row("Enter", "Launch selected preset"),
		row("Esc, Ctrl+C", "Cancel active fetch or quit launcher"),
		"",
		h("Fuzzy Search Tokens"),
		"  Filter by title, WAD filename, game (doom, heretic, hexen, strife), engine,",
		"  category, genre (slaughter, cyberpunk), difficulty (casual, hard, extreme),",
		"  quality (s-tier), compatibility (MBF21, Boom, Vanilla, ID24), or readiness.",
	}

	launchTitle := "Launch Options & Preset Config (Ctrl+O / F4)"
	if fromMode == helpModeLaunchOptions {
		launchTitle += " — Active View"
	}
	launchSection := []string{
		h(launchTitle),
		row("↑/↓, Tab/Shift+Tab", "Move between active option fields"),
		row("←/→", "Cycle spin selector values (Engine, Skill, Complevel)"),
		row("Space", "Toggle checkboxes (-pistolstart, -fast, Optional WADs)"),
		row("Enter", "Ephemeral one-shot launch with current options"),
		row("Ctrl+S", "Save persistent preset override to catalog.d"),
		row("Ctrl+R", "Reset persistent preset override to catalog defaults"),
		row("F1, ?", "Open Contextual Help (? on non-text fields)"),
		row("Esc", "Return to Preset Browser without launching"),
	}

	cfgTitle := "3-Tab Config Manager (Ctrl+G / F2)"
	if fromMode == helpModeConfigManager {
		cfgTitle += " — Active View"
	}
	cfgSection := []string{
		h(cfgTitle),
		row("1/2/3, [/]", "Switch between tabs (1. CLI, 2. Engines, 3. Ports)"),
		row("↑/↓, Tab/Shift+Tab", "Move between settings fields"),
		row("←/→", "Cycle spin selectors or Port Config action buttons"),
		row("Space, Enter", "Toggle checkboxes or trigger Port Config action"),
		row("Ctrl+S", "Save active tab changes or personal port overrides"),
		row("Ctrl+R", "Reset selected engine or port override (Tab 2 / Tab 3)"),
		row("g/G, PgUp/PgDn", "Scroll Configuration Diff viewer (Tab 3)"),
		row("F1, ?", "Open Contextual Help (? on non-text fields and Diff)"),
		row("Esc", "Close Config Manager or Diff and revert unsaved previews"),
	}

	readmeTitle := "README Viewer (Tab / Ctrl+R)"
	if fromMode == helpModeReadme {
		readmeTitle += " — Active View"
	}
	readmeSection := []string{
		h(readmeTitle),
		row("↑/↓, PgUp/PgDn", "Scroll document up or down"),
		row("g/G, Home/End", "Jump to top or bottom of document"),
		row("Enter", "Launch selected preset"),
		row("F1, ?", "Open Contextual Help"),
		row("Esc, q, Tab, Ctrl+R", "Return to Preset Browser"),
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
