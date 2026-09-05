package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lock14/doom-cli/internal/preset"
)

func mockCatalog() *preset.Catalog {
	return &preset.Catalog{
		Presets: []preset.Preset{
			{
				Name:        "Alien Vendetta",
				Engine:      "dsda-doom",
				IWAD:        "doom2.wad",
				Mappacks:    []string{"av.wad"},
				Author:      "Anders Johnsen, Brad Spencer, et al.",
				ReleaseDate: "2002",
			},
			{
				Name:        "Eviternity II",
				Engine:      "uzdoom",
				IWAD:        "doom2.wad",
				Mappacks:    []string{"eviternityii.wad"},
				Author:      "Joshua \"Dragonfly\" O'Sullivan et al.",
				ReleaseDate: "2023",
			},
			{
				Name:        "Sunder",
				Engine:      "dsda-doom",
				IWAD:        "doom2.wad",
				Mappacks:    []string{"sunder.wad"},
				Author:      "Insane_Gazebo",
				ReleaseDate: "2009",
			},
		},
	}
}

func TestModel_Filtering(t *testing.T) {
	cat := mockCatalog()
	m := initialModel(cat, "")

	if len(m.filtered) != 3 {
		t.Fatalf("expected 3 initial presets, got %d", len(m.filtered))
	}

	m.updateFiltered("evit")
	if len(m.filtered) != 1 || m.filtered[0].Name != "Eviternity II" {
		t.Fatalf("expected 1 match 'Eviternity II', got: %v", m.filtered)
	}

	m.updateFiltered("")
	if len(m.filtered) != 3 {
		t.Fatalf("expected reset to 3 presets on empty filter, got %d", len(m.filtered))
	}
}

func TestModel_Navigation(t *testing.T) {
	cat := mockCatalog()
	m := initialModel(cat, "")

	// Down key
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(model)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after Down, got %d", m.cursor)
	}

	// Enter key
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if m.selected == nil || m.selected.Name != "Eviternity II" {
		t.Fatalf("expected 'Eviternity II' selected, got: %v", m.selected)
	}
}

func TestRunNumberedMenu(t *testing.T) {
	cat := mockCatalog()

	// Test selecting item 2
	in := strings.NewReader("2\n")
	var out bytes.Buffer
	selected, err := RunNumberedMenu(cat, in, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected == nil || selected.Name != "Eviternity II" {
		t.Fatalf("expected Eviternity II, got %v", selected)
	}

	// Test quit 'q'
	in = strings.NewReader("q\n")
	out.Reset()
	selected, err = RunNumberedMenu(cat, in, &out)
	if err != nil {
		t.Fatalf("unexpected error on quit: %v", err)
	}
	if selected != nil {
		t.Fatalf("expected nil selection on quit, got: %v", selected)
	}
}

func TestModel_View_Layouts(t *testing.T) {
	cat := mockCatalog()

	tests := []struct {
		name        string
		width       int
		height      int
		expectSide  bool
		expectTerms []string
	}{
		{
			name:        "wide terminal side-by-side",
			width:       120,
			height:      30,
			expectSide:  true,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM PRESET LAUNCHER", "Preset:"},
		},
		{
			name:        "tall wide terminal",
			width:       120,
			height:      50,
			expectSide:  true,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM PRESET LAUNCHER", "Preset:"},
		},
		{
			name:        "short wide terminal",
			width:       100,
			height:      14,
			expectSide:  true,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM PRESET LAUNCHER", "Preset:"},
		},
		{
			name:        "narrow terminal stacked",
			width:       80,
			height:      24,
			expectSide:  false,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM PRESET LAUNCHER", "Preset:"},
		},
		{
			name:        "tall narrow terminal stacked",
			width:       80,
			height:      45,
			expectSide:  false,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM PRESET LAUNCHER", "Preset:"},
		},
		{
			name:        "short narrow terminal stacked",
			width:       80,
			height:      16,
			expectSide:  false,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM PRESET LAUNCHER", "Preset:"},
		},
		{
			name:        "extra wide terminal side-by-side (190 columns)",
			width:       190,
			height:      40,
			expectSide:  true,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM PRESET LAUNCHER", "Preset:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel(cat, "")
			newM, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			m = newM.(model)

			view := m.View()
			for _, term := range tt.expectTerms {
				if !strings.Contains(view, term) {
					t.Errorf("expected view to contain %q, view was:\n%s", term, view)
				}
			}

			// Verify that every line in view fits within the terminal width and has proper borders
			lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
			if len(lines) > tt.height {
				t.Errorf("view height %d exceeds terminal height %d", len(lines), tt.height)
			}
			var borderLineCount int
			for lineIdx, line := range lines {
				w := lipgloss.Width(line)
				if w > tt.width {
					t.Errorf("line %d width %d exceeds terminal width %d: %q", lineIdx, w, tt.width, line)
				}
				trimmed := strings.TrimRight(line, " ")
				if strings.Contains(line, "╭") {
					borderLineCount++
					if !strings.HasSuffix(trimmed, "╮") {
						t.Errorf("top border line %d expected to end with '╮', got %q", lineIdx, trimmed)
					}
					if w != tt.width {
						t.Errorf("top border line %d width %d != terminal width %d", lineIdx, w, tt.width)
					}
				}
				if strings.Contains(line, "╰") {
					borderLineCount++
					if !strings.HasSuffix(trimmed, "╯") {
						t.Errorf("bottom border line %d expected to end with '╯', got %q", lineIdx, trimmed)
					}
					if w != tt.width {
						t.Errorf("bottom border line %d width %d != terminal width %d", lineIdx, w, tt.width)
					}
				}
				if strings.Contains(line, "│") {
					borderLineCount++
					if !strings.HasSuffix(trimmed, "│") {
						t.Errorf("content line %d expected to end with '│', got %q", lineIdx, trimmed)
					}
					if w != tt.width {
						t.Errorf("content line %d width %d != terminal width %d", lineIdx, w, tt.width)
					}
				}
			}
			if borderLineCount == 0 {
				t.Errorf("expected border lines in view, found none")
			}

			// Move cursor down to UZDoom preset
			newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = newM.(model)
			viewUZDoom := m.View()

			if !strings.Contains(viewUZDoom, "[UZDoom]") {
				t.Errorf("expected view to contain [UZDoom], view was:\n%s", viewUZDoom)
			}
		})
	}
}

func TestModel_View_QuittingAndSelected(t *testing.T) {
	cat := mockCatalog()
	m := initialModel(cat, "")

	m.quitting = true
	if view := m.View(); !strings.Contains(view, "Cancelled.") {
		t.Errorf("expected 'Cancelled.', got %q", view)
	}

	m.quitting = false
	m.selected = &cat.Presets[0]
	if view := m.View(); !strings.Contains(view, "Launching Alien Vendetta") {
		t.Errorf("expected 'Launching Alien Vendetta', got %q", view)
	}
}

func TestModel_ReadmeViewer(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "av.txt")
	txtContent := []byte(
		"Title: Alien Vendetta\n\xdb\xb0\xdb\xb1\xdb\xb2\nAuthor: Anders Johnsen\nDescription: Megawad",
	)
	if err := os.WriteFile(txtPath, txtContent, 0644); err != nil {
		t.Fatal(err)
	}

	cat := mockCatalog()
	m := initialModel(cat, tmpDir)

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
