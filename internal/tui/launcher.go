// Package tui provides an interactive terminal user interface for launching Doom presets.
package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"golang.org/x/term"

	"github.com/lock14/doom-cli/internal/preset"
)

const (
	sideBySideMinWidth = 90
	leftPanelWidth     = 44 // interior content width (42) + horizontal padding (2)
	gutterWidth        = 2  // horizontal space between panes
	boxBorderColumns   = 4  // 2 border chars on left box + 2 border chars on right box

	minSearchWidth = 20
	maxSearchWidth = 50

	minBoxWidth  = 36
	minBoxHeight = 3
)

var (
	// Brand Capsule (Powerlevel10k rounded pill with matched background)
	brandCapStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))
	brandBodyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("1")).
			Foreground(lipgloss.Color("7")).
			Bold(true)

	// Stats Capsule (Powerlevel10k rounded pill with matched background)
	statsCapStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
	statsBodyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("8")).
			Foreground(lipgloss.Color("7")).
			Bold(true)

	// Filter Prompt
	filterPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")).
				Bold(true)

	// Panel Boxes: Active (Cyan) vs Inactive (Gray)
	panelBoxFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("6")).
				Padding(0, 1)

	panelBoxInactiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8")).
				Padding(0, 1)

	// Border Embedded Titles
	borderTitleFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")).
				Bold(true)

	borderTitleInactiveStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("8")).
					Bold(true)

	// Cursor & Selection Bar
	cursorBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

	cursorTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

	tagDSDAStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	tagUZDoomStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6"))

	valueBoldStyle = lipgloss.NewStyle().
			Bold(true)

	foundStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("2"))

	missingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("1"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	// Two-Tone Keybinding Footer
	keyHelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6"))

	keyHelpDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))

	bulletStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)

// calculateSearchWidth returns the clamped width for the preset search input.
func calculateSearchWidth(termWidth int) int {
	w := termWidth - 45
	if w > maxSearchWidth {
		return maxSearchWidth
	}
	if w < minSearchWidth {
		return minSearchWidth
	}
	return w
}

// calculateInteriorHeight returns the shared interior box height for both the main panes and README viewer.
func calculateInteriorHeight(termHeight int) int {
	h := termHeight - 8
	if h < 5 {
		return 5
	}
	return h
}

// calculateReadmeDimensions returns the responsive outer box and viewport dimensions for the README viewer.
func calculateReadmeDimensions(termWidth, termHeight int) (boxWidth, vpWidth, vpHeight int) {
	boxWidth = termWidth - 2
	if boxWidth < minBoxWidth {
		boxWidth = minBoxWidth
	}
	vpWidth = boxWidth - 2
	if vpWidth < 36 {
		vpWidth = 36
	}
	vpHeight = calculateInteriorHeight(termHeight)
	return boxWidth, vpWidth, vpHeight
}

type model struct {
	catalog  *preset.Catalog
	wadsDir  string
	input    textinput.Model
	filtered []preset.Preset
	cursor   int
	selected *preset.Preset
	width    int
	height   int
	quitting bool

	// Readme viewer state
	viewingReadme bool
	readmeTitle   string
	viewport      viewport.Model
}

func initialModel(catalog *preset.Catalog, wadsDir string, initialPreset ...string) model {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		w = 100
		h = 24
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Type to search presets..."
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = calculateSearchWidth(w)

	cursor := 0
	if len(initialPreset) > 0 && initialPreset[0] != "" && catalog != nil {
		for i, p := range catalog.Presets {
			if strings.EqualFold(p.Name, initialPreset[0]) {
				cursor = i
				break
			}
		}
	}

	return model{
		catalog:  catalog,
		wadsDir:  wadsDir,
		input:    ti,
		filtered: catalog.Presets,
		cursor:   cursor,
		width:    w,
		height:   h,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.viewingReadme {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
			return m, nil

		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc", "q", "tab", "ctrl+r":
				m.viewingReadme = false
				return m, nil
			case "enter":
				if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
					m.selected = &m.filtered[m.cursor]
					return m, tea.Quit
				}
			}
		}

		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = calculateSearchWidth(m.width)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
				m.selected = &m.filtered[m.cursor]
				return m, tea.Quit
			}

		case "tab", "ctrl+r":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				cur := m.filtered[m.cursor]
				if txtPath, ok := preset.ResolveReadme(m.wadsDir, cur); ok {
					content, err := preset.ReadReadme(txtPath)
					if err == nil {
						m.viewingReadme = true
						m.readmeTitle = fmt.Sprintf("README: %s (%s)", cur.Name, filepath.Base(txtPath))
						_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
						m.viewport = viewport.New(vpWidth, vpHeight)
						m.viewport.SetContent(content)
						return m, nil
					}
				}
			}
			return m, nil

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			} else if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return m, nil

		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
			return m, nil
		}
	}

	oldVal := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	newVal := m.input.Value()

	if oldVal != newVal {
		m.updateFiltered(newVal)
		m.cursor = 0
	}

	return m, cmd
}

