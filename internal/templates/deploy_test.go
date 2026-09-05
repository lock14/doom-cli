package templates

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-configs/internal/config"
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
		DoomRunnerDir: filepath.Join(tmpDir, "DoomRunner"),
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
