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

func TestModel_LaunchOptionsView_EphemeralAndPersistent(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	cat, err := catalog.LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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
	_, ov, err := catalog.LoadPresetBaseAndOverride(userDir, "doom-ii")
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
	if _, ovAfter, _ := catalog.LoadPresetBaseAndOverride(userDir, "doom-ii"); ovAfter != nil {
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
	m.dirIndex = catalog.NewDirIndex(wadsDir).WithCatalog(cat)

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

func TestLaunchOptions_PreExistingOverride_IdempotentSaveAndReset(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	wadsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)

	// Write an override with set.args and exclude-files before starting initialModel
	ovJSON := `{"kind":"override","target":{"kind":"preset","id":"doom-ii"},` +
		`"set":{"args":["-skill","3"]},"exclude-files":["idkfa 2024.wad"]}`
	if err := os.WriteFile(
		filepath.Join(userDir, "override.preset.doom-ii.json"), []byte(ovJSON), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	cat, err := catalog.LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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

func TestLaunchOptions_BoomFieldSkipping_SetArgs_And_SurgicalConfigSave(t *testing.T) {
	t.Parallel()
	paths := isolatedConfigPaths(t)
	cat, err := catalog.LoadLayered(paths.CatalogDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
	}

	// 1. On UZDoom preset ("eviternity"), Solo-Net and Complevel are skipped by focus and omitted from args
	m := initialModel(cat, []string{paths.WadsDir}, ClassicTheme, false, "eviternity")
	m.paths = paths
	m.appConfig = &config.AppConfig{}
	m.customCatalogDir = paths.CatalogDir
	m.detectedRes = "1920x1080"
	m.detectedRate = 60

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = newM.(model)
	// Pre-populate soloNet/complevelIdx to verify synthesizeAppendArgs suppresses Boom flags on UZDoom
	m.launchOpts.soloNet = true
	m.launchOpts.complevelIdx = 6
	if args := m.launchOpts.synthesizeAppendArgs(); len(args) != 0 {
		t.Errorf("expected empty Boom flags on UZDoom, got %v", args)
	}
	// Step down from Respawn (index 6) and verify Solo-Net (7) and Complevel (8) are skipped
	m.launchOpts.focusIdx = launchFieldRespawn
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(model)
	if m.launchOpts.focusIdx == launchFieldSoloNet || m.launchOpts.focusIdx == launchFieldComplevel {
		t.Errorf("expected Down on UZDoom to skip Boom-only fields, landed on %d", m.launchOpts.focusIdx)
	}
	// Verify '?' on non-text field opens Contextual Help
	m.launchOpts.focusIdx = launchFieldEngine
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newM.(model)
	if !m.viewingHelp || m.helpMode != helpModeLaunchOptions {
		t.Errorf("expected '?' on non-text field to open helpModeLaunchOptions, got %v/%d",
			m.viewingHelp, m.helpMode)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newM.(model)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(model)

	// 2. Preset with curated complevel ("sigil" has complevel 3) displays Default (3: Ult. Doom)
	// and pre-existing set.args preserves replacement set.args on Ctrl+S and omits redundant set.engine
	ovSigil := `{"kind":"override","target":{"kind":"preset","id":"sigil"},"set":{"args":["-skill","4"]}}`
	if err := os.WriteFile(
		filepath.Join(paths.CatalogDir, "override.preset.sigil.json"), []byte(ovSigil), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	mSigil := initialModel(cat, []string{paths.WadsDir}, ClassicTheme, false, "sigil")
	mSigil.customCatalogDir = paths.CatalogDir
	newM, _ = mSigil.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	mSigil = newM.(model)
	if !strings.Contains(mSigil.View(), "Default (3: Ult. Doom)") {
		t.Errorf("expected sigil Launch Options to display 'Default (3: Ult. Doom)', got:\n%s", mSigil.View())
	}
	newM, _ = mSigil.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	mSigil = newM.(model)

	_, savedOv, err := catalog.LoadPresetBaseAndOverride(paths.CatalogDir, "sigil")
	if err != nil || savedOv == nil || savedOv.Set == nil || savedOv.Set.Args == nil {
		t.Fatalf("expected set.args preserved on sigil after Ctrl+S, got ov=%+v err=%v", savedOv, err)
	}
	if savedOv.Set.Engine != nil {
		t.Errorf("expected unchanged engine to remain nil in set.engine, got %q", *savedOv.Set.Engine)
	}
	pSigil, err := mSigil.catalog.FindExact("sigil")
	if err != nil || strings.Join(pSigil.Args, " ") != "-skill 4" {
		t.Errorf("expected replacement Args=[-skill 4] on sigil without base -warp 5 1, got %v (err=%v)",
			pSigil.Args, err)
	}

	// 3. Config Manager Tab 1 surgical save keeps default theme/nerd_fonts sources untouched,
	// and Tab 2 warns before discarding unsaved engine edits
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = newM.(model)
	m.cfgMgr.focusIdx = cliFieldBinDir
	customBin := filepath.Join(paths.ConfigDir, "custom-bin")
	m.cfgMgr.binDirInput.SetValue(customBin)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = newM.(model)
	if m.themeSource != "default" || m.nerdFontsSource != "default" {
		t.Errorf("expected untouched theme/nerdFonts sources to stay 'default', got %q / %q",
			m.themeSource, m.nerdFontsSource)
	}
	loadedCfg, err := config.LoadConfig(paths)
	if err != nil || loadedCfg.Theme != "" || loadedCfg.HasNerdFonts() {
		t.Errorf("expected untouched theme/nerd_fonts omitted from config.json, got %+v (err=%v)",
			loadedCfg, err)
	}

	// Tab 2 unsaved switch guard
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = newM.(model)
	m.cfgMgr.engBadgeInput.SetValue("CUST")
	m.cfgMgr.focusIdx = engFieldSelect
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(model)
	if m.cfgMgr.engineIdx != 0 || !strings.Contains(m.cfgMgr.statusMsg, "Unsaved") {
		t.Errorf("expected first Right press with dirty Tab 2 to warn without switching, got idx=%d msg=%q",
			m.cfgMgr.engineIdx, m.cfgMgr.statusMsg)
	}
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(model)
	if m.cfgMgr.engineIdx == 0 {
		t.Errorf("expected second Right press to confirm discard and switch engine")
	}
}
