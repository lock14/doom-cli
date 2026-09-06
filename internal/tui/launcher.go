// Package tui provides an interactive terminal user interface for launching Doom presets.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	"golang.org/x/term"

	"github.com/lock14/doom-cli/internal/preset"
)

var defaultStyles = CompileStyles(DefaultTheme)

type model struct {
	theme     Theme
	styles    ThemeStyles
	nerdFonts bool
	catalog   *preset.Catalog
	wadsDir   string
	input     textinput.Model
	filtered  []preset.Preset
	cursor    int
	selected  *preset.Preset
	width     int
	height    int
	quitting  bool

	// Readme viewer state
	viewingReadme bool
	readmeTitle   string
	viewport      viewport.Model
}

func initialModel(
	catalog *preset.Catalog,
	wadsDir string,
	theme Theme,
	nerdFonts bool,
	initialPreset ...string,
) model {
	if theme.Name == "" {
		theme = DefaultTheme
	}
	styles := CompileStyles(theme)

	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		w = 100
		h = 24
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Type to search presets..."
	ti.PlaceholderStyle = styles.Placeholder
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
		theme:     theme,
		styles:    styles,
		nerdFonts: nerdFonts,
		catalog:   catalog,
		wadsDir:   wadsDir,
		input:     ti,
		filtered:  catalog.Presets,
		cursor:    cursor,
		width:     w,
		height:    h,
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
func RunInteractiveLauncher(
	catalog *preset.Catalog,
	wadsDir string,
	theme Theme,
	nerdFonts bool,
	initialPreset ...string,
) (*preset.Preset, error) {
	// If not running in a terminal, fallback to numbered menu
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return RunNumberedMenu(catalog, os.Stdin, os.Stdout)
	}

	m := initialModel(catalog, wadsDir, theme, nerdFonts, initialPreset...)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	res := finalModel.(model)
	return res.selected, nil
}