func (m *model) updateFiltered(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		m.filtered = m.catalog.Presets
		return
	}

	// Prepare search items
	var names []string
	for _, p := range m.catalog.Presets {
		names = append(names, fmt.Sprintf("%s %s %s", p.Name, p.Engine, p.Category))
	}

	matches := fuzzy.Find(query, names)
	var result []preset.Preset
	for _, match := range matches {
		result = append(result, m.catalog.Presets[match.Index])
	}
	m.filtered = result
}

type layoutGeometry struct {
	sideBySide     bool
	maxVisible     int
	interiorHeight int
	listHeight     int
	detailsHeight  int
	leftWidth      int
	rightWidth     int
	boxWidth       int
	gutter         int
}

func (m model) computeLayout() layoutGeometry {
	interiorHeight := calculateInteriorHeight(m.height)
	if m.width >= sideBySideMinWidth {
		rightWidth := m.width - leftPanelWidth - gutterWidth - boxBorderColumns
		if rightWidth < 38 {
			rightWidth = 38
		}
		return layoutGeometry{
			sideBySide:     true,
			maxVisible:     interiorHeight,
			interiorHeight: interiorHeight,
			leftWidth:      leftPanelWidth,
			rightWidth:     rightWidth,
			gutter:         gutterWidth,
		}
	}

	boxWidth := m.width - 2
	if boxWidth < minBoxWidth {
		boxWidth = minBoxWidth
	}
	availHeight := m.height - 5
	if availHeight < 6 {
		availHeight = 6
	}
	listHeight := availHeight / 2
	if listHeight < minBoxHeight {
		listHeight = minBoxHeight
	}
	detailsHeight := availHeight - listHeight
	if detailsHeight < minBoxHeight {
		detailsHeight = minBoxHeight
	}
	maxVisible := listHeight - 2
	if maxVisible < 1 {
		maxVisible = 1
	}

	return layoutGeometry{
		sideBySide:    false,
		maxVisible:    maxVisible,
		listHeight:    listHeight,
		detailsHeight: detailsHeight,
		boxWidth:      boxWidth,
	}
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
		tag := tagUZDoomStyle.Render("[UZDoom]")
		if p.Engine == "dsda-doom" {
			tag = tagDSDAStyle.Render("[DSDA]") + "  "
		}

		name := p.Name
		runes := []rune(name)
		if len(runes) > 30 {
			name = string(runes[:29]) + "…"
		}
		paddedName := fmt.Sprintf("%-30s", name)

		if i == m.cursor {
			listLines = append(listLines, cursorBarStyle.Render("▎ ")+cursorTextStyle.Render(paddedName)+" "+tag)
		} else {
			listLines = append(listLines, "  "+paddedName+" "+tag)
		}
	}
	return listLines
}

