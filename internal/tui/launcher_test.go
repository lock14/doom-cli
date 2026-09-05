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
