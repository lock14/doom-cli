package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/catalog"
)

func TestSetupCommandPartialFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	sfConflictFile := filepath.Join(tempDir, "sf-conflict-file")
	if err := os.WriteFile(sfConflictFile, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(tempDir, "bin")
	wadsDir := filepath.Join(tempDir, "wads")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]string{
		{"bin-dir", binDir},
		{"wads-dir", wadsDir},
		{"soundfonts-dir", sfConflictFile},
	} {
		if err := runConfigSet(nil, []string{kv[0], kv[1]}); err != nil {
			t.Fatal(err)
		}
	}
	for _, binName := range []string{"uzdoom", "dsda-doom", "uzdoom.exe", "dsda-doom.exe"} {
		if err := os.WriteFile(filepath.Join(binDir, binName), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range cat.Presets {
		for _, f := range p.Files {
			if !f.Optional {
				if err := os.WriteFile(filepath.Join(wadsDir, f.Name), []byte("wad"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	var outBuf, errBuf bytes.Buffer
	setupCmd := newSetupCmd()
	setupCmd.SetOut(&outBuf)
	setupCmd.SetErr(&errBuf)
	err = setupCmd.Execute()
	if err == nil {
		t.Fatal("expected doom setup to return non-nil error on step failure, got nil")
	}
	if !strings.Contains(err.Error(), "soundfont") {
		t.Errorf("expected setup error to mention failed step 'soundfont', got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Warning: soundfont installer encountered:") {
		t.Errorf("expected stderr to contain soundfont warning, got:\n%s", errBuf.String())
	}
	if !strings.Contains(outBuf.String(), "! Doom setup finished with errors in: soundfont") {
		t.Errorf("expected stdout to contain partial failure summary, got:\n%s", outBuf.String())
	}
}
