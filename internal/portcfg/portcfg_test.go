package portcfg

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/config"
)

func TestDeployConfigs_And_Diff(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	paths := &config.Paths{
		BinDir:        filepath.Join(tmpDir, "bin"),
		WadsDir:       filepath.Join(tmpDir, "wads"),
		SoundFontsDir: filepath.Join(tmpDir, "soundfonts"),
		SoundFontFile: filepath.Join(tmpDir, "soundfonts", "GeneralUser-GS.sf2"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda-doom"),
	}

	if err := DeployConfigsTo(paths, io.Discard); err != nil {
		t.Fatalf("DeployConfigsTo failed: %v", err)
	}

	autoexecPath := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	dataAuto, err := os.ReadFile(autoexecPath)
	if err != nil {
		t.Fatalf("autoexec.cfg missing: %v", err)
	}
	if strings.Contains(string(dataAuto), "__REFRESH_RATE__") {
		t.Errorf("autoexec.cfg still contains __REFRESH_RATE__ placeholder")
	}
	if strings.Contains(string(dataAuto), "__SOUNDFONT__") {
		t.Errorf("autoexec.cfg still contains __SOUNDFONT__ placeholder")
	}

	dsdaPath := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	dataDSDA, err := os.ReadFile(dsdaPath)
	if err != nil {
		t.Fatalf("dsda-doom.cfg missing: %v", err)
	}
	if strings.Contains(string(dataDSDA), "__RESOLUTION__") {
		t.Errorf("dsda-doom.cfg still contains __RESOLUTION__ placeholder")
	}
	if strings.Contains(string(dataDSDA), "__SOUNDFONT__") {
		t.Errorf("dsda-doom.cfg still contains __SOUNDFONT__ placeholder")
	}

	var diffOut bytes.Buffer
	if err := DiffConfigs(paths, &diffOut); err != nil {
		t.Fatalf("DiffConfigs failed: %v", err)
	}
	if !strings.Contains(diffOut.String(), "is in sync with system") {
		t.Errorf("expected 'is in sync', got:\n%s", diffOut.String())
	}
}

func TestSyncConfigs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	paths := &config.Paths{
		UZDoomDir: filepath.Join(tmpDir, "sys_uzdoom"),
		DSDADir:   filepath.Join(tmpDir, "sys_dsda"),
	}
	_ = os.MkdirAll(paths.UZDoomDir, 0o755)
	_ = os.MkdirAll(paths.DSDADir, 0o755)

	liveAuto := "fluid_patchset \"/custom/soundfont.sf2\"\nvid_maxfps 144\n"
	_ = os.WriteFile(filepath.Join(paths.UZDoomDir, "autoexec.cfg"), []byte(liveAuto), 0o644)

	liveDSDA := "screen_resolution \"2560x1440\"\nsnd_soundfont \"/custom/soundfont.sf2\"\n"
	_ = os.WriteFile(filepath.Join(paths.DSDADir, "dsda-doom.cfg"), []byte(liveDSDA), 0o644)

	repoDir := filepath.Join(tmpDir, "repo")
	_ = os.MkdirAll(filepath.Join(repoDir, "configs", "uzdoom"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoDir, "configs", "dsda-doom"), 0o755)

	var out bytes.Buffer
	if err := SyncConfigs(paths, repoDir, &out); err != nil {
		t.Fatalf("SyncConfigs failed: %v", err)
	}

	syncedAuto, err := os.ReadFile(filepath.Join(repoDir, "configs", "uzdoom", "autoexec.cfg"))
	if err != nil {
		t.Fatalf("failed reading synced uzdoom autoexec: %v", err)
	}
	if !strings.Contains(string(syncedAuto), `fluid_patchset "__SOUNDFONT__"`) {
		t.Errorf("expected __SOUNDFONT__ token in synced autoexec, got: %s", string(syncedAuto))
	}
	if !strings.Contains(string(syncedAuto), `vid_maxfps __REFRESH_RATE__`) {
		t.Errorf("expected __REFRESH_RATE__ token in synced autoexec, got: %s", string(syncedAuto))
	}

	syncedDSDA, err := os.ReadFile(filepath.Join(repoDir, "configs", "dsda-doom", "dsda-doom.cfg"))
	if err != nil {
		t.Fatalf("failed reading synced dsda-doom cfg: %v", err)
	}
	if !strings.Contains(string(syncedDSDA), `screen_resolution               "__RESOLUTION__"`) {
		t.Errorf("expected __RESOLUTION__ token in synced dsda-doom, got: %s", string(syncedDSDA))
	}
	if !strings.Contains(string(syncedDSDA), `snd_soundfont                   "__SOUNDFONT__"`) {
		t.Errorf("expected __SOUNDFONT__ token in synced dsda-doom, got: %s", string(syncedDSDA))
	}
}

