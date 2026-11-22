package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupCommandPartialFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)
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

	var outBuf, errBuf bytes.Buffer
	setupCmd := newSetupCmd()
	setupCmd.SetOut(&outBuf)
	setupCmd.SetErr(&errBuf)
	err := setupCmd.Execute()
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

func TestSetupCommand_CoreStepsGamesAndWadsSelection(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	binDir := filepath.Join(tempDir, "bin")
	wadsDir := filepath.Join(tempDir, "wads")
	sfDir := filepath.Join(tempDir, "sf")
	for _, d := range []string{binDir, wadsDir, sfDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, kv := range [][2]string{
		{"bin-dir", binDir},
		{"wads-dir", wadsDir},
		{"soundfonts-dir", sfDir},
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
	if err := os.WriteFile(filepath.Join(sfDir, "GeneralUser-GS.sf2"), []byte("sf2"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Populate mock GOG/Nightdive roots recognized across Linux, macOS, and Windows
	for _, steamSub := range []string{
		filepath.Join(tempDir, "GOG Games", "bundle"),
		filepath.Join(tempDir, "Saved Games", "Nightdive Studios", "DOOM", "bundle"),
	} {
		if err := os.MkdirAll(steamSub, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, fname := range []string{"DOOM2.WAD", "HERETIC.WAD", "HEXEN.WAD", "STRIFE1.WAD"} {
			if err := os.WriteFile(filepath.Join(steamSub, fname), []byte("IWAD-"+fname), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	// 1. Invalid --games or --wads fails fast before Step 1
	for _, badArgs := range [][]string{
		{"--games", "quake"},
		{"--games", "all,heretic"},
		{"--wads", "nonexistent-megawad"},
		{"--wads", "all,sunlust"},
		{"--wads", "heretic"},
	} {
		cmd := newSetupCmd()
		cmd.SetArgs(badArgs)
		if err := cmd.Execute(); err == nil {
			t.Errorf("expected setup %v to fail validation, got nil", badArgs)
		}
	}

	// 2. Default setup runs 4 core steps (Doom-only, --wads none)
	var outBuf, errBuf bytes.Buffer
	setupCmd := newSetupCmd()
	setupCmd.SetOut(&outBuf)
	setupCmd.SetErr(&errBuf)
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("default doom setup failed: %v\nerr:\n%s", err, errBuf.String())
	}
	outStr := outBuf.String()
	if !strings.Contains(outStr, "Step 4/4") || strings.Contains(outStr, "Step 5/") {
		t.Errorf("expected 4 core steps by default, got:\n%s", outStr)
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "DOOM2.WAD")); err != nil {
		t.Errorf("expected DOOM2.WAD extracted during default setup: %v", err)
	}
	for _, skippedIWAD := range []string{"HERETIC.WAD", "HEXEN.WAD", "STRIFE1.WAD"} {
		if _, err := os.Stat(filepath.Join(wadsDir, skippedIWAD)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be skipped by default Doom-only setup", skippedIWAD)
		}
	}

	// 3. --games heretic extracts HERETIC.WAD while skipping HEXEN.WAD and STRIFE1.WAD
	outBuf.Reset()
	setupCmd = newSetupCmd()
	setupCmd.SetOut(&outBuf)
	setupCmd.SetErr(&errBuf)
	setupCmd.SetArgs([]string{"--games", "heretic"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("doom setup --games heretic failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "HERETIC.WAD")); err != nil {
		t.Errorf("expected HERETIC.WAD extracted with --games heretic: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "HEXEN.WAD")); !os.IsNotExist(err) {
		t.Errorf("expected HEXEN.WAD still skipped with --games heretic")
	}

	// 4. --games all extracts remaining optional IWADs, and --wads doom-ii expands to 5 steps
	outBuf.Reset()
	setupCmd = newSetupCmd()
	setupCmd.SetOut(&outBuf)
	setupCmd.SetErr(&errBuf)
	setupCmd.SetArgs([]string{"--games", "all", "--wads", "doom-ii"})
	if err := setupCmd.Execute(); err != nil {
		t.Fatalf("doom setup --games all --wads doom-ii failed: %v", err)
	}
	if !strings.Contains(outBuf.String(), "Step 5/5") {
		t.Errorf("expected Step 5/5 with --wads doom-ii, got:\n%s", outBuf.String())
	}
	for _, wantIWAD := range []string{"HEXEN.WAD", "STRIFE1.WAD"} {
		if _, err := os.Stat(filepath.Join(wadsDir, wantIWAD)); err != nil {
			t.Errorf("expected %s extracted with --games all: %v", wantIWAD, err)
		}
	}
}