func (m model) renderPreviewLines() ([]string, bool) {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return []string{helpStyle.Render("No preset details available.")}, false
	}

	cur := m.filtered[m.cursor]
	readmePath, hasReadme := preset.ResolveReadme(m.wadsDir, cur)

	var previewLines []string
	previewLines = append(previewLines,
		fmt.Sprintf("%s%s", labelStyle.Render("Preset:        "), valueBoldStyle.Render(cur.Name)),
	)
	if cur.Author != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", labelStyle.Render("Author:        "), cur.Author),
		)
	}
	if cur.ReleaseDate != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", labelStyle.Render("Released:      "), cur.ReleaseDate),
		)
	}
	engStr := "UZDoom (Software-Plus / Advanced)"
	if cur.Engine == "dsda-doom" {
		engStr = "DSDA-Doom (MBF21 / Speedrun)"
	}
	previewLines = append(previewLines,
		fmt.Sprintf("%s%s", labelStyle.Render("Engine:        "), engStr),
	)
	if cur.Category != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", labelStyle.Render("Category:      "), cur.Category),
		)
	}
	if cur.Compatibility != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", labelStyle.Render("Compatibility: "), cur.Compatibility),
		)
	}
	if cur.Description != "" {
		previewLines = append(previewLines,
			fmt.Sprintf("%s%s", labelStyle.Render("Description:   "), cur.Description),
		)
	}

	// Check IWAD
	iwadStatus := missingStyle.Render("[✗ Missing]")
	if _, ok := preset.ResolveFile(m.wadsDir, cur.IWAD); ok {
		iwadStatus = foundStyle.Render("[✓ Found]")
	}
	previewLines = append(previewLines,
		fmt.Sprintf("%s%s %s", labelStyle.Render("IWAD:          "), cur.IWAD, iwadStatus),
	)

	if len(cur.Mappacks) > 0 || hasReadme {
		previewLines = append(previewLines, labelStyle.Render("Files:"))
		for _, mapfile := range cur.Mappacks {
			fStatus := missingStyle.Render("[✗ Missing]")
			if _, ok := preset.ResolveFile(m.wadsDir, mapfile); ok {
				fStatus = foundStyle.Render("[✓ Found]")
			} else if strings.EqualFold(mapfile, "idkfa 2024.wad") {
				fStatus = helpStyle.Render("[Optional]")
			}
			previewLines = append(previewLines, fmt.Sprintf("  - %-22s %s", mapfile, fStatus))
		}
		if hasReadme {
			previewLines = append(previewLines,
				fmt.Sprintf("  - %-22s %s", filepath.Base(readmePath), tagUZDoomStyle.Render("[✓ Readme]")),
			)
		}
	}

	return previewLines, hasReadme
}

func renderBoxWithTitle(content string, width int, title string, focused bool) string {
	var boxStyle lipgloss.Style
	var borderFg lipgloss.Style
	var titleStyle lipgloss.Style

	if focused {
		boxStyle = panelBoxFocusedStyle
		borderFg = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
		titleStyle = borderTitleFocusedStyle
	} else {
		boxStyle = panelBoxInactiveStyle
		borderFg = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		titleStyle = borderTitleInactiveStyle
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
	return renderCapsule(brandCapStyle, brandBodyStyle, " 💀 DOOM ")
}

func renderStatsPill(count, total int) string {
	return renderCapsule(statsCapStyle, statsBodyStyle, fmt.Sprintf(" %d / %d presets ", count, total))
}

func renderScrollPill(percent float64) string {
	pct := int(percent * 100)
	pctText := fmt.Sprintf(" %d%% ", pct)
	if pct <= 0 {
		pctText = " Top "
	} else if pct >= 100 {
		pctText = " End "
	}
	return renderCapsule(statsCapStyle, statsBodyStyle, pctText)
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
	leftPart := renderBrandPill() + filterPromptStyle.Render("   Filter: ") + m.input.View()
	rightPart := renderStatsPill(len(m.filtered), len(m.catalog.Presets))
	return renderHeaderBar(leftPart, rightPart, m.width)
}

func (m model) renderPanels(geom layoutGeometry, listLines, previewLines []string) string {
	leftTitle := fmt.Sprintf("Presets (%d)", len(m.filtered))
	rightTitle := "Preset Details"

	if geom.sideBySide {
		leftText := formatBoxContent(listLines, geom.leftWidth-2, geom.interiorHeight)
		rightText := formatBoxContent(previewLines, geom.rightWidth-2, geom.interiorHeight)
		leftBox := renderBoxWithTitle(leftText, geom.leftWidth, leftTitle, true)
		rightBox := renderBoxWithTitle(rightText, geom.rightWidth, rightTitle, true)
		gutterStr := strings.Repeat(" ", geom.gutter)
		return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, gutterStr, rightBox)
	}

	leftText := formatBoxContent(listLines, geom.boxWidth-2, geom.listHeight-2)
	rightText := formatBoxContent(previewLines, geom.boxWidth-2, geom.detailsHeight-2)
	leftBox := renderBoxWithTitle(leftText, geom.boxWidth, leftTitle, true)
	rightBox := renderBoxWithTitle(rightText, geom.boxWidth, rightTitle, true)
	return leftBox + "\n" + rightBox
}

func (m model) renderReadmeHeader() string {
	leftPart := renderBrandPill() + filterPromptStyle.Render("   README Viewer")
	rightPart := renderScrollPill(m.viewport.ScrollPercent())
	return renderHeaderBar(leftPart, rightPart, m.width)
}

