package templates

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/config"
)

func TestBackupFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "deploy_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	targetFile := filepath.Join(tmpDir, "test.cfg")
	_ = os.WriteFile(targetFile, []byte("original-content"), 0644)

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
	tmpDir, err := os.MkdirTemp("", "deploy_full_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	paths := &config.Paths{
		DataDir:       tmpDir,
		BinDir:        filepath.Join(tmpDir, "bin"),
		WadsDir:       filepath.Join(tmpDir, "wads"),
		SoundFontsDir: filepath.Join(tmpDir, "soundfonts"),
		SoundFontFile: filepath.Join(tmpDir, "soundfonts", "GeneralUser-GS.sf2"),
		UZDoomDir:     filepath.Join(tmpDir, "uzdoom"),
		DSDADir:       filepath.Join(tmpDir, "dsda-doom"),
	}

	if err := DeployConfigs(paths); err != nil {
		t.Fatalf("DeployConfigs failed: %v", err)
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
	_ = os.MkdirAll(filepath.Join(repoDir, "internal", "templates", "data"), 0o755)

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

	// Verify embedded template internal/templates/data/autoexec.cfg
	embeddedAuto, err := os.ReadFile(filepath.Join(repoDir, "internal", "templates", "data", "autoexec.cfg"))
	if err != nil {
		t.Fatalf("failed reading embedded autoexec: %v", err)
	}
	if string(embeddedAuto) != string(syncedAuto) {
		t.Errorf("embedded autoexec does not match root template")
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

	// Verify embedded template internal/templates/data/dsda-doom.cfg
	embeddedDSDA, err := os.ReadFile(filepath.Join(repoDir, "internal", "templates", "data", "dsda-doom.cfg"))
	if err != nil {
		t.Fatalf("failed reading embedded dsda-doom cfg: %v", err)
	}
	if string(embeddedDSDA) != string(syncedDSDA) {
		t.Errorf("embedded dsda cfg does not match root template")
	}
}
