package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/config"
)

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
	cat, err := catalog.LoadLayered(userCatDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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
	cat, err := catalog.LoadLayered(userCatDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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

func TestConfigManager_CtrlJKAndDiffHelpAndWidth90Footer(t *testing.T) {
	t.Parallel()
	cat := mockCatalog()
	m := initialModel(cat, nil, ClassicTheme, false)
	m.paths = isolatedConfigPaths(t)
	m.appConfig = &config.AppConfig{}
	m.detectedRes = "1920x1080"
	m.detectedRate = 60
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
