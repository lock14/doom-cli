package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/config"
)

func mockCatalog() *catalog.Catalog {
	return &catalog.Catalog{
		Presets: []catalog.Preset{
			{
				ID:          "alien-vendetta",
				Name:        "Alien Vendetta",
				Engine:      "dsda-doom",
				IWAD:        "doom2.wad",
				Files:       []catalog.FileSpec{{Name: "av.wad"}},
				Author:      "Anders Johnsen, Brad Spencer, et al.",
				ReleaseDate: "2002",
			},
			{
				ID:          "eviternity-ii",
				Name:        "Eviternity II",
				Engine:      "uzdoom",
				IWAD:        "doom2.wad",
				Files:       []catalog.FileSpec{{Name: "eviternityii.wad"}},
				Author:      "Joshua \"Dragonfly\" O'Sullivan et al.",
				ReleaseDate: "2023",
			},
			{
				ID:          "sunder",
				Name:        "Sunder",
				Engine:      "dsda-doom",
				IWAD:        "doom2.wad",
				Files:       []catalog.FileSpec{{Name: "sunder.wad"}},
				Author:      "Insane_Gazebo",
				ReleaseDate: "2009",
			},
		},
	}
}

func isolatedConfigPaths(t *testing.T) *config.Paths {
	t.Helper()
	tmpDir := t.TempDir()
	catDir := filepath.Join(tmpDir, "catalog.d")
	_ = os.MkdirAll(catDir, 0o755)
	return &config.Paths{
		ConfigDir:     tmpDir,
		ConfigFile:    filepath.Join(tmpDir, "config.json"),
		ThemesDir:     filepath.Join(tmpDir, "themes"),
		CatalogDir:    catDir,
		WadsDir:       filepath.Join(tmpDir, "wads"),
		BinDir:        filepath.Join(tmpDir, "bin"),
		SoundFontsDir: filepath.Join(tmpDir, "soundfonts"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda-doom"),
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
	m.dirIndex = catalog.NewDirIndex(wadsDir).WithCatalog(cat)

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

func TestModel_CustomPresetAndEngine(t *testing.T) {
	cat := &catalog.Catalog{
		Engines: map[string]catalog.EngineConfig{
			"woof": {
				ID:          "woof",
				DisplayName: "Woof!",
				Badge:       "WOOF",
				Description: "Woof! MBF21 Engine",
			},
		},
		Presets: []catalog.Preset{
			{
				ID:     "custom-map",
				Name:   "CustomMap",
				Engine: "woof",
				IWAD:   "DOOM2.WAD",
				Files:  []catalog.FileSpec{{Name: "custom.wad"}},
				Args:   []string{"-skill", "4"},
				Layer:  catalog.LayerUser,
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

func TestModel_PagingAndControlNavigation(t *testing.T) {
	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
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

func TestModel_SearchTokens_CompatibilityAndReadiness(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.wad"), []byte("pwad"), 0o644)

	cat := &catalog.Catalog{
		Presets: []catalog.Preset{
			{
				ID:            "alien-vendetta",
				Name:          "Alien Vendetta",
				Engine:        "dsda-doom",
				IWAD:          "DOOM2.WAD",
				Compatibility: "Vanilla",
				Files:         []catalog.FileSpec{{Name: "av.wad"}},
			},
			{
				ID:            "eviternity-ii",
				Name:          "Eviternity II",
				Engine:        "uzdoom",
				IWAD:          "DOOM2.WAD",
				Compatibility: "MBF21",
				Files:         []catalog.FileSpec{{Name: "Eviternity II.wad"}},
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

	// 2. Missing required files with isolated empty steamSearchRoots dispatches async tea.Cmd
	emptyWads := t.TempDir()
	emptySteam := t.TempDir()
	mMiss := initialModel(cat, []string{emptyWads}, ClassicTheme, false)
	mMiss.steamSearchRoots = []string{emptySteam}
	newM, cmd := mMiss.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	mMiss = newM.(model)
	if !mMiss.fetching || cmd == nil {
		t.Fatalf("expected async fetch in progress (fetching=%v, cmd=%v)", mMiss.fetching, cmd)
	}
	if !strings.Contains(mMiss.statusMsg, "Fetching") {
		t.Errorf("expected Fetching statusMsg during async command, got %q", mMiss.statusMsg)
	}

	// Complete async command and feed message back into Update
	newM, _ = mMiss.Update(cmd())
	mMiss = newM.(model)
	if mMiss.fetching {
		t.Error("expected fetching == false after fetchCompleteMsg")
	}
	if !strings.HasPrefix(mMiss.statusMsg, "✗ Missing required files") {
		t.Errorf("expected '✗ Missing required files' statusMsg, got %q", mMiss.statusMsg)
	}

	// 3. Pressing Esc while fetching cancels fetch without quitting launcher
	newM, _ = mMiss.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	mMiss = newM.(model)
	newM, quitCmd := mMiss.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mMiss = newM.(model)
	if mMiss.quitting || quitCmd != nil || mMiss.fetching {
		t.Errorf("expected Esc during fetch to cancel without quitting, got quitting=%v fetching=%v",
			mMiss.quitting, mMiss.fetching)
	}
	if !strings.Contains(mMiss.statusMsg, "cancelled") {
		t.Errorf("expected cancelled statusMsg, got %q", mMiss.statusMsg)
	}
}

func TestInitialModeConfigAndEngines(t *testing.T) {
	t.Parallel()
	cat := mockCatalog()

	// 1. InitialModeConfig starts in Config Manager Tab 1 (CLI Settings) and exits silently on Esc
	mCfg := initialModel(cat, nil, ClassicTheme, false)
	mCfg.detectedRes = "1920x1080"
	mCfg.detectedRate = 60
	mCfg.applyOptions(LauncherOptions{
		Paths:       isolatedConfigPaths(t),
		AppConfig:   &config.AppConfig{},
		InitialMode: InitialModeConfig,
	})
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
	mEng.detectedRes = "1920x1080"
	mEng.detectedRate = 60
	mEng.applyOptions(LauncherOptions{
		Paths:       isolatedConfigPaths(t),
		AppConfig:   &config.AppConfig{},
		InitialMode: InitialModeEngines,
	})
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

func TestModel_FilteringAndPreviewTaxonomyMetadata(t *testing.T) {
	t.Parallel()
	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	m := initialModel(cat, nil, ClassicTheme, false)
	m.width = 160
	m.height = 30

	// Filter by genre
	m.updateFiltered("cyberpunk")
	if len(m.filtered) != 1 || m.filtered[0].ID != "auger-zenith" {
		t.Fatalf("expected 1 match 'auger-zenith' for 'cyberpunk', got %d", len(m.filtered))
	}

	// Filter by difficulty
	m.updateFiltered("extreme")
	if len(m.filtered) < 2 {
		t.Fatalf("expected at least 2 matches for 'extreme' difficulty, got %d", len(m.filtered))
	}

	// Filter by quality
	m.updateFiltered("10/10")
	if len(m.filtered) < 4 {
		t.Fatalf("expected at least 4 matches for '10/10' quality, got %d", len(m.filtered))
	}

	// Render preview for AUGER;ZENITH and verify Genre, Difficulty, Quality, and Link appear
	m.updateFiltered("AUGER;ZENITH")
	m.cursor = 0
	view := m.View()
	for _, want := range []string{
		"Genre:", "Cyberpunk Adventure",
		"Difficulty:", "Moderate",
		"Quality:", "S-Tier (9.5/10)",
		"Link:", "https://www.doomworld.com/idgames/levels/doom2/Ports/megawads/dbp37_augzen",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("expected TUI view to contain %q, got:\n%s", want, view)
		}
	}
}
