package templates

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

	// Verify autoexec.cfg
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

	// Verify dsda-doom.cfg
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

	// Verify DiffConfigs shows in sync
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

	// 1. Mock system config dirs
	paths := &config.Paths{
		UZDoomDir: filepath.Join(tmpDir, "sys_uzdoom"),
		DSDADir:   filepath.Join(tmpDir, "sys_dsda"),
	}
	_ = os.MkdirAll(paths.UZDoomDir, 0o755)
	_ = os.MkdirAll(paths.DSDADir, 0o755)

	// Live system configs with modified settings
	liveAuto := "fluid_patchset \"/custom/soundfont.sf2\"\nvid_maxfps 144\n"
	_ = os.WriteFile(filepath.Join(paths.UZDoomDir, "autoexec.cfg"), []byte(liveAuto), 0o644)

	liveDSDA := "screen_resolution \"2560x1440\"\nsnd_soundfont \"/custom/soundfont.sf2\"\n"
	_ = os.WriteFile(filepath.Join(paths.DSDADir, "dsda-doom.cfg"), []byte(liveDSDA), 0o644)

	// 2. Mock repo dirs
	repoDir := filepath.Join(tmpDir, "repo")
	_ = os.MkdirAll(filepath.Join(repoDir, "uzdoom"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoDir, "dsda-doom"), 0o755)

	var out bytes.Buffer
	if err := SyncConfigs(paths, repoDir, &out); err != nil {
		t.Fatalf("SyncConfigs failed: %v", err)
	}

	// Verify root template uzdoom/autoexec.cfg
	syncedAuto, err := os.ReadFile(filepath.Join(repoDir, "uzdoom", "autoexec.cfg"))
	if err != nil {
		t.Fatalf("failed reading synced uzdoom autoexec: %v", err)
	}
	if !strings.Contains(string(syncedAuto), `fluid_patchset "__SOUNDFONT__"`) {
		t.Errorf("expected __SOUNDFONT__ token in synced autoexec, got: %s", string(syncedAuto))
	}
	if !strings.Contains(string(syncedAuto), `vid_maxfps __REFRESH_RATE__`) {
		t.Errorf("expected __REFRESH_RATE__ token in synced autoexec, got: %s", string(syncedAuto))
	}

	// Verify root template dsda-doom/dsda-doom.cfg
	syncedDSDA, err := os.ReadFile(filepath.Join(repoDir, "dsda-doom", "dsda-doom.cfg"))
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
			rootFile:     filepath.Join(rootDir, "dsda-doom", "dsda-doom.cfg"),
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
			rootFile:       filepath.Join(rootDir, "uzdoom", "autoexec.cfg"),
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

	// With force=true, existing autoexec.cfg is backed up and overwritten
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
	// Nil paths guard returns cleanly without panicking.
	if got := RenderAutoexecWithRate(nil, 144); len(got) == 0 {
		t.Fatal("expected non-empty default render when paths == nil")
	}

	// Windows backslash SoundFont path is normalized to forward slashes in UZDoom autoexec.cfg.
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

	// CRLF line endings on disk do not produce false-positive diffs or dirty synced templates.
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
	_ = os.MkdirAll(filepath.Join(repoDir, "uzdoom"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoDir, "dsda-doom"), 0o755)
	if err := SyncConfigs(paths, repoDir, io.Discard); err != nil {
		t.Fatalf("SyncConfigs: %v", err)
	}
	syncedDSDA, err := os.ReadFile(filepath.Join(repoDir, "dsda-doom", "dsda-doom.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(syncedDSDA, []byte("\r\n")) {
		t.Errorf("expected SyncConfigs to write LF-only template without CRLF")
	}
}
