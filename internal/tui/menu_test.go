package tui

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestRunNumberedMenu_EOFAndNilCatalog(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer

	// Empty reader (EOF before newline) returns nil, nil cleanly
	selected, err := RunNumberedMenu(mockCatalog(), strings.NewReader(""), &out)
	if err != nil || selected != nil {
		t.Fatalf("RunNumberedMenu on EOF = (%v, %v), want (nil, nil)", selected, err)
	}

	// Nil catalog does not panic
	out.Reset()
	selected, err = RunNumberedMenu(nil, strings.NewReader(""), &out)
	if err != nil || selected != nil {
		t.Fatalf("RunNumberedMenu on nil catalog = (%v, %v), want (nil, nil)", selected, err)
	}

	// initialModel with nil catalog does not panic on View or updateFiltered
	m := initialModel(nil, nil, ClassicTheme, false)
	m.updateFiltered("doom")
	view := m.View()
	if !strings.Contains(view, "0 / 0 presets") {
		t.Errorf("expected '0 / 0 presets' for nil catalog view, got:\n%s", view)
	}
}
