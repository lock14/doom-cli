package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lock14/doom-cli/internal/config"
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

	// Enter key with missing required WADs stays in TUI and sets status message
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if m.selected != nil {
		t.Fatalf("expected nil selected when required WADs are missing, got: %v", m.selected)
	}
	wantStatus := "✗ Missing required files for Eviternity II (press Ctrl+F to fetch)"
	if m.statusMsg != wantStatus {
		t.Errorf("statusMsg = %q, want %q", m.statusMsg, wantStatus)
	}

	// Enter key in README Viewer with missing required WADs closes README Viewer and sets status message
	m.viewingReadme = true
	m.statusMsg = ""
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if m.viewingReadme || m.selected != nil || m.statusMsg != wantStatus {
		t.Fatalf(
			"expected README Enter with missing WADs to return to browser with status %q, "+
				"got viewingReadme=%v selected=%v statusMsg=%q",
			wantStatus, m.viewingReadme, m.selected, m.statusMsg,
		)
	}

	// Provide required WADs on disk; first verify empty IWAD is still blocked by isPresetReady
	wadsDir := t.TempDir()
	for _, fname := range []string{"doom2.wad", "eviternityii.wad"} {
		if err := os.WriteFile(filepath.Join(wadsDir, fname), []byte("WAD"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m.dirIndex = preset.NewDirIndex(wadsDir).WithCatalog(cat)

	savedIWAD := m.filtered[m.cursor].IWAD
	m.filtered[m.cursor].IWAD = ""
	m.statusMsg = ""
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if m.selected != nil || m.statusMsg != wantStatus {
		t.Fatalf("expected empty IWAD preset to be blocked on Enter with status %q, got selected=%v statusMsg=%q",
			wantStatus, m.selected, m.statusMsg)
	}
	m.filtered[m.cursor].IWAD = savedIWAD

	// With valid IWAD and PWADs present, Enter selects the preset
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if m.selected == nil || m.selected.Name != "Eviternity II" {
		t.Fatalf("expected 'Eviternity II' selected when WADs present, got: %v", m.selected)
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

func TestModel_View_QuittingAndSelected(t *testing.T) {
	cat := mockCatalog()
	m := initialModel(cat, nil, ClassicTheme, false)

	m.quitting = true
	if view := m.View(); !strings.Contains(view, "Cancelled.") {
		t.Errorf("expected 'Cancelled.', got %q", view)
	}

	m.selected = &cat.Presets[0]
	if view := m.View(); !strings.Contains(view, "Launching Alien Vendetta") {
		t.Errorf("expected 'Launching Alien Vendetta' when selected and quitting, got %q", view)
	}

	m.quitting = false
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
	brand := defaultStyles.RenderBrandPill(false)
	if !strings.Contains(brand, "DOOM") {
		t.Errorf("RenderBrandPill(false) = %q, expected 'DOOM'", brand)
	}

	nerdBrand := defaultStyles.RenderBrandPill(true)
	if !strings.Contains(nerdBrand, "DOOM") || !strings.Contains(nerdBrand, "") || !strings.Contains(nerdBrand, "") {
		t.Errorf("RenderBrandPill(true) = %q, expected DOOM with capsule caps", nerdBrand)
	}

	stats := defaultStyles.RenderStatsPill(10, 32, false)
	if !strings.Contains(stats, "10 / 32 presets") {
		t.Errorf("RenderStatsPill(10, 32, false) = %q, expected '10 / 32 presets'", stats)
	}

	topPill := defaultStyles.RenderScrollPill(0.0, false)
	if !strings.Contains(topPill, "Top") {
		t.Errorf("RenderScrollPill(0.0, false) = %q, expected 'Top'", topPill)
	}
	midPill := defaultStyles.RenderScrollPill(0.5, false)
	if !strings.Contains(midPill, "50%") {
		t.Errorf("RenderScrollPill(0.5, false) = %q, expected '50%%'", midPill)
	}
	endPill := defaultStyles.RenderScrollPill(1.0, false)
	if !strings.Contains(endPill, "End") {
		t.Errorf("RenderScrollPill(1.0, false) = %q, expected 'End'", endPill)
	}

	help := defaultStyles.FormatKeyHelp([]keyHelp{
		{"Enter", "Launch"},
		{"Esc", "Quit"},
	})
	if !strings.Contains(help, "Enter") || !strings.Contains(help, "Launch") || !strings.Contains(help, "•") {
		t.Errorf("FormatKeyHelp() = %q, expected keys and separator", help)
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
			rendered := formatEngineTagWithCatalog(nil, tt.engine, styles)
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

func TestRenderFooter_NarrowTruncation(t *testing.T) {
	t.Parallel()
	cat := mockCatalog()
	tests := []struct {
		name             string
		width            int
		wantWithReadme   bool
		wantNoReadme     bool
		wantReadmeFooter bool
	}{
		{name: "narrow 60 columns", width: 60, wantWithReadme: true, wantNoReadme: false, wantReadmeFooter: true},
		{name: "compact 50 columns", width: 50, wantWithReadme: true, wantNoReadme: true, wantReadmeFooter: true},
		{name: "compact 40 columns", width: 40, wantWithReadme: true, wantNoReadme: true, wantReadmeFooter: true},
		{name: "standard 80 columns", width: 80, wantWithReadme: false, wantNoReadme: false, wantReadmeFooter: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := initialModel(cat, nil, ClassicTheme, false)
			m.width = tt.width
			m.height = 24

			footerWithReadme := m.renderFooter(true)
			footerNoReadme := m.renderFooter(false)
			readmeFooter := m.renderReadmeFooter()

			checks := []struct {
				label      string
				rendered   string
				wantEllips bool
			}{
				{label: "renderFooter(true)", rendered: footerWithReadme, wantEllips: tt.wantWithReadme},
				{label: "renderFooter(false)", rendered: footerNoReadme, wantEllips: tt.wantNoReadme},
				{label: "renderReadmeFooter()", rendered: readmeFooter, wantEllips: tt.wantReadmeFooter},
			}

			for _, chk := range checks {
				if !strings.HasPrefix(chk.rendered, "\n\n") {
					t.Errorf("%s expected leading \\n\\n, got %q", chk.label, chk.rendered)
				}
				line := strings.TrimPrefix(chk.rendered, "\n\n")
				if strings.Contains(line, "\n") {
					t.Errorf("%s wrapped onto multiple lines: %q", chk.label, line)
				}
				if gotW := lipgloss.Width(line); gotW > tt.width {
					t.Errorf("%s at width %d has visible width %d > %d: %q",
						chk.label, tt.width, gotW, tt.width, line)
				}
				if chk.wantEllips && !strings.HasSuffix(line, "…") {
					t.Errorf("%s at width %d expected trailing ellipsis '…', got %q",
						chk.label, tt.width, line)
				}
				if !chk.wantEllips && strings.HasSuffix(line, "…") {
					t.Errorf("%s at width %d expected no truncation, got %q",
						chk.label, tt.width, line)
				}
			}
		})
	}
}

func TestModel_LaunchOptionsView_EphemeralAndPersistent(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	cat, err := preset.LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}

	m := initialModel(cat, nil, ClassicTheme, false, "Doom II")
	m.customCatalogDir = userDir
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	// Open Launch Options via Ctrl+O
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = newM.(model)
	if !m.viewingLaunchOpts || m.launchOpts == nil {
		t.Fatal("expected viewingLaunchOpts == true after Ctrl+O")
	}

	// Verify 80x24 height budget
	if lines := strings.Split(m.View(), "\n"); len(lines) > 24 {
		t.Errorf("Launch Options View() at 80x24 produced %d lines, want <= 24", len(lines))
	}

	// Move to Skill field (cursor 1) and select Ultra-Violence (4 right presses)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(model)
	for i := 0; i < 4; i++ {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		m = newM.(model)
	}

	// Move to Warp field (cursor 2) and type "15"
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1', '5'}})
	m = newM.(model)

	// Move to Pistol Start (cursor 3) and toggle via Space
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newM.(model)

	// Move to Optional WAD checkbox (focusIdx 9 for DOOM II's idkfa 2024.wad) and uncheck it
	for m.launchOpts.focusIdx < 9 {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newM.(model)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newM.(model)

	// Save persistent override via Ctrl+S
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = newM.(model)
	if !strings.Contains(m.launchOpts.statusMsg, "Saved override") {
		t.Fatalf("expected Saved override status message, got %q", m.launchOpts.statusMsg)
	}

	// Verify override was persisted to disk with exclude_files and args
	_, ov, err := preset.LoadPresetBaseAndOverride(userDir, "doom-ii")
	if err != nil || ov == nil {
		t.Fatalf("expected saved override on disk, err=%v ov=%v", err, ov)
	}
	if len(ov.ExcludeFiles) != 1 {
		t.Errorf("expected 1 excluded optional file, got %v", ov.ExcludeFiles)
	}

	// Reset override via Ctrl+R
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = newM.(model)
	if !strings.Contains(m.launchOpts.statusMsg, "Reset") {
		t.Fatalf("expected Reset status message, got %q", m.launchOpts.statusMsg)
	}
	if _, ovAfter, _ := preset.LoadPresetBaseAndOverride(userDir, "doom-ii"); ovAfter != nil {
		t.Errorf("expected override removed after Ctrl+R, got %+v", ovAfter)
	}

	// Ephemeral launch via Enter when DOOM2.WAD is missing stays in TUI and returns to browser with error status
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if m.quitting || m.selected != nil || m.viewingLaunchOpts {
		t.Fatalf(
			"expected Enter in Launch Options with missing IWAD to return to browser without quitting, "+
				"got quitting=%v selected=%v viewingLaunchOpts=%v",
			m.quitting, m.selected, m.viewingLaunchOpts,
		)
	}
	wantStatus := "✗ Missing required files for Doom II (press Ctrl+F to fetch)"
	if m.statusMsg != wantStatus {
		t.Errorf("statusMsg = %q, want %q", m.statusMsg, wantStatus)
	}

	// Provide DOOM2.WAD (while optional idkfa 2024.wad remains absent) and launch from Launch Options
	wadsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.dirIndex = preset.NewDirIndex(wadsDir).WithCatalog(cat)

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if !m.quitting || m.selected == nil {
		t.Fatal("expected Enter in Launch Options with DOOM2.WAD present to select preset and quit")
	}
	if !m.selected.EngineOverridden {
		t.Error("expected selected.EngineOverridden == true from Launch Options")
	}
	if gotView := m.View(); gotView != "Launching Doom II...\n" {
		t.Errorf("View() after Launch Options Enter = %q, want %q", gotView, "Launching Doom II...\n")
	}
}

func TestModel_ConfigManagerView_TabsSaveAndRevert(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	userCatDir := filepath.Join(tmpDir, "catalog.d")
	_ = os.MkdirAll(userCatDir, 0o755)
	paths := &config.Paths{
		ConfigDir:     tmpDir,
		ConfigFile:    filepath.Join(tmpDir, "config.json"),
		ThemesDir:     filepath.Join(tmpDir, "themes"),
		CatalogDir:    userCatDir,
		WadsDir:       filepath.Join(tmpDir, "wads"),
		BinDir:        filepath.Join(tmpDir, "bin"),
		SoundFontsDir: filepath.Join(tmpDir, "soundfonts"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda-doom"),
	}
	cfg := &config.AppConfig{Theme: "classic"}
	cat, err := preset.LoadLayeredCatalog(userCatDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}

	m := initialModel(cat, paths.WadRoots(), ClassicTheme, false)
	m.paths = paths
	m.appConfig = cfg
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	// Open Config Manager via Ctrl+G
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = newM.(model)
	if !m.viewingConfigMgr || m.cfgMgr == nil {
		t.Fatal("expected viewingConfigMgr == true after Ctrl+G")
	}

	// Verify 80x24 height budget on Tab 1
	if lines := strings.Split(m.View(), "\n"); len(lines) > 24 {
		t.Errorf("Config Manager Tab 1 at 80x24 produced %d lines, want <= 24", len(lines))
	}

	// Cycle Theme on Tab 1 (live preview) and toggle Nerd Fonts, then press Esc to verify revert
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(model)
	if m.theme.Name == "classic" {
		t.Errorf("expected live theme preview to change from classic, got %q", m.theme.Name)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newM.(model)
	if !m.nerdFonts {
		t.Error("expected live Nerd Fonts preview to toggle to true")
	}

	// Esc reverts unsaved live previews
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(model)
	if m.viewingConfigMgr {
		t.Fatal("expected viewingConfigMgr == false after Esc")
	}
	if m.theme.Name != "classic" || m.nerdFonts {
		t.Errorf("expected Esc to revert theme=classic and nerdFonts=false, got theme=%q nerdFonts=%v",
			m.theme.Name, m.nerdFonts)
	}

	// Re-open Config Manager, change Theme, and press Ctrl+S to persist
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(model)
	savedTheme := m.theme.Name
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = newM.(model)

	loadedCfg, err := config.LoadConfig(paths)
	if err != nil || loadedCfg.Theme != savedTheme {
		t.Fatalf("expected saved theme %q in config.json, got %+v (err=%v)", savedTheme, loadedCfg, err)
	}

	// Switch to Tab 2 (Engine Overrides) via ']'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = newM.(model)
	if m.cfgMgr.activeTab != configTabEngines {
		t.Fatalf("expected activeTab == configTabEngines, got %d", m.cfgMgr.activeTab)
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) > 24 {
		t.Errorf("Config Manager Tab 2 at 80x24 produced %d lines, want <= 24", len(lines))
	}

	// Switch to Tab 3 (Port Configs) via '3'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = newM.(model)
	if m.cfgMgr.activeTab != configTabPorts {
		t.Fatalf("expected activeTab == configTabPorts, got %d", m.cfgMgr.activeTab)
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) > 24 {
		t.Errorf("Config Manager Tab 3 at 80x24 produced %d lines, want <= 24", len(lines))
	}

	// Trigger < Deploy Curated Configs > on Tab 3 (Right arrow then Enter)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if m.cfgMgr.uzdoomStatus != "[✓ In Sync]" || m.cfgMgr.dsdaStatus != "[✓ In Sync]" {
		t.Errorf("expected both port configs In Sync after deploy, got uzdoom=%q dsda=%q",
			m.cfgMgr.uzdoomStatus, m.cfgMgr.dsdaStatus)
	}
}

func TestModel_SearchTokens_CompatibilityAndReadiness(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.wad"), []byte("pwad"), 0o644)

	cat := &preset.Catalog{
		Presets: []preset.Preset{
			{
				ID:            "alien-vendetta",
				Name:          "Alien Vendetta",
				Engine:        "dsda-doom",
				IWAD:          "DOOM2.WAD",
				Compatibility: "Vanilla",
				Files:         []preset.FileSpec{{Name: "av.wad"}},
			},
			{
				ID:            "eviternity-ii",
				Name:          "Eviternity II",
				Engine:        "uzdoom",
				IWAD:          "DOOM2.WAD",
				Compatibility: "MBF21",
				Files:         []preset.FileSpec{{Name: "Eviternity II.wad"}},
			},
		},
	}

	m := initialModel(cat, []string{wadsDir}, ClassicTheme, false)

	// Filter by compatibility "MBF21"
	m.updateFiltered("MBF21")
	if len(m.filtered) != 1 || m.filtered[0].ID != "eviternity-ii" {
		t.Errorf("expected filter 'MBF21' to match eviternity-ii, got %+v", m.filtered)
	}

	// Filter by readiness "ready"
	m.updateFiltered("ready")
	if len(m.filtered) != 1 || m.filtered[0].ID != "alien-vendetta" {
		t.Errorf("expected filter 'ready' to match installed alien-vendetta, got %+v", m.filtered)
	}

	// Filter by readiness "missing"
	m.updateFiltered("missing")
	if len(m.filtered) != 1 || m.filtered[0].ID != "eviternity-ii" {
		t.Errorf("expected filter 'missing' to match uninstalled eviternity-ii, got %+v", m.filtered)
	}

	// Filter by PWAD filename "av.wad"
	m.updateFiltered("av.wad")
	if len(m.filtered) != 1 || m.filtered[0].ID != "alien-vendetta" {
		t.Errorf("expected filter 'av.wad' to match alien-vendetta, got %+v", m.filtered)
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

func TestLaunchOptions_PreExistingOverride_IdempotentSaveAndReset(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	wadsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)

	// Write an override with set.args and exclude_files before starting initialModel
	ovJSON := `{"kind":"override","target":{"kind":"preset","id":"doom-ii"},` +
		`"set":{"args":["-skill","3"]},"exclude_files":["idkfa 2024.wad"]}`
	if err := os.WriteFile(
		filepath.Join(userDir, "override.preset.doom-ii.json"), []byte(ovJSON), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	cat, err := preset.LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}

	m := initialModel(cat, []string{wadsDir}, ClassicTheme, false, "Doom II")
	m.customCatalogDir = userDir
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	// Open Launch Options via Ctrl+O and verify set.args="-skill 3" loaded into skillIdx
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = newM.(model)
	if m.launchOpts.skillIdx != 3 {
		t.Errorf("expected skillIdx=3 from set.args, got %d", m.launchOpts.skillIdx)
	}

	// Open Contextual Help from Launch Options via F1 and verify Launch Options is first
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = newM.(model)
	if !strings.Contains(m.helpViewport.View(), "Launch Options & Preset Config (Ctrl+O / F4) — Active View") {
		t.Errorf("expected Launch Options active view heading in F1 help:\n%s", m.helpViewport.View())
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(model)

	// Save via Ctrl+S and verify args are not duplicated in m.catalog
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = newM.(model)
	pSaved, err := m.catalog.FindExact("doom-ii")
	if err != nil {
		t.Fatalf("FindExact doom-ii after Ctrl+S: %v", err)
	}
	if len(pSaved.Args) != 2 || pSaved.Args[0] != "-skill" || pSaved.Args[1] != "3" {
		t.Errorf("expected non-duplicated Args=[-skill 3] after Ctrl+S, got %v", pSaved.Args)
	}

	// Reset via Ctrl+R and verify curated defaults are restored in m.catalog
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = newM.(model)
	pReset, err := m.catalog.FindExact("doom-ii")
	if err != nil {
		t.Fatalf("FindExact doom-ii after Ctrl+R: %v", err)
	}
	if len(pReset.Args) != 0 || len(pReset.Files) == 0 || pReset.Overridden {
		t.Errorf("expected curated doom-ii restored after Ctrl+R, got Args=%v Files=%v Overridden=%v",
			pReset.Args, pReset.Files, pReset.Overridden)
	}
}

func TestConfigManager_DigitInputAndFlagPreservationAndEngineAppendArgs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	userCatDir := filepath.Join(tmpDir, "catalog.d")
	if err := os.MkdirAll(userCatDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-populate engine override with display_name and append_args
	engOv := `{"kind":"override","target":{"kind":"engine","id":"dsda-doom"},` +
		`"set":{"display_name":"Custom DSDA"},"append_args":["-complevel","21"]}`
	if err := os.WriteFile(
		filepath.Join(userCatDir, "override.engine.dsda-doom.json"), []byte(engOv), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	paths := &config.Paths{
		ConfigDir:     tmpDir,
		ConfigFile:    filepath.Join(tmpDir, "config.json"),
		ThemesDir:     filepath.Join(tmpDir, "themes"),
		CatalogDir:    userCatDir,
		WadsDir:       "/saved/in/config",
		BinDir:        filepath.Join(tmpDir, "bin"),
		SoundFontsDir: filepath.Join(tmpDir, "soundfonts"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda-doom"),
	}
	cfg := &config.AppConfig{
		Theme:   "classic",
		WadsDir: "/saved/in/config",
	}
	cat, err := preset.LoadLayeredCatalog(userCatDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}

	m := initialModel(cat, paths.WadRoots(), ClassicTheme, false)
	m.paths = paths
	m.appConfig = cfg
	m.wadsDirSource = "config"
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(model)

	// Verify preview pane renders overridden engine DisplayName ("Custom DSDA")
	if view := m.View(); !strings.Contains(view, "Custom DSDA") {
		t.Errorf("expected launcher preview pane to display overridden engine name 'Custom DSDA', got:\n%s", view)
	}

	// Open Config Manager via Ctrl+G
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = newM.(model)

	// Save Tab 1 without modifying WADs Dir; config.json's WadsDir must remain '/saved/in/config'
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = newM.(model)
	if m.appConfig.WadsDir != "/saved/in/config" {
		t.Errorf("expected unmodified WadsDir in config.json to remain '/saved/in/config', got %q",
			m.appConfig.WadsDir)
	}

	// Focus Extra WADs Dirs (field 3) and type '1', '2', '3' as the first characters
	for m.cfgMgr.focusIdx < cliFieldExtraWadsDirs {
		newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newM.(model)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1', '2', '3'}})
	m = newM.(model)
	if m.cfgMgr.activeTab != configTabCLI {
		t.Fatalf("typing digits into focused text input switched tabs to %d", m.cfgMgr.activeTab)
	}
	if m.cfgMgr.extraDirsInput.Value() != "123" {
		t.Errorf("expected extraDirsInput='123', got %q", m.cfgMgr.extraDirsInput.Value())
	}

	// Save Tab 1, close Config Manager via Esc, and reopen via Ctrl+G; updated provenance must persist on model
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = newM.(model)
	if m.cfgMgr.extraDirsSource != "config" {
		t.Errorf("expected extraDirsSource='config' after saving and reopening Config Manager, got %q",
			m.cfgMgr.extraDirsSource)
	}

	// Switch to Tab 2 via ']' and press Ctrl+S; DefaultArgs must not duplicate append_args
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = newM.(model)
	eng, ok := m.catalog.FindEngine("dsda-doom")
	if !ok {
		t.Fatal("dsda-doom not found")
	}
	if len(eng.DefaultArgs) != 2 || eng.DefaultArgs[0] != "-complevel" || eng.DefaultArgs[1] != "21" {
		t.Errorf("expected non-duplicated DefaultArgs=[-complevel 21], got %v", eng.DefaultArgs)
	}

	// Reset engine override via Ctrl+R and verify DefaultArgs is empty
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = newM.(model)
	engReset, _ := m.catalog.FindEngine("dsda-doom")
	if len(engReset.DefaultArgs) != 0 || engReset.Overridden {
		t.Errorf("expected curated dsda-doom after Ctrl+R, got DefaultArgs=%v Overridden=%v",
			engReset.DefaultArgs, engReset.Overridden)
	}
}

func TestHandleFetchAssets_MissingRequiredAndEmptyRoots(t *testing.T) {
	t.Parallel()
	cat := mockCatalog()

	// 1. Empty wadRoots and nil paths
	mEmpty := initialModel(cat, nil, ClassicTheme, false)
	newM, _ := mEmpty.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	mEmpty = newM.(model)
	if !strings.HasPrefix(mEmpty.statusMsg, "✗") {
		t.Errorf("expected error statusMsg when no WAD dir configured, got %q", mEmpty.statusMsg)
	}

	// 2. Missing required files with isolated empty steamSearchRoots
	emptyWads := t.TempDir()
	emptySteam := t.TempDir()
	mMiss := initialModel(cat, []string{emptyWads}, ClassicTheme, false)
	mMiss.steamSearchRoots = []string{emptySteam}
	newM, _ = mMiss.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	mMiss = newM.(model)
	if !strings.HasPrefix(mMiss.statusMsg, "✗ Missing required files") {
		t.Errorf("expected '✗ Missing required files' statusMsg, got %q", mMiss.statusMsg)
	}
}

func TestInitialModeConfigAndEngines(t *testing.T) {
	t.Parallel()
	cat := mockCatalog()

	// 1. InitialModeConfig starts in Config Manager Tab 1 (CLI Settings) and exits silently on Esc
	mCfg := initialModel(cat, nil, ClassicTheme, false)
	mCfg.applyOptions(LauncherOptions{InitialMode: InitialModeConfig})
	if !mCfg.viewingConfigMgr || mCfg.cfgMgr == nil || mCfg.cfgMgr.activeTab != configTabCLI {
		t.Fatalf("expected InitialModeConfig to open Config Manager on Tab 1, got viewing=%v cfgMgr=%v",
			mCfg.viewingConfigMgr, mCfg.cfgMgr)
	}
	newM, cmd := mCfg.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mCfg = newM.(model)
	if !mCfg.quitting || cmd == nil {
		t.Errorf("expected Esc in InitialModeConfig to quit, got quitting=%v cmd=%v", mCfg.quitting, cmd)
	}
	if got := mCfg.View(); got != "" {
		t.Errorf("expected empty View() when quitting from InitialModeConfig, got %q", got)
	}

	// 2. InitialModeEngines starts in Config Manager Tab 2 (Engine Overrides) and exits silently on Esc
	mEng := initialModel(cat, nil, ClassicTheme, false)
	mEng.applyOptions(LauncherOptions{InitialMode: InitialModeEngines})
	if !mEng.viewingConfigMgr || mEng.cfgMgr == nil || mEng.cfgMgr.activeTab != configTabEngines {
		t.Fatalf("expected InitialModeEngines to open Config Manager on Tab 2, got viewing=%v cfgMgr=%v",
			mEng.viewingConfigMgr, mEng.cfgMgr)
	}
	newM, cmd = mEng.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mEng = newM.(model)
	if !mEng.quitting || cmd == nil {
		t.Errorf("expected Esc in InitialModeEngines to quit, got quitting=%v cmd=%v", mEng.quitting, cmd)
	}
	if got := mEng.View(); got != "" {
		t.Errorf("expected empty View() when quitting from InitialModeEngines, got %q", got)
	}
}

func TestConfigManager_CtrlJKAndDiffHelpAndWidth90Footer(t *testing.T) {
	t.Parallel()
	cat := mockCatalog()
	m := initialModel(cat, nil, ClassicTheme, false)
	m.width = 90
	m.height = 24

	// At width 90 (== sideBySideMinWidth), renderFooter should use the full side-by-side footer strip
	footer90 := m.renderFooter(true)
	if !strings.Contains(footer90, "Ctrl+O") || !strings.Contains(footer90, "F1") {
		t.Errorf("expected width=90 footer to include Ctrl+O and F1, got %q", footer90)
	}

	m.openConfigManager()
	if m.cfgMgr.focusIdx != 0 {
		t.Fatalf("expected initial focusIdx=0, got %d", m.cfgMgr.focusIdx)
	}
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = newM.(model)
	if m.cfgMgr.focusIdx != 1 {
		t.Errorf("expected ctrl+j to advance focusIdx to 1, got %d", m.cfgMgr.focusIdx)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = newM.(model)
	if m.cfgMgr.focusIdx != 0 {
		t.Errorf("expected ctrl+k to move focusIdx back to 0, got %d", m.cfgMgr.focusIdx)
	}

	// Open diff view on Tab 3 and verify F1 opens contextual help
	m.cfgMgr.activeTab = configTabPorts
	m.cfgMgr.focusIdx = 0
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(model)
	if !m.cfgMgr.viewingDiff {
		t.Fatalf("expected viewingDiff=true")
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = newM.(model)
	if !m.viewingHelp || m.helpMode != helpModeConfigManager {
		t.Errorf("expected F1 in diff view to open helpModeConfigManager, got viewingHelp=%v helpMode=%d",
			m.viewingHelp, m.helpMode)
	}
}
