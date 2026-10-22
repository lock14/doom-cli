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
				ID:          "alien-vendetta",
				Name:        "Alien Vendetta",
				Engine:      "dsda-doom",
				IWAD:        "doom2.wad",
				Files:       []preset.FileSpec{{Name: "av.wad"}},
				Author:      "Anders Johnsen, Brad Spencer, et al.",
				ReleaseDate: "2002",
			},
			{
				ID:          "eviternity-ii",
				Name:        "Eviternity II",
				Engine:      "uzdoom",
				IWAD:        "doom2.wad",
				Files:       []preset.FileSpec{{Name: "eviternityii.wad"}},
				Author:      "Joshua \"Dragonfly\" O'Sullivan et al.",
				ReleaseDate: "2023",
			},
			{
				ID:          "sunder",
				Name:        "Sunder",
				Engine:      "dsda-doom",
				IWAD:        "doom2.wad",
				Files:       []preset.FileSpec{{Name: "sunder.wad"}},
				Author:      "Insane_Gazebo",
				ReleaseDate: "2009",
			},
		},
	}
}

func TestModel_Filtering(t *testing.T) {
	cat := mockCatalog()
	m := initialModel(cat, nil, ClassicTheme, false)

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
	m := initialModel(cat, nil, ClassicTheme, false)

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
	m := initialModel(cat, nil, ClassicTheme, false)

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
			wantW:   36,
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

func TestCapsuleAndKeyHelpRenderers(t *testing.T) {
	brand := renderBrandPill()
	if !strings.Contains(brand, "DOOM") {
		t.Errorf("renderBrandPill() = %q, expected 'DOOM'", brand)
	}

	nerdBrand := defaultStyles.RenderBrandPill(true)
	if !strings.Contains(nerdBrand, "DOOM") || !strings.Contains(nerdBrand, "") || !strings.Contains(nerdBrand, "") {
		t.Errorf("RenderBrandPill(true) = %q, expected DOOM with capsule caps", nerdBrand)
	}

	stats := renderStatsPill(10, 32)
	if !strings.Contains(stats, "10 / 32 presets") {
		t.Errorf("renderStatsPill(10, 32) = %q, expected '10 / 32 presets'", stats)
	}

	topPill := renderScrollPill(0.0)
	if !strings.Contains(topPill, "Top") {
		t.Errorf("renderScrollPill(0.0) = %q, expected 'Top'", topPill)
	}
	midPill := renderScrollPill(0.5)
	if !strings.Contains(midPill, "50%") {
		t.Errorf("renderScrollPill(0.5) = %q, expected '50%%'", midPill)
	}
	endPill := renderScrollPill(1.0)
	if !strings.Contains(endPill, "End") {
		t.Errorf("renderScrollPill(1.0) = %q, expected 'End'", endPill)
	}

	help := formatKeyHelp([]keyHelp{
		{"Enter", "Launch"},
		{"Esc", "Quit"},
	})
	if !strings.Contains(help, "Enter") || !strings.Contains(help, "Launch") || !strings.Contains(help, "•") {
		t.Errorf("formatKeyHelp() = %q, expected keys and separator", help)
	}
}

func TestInitialModel_WithInitialPreset(t *testing.T) {
	cat := mockCatalog()

	tests := []struct {
		name          string
		initialPreset []string
		expectedIdx   int
	}{
		{
			name:          "no initial preset provided",
			initialPreset: nil,
			expectedIdx:   0,
		},
		{
			name:          "empty string initial preset",
			initialPreset: []string{""},
			expectedIdx:   0,
		},
		{
			name:          "matching preset exact case",
			initialPreset: []string{"Eviternity II"},
			expectedIdx:   1,
		},
		{
			name:          "matching preset case-insensitive",
			initialPreset: []string{"sunder"},
			expectedIdx:   2,
		},
		{
			name:          "unknown preset falls back to 0",
			initialPreset: []string{"NonExistentWad"},
			expectedIdx:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel(cat, nil, ClassicTheme, false, tt.initialPreset...)
			if m.cursor != tt.expectedIdx {
				t.Errorf("initialModel(cat, \"\", %v) cursor = %d, expected %d",
					tt.initialPreset, m.cursor, tt.expectedIdx)
			}
		})
	}
}