func (m model) renderReadmeView() string {
	boxWidth, _, _ := calculateReadmeDimensions(m.width, m.height)
	header := m.renderReadmeHeader()
	box := renderBoxWithTitle(m.viewport.View(), boxWidth, m.readmeTitle, true)
	footer := m.renderReadmeFooter()
	return "\n" + header + box + footer + "\n"
}

type keyHelp struct {
	key  string
	desc string
}

func formatKeyHelp(items []keyHelp) string {
	var parts []string
	for _, item := range items {
		parts = append(parts, keyHelpKeyStyle.Render(item.key)+" "+keyHelpDescStyle.Render(item.desc))
	}
	sep := bulletStyle.Render("  •  ")
	return "\n\n" + strings.Join(parts, sep)
}

func (m model) renderReadmeFooter() string {
	return formatKeyHelp([]keyHelp{
		{"↑/↓/PgUp/PgDn", "Scroll"},
		{"Enter", "Launch"},
		{"Tab/Esc/q", "Back"},
	})
}

func (m model) renderFooter(hasReadme bool) string {
	items := []keyHelp{
		{"↑/↓", "Navigate"},
		{"Enter", "Launch"},
	}
	if hasReadme {
		items = append(items, keyHelp{"Tab", "Readme"})
	}
	items = append(items, keyHelp{"Esc", "Quit"})
	return formatKeyHelp(items)
}

func (m model) View() string {
	if m.quitting {
		return "Cancelled.\n"
	}
	if m.selected != nil {
		return fmt.Sprintf("Launching %s...\n", m.selected.Name)
	}
	if m.viewingReadme {
		return m.renderReadmeView()
	}

	geom := m.computeLayout()
	listLines := m.renderListLines(geom)
	previewLines, hasReadme := m.renderPreviewLines()
	panels := m.renderPanels(geom, listLines, previewLines)

	header := m.renderHeader()
	footer := m.renderFooter(hasReadme)

	return "\n" + header + panels + footer + "\n"
}

// RunInteractiveLauncher runs the interactive Bubble Tea UI launcher.
// If initialPreset is provided, the launcher pre-selects that preset.
func RunInteractiveLauncher(catalog *preset.Catalog, wadsDir string, initialPreset ...string) (*preset.Preset, error) {
	// If not running in a terminal, fallback to numbered menu
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return RunNumberedMenu(catalog, os.Stdin, os.Stdout)
	}

	m := initialModel(catalog, wadsDir, initialPreset...)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	res := finalModel.(model)
	return res.selected, nil
}

// RunNumberedMenu provides a standard interactive menu without full-screen TUI.
func RunNumberedMenu(catalog *preset.Catalog, in io.Reader, out io.Writer) (*preset.Preset, error) {
	fmt.Fprintln(out, "======================================================")
	fmt.Fprintln(out, "               DOOM PRESET LAUNCHER                   ")
	fmt.Fprintln(out, "======================================================")

	for i, p := range catalog.Presets {
		eng := "UZDoom"
		if p.Engine == "dsda-doom" {
			eng = "DSDA-Doom"
		}
		fmt.Fprintf(out, "%2d. %-30s [%s]\n", i+1, p.Name, eng)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Enter preset number to launch (or 'q' to quit): ")

	scanner := bufio.NewScanner(in)
	if scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(text, "q") || text == "" {
			return nil, nil
		}
		idx, err := strconv.Atoi(text)
		if err == nil && idx >= 1 && idx <= len(catalog.Presets) {
			return &catalog.Presets[idx-1], nil
		}
	}

	return nil, fmt.Errorf("invalid menu selection")
}

// formatBoxContent wraps and clamps or pads lines to fit exactly targetHeight lines within innerWidth.
func formatBoxContent(lines []string, innerWidth, targetHeight int) string {
	if innerWidth < 1 {
		innerWidth = 1
	}
	if targetHeight < 1 {
		targetHeight = 1
	}
	wrapStyle := lipgloss.NewStyle().Width(innerWidth)
	wrapped := wrapStyle.Render(strings.Join(lines, "\n"))
	contentLines := strings.Split(wrapped, "\n")
	if len(contentLines) > targetHeight {
		contentLines = contentLines[:targetHeight]
	}
	for len(contentLines) < targetHeight {
		contentLines = append(contentLines, "")
	}
	return strings.Join(contentLines, "\n")
}