func TestTemplateInvariants(t *testing.T) {
	t.Parallel()
	rootDir := filepath.Join("..", "..")

	pairs := []struct {
		name           string
		rootFile       string
		embeddedData   []byte
		requiredTokens []string
	}{
		{
			name:         "dsda-doom.cfg",
			rootFile:     filepath.Join(rootDir, "configs", "dsda-doom", "dsda-doom.cfg"),
			embeddedData: embeddedDSDADoom,
			requiredTokens: []string{
				"__RESOLUTION__",
				"__SOUNDFONT__",
				"sts_solid_bg_color              0",
				"render_doom_lightmaps           1",
				"gl_fade_mode                    1",
			},
		},
		{
			name:           "autoexec.cfg",
			rootFile:       filepath.Join(rootDir, "configs", "uzdoom", "autoexec.cfg"),
			embeddedData:   embeddedAutoexec,
			requiredTokens: []string{"__REFRESH_RATE__", "__SOUNDFONT__"},
		},
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			rootData, err := os.ReadFile(p.rootFile)
			if err != nil {
				t.Fatalf("reading %s: %v", p.rootFile, err)
			}

			if !bytes.Equal(rootData, p.embeddedData) {
				t.Errorf("%s and embedded %s do not match", p.rootFile, p.name)
			}
			for _, tok := range p.requiredTokens {
				if !strings.Contains(string(p.embeddedData), tok) {
					t.Errorf("%s missing required token %s", p.name, tok)
				}
			}
		})
	}
}