func TestModel_WithCustomTheme(t *testing.T) {
	cat := mockCatalog()
	m := initialModel(cat, nil, PlasmaTheme, false)

	if m.theme.Name != "plasma" {
		t.Fatalf("expected theme 'plasma', got %q", m.theme.Name)
	}

	view := m.View()
	if !strings.Contains(view, "Alien Vendetta") {
		t.Errorf("expected view to contain Alien Vendetta with custom theme")
	}
}

func TestFormatEngineTag(t *testing.T) {
	styles := CompileStyles(ClassicTheme)
	tests := []struct {
		engine   string
		contains string
	}{
		{"dsda-doom", "[DSDA]"},
		{"uzdoom", "[UZDoom]"},
		{"woof", "[WOOF"},
		{"crispy-doom", "[CRISPY]"},
	}

	for _, tt := range tests {
		t.Run(tt.engine, func(t *testing.T) {
			rendered := formatEngineTag(tt.engine, styles)
			if !strings.Contains(rendered, tt.contains) {
				t.Errorf("expected tag for %s to contain %q, got %q", tt.engine, tt.contains, rendered)
			}
		})
	}
}

func TestModel_CustomPresetAndEngine(t *testing.T) {
	cat := &preset.Catalog{
		Engines: map[string]preset.EngineConfig{
			"woof": {
				ID:          "woof",
				DisplayName: "Woof!",
				Badge:       "WOOF",
				Description: "Woof! MBF21 Engine",
			},
		},
		Presets: []preset.Preset{
			{
				ID:     "custom-map",
				Name:   "CustomMap",
				Engine: "woof",
				IWAD:   "DOOM2.WAD",
				Files:  []preset.FileSpec{{Name: "custom.wad"}},
				Args:   []string{"-skill", "4"},
				Layer:  preset.LayerUser,
			},
		},
	}

	m := initialModel(cat, nil, ClassicTheme, false)
	view := m.View()

	if !strings.Contains(view, "[WOOF") {
		t.Errorf("expected custom engine tag [WOOF] in view:\n%s", view)
	}
	if !strings.Contains(view, "[Custom WAD]") {
		t.Errorf("expected [Custom WAD] badge in view:\n%s", view)
	}
	if !strings.Contains(view, "Woof! MBF21 Engine") {
		t.Errorf("expected custom engine description in preview:\n%s", view)
	}
	if !strings.Contains(view, "-skill 4") {
		t.Errorf("expected launch args in preview:\n%s", view)
	}

	var menuOut bytes.Buffer
	_, _ = RunNumberedMenu(cat, strings.NewReader("q\n"), &menuOut)
	if !strings.Contains(menuOut.String(), "[Woof!]") {
		t.Errorf("expected numbered menu to use custom engine DisplayName [Woof!], got:\n%s", menuOut.String())
	}
}

func TestTruncateANSI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
		wantCols int
	}{
		{
			name:     "fits unchanged",
			input:    "Alien Vendetta",
			maxWidth: 20,
			want:     "Alien Vendetta",
			wantCols: 14,
		},
		{
			name:     "plain truncation",
			input:    "Alien Vendetta",
			maxWidth: 6,
			want:     "Alien…",
			wantCols: 6,
		},
		{
			name:     "ansi styled truncation preserves sgr and resets",
			input:    "\x1b[38;2;255;0;0mAlien Vendetta\x1b[0m",
			maxWidth: 6,
			want:     "\x1b[38;2;255;0;0mAlien\x1b[0m…",
			wantCols: 6,
		},
		{
			name:     "width 1",
			input:    "\x1b[31mHello\x1b[0m",
			maxWidth: 1,
			want:     "…",
			wantCols: 1,
		},
		{
			name:     "width 0",
			input:    "Hello",
			maxWidth: 0,
			want:     "",
			wantCols: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateANSI(tc.input, tc.maxWidth)
			if got != tc.want {
				t.Errorf("truncateANSI(%q, %d) = %q, want %q", tc.input, tc.maxWidth, got, tc.want)
			}
			if cols := lipgloss.Width(got); cols != tc.wantCols {
				t.Errorf("lipgloss.Width(truncateANSI) = %d, want %d", cols, tc.wantCols)
			}
		})
	}
}

