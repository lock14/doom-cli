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

func TestBackupFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	targetFile := filepath.Join(tmpDir, "test.cfg")
	_ = os.WriteFile(targetFile, []byte("original-content"), 0o644)

	bkp, err := BackupFile(targetFile)
	if err != nil {
		t.Fatalf("BackupFile failed: %v", err)
	}
	if bkp == "" || !strings.Contains(bkp, ".bak.") {
		t.Errorf("unexpected backup filename: %s", bkp)
	}

	bkpData, err := os.ReadFile(bkp)
	if err != nil || string(bkpData) != "original-content" {
		t.Errorf("backup content mismatch: %s", string(bkpData))
	}
}

func TestDeployConfigs_And_Diff(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	paths := &config.Paths{
		DataDir:       tmpDir,
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
			name:           "dsda-doom.cfg",
			rootFile:       filepath.Join(rootDir, "dsda-doom", "dsda-doom.cfg"),
			embeddedData:   embeddedDSDADoom,
			requiredTokens: []string{"__RESOLUTION__", "__SOUNDFONT__"},
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
		DataDir:       tmpDir,
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
