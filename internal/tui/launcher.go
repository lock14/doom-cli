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

	minSearchWidth = 30
	maxSearchWidth = 60

	minBoxWidth  = 36
	minBoxHeight = 3
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("1"))

	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6"))

	tagDSDAStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	tagUZDoomStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	panelBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

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
)

// calculateSearchWidth returns the clamped width for the preset search input.
func calculateSearchWidth(termWidth int) int {
	w := termWidth - 16
	if w > maxSearchWidth {
		return maxSearchWidth
	}
	if w < minSearchWidth {
		return minSearchWidth
	}
	return w
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
	vpHeight = termHeight - 9
	if vpHeight < 5 {
		vpHeight = 5
	}
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

func initialModel(catalog *preset.Catalog, wadsDir string) model {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		w = 100
		h = 24
	}

	ti := textinput.New()
	ti.Placeholder = "Type to search presets..."
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = calculateSearchWidth(w)

	return model{
		catalog:  catalog,
		wadsDir:  wadsDir,
		input:    ti,
		filtered: catalog.Presets,
		cursor:   0,
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
	if m.width >= sideBySideMinWidth {
		availHeight := m.height - 7
		if availHeight < 6 {
			availHeight = 6
		}
		interiorHeight := availHeight - 3
		if interiorHeight < minBoxHeight {
			interiorHeight = minBoxHeight
		}
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
	availHeight := m.height - 9
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
			listLines = append(listLines, cursorStyle.Render("> "+paddedName)+" "+tag)
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

func (m model) renderPanels(geom layoutGeometry, listLines, previewLines []string) string {
	if geom.sideBySide {
		leftText := formatBoxContent(listLines, geom.leftWidth-2, geom.interiorHeight)
		rightText := formatBoxContent(previewLines, geom.rightWidth-2, geom.interiorHeight)
		leftBox := panelBoxStyle.Width(geom.leftWidth).Render(leftText)
		rightBox := panelBoxStyle.Width(geom.rightWidth).Render(rightText)
		gutterStr := strings.Repeat(" ", geom.gutter)
		return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, gutterStr, rightBox)
	}

	leftText := formatBoxContent(listLines, geom.boxWidth-2, geom.listHeight-2)
	rightText := formatBoxContent(previewLines, geom.boxWidth-2, geom.detailsHeight-2)
	leftBox := panelBoxStyle.Width(geom.boxWidth).Render(leftText)
	rightBox := panelBoxStyle.Width(geom.boxWidth).Render(rightText)
	return leftBox + "\n" + rightBox
}

func (m model) renderReadmeView() string {
	boxWidth, _, _ := calculateReadmeDimensions(m.width, m.height)
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render(m.readmeTitle) + "\n\n"
	box := panelBoxStyle.Width(boxWidth).Height(m.viewport.Height).Render(m.viewport.View())
	footer := "\n\n" + helpStyle.Render("↑/↓/PgUp/PgDn: Scroll • Enter: Launch • Tab/Esc/q: Back")
	return header + box + footer + "\n"
}

func (m model) renderFooter(hasReadme bool) string {
	footerText := "↑/↓: Navigate • Enter: Launch • Esc: Quit"
	if hasReadme {
		footerText = "↑/↓: Navigate • Enter: Launch • Tab: Readme • Esc: Quit"
	}
	return "\n\n" + helpStyle.Render(footerText)
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

	header := titleStyle.Render("DOOM PRESET LAUNCHER") + "\n\n"
	search := fmt.Sprintf("Search: %s\n\n", m.input.View())
	footer := m.renderFooter(hasReadme)

	return header + search + panels + footer + "\n"
}

// RunInteractiveLauncher runs the interactive Bubble Tea UI launcher.
func RunInteractiveLauncher(catalog *preset.Catalog, wadsDir string) (*preset.Preset, error) {
	// If not running in a terminal, fallback to numbered menu
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return RunNumberedMenu(catalog, os.Stdin, os.Stdout)
	}

	m := initialModel(catalog, wadsDir)
	p := tea.NewProgram(m)
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
