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

var defaultStyles = CompileStyles(ClassicTheme)

type model struct {
	theme         Theme
	styles        ThemeStyles
	nerdFonts     bool
	catalog       *preset.Catalog
	dirIndex      *preset.DirIndex
	readmeCache   map[string]string
	searchTargets []string
	input         textinput.Model
	filtered      []preset.Preset
	cursor        int
	selected      *preset.Preset
	width         int
	height        int
	quitting      bool

	// Readme viewer state
	viewingReadme bool
	readmeTitle   string
	viewport      viewport.Model
}

func readmeCacheKey(p preset.Preset) string {
	if p.ID != "" {
		return p.ID
	}
	return p.Name
}

func (m model) resolveFile(name string) (string, bool) {
	if m.dirIndex != nil {
		return m.dirIndex.ResolveFile(name)
	}
	return "", false
}

func (m model) resolveReadme(p preset.Preset) (string, bool) {
	if m.readmeCache != nil {
		if path, cached := m.readmeCache[readmeCacheKey(p)]; cached {
			return path, path != ""
		}
	}
	if m.dirIndex != nil {
		return m.dirIndex.ResolveReadme(p)
	}
	return "", false
}

func initialModel(
	catalog *preset.Catalog,
	wadRoots []string,
	theme Theme,
	nerdFonts bool,
	initialPreset ...string,
) model {
	if catalog == nil {
		catalog = &preset.Catalog{}
	}
	if theme.Name == "" {
		theme = ClassicTheme
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
	if len(initialPreset) > 0 && initialPreset[0] != "" {
		for i, p := range catalog.Presets {
			if strings.EqualFold(p.Name, initialPreset[0]) || strings.EqualFold(p.ID, initialPreset[0]) {
				cursor = i
				break
			}
		}
	}

	dirIndex := preset.NewDirIndex(wadRoots...).WithCatalog(catalog)
	readmeCache := make(map[string]string, len(catalog.Presets))
	searchTargets := make([]string, 0, len(catalog.Presets))
	for _, p := range catalog.Presets {
		searchTargets = append(searchTargets, fmt.Sprintf("%s %s %s", p.Name, p.Engine, p.Category))
		if path, ok := dirIndex.ResolveReadme(p); ok {
			readmeCache[readmeCacheKey(p)] = path
		} else {
			readmeCache[readmeCacheKey(p)] = ""
		}
	}

	return model{
		theme:         theme,
		styles:        styles,
		nerdFonts:     nerdFonts,
		catalog:       catalog,
		dirIndex:      dirIndex,
		readmeCache:   readmeCache,
		searchTargets: searchTargets,
		input:         ti,
		filtered:      catalog.Presets,
		cursor:        cursor,
		width:         w,
		height:        h,
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
			m.input.Width = calculateSearchWidth(m.width)
			_, vpWidth, vpHeight := calculateReadmeDimensions(m.width, m.height)
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
			return m, nil

		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc", "q", "Q", "tab", "ctrl+r":
				m.viewingReadme = false
				return m, nil
			case "g", "home":
				m.viewport.GotoTop()
				return m, nil
			case "G", "end":
				m.viewport.GotoBottom()
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
			if len(m.filtered) > 0 && m.cursor >= 0 && m.cursor < len(m.filtered) {
				cur := m.filtered[m.cursor]
				if txtPath, ok := m.resolveReadme(cur); ok {
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

		case "up", "ctrl+p", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
			} else if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return m, nil

		case "down", "ctrl+n", "ctrl+j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
			return m, nil

		case "pgup", "ctrl+u":
			if len(m.filtered) > 0 {
				step := m.computeLayout().maxVisible
				if msg.String() == "ctrl+u" {
					step /= 2
				}
				if step < 1 {
					step = 1
				}
				m.cursor -= step
				if m.cursor < 0 {
					m.cursor = 0
				}
			}
			return m, nil

		case "pgdown", "ctrl+d":
			if len(m.filtered) > 0 {
				step := m.computeLayout().maxVisible
				if msg.String() == "ctrl+d" {
					step /= 2
				}
				if step < 1 {
					step = 1
				}
				m.cursor += step
				if m.cursor >= len(m.filtered) {
					m.cursor = len(m.filtered) - 1
				}
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
	if m.catalog == nil {
		m.filtered = nil
		return
	}
	query = strings.TrimSpace(query)
	if query == "" {
		m.filtered = m.catalog.Presets
		return
	}

	targets := m.searchTargets
	if len(targets) != len(m.catalog.Presets) {
		targets = make([]string, 0, len(m.catalog.Presets))
		for _, p := range m.catalog.Presets {
			targets = append(targets, fmt.Sprintf("%s %s %s", p.Name, p.Engine, p.Category))
		}
	}

	matches := fuzzy.Find(query, targets)
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
	previewLines, hasReadme := m.renderPreviewLines(geom)
	panels := m.renderPanels(geom, listLines, previewLines)

	header := m.renderHeader()
	footer := m.renderFooter(hasReadme)

	return "\n" + header + panels + footer + "\n"
}

// RunInteractiveLauncher runs the interactive Bubble Tea UI launcher across one or more WAD roots.
func RunInteractiveLauncher(
	catalog *preset.Catalog,
	wadRoots []string,
	theme Theme,
	nerdFonts bool,
	initialPreset ...string,
) (*preset.Preset, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return RunNumberedMenu(catalog, os.Stdin, os.Stdout)
	}

	m := initialModel(catalog, wadRoots, theme, nerdFonts, initialPreset...)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	res := finalModel.(model)
	return res.selected, nil
}
