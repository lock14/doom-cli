package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestModel_View_Layouts(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	// 3 corresponds to termenv.Ascii (uncolored profile).
	lipgloss.SetColorProfile(3)
	defer lipgloss.SetColorProfile(origProfile)

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
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM", "Filter:", "Presets (", "Preset Details"},
		},
		{
			name:        "tall wide terminal",
			width:       120,
			height:      50,
			expectSide:  true,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM", "Filter:", "Presets (", "Preset Details"},
		},
		{
			name:        "short wide terminal",
			width:       100,
			height:      14,
			expectSide:  true,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM", "Filter:", "Presets (", "Preset Details"},
		},
		{
			name:        "narrow terminal stacked",
			width:       80,
			height:      24,
			expectSide:  false,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM", "Filter:", "Presets (", "Preset Details"},
		},
		{
			name:        "tall narrow terminal stacked",
			width:       80,
			height:      45,
			expectSide:  false,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM", "Filter:", "Presets (", "Preset Details"},
		},
		{
			name:        "short narrow terminal stacked",
			width:       80,
			height:      16,
			expectSide:  false,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM", "Filter:", "Presets (", "Preset Details"},
		},
		{
			name:        "extra wide terminal side-by-side (190 columns)",
			width:       190,
			height:      40,
			expectSide:  true,
			expectTerms: []string{"Alien Vendetta", "[DSDA]", "DOOM", "Filter:", "Presets (", "Preset Details"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel(cat, nil, ClassicTheme, false)
			newM, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			m = newM.(model)

			view := m.View()
			for _, term := range tt.expectTerms {
				if !strings.Contains(view, term) {
					t.Errorf("expected view to contain %q, view was:\n%s", term, view)
				}
			}

			// Verify that every line in view fits within the terminal width and has proper borders
			rawLines := strings.Split(view, "\n")
			if len(rawLines) > tt.height {
				t.Errorf("raw view line count %d exceeds terminal height %d", len(rawLines), tt.height)
			}
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

func TestCalculateSearchWidth(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		expected  int
	}{
		{name: "narrow terminal clamped to min", termWidth: 40, expected: minSearchWidth},
		{name: "medium terminal responsive", termWidth: 80, expected: 35},
		{name: "wide terminal clamped to max", termWidth: 120, expected: maxSearchWidth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSearchWidth(tt.termWidth)
			if got != tt.expected {
				t.Errorf("calculateSearchWidth(%d) = %d, want %d", tt.termWidth, got, tt.expected)
			}
		})
	}
}

func TestCalculateReadmeDimensions(t *testing.T) {
	tests := []struct {
		name           string
		width, height  int
		wantBox, wantW int
		wantH          int
	}{
		{
			name:    "standard dimensions",
			width:   100,
			height:  30,
			wantBox: 98,
			wantW:   96,
			wantH:   22,
		},
		{
			name:    "narrow and short dimensions clamped",
			width:   30,
			height:  10,
			wantBox: minBoxWidth,
			wantW:   minBoxWidth - 2,
			wantH:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boxW, vpW, vpH := calculateReadmeDimensions(tt.width, tt.height)
			if boxW != tt.wantBox || vpW != tt.wantW || vpH != tt.wantH {
				t.Errorf("calculateReadmeDimensions(%d, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tt.width, tt.height, boxW, vpW, vpH, tt.wantBox, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestComputeLayout(t *testing.T) {
	cat := mockCatalog()
	tests := []struct {
		name       string
		width      int
		height     int
		wantSide   bool
		minVisible int
	}{
		{name: "standard side-by-side 120x30", width: 120, height: 30, wantSide: true, minVisible: 15},
		{name: "wide terminal 190x40", width: 190, height: 40, wantSide: true, minVisible: 25},
		{name: "exact side-by-side boundary 90x24", width: 90, height: 24, wantSide: true, minVisible: 10},
		{name: "stacked mode 80x24", width: 80, height: 24, wantSide: false, minVisible: 5},
		{name: "short stacked mode 80x16", width: 80, height: 16, wantSide: false, minVisible: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel(cat, nil, ClassicTheme, false)
			m.width = tt.width
			m.height = tt.height

			geom := m.computeLayout()
			if geom.sideBySide != tt.wantSide {
				t.Fatalf("expected sideBySide=%v, got %v", tt.wantSide, geom.sideBySide)
			}

			if geom.maxVisible < tt.minVisible {
				t.Errorf("expected maxVisible >= %d, got %d", tt.minVisible, geom.maxVisible)
			}

			if geom.sideBySide {
				totalW := geom.leftWidth + 2 + geom.gutter + geom.rightWidth + 2
				if totalW != tt.width {
					t.Errorf("side-by-side total width %d != terminal width %d", totalW, tt.width)
				}
			} else {
				totalW := geom.boxWidth + 2
				if totalW != tt.width {
					t.Errorf("stacked total width %d != terminal width %d", totalW, tt.width)
				}
			}
		})
	}
}

func TestCalculateInteriorHeight(t *testing.T) {
	tests := []struct {
		name       string
		termHeight int
		expected   int
	}{
		{name: "standard 30-line terminal", termHeight: 30, expected: 22},
		{name: "standard 24-line terminal", termHeight: 24, expected: 16},
		{name: "short 14-line terminal", termHeight: 14, expected: 6},
		{name: "very short 10-line terminal clamped to min", termHeight: 10, expected: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateInteriorHeight(tt.termHeight)
			if got != tt.expected {
				t.Errorf("calculateInteriorHeight(%d) = %d, want %d", tt.termHeight, got, tt.expected)
			}
		})
	}
}

func TestModel_View_24RowHeightBudget(t *testing.T) {
	t.Parallel()
	cat := mockCatalog()
	tests := []struct {
		name  string
		width int
	}{
		{name: "stacked 80x24", width: 80},
		{name: "side-by-side 100x24", width: 100},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := initialModel(cat, nil, ClassicTheme, false)
			newM, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: 24})
			m = newM.(model)

			lines := strings.Split(m.View(), "\n")
			if len(lines) > 24 {
				t.Errorf("%s View() produced %d lines, want <= 24", tt.name, len(lines))
			}
		})
	}
}
