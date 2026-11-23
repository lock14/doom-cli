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
		{
			name:     "osc hyperlink sequence skipped cleanly",
			input:    "\x1b]8;;https://example.com\x07Alien Vendetta\x1b]8;;\x07",
			maxWidth: 6,
			want:     "\x1b]8;;https://example.com\x07Alien\x1b[0m…",
			wantCols: 6,
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

func TestModel_80x24_AllCuratedPresetsShowIWADAndFiles(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(3)
	defer lipgloss.SetColorProfile(origProfile)

	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}

	tmpDir := t.TempDir()
	// Create a readme for Legacy of Rust and a synthetic 6-file preset to test worst-case 80x24 file density
	if err := os.WriteFile(filepath.Join(tmpDir, "id1.txt"), []byte("Legacy of Rust readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "dense1.txt"), []byte("Dense readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	cl21 := 21
	cat.Presets = append(cat.Presets, catalog.Preset{
		ID:            "dense-six-pack",
		Name:          "Dense Six Pack",
		Engine:        "dsda-doom",
		IWAD:          "DOOM2.WAD",
		Complevel:     &cl21,
		Category:      "Community Megawad",
		Genre:         "Run-and-Gun",
		Difficulty:    "Hard",
		Quality:       "A-Tier",
		Compatibility: "MBF21",
		Description:   "Six-file layout stress test",
		Files: []catalog.FileSpec{
			{Name: "dense1.wad"},
			{Name: "dense2.wad"},
			{Name: "dense3.wad"},
			{Name: "dense4.wad"},
			{Name: "dense5.deh"},
			{Name: "idkfa 2024.wad", Optional: true},
		},
	})

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
		if (p.ID == "legacy-of-rust" || p.ID == "dense-six-pack") && !strings.Contains(view, "[✓ Readme]") {
			t.Errorf("expected %s at 80x24 to show [✓ Readme], got:\n%s", p.ID, view)
		}
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

	cat := &catalog.Catalog{
		Engines: map[string]catalog.EngineConfig{
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

	cat := &catalog.Catalog{
		Presets: []catalog.Preset{
			{
				ID:     "custom-long-6",
				Name:   "Custom Long Filenames",
				Engine: "dsda-doom",
				IWAD:   "DOOM2.WAD",
				Files: []catalog.FileSpec{
					{Name: longFirst},
					{Name: "second_col_patch.deh"},
					{Name: "third_long_custom_resource_pack_v1.wad"},
					{Name: "fourth_col_music.wad", Optional: true},
					{Name: "fifth_disabled_ost.wad", Optional: true, Disabled: true},
					{Name: "sixth_col_fix.bex"},
				},
				Layer: catalog.LayerUser,
			},
			{
				ID:     "custom-many-12",
				Name:   "Custom Many Files",
				Engine: "uzdoom",
				IWAD:   "DOOM2.WAD",
				Files: []catalog.FileSpec{
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
				Layer: catalog.LayerUser,
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
		"[Disabled]",
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
		{name: "narrow 60 columns", width: 60, wantWithReadme: true, wantNoReadme: true, wantReadmeFooter: true},
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
