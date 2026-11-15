package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lock14/doom-cli/internal/catalog"
)

func TestModel_ReadmeViewer(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	// 3 corresponds to termenv.Ascii (uncolored profile).
	lipgloss.SetColorProfile(3)
	defer lipgloss.SetColorProfile(origProfile)

	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "av.txt")
	txtContent := []byte(
		"Title: Alien Vendetta\n\xdb\xb0\xdb\xb1\xdb\xb2\nAuthor: Anders Johnsen\nDescription: Megawad",
	)
	if err := os.WriteFile(txtPath, txtContent, 0644); err != nil {
		t.Fatal(err)
	}

	cat := mockCatalog()
	m := initialModel(cat, []string{tmpDir}, ClassicTheme, false)

	// In initial view, readme tag and Author/Released should appear in preview
	viewInitial := m.View()
	if !strings.Contains(viewInitial, "Anders Johnsen") {
		t.Errorf("expected view to contain Author, got:\n%s", viewInitial)
	}
	if !strings.Contains(viewInitial, "Released:      2002") {
		t.Errorf("expected view to contain Released, got:\n%s", viewInitial)
	}
	if !strings.Contains(viewInitial, "av.txt") {
		t.Errorf("expected view to list av.txt, got:\n%s", viewInitial)
	}

	// Pressing tab opens the readme
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(model)
	if !m.viewingReadme {
		t.Fatalf("expected viewingReadme to be true after tab")
	}

	viewReadme := m.View()
	if !strings.Contains(viewReadme, "README: Alien Vendetta") {
		t.Errorf("expected view to contain README header, got:\n%s", viewReadme)
	}
	if !strings.Contains(viewReadme, "Title: Alien Vendetta") {
		t.Errorf("expected view to contain readme content, got:\n%s", viewReadme)
	}
	if strings.Contains(viewReadme, "۰") {
		t.Errorf("expected view not to contain Arabic numeral from CP437, got:\n%s", viewReadme)
	}
	if !strings.Contains(viewReadme, "█░█▒█▓") {
		t.Errorf("expected view to contain decoded CP437 block art █░█▒█▓, got:\n%s", viewReadme)
	}

	// Verify that readme view lines fit within terminal dimensions
	readmeLines := strings.Split(strings.TrimSuffix(viewReadme, "\n"), "\n")
	if len(readmeLines) > m.height {
		t.Errorf("readme view height %d exceeds terminal height %d", len(readmeLines), m.height)
	}
	for lineIdx, line := range readmeLines {
		w := lipgloss.Width(line)
		if w > m.width {
			t.Errorf("readme line %d width %d exceeds terminal width %d: %q", lineIdx, w, m.width, line)
		}
	}

	// Pressing esc closes the readme
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(model)
	if m.viewingReadme {
		t.Fatalf("expected viewingReadme to be false after esc")
	}
}

func TestModel_ReadmeViewer_TopBottomAndShiftQ(t *testing.T) {
	tmpDir := t.TempDir()
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, "Readme line content for scrolling test")
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "av.txt"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := mockCatalog()
	m := initialModel(cat, []string{tmpDir}, ClassicTheme, false)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = newM.(model)

	// Open readme viewer
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(model)
	if !m.viewingReadme {
		t.Fatal("expected viewingReadme == true")
	}

	// Resizing while inside README viewer updates both viewport and search input width
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	m = newM.(model)
	if m.input.Width != calculateSearchWidth(60) {
		t.Errorf("expected m.input.Width == %d after resize in README viewer, got %d",
			calculateSearchWidth(60), m.input.Width)
	}

	// Jump to bottom with 'G'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = newM.(model)
	if !m.viewport.AtBottom() {
		t.Errorf("expected viewport at bottom after 'G'")
	}

	// Jump to top with 'g'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newM.(model)
	if !m.viewport.AtTop() {
		t.Errorf("expected viewport at top after 'g'")
	}

	// Jump to bottom with End and top with Home
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = newM.(model)
	if !m.viewport.AtBottom() {
		t.Errorf("expected viewport at bottom after End")
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = newM.(model)
	if !m.viewport.AtTop() {
		t.Errorf("expected viewport at top after Home")
	}

	// Close with 'Q'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	m = newM.(model)
	if m.viewingReadme {
		t.Errorf("expected viewingReadme == false after 'Q'")
	}
}

func TestModel_ContextualHelpView_AndFetchAssets(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.wad"), []byte("pwad"), 0o644)

	cat := mockCatalog()
	m := initialModel(cat, []string{wadsDir}, ClassicTheme, false)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	// Press F1 to open Contextual Help
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = newM.(model)
	if !m.viewingHelp {
		t.Fatal("expected viewingHelp == true after F1")
	}
	helpView := m.View()
	if lines := strings.Split(helpView, "\n"); len(lines) > 24 {
		t.Errorf("Help View() at 80x24 produced %d lines, want <= 24", len(lines))
	}
	if !strings.Contains(helpView, "Contextual Help") {
		t.Errorf("expected 'Contextual Help' header in Help view:\n%s", helpView)
	}

	// Close Help via F1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = newM.(model)
	if m.viewingHelp {
		t.Fatal("expected viewingHelp == false after second F1")
	}

	// Press Ctrl+F on already installed Alien Vendetta
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = newM.(model)
	if !strings.Contains(m.statusMsg, "Ready") {
		t.Errorf("expected statusMsg to report Ready, got %q", m.statusMsg)
	}
}

func TestHelpTrueColorAlignment_And_80x24LongTitleHeader(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	// 0 corresponds to termenv.TrueColor so ANSI SGR codes are emitted.
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(origProfile)

	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	m := initialModel(cat, nil, PlasmaTheme, false, "back-to-saturn-x-episode-2")
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	// 1. Verify TrueColor buildHelpContent aligns all keybinding description columns
	helpText := m.buildHelpContent(helpModeBrowser)
	targets := []string{
		"Move selection up or down",
		"Open Contextual Help",
		"Launch selected preset",
		"Ephemeral one-shot launch with current options",
	}
	wantCol := -1
	for _, line := range strings.Split(helpText, "\n") {
		for _, desc := range targets {
			idx := strings.Index(line, desc)
			if idx > 0 {
				col := lipgloss.Width(line[:idx])
				if wantCol == -1 {
					wantCol = col
				} else if col != wantCol {
					t.Errorf("TrueColor help description %q starts at col %d, want %d (line=%q)",
						desc, col, wantCol, line)
				}
			}
		}
	}
	if wantCol != 26 {
		t.Errorf("expected TrueColor help descriptions at col 26, got %d", wantCol)
	}

	// 2. Verify Launch Options on long title ("Back to Saturn X Episode 2: Tower in the Fountain of Sparks")
	// truncates header cleanly within 80 columns and stays <= 24 lines.
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = newM.(model)
	optView := m.View()
	rawLines := strings.Split(optView, "\n")
	if len(rawLines) > 24 {
		t.Errorf("Launch Options View() for long title at 80x24 produced %d lines, want <= 24", len(rawLines))
	}
	for i, line := range rawLines {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line %d visible width %d > 80: %q", i, w, line)
		}
	}
}
