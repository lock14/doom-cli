package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock14/doom-cli/internal/preset"
)

func mockCatalog() *preset.Catalog {
	return &preset.Catalog{
		Presets: []preset.Preset{
			{Name: "Alien Vendetta", Engine: "dsda-doom", IWAD: "doom2.wad", Mappacks: []string{"av.wad"}},
			{Name: "Eviternity II", Engine: "uzdoom", IWAD: "doom2.wad", Mappacks: []string{"eviternityii.wad"}},
			{Name: "Sunder", Engine: "dsda-doom", IWAD: "doom2.wad", Mappacks: []string{"sunder.wad"}},
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
			name:        "narrow terminal stacked",
			width:       80,
			height:      24,
			expectSide:  false,
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