func TestDeployConfigsTo(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	paths := &config.Paths{
		BinDir:        filepath.Join(tmpDir, "bin"),
		WadsDir:       filepath.Join(tmpDir, "wads"),
		SoundFontsDir: filepath.Join(tmpDir, "soundfonts"),
		SoundFontFile: filepath.Join(tmpDir, "soundfonts", "GeneralUser-GS.sf2"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda-doom"),
	}

	var buf bytes.Buffer
	if err := DeployConfigsTo(paths, &buf); err != nil {
		t.Fatalf("DeployConfigsTo failed: %v", err)
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "Installed") {
		t.Errorf("expected output to contain 'Installed', got: %s", outStr)
	}

	autoexecPath := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	if _, err := os.Stat(autoexecPath); err != nil {
		t.Errorf("expected autoexec.cfg to exist: %v", err)
	}
}

func TestDeployConfigsWithOptions_PreservesExistingWhenNotForced(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	paths := &config.Paths{
		BinDir:        filepath.Join(tmpDir, "bin"),
		WadsDir:       filepath.Join(tmpDir, "wads"),
		SoundFontsDir: filepath.Join(tmpDir, "soundfonts"),
		SoundFontFile: filepath.Join(tmpDir, "soundfonts", "GeneralUser-GS.sf2"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda-doom"),
	}

	if err := os.MkdirAll(paths.UZDoomDir, 0o755); err != nil {
		t.Fatal(err)
	}
	autoexecPath := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	if err := os.WriteFile(autoexecPath, []byte("custom-user-autoexec"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := DeployConfigsWithOptions(paths, false, &buf); err != nil {
		t.Fatalf("DeployConfigsWithOptions(force=false) failed: %v", err)
	}
	if !strings.Contains(buf.String(), "UZDoom config already installed") {
		t.Errorf("expected UZDoom config already installed message, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "Installed DSDA-Doom config") {
		t.Errorf("expected DSDA-Doom config to be installed when missing, got:\n%s", buf.String())
	}
	gotAuto, err := os.ReadFile(autoexecPath)
	if err != nil || string(gotAuto) != "custom-user-autoexec" {
		t.Errorf("expected existing autoexec.cfg preserved when force=false, got %q (err=%v)", string(gotAuto), err)
	}

	buf.Reset()
	if err := DeployConfigsWithOptions(paths, true, &buf); err != nil {
		t.Fatalf("DeployConfigsWithOptions(force=true) failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Backing up existing autoexec.cfg") {
		t.Errorf("expected backup message when force=true, got:\n%s", buf.String())
	}
	gotAutoForced, err := os.ReadFile(autoexecPath)
	if err != nil || string(gotAutoForced) == "custom-user-autoexec" {
		t.Errorf("expected autoexec.cfg overwritten when force=true")
	}
}

func TestRenderWindowsBackslashAndCRLFNormalization(t *testing.T) {
	t.Parallel()
	if got := RenderAutoexecWithRate(nil, 144); len(got) == 0 {
		t.Fatal("expected non-empty default render when paths == nil")
	}

	winPaths := &config.Paths{
		SoundFontFile: `D:\Games\Doom\soundfonts\GeneralUser-GS.sf2`,
	}
	renderedAuto := string(RenderAutoexecWithRate(winPaths, 165))
	if !strings.Contains(renderedAuto, `fluid_patchset "D:/Games/Doom/soundfonts/GeneralUser-GS.sf2"`) {
		t.Fatalf("expected forward-slash fluid_patchset on Windows, got:\n%s", renderedAuto)
	}
	if !strings.Contains(renderedAuto, "vid_preferbackend 1") {
		t.Errorf("expected vid_preferbackend 1 (Vulkan) in autoexec.cfg")
	}

	tmpDir := t.TempDir()
	paths := &config.Paths{
		SoundFontFile: filepath.Join(tmpDir, "sf", "GeneralUser-GS.sf2"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda"),
	}
	if err := DeployConfigsWithDisplay(paths, "2560x1440", 144, true, io.Discard); err != nil {
		t.Fatalf("DeployConfigsWithDisplay: %v", err)
	}
	dsdaFile := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	rawLF, err := os.ReadFile(dsdaFile)
	if err != nil {
		t.Fatal(err)
	}
	rawCRLF := bytes.ReplaceAll(rawLF, []byte("\n"), []byte("\r\n"))
	if err := os.WriteFile(dsdaFile, rawCRLF, 0o644); err != nil {
		t.Fatal(err)
	}

	var diffBuf bytes.Buffer
	if err := DiffConfigsWithDisplay(paths, "2560x1440", 144, &diffBuf); err != nil {
		t.Fatalf("DiffConfigsWithDisplay: %v", err)
	}
	if !strings.Contains(diffBuf.String(), "dsda-doom.cfg is in sync with system") {
		t.Errorf("expected CRLF dsda-doom.cfg to report in sync, got:\n%s", diffBuf.String())
	}

	repoDir := filepath.Join(tmpDir, "repo")
	_ = os.MkdirAll(filepath.Join(repoDir, "configs", "uzdoom"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoDir, "configs", "dsda-doom"), 0o755)
	if err := SyncConfigs(paths, repoDir, io.Discard); err != nil {
		t.Fatalf("SyncConfigs: %v", err)
	}
	syncedDSDA, err := os.ReadFile(filepath.Join(repoDir, "configs", "dsda-doom", "dsda-doom.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(syncedDSDA, []byte("\r\n")) {
		t.Errorf("expected SyncConfigs to write LF-only template without CRLF")
	}
}

func TestUserPortOverlays_SaveDeployDiffResetAndSyncIsolation(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	paths := &config.Paths{
		BinDir:        filepath.Join(tmpDir, "bin"),
		WadsDir:       filepath.Join(tmpDir, "wads"),
		SoundFontsDir: filepath.Join(tmpDir, "soundfonts"),
		SoundFontFile: filepath.Join(tmpDir, "soundfonts", "GeneralUser-GS.sf2"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda-doom"),
		PortCfgDir:    filepath.Join(tmpDir, "doom-cli", "portcfg"),
	}

	// 1. Deploy stock configs at 1920x1080 @ 60Hz
	if err := DeployConfigsWithDisplay(paths, "1920x1080", 60, true, io.Discard); err != nil {
		t.Fatalf("initial DeployConfigsWithDisplay: %v", err)
	}

	// 2. Simulate user customizing binds in-game (DSDA-Doom) and appending binds in UZDoom autoexec.cfg
	dsdaFile := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	rawDSDA, err := os.ReadFile(dsdaFile)
	if err != nil {
		t.Fatal(err)
	}
	modDSDA := strings.Replace(
		string(rawDSDA),
		"input_turnleft                  172 -1 -1 | 101 -1 -1 | 101 -1 -1",
		"input_turnleft                  108 -1 -1 | 101 -1 -1 | 101 -1 -1",
		1,
	)
	modDSDA = strings.Replace(
		modDSDA,
		"input_fire                      157 0 22 | 157 0 22 | 157 0 22",
		"input_fire                      111,112 0 -1 | 157 0 22 | 157 0 22",
		1,
	)
	if err := os.WriteFile(dsdaFile, []byte(modDSDA), 0o644); err != nil {
		t.Fatal(err)
	}

	autoFile := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	rawAuto, err := os.ReadFile(autoFile)
	if err != nil {
		t.Fatal(err)
	}
	modAuto := string(rawAuto) + "\n// --- CAMERA CONTROLS ---\nbind \"l\" \"+left\"\nm_sensitivity_x 0.6\n"
	if err := os.WriteFile(autoFile, []byte(modAuto), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify LCS diff shows only the modified lines (not 400+ shifted lines on an inserted line)
	var diffOut bytes.Buffer
	if err := DiffConfigsWithDisplay(paths, "1920x1080", 60, &diffOut); err != nil {
		t.Fatalf("DiffConfigsWithDisplay before save: %v", err)
	}
	if strings.Contains(diffOut.String(), "more line differences") {
		t.Errorf("expected concise LCS diff without cascading shifts, got:\n%s", diffOut.String())
	}

	// 3. Save personal overrides via SaveUserConfigs
	var saveOut bytes.Buffer
	if err := SaveUserConfigs(paths, "all", &saveOut); err != nil {
		t.Fatalf("SaveUserConfigs: %v", err)
	}
	uzLines, dsdaKeys := UserOverlayCounts(paths)
	if uzLines != 2 || dsdaKeys != 2 {
		t.Fatalf("UserOverlayCounts = (%d, %d), want (2, 2)", uzLines, dsdaKeys)
	}

	// Verify saved overlay files exclude machine-specific resolution, refresh rate, and SoundFont paths
	savedDSDAOv, err := os.ReadFile(filepath.Join(paths.PortCfgDir, "dsda-doom.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(savedDSDAOv), "screen_resolution") ||
		strings.Contains(string(savedDSDAOv), "snd_soundfont") ||
		strings.Contains(string(savedDSDAOv), "GeneralUser-GS.sf2") {
		t.Errorf("saved dsda-doom overlay leaked machine settings:\n%s", string(savedDSDAOv))
	}
	savedAutoOv, err := os.ReadFile(filepath.Join(paths.PortCfgDir, "autoexec.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(savedAutoOv), "vid_maxfps") ||
		strings.Contains(string(savedAutoOv), "fluid_patchset") {
		t.Errorf("saved autoexec overlay leaked machine settings:\n%s", string(savedAutoOv))
	}

	// 4. Redeploy on a new 4K 240Hz monitor with force=true: resolution updates while personal binds survive!
	if err := DeployConfigsWithDisplay(paths, "3840x2160", 240, true, io.Discard); err != nil {
		t.Fatalf("redeploy on new monitor: %v", err)
	}
	redeployedDSDA, _ := os.ReadFile(dsdaFile)
	if !strings.Contains(string(redeployedDSDA), `screen_resolution               "3840x2160"`) ||
		!strings.Contains(string(redeployedDSDA), "input_turnleft                  108 -1 -1") {
		t.Errorf("expected 3840x2160 and preserved input_turnleft 108 after redeploy, got:\n%s", string(redeployedDSDA))
	}
	redeployedAuto, _ := os.ReadFile(autoFile)
	if !strings.Contains(string(redeployedAuto), "vid_maxfps 240") ||
		!strings.Contains(string(redeployedAuto), `bind "l" "+left"`) {
		t.Errorf("expected vid_maxfps 240 and preserved bind \"l\" after redeploy, got:\n%s", string(redeployedAuto))
	}

	diffOut.Reset()
	if err := DiffConfigsWithDisplay(paths, "3840x2160", 240, &diffOut); err != nil {
		t.Fatalf("DiffConfigsWithDisplay after redeploy: %v", err)
	}
	if strings.Count(diffOut.String(), "is in sync with system") != 2 {
		t.Errorf("expected both configs in sync with overlay active, got:\n%s", diffOut.String())
	}

	// 5. Maintainer SyncConfigs strips personal PortCfgDir overrides so repo templates stay clean
	repoDir := filepath.Join(tmpDir, "repo")
	_ = os.MkdirAll(filepath.Join(repoDir, "configs", "uzdoom"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoDir, "configs", "dsda-doom"), 0o755)
	if err := SyncConfigs(paths, repoDir, io.Discard); err != nil {
		t.Fatalf("SyncConfigs with active overlays: %v", err)
	}
	repoDSDA, _ := os.ReadFile(filepath.Join(repoDir, "configs", "dsda-doom", "dsda-doom.cfg"))
	if strings.Contains(string(repoDSDA), "input_turnleft                  108") {
		t.Errorf("SyncConfigs leaked personal DSDA-Doom bind into repo template")
	}
	repoAuto, _ := os.ReadFile(filepath.Join(repoDir, "configs", "uzdoom", "autoexec.cfg"))
	if strings.Contains(string(repoAuto), `bind "l" "+left"`) {
		t.Errorf("SyncConfigs leaked personal UZDoom bind into repo template")
	}

	// 6. ResetUserConfigs removes overlays and backups cleanly
	removed, err := ResetUserConfigs(paths, "all", io.Discard)
	if err != nil || !removed {
		t.Fatalf("ResetUserConfigs = (%v, %v), want (true, nil)", removed, err)
	}
	if uz, ds := UserOverlayCounts(paths); uz != 0 || ds != 0 {
		t.Errorf("expected (0, 0) overlay counts after reset, got (%d, %d)", uz, ds)
	}
}