func TestModel_PagingAndControlNavigation(t *testing.T) {
	cat, err := preset.LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog: %v", err)
	}
	m := initialModel(cat, nil, ClassicTheme, false)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	// Ctrl+J moves down, Ctrl+K moves up
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = newM.(model)
	if m.cursor != 1 {
		t.Fatalf("expected cursor 1 after Ctrl+J, got %d", m.cursor)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = newM.(model)
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0 after Ctrl+K, got %d", m.cursor)
	}

	// PgDown advances by full page (geom.maxVisible)
	geom := m.computeLayout()
	halfStep := geom.maxVisible / 2
	if halfStep < 1 {
		halfStep = 1
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = newM.(model)
	if m.cursor != geom.maxVisible {
		t.Fatalf("expected cursor %d after PgDown, got %d", geom.maxVisible, m.cursor)
	}

	// Ctrl+D advances by half page (halfStep)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = newM.(model)
	if m.cursor != geom.maxVisible+halfStep {
		t.Fatalf("expected cursor %d after Ctrl+D, got %d", geom.maxVisible+halfStep, m.cursor)
	}

	// PgUp moves back by full page (geom.maxVisible)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = newM.(model)
	if m.cursor != halfStep {
		t.Fatalf("expected cursor %d after PgUp, got %d", halfStep, m.cursor)
	}

	// Ctrl+U moves back by half page to 0
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = newM.(model)
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0 after Ctrl+U, got %d", m.cursor)
	}

	// Printable 'q' and 'i' on empty filter feed into text input rather than quitting
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = newM.(model)
	if m.quitting {
		t.Fatal("expected 'q' not to quit launcher when filter is focused")
	}
	if m.input.Value() != "q" {
		t.Fatalf("expected filter input 'q', got %q", m.input.Value())
	}

	// Home and End move the filter text input cursor without resetting preset list cursor
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u', 'a', 'k', 'e'}})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = newM.(model)
	if m.input.Position() != 0 {
		t.Fatalf("expected filter input cursor at 0 after Home, got %d", m.input.Position())
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = newM.(model)
	if m.input.Position() != len(m.input.Value()) {
		t.Fatalf("expected filter input cursor at %d after End, got %d", len(m.input.Value()), m.input.Position())
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

func TestModel_80x24_AllCuratedPresetsShowIWADAndFiles(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(3)
	defer lipgloss.SetColorProfile(origProfile)

	cat, err := preset.LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog: %v", err)
	}

	tmpDir := t.TempDir()
	// Create a readme for Legacy of Rust (6 files) to test worst-case 80x24 file density
	if err := os.WriteFile(filepath.Join(tmpDir, "id1.txt"), []byte("Legacy of Rust readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := initialModel(cat, []string{tmpDir}, ClassicTheme, false)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	for idx, p := range m.filtered {
		m.cursor = idx
		view := m.View()
		if !strings.Contains(view, "IWAD:") || !strings.Contains(view, p.IWAD) {
			t.Errorf("preset %q at 80x24 missing IWAD %q in view:\n%s", p.Name, p.IWAD, view)
		}
		if len(p.Files) > 0 {
			if !strings.Contains(view, "Files:") {
				t.Errorf("preset %q at 80x24 missing Files: label in view:\n%s", p.Name, view)
			}
			for _, f := range p.Files {
				if !strings.Contains(view, f.Name) {
					t.Errorf("preset %q at 80x24 clipped required/optional file %q in view:\n%s", p.Name, f.Name, view)
				}
			}
		}
		if p.ID == "legacy-of-rust" && !strings.Contains(view, "[✓ Readme]") {
			t.Errorf("expected Legacy of Rust at 80x24 to show [✓ Readme], got:\n%s", view)
		}
	}
}

func TestModel_ReadmeCache_NoDiskReadOnRender(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "av.txt")
	if err := os.WriteFile(txtPath, []byte("Readme content"), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := mockCatalog()
	m := initialModel(cat, []string{tmpDir}, ClassicTheme, false)

	// Remove the file from disk after initialModel has populated readmeCache
	if err := os.Remove(txtPath); err != nil {
		t.Fatal(err)
	}

	got, ok := m.resolveReadme(cat.Presets[0])
	if !ok || got != txtPath {
		t.Fatalf("resolveReadme = (%q, %v), want cached (%q, true)", got, ok, txtPath)
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

func TestPadOrTruncateName(t *testing.T) {
	t.Parallel()
	if got := padOrTruncateName("Doom", 10); lipgloss.Width(got) != 10 {
		t.Errorf("padOrTruncateName(\"Doom\", 10) width = %d, want 10", lipgloss.Width(got))
	}
	if got := padOrTruncateName("Back to Saturn X Episode 1: Get Out Of My Stations", 18); lipgloss.Width(got) != 18 ||
		!strings.HasSuffix(got, "…") {
		t.Errorf(
			"padOrTruncateName long string = %q (width %d), want width 18 ending with '…'",
			got, lipgloss.Width(got),
		)
	}
	if got := padOrTruncateName("日本語テストマップ", 10); lipgloss.Width(got) != 10 {
		t.Errorf("padOrTruncateName wide CJK string = %q (width %d), want width 10", got, lipgloss.Width(got))
	}
}

func TestRenderBoxWithTitle_WideCharactersAndEngineBadge(t *testing.T) {
	t.Parallel()
	box := renderBoxWithTitle("body", 24, "README: 日本語テストマップ (test.txt)", true, defaultStyles)
	lines := strings.Split(box, "\n")
	if len(lines) == 0 {
		t.Fatal("expected non-empty box lines")
	}
	if gotW := lipgloss.Width(lines[0]); gotW != 26 {
		t.Errorf("top border width with CJK title = %d, want 26: %q", gotW, lines[0])
	}

	cat := &preset.Catalog{
		Engines: map[string]preset.EngineConfig{
			"cjk-eng": {
				ID:    "cjk-eng",
				Badge: "日本語エンジン",
			},
		},
	}
	tag := formatEngineTagWithCatalog(cat, "cjk-eng", defaultStyles)
	if gotW := lipgloss.Width(tag); gotW != 8 {
		t.Errorf("formatEngineTagWithCatalog wide CJK badge width = %d, want 8: %q", gotW, tag)
	}
}

func TestModel_80x24_CustomPresetLongFilenamesAndManyFiles(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(3)
	defer lipgloss.SetColorProfile(origProfile)

	tmpDir := t.TempDir()
	longFirst := "very_long_custom_megawad_episode_one_final_v2.wad"
	if err := os.WriteFile(filepath.Join(tmpDir, longFirst), []byte("PWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "many.txt"), []byte("Readme text"), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := &preset.Catalog{
		Presets: []preset.Preset{
			{
				ID:     "custom-long-6",
				Name:   "Custom Long Filenames",
				Engine: "dsda-doom",
				IWAD:   "DOOM2.WAD",
				Files: []preset.FileSpec{
					{Name: longFirst},
					{Name: "second_col_patch.deh"},
					{Name: "third_long_custom_resource_pack_v1.wad"},
					{Name: "fourth_col_music.wad", Optional: true},
					{Name: "fifth_extra_textures_pack_final.wad"},
					{Name: "sixth_col_fix.bex"},
				},
				Layer: preset.LayerUser,
			},
			{
				ID:     "custom-many-12",
				Name:   "Custom Many Files",
				Engine: "uzdoom",
				IWAD:   "DOOM2.WAD",
				Files: []preset.FileSpec{
					{Name: "many.wad"},
					{Name: "part02.wad"},
					{Name: "part03.wad"},
					{Name: "part04.wad"},
					{Name: "part05.wad"},
					{Name: "part06.wad"},
					{Name: "part07.wad"},
					{Name: "part08.wad"},
					{Name: "part09.wad"},
					{Name: "part10.wad"},
					{Name: "part11.wad"},
					{Name: "part12.wad"},
				},
				Layer: preset.LayerUser,
			},
		},
	}

	m := initialModel(cat, []string{tmpDir}, ClassicTheme, false)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	// 1. Verify long filenames in column 1 do not push column 2 or status badges off-screen at 80x24
	viewLong := m.View()
	for _, want := range []string{
		"IWAD:",
		"Files:",
		"[✓ Found]",
		"second_col_patch… [✗ Missing]",
		"[Optional]",
		"sixth_col_fix.bex [✗ Missing]",
	} {
		if !strings.Contains(viewLong, want) {
			t.Errorf("expected 80x24 view for custom-long-6 to contain %q, got:\n%s", want, viewLong)
		}
	}

	// 2. Verify >10 files + README at 80x24 displays (+N more files) and preserves [✓ Readme]
	m.cursor = 1
	viewMany := m.View()
	for _, want := range []string{
		"IWAD:",
		"Files:",
		"(+4 more files)",
		"many.txt",
		"[✓ Readme]",
	} {
		if !strings.Contains(viewMany, want) {
			t.Errorf("expected 80x24 view for custom-many-12 to contain %q, got:\n%s", want, viewMany)
		}
	}
}
