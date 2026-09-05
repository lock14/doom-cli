// Package tui provides an interactive terminal user interface for launching Doom presets.
package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"golang.org/x/term"

	"github.com/lock14/doom-cli/internal/preset"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#B30000")).
			Padding(0, 1)

	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FFFF"))

	tagDSDASStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))

	tagUZDoomStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF66"))

	previewBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#666666")).
			Padding(0, 1)

	foundStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	missingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))
)

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
}

func initialModel(catalog *preset.Catalog, wadsDir string) model {
	ti := textinput.New()
	ti.Placeholder = "Type to search presets..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 35

	return model{
		catalog:  catalog,
		wadsDir:  wadsDir,
		input:    ti,
		filtered: catalog.Presets,
		cursor:   0,
		width:    80,
		height:   24,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

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

	header := titleStyle.Render("DOOM PRESET LAUNCHER") + "\n\n"
	search := fmt.Sprintf("Search: %s\n\n", m.input.View())

	// Build List View (show up to 10 items)
	maxVisible := 10
	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	var listLines []string
	for i := startIdx; i < endIdx; i++ {
		p := m.filtered[i]
		tag := "[UZDoom]"
		tagRender := tagUZDoomStyle.Render(tag)
		if p.Engine == "dsda-doom" {
			tag = "[DSDA]"
			tagRender = tagDSDASStyle.Render(tag)
		}

		line := fmt.Sprintf("%-28s %s", p.Name, tagRender)
		if i == m.cursor {
			listLines = append(listLines, cursorStyle.Render("> "+line))
		} else {
			listLines = append(listLines, "  "+line)
		}
	}

	if len(m.filtered) == 0 {
		listLines = append(listLines, "  No presets matching search.")
	}

	listPane := strings.Join(listLines, "\n")

	// Build Preview View for current item
	var previewPane string
	if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
		cur := m.filtered[m.cursor]
		var previewLines []string
		previewLines = append(previewLines, fmt.Sprintf("Preset:        %s", cur.Name))
		engStr := "UZDoom (Software-Plus / Advanced)"
		if cur.Engine == "dsda-doom" {
			engStr = "DSDA-Doom (MBF21 / Speedrun)"
		}
		previewLines = append(previewLines, fmt.Sprintf("Engine:        %s", engStr))
		if cur.Category != "" {
			previewLines = append(previewLines, fmt.Sprintf("Category:      %s", cur.Category))
		}
		if cur.Compatibility != "" {
			previewLines = append(previewLines, fmt.Sprintf("Compatibility: %s", cur.Compatibility))
		}
		if cur.Description != "" {
			previewLines = append(previewLines, fmt.Sprintf("Description:   %s", cur.Description))
		}

		// Check IWAD
		iwadStatus := missingStyle.Render("[✗ Missing]")
		if _, ok := preset.ResolveFile(m.wadsDir, cur.IWAD); ok {
			iwadStatus = foundStyle.Render("[✓ Found]")
		}
		previewLines = append(previewLines, fmt.Sprintf("IWAD:          %s %s", cur.IWAD, iwadStatus))

		if len(cur.Mappacks) > 0 {
			previewLines = append(previewLines, "Files:")
			for _, mapfile := range cur.Mappacks {
				fStatus := missingStyle.Render("[✗ Missing]")
				if _, ok := preset.ResolveFile(m.wadsDir, mapfile); ok {
					fStatus = foundStyle.Render("[✓ Found]")
				} else if strings.EqualFold(mapfile, "idkfa 2024.wad") {
					fStatus = helpStyle.Render("[Optional]")
				}
				previewLines = append(previewLines, fmt.Sprintf("  - %-22s %s", mapfile, fStatus))
			}
		}

		previewPane = previewBoxStyle.Width(45).Render(strings.Join(previewLines, "\n"))
	}

	// Two columns layout if terminal is wide enough
	var content string
	if m.width >= 80 {
		listCol := lipgloss.NewStyle().Width(38).Render(listPane)
		content = lipgloss.JoinHorizontal(lipgloss.Top, listCol, "   ", previewPane)
	} else {
		content = listPane + "\n\n" + previewPane
	}

	footer := "\n\n" + helpStyle.Render("↑/↓: Navigate • Enter: Launch • Esc: Quit")
	return header + search + content + footer + "\n"
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
