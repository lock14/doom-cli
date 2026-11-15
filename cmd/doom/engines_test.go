package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEnginesCommands(t *testing.T) {
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	wadsDir := filepath.Join(tempDir, "wads")
	sfDir := filepath.Join(tempDir, "sf")
	for _, d := range []string{binDir, wadsDir, sfDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("PATH", binDir)
	for _, kv := range [][2]string{
		{"bin-dir", binDir},
		{"wads-dir", wadsDir},
		{"soundfonts-dir", sfDir},
	} {
		if err := runConfigSet(nil, []string{kv[0], kv[1]}); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Naked `doom engines` in non-TTY falls back to runEnginesList
	var listOut bytes.Buffer
	enginesCmd := newEnginesCmd()
	enginesCmd.SetOut(&listOut)
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("naked engines command failed: %v", err)
	}
	if !strings.Contains(listOut.String(), "=== Available Doom Source Port Engines ===") {
		t.Errorf("expected naked engines in non-TTY to list engines, got:\n%s", listOut.String())
	}

	// 2. engines add with DisplayName "Woof!"
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{
		"add", "Woof!",
		"--id", "woof",
		"--binary", "woof",
		"--style", "boom",
		"--prefix-args", "-config /tmp/woof.cfg",
		"--description", "Woof MBF21",
	})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines add failed: %v", err)
	}

	_, _, cat, err := loadAppState()
	if err != nil {
		t.Fatalf("loadAppState failed: %v", err)
	}
	if eng, ok := cat.Engines["woof"]; !ok || eng.ID != "woof" ||
		!reflect.DeepEqual(eng.PrefixArgs, []string{"-config", "/tmp/woof.cfg"}) {
		t.Fatalf("expected woof in cat.Engines with prefix_args, got %+v", cat.Engines)
	}

	// 2b. engines set using DisplayName "Woof!"
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"set", "Woof!", "--badge", "WOOF+"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines set 'Woof!' by DisplayName failed: %v", err)
	}

	// 3. engines set on built-in dsda-doom across multiple calls, clearing fields, and engines reset
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{
		"set", "dsda-doom", "--binary", "/custom/dsda-doom", "--badge", "DSDA+", "--args", "-fast -nomonsters",
	})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines set dsda-doom failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	eng, ok := cat.FindEngine("dsda-doom")
	if !ok || eng.Binary != "/custom/dsda-doom" || eng.Badge != "DSDA+" ||
		!reflect.DeepEqual(eng.DefaultArgs, []string{"-fast", "-nomonsters"}) {
		t.Fatalf("expected overridden dsda-doom with args, got %+v", eng)
	}

	// Verify engines list displays (custom) for woof and (overridden) for dsda-doom
	listOut.Reset()
	enginesCmd = newEnginesCmd()
	enginesCmd.SetOut(&listOut)
	enginesCmd.SetArgs([]string{"list"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines list failed: %v", err)
	}
	if !strings.Contains(listOut.String(), "(custom)") || !strings.Contains(listOut.String(), "(overridden)") {
		t.Errorf("expected engines list to show (custom) and (overridden), got:\n%s", listOut.String())
	}

	// Clear badge and args back to empty via CLI flags
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"set", "dsda-doom", "--badge", "", "--args", ""})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines set clear flags failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	eng, ok = cat.FindEngine("dsda-doom")
	if !ok || eng.Badge != "" || len(eng.DefaultArgs) != 0 {
		t.Fatalf("expected cleared Badge and DefaultArgs on dsda-doom, got %+v", eng)
	}

	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"reset", "dsda-doom"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines reset dsda-doom failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	if eng, ok := cat.FindEngine("dsda-doom"); !ok || eng.Binary != "dsda-doom" {
		t.Fatalf("expected reset dsda-doom binary, got %+v", eng)
	}

	// 4. engines remove built-in (fails) and custom (succeeds)
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"remove", "uzdoom"})
	if err := enginesCmd.Execute(); err == nil {
		t.Errorf("expected error removing built-in uzdoom, got nil")
	}

	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"remove", "Woof!"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines remove Woof! failed: %v", err)
	}

	_, _, cat, _ = loadAppState()
	if _, ok := cat.Engines["woof"]; ok {
		t.Errorf("expected woof removed from cat.Engines, got %+v", cat.Engines)
	}

	// 5. engines cfg (default diff, cfg install, cfg diff)
	var cfgOut bytes.Buffer
	enginesCmd = newEnginesCmd()
	enginesCmd.SetOut(&cfgOut)
	enginesCmd.SetArgs([]string{"cfg", "install"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines cfg install failed: %v", err)
	}

	cfgOut.Reset()
	enginesCmd = newEnginesCmd()
	enginesCmd.SetOut(&cfgOut)
	enginesCmd.SetArgs([]string{"cfg"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines cfg (default diff) failed: %v", err)
	}
	if !strings.Contains(cfgOut.String(), "=== UZDoom Diff ===") {
		t.Errorf("expected engines cfg to run diff, got:\n%s", cfgOut.String())
	}

	// 6. engines install (without --force) preserves existing .cfg files and verifies SoundFont
	for _, binName := range []string{"uzdoom", "dsda-doom", "uzdoom.exe", "dsda-doom.exe"} {
		if err := os.WriteFile(filepath.Join(binDir, binName), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sfDir, "GeneralUser-GS.sf2"), []byte("sf2"), 0o644); err != nil {
		t.Fatal(err)
	}
	var instOut bytes.Buffer
	enginesCmd = newEnginesCmd()
	enginesCmd.SetOut(&instOut)
	enginesCmd.SetArgs([]string{"install"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines install failed: %v", err)
	}
	if !strings.Contains(instOut.String(), "SoundFont already installed") ||
		!strings.Contains(instOut.String(), "UZDoom config already installed") {
		t.Errorf(
			"expected engines install without --force to preserve existing configs and SoundFont, got:\n%s",
			instOut.String(),
		)
	}

	listOut.Reset()
	enginesCmd = newEnginesCmd()
	enginesCmd.SetOut(&listOut)
	enginesCmd.SetArgs([]string{"list"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines list after install failed: %v", err)
	}
	if !strings.Contains(listOut.String(), "[✓ Installed:") {
		t.Errorf("expected engines list to show [✓ Installed: ...], got:\n%s", listOut.String())
	}
}
