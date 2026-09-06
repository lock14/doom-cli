package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/preset"
)

func TestPrepareLaunch_DSDADoom(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "engine_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0755)
	_ = os.MkdirAll(wadsDir, 0755)

	// Create fake engine binary
	fakeBin := filepath.Join(binDir, "dsda-doom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755)

	// Create fake WADs & DEH
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.wad"), []byte("wad"), 0644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.deh"), []byte("deh"), 0644)

	p := preset.Preset{
		Name:           "Alien Vendetta",
		Engine:         "dsda-doom",
		IWAD:           "doom2.wad",
		Mappacks:       []string{"av.wad", "av.deh"},
		AdditionalArgs: "-complevel 2",
	}

	opts := LaunchOptions{
		BinDir:    binDir,
		WadsDir:   wadsDir,
		DryRun:    true,
		ExtraArgs: []string{"-skill", "4"},
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}

	if plan.Engine != "dsda-doom" {
		t.Errorf("expected dsda-doom, got %s", plan.Engine)
	}
	if plan.EngineBin != fakeBin {
		t.Errorf("expected binary %s, got %s", fakeBin, plan.EngineBin)
	}

	// Verify args:
	// -iwad <doom2.wad> -file <av.wad> -deh <av.deh> -complevel 2 -skill 4
	joinedArgs := strings.Join(plan.Args, " ")
	expectedParts := []string{
		"-iwad " + filepath.Join(wadsDir, "DOOM2.WAD"),
		"-file " + filepath.Join(wadsDir, "av.wad"),
		"-deh " + filepath.Join(wadsDir, "av.deh"),
		"-complevel 2",
		"-skill 4",
	}
	lowerArgs := strings.ToLower(joinedArgs)
	for _, part := range expectedParts {
		if !strings.Contains(lowerArgs, strings.ToLower(part)) {
			t.Errorf("expected args to contain %q, got: %s", part, joinedArgs)
		}
	}

	// Test Execute with DryRun
	var out bytes.Buffer
	if err := Execute(plan, &out, &out); err != nil {
		t.Fatalf("dry run Execute failed: %v", err)
	}
	if !strings.Contains(out.String(), "Launching Preset: Alien Vendetta") {
		t.Errorf("expected banner in output, got:\n%s", out.String())
	}
}

func TestPrepareLaunch_UZDoom_OptionalAudio(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "uzdoom_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0755)
	_ = os.MkdirAll(wadsDir, 0755)

	fakeBin := filepath.Join(binDir, "uzdoom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755)

	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0644)
	_ = os.WriteFile(filepath.Join(wadsDir, "eviternity.wad"), []byte("wad"), 0644)

	// Note: "idkfa 2024.wad" is intentionally NOT created to test graceful degradation
	p := preset.Preset{
		Name:     "Eviternity",
		Engine:   "uzdoom",
		IWAD:     "doom2.wad",
		Mappacks: []string{"idkfa 2024.wad", "eviternity.wad"},
	}

	var out bytes.Buffer
	opts := LaunchOptions{
		BinDir:  binDir,
		WadsDir: wadsDir,
		DryRun:  true,
		Out:     &out,
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}

	if !strings.Contains(out.String(), "Optional soundtrack 'idkfa 2024.wad' not found") {
		t.Errorf("expected optional soundtrack warning, got: %s", out.String())
	}

	joinedArgs := strings.Join(plan.Args, " ")
	if strings.Contains(joinedArgs, "idkfa 2024.wad") {
		t.Errorf("did not expect missing idkfa 2024.wad in args: %s", joinedArgs)
	}
	if !strings.Contains(joinedArgs, filepath.Join(wadsDir, "eviternity.wad")) {
		t.Errorf("expected eviternity.wad in args: %s", joinedArgs)
	}
}

func TestPrepareLaunch_EngineOverride(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "override_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0755)
	_ = os.MkdirAll(wadsDir, 0755)

	fakeBin := filepath.Join(binDir, "dsda-doom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755)
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM.WAD"), []byte("iwad"), 0644)

	p := preset.Preset{
		Name:   "Doom 1",
		Engine: "uzdoom",
		IWAD:   "doom.wad",
	}

	opts := LaunchOptions{
		EngineOverride: "dsda-doom",
		BinDir:         binDir,
		WadsDir:        wadsDir,
		DryRun:         true,
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}

	if plan.Engine != "dsda-doom" {
		t.Errorf("expected overridden engine dsda-doom, got %s", plan.Engine)
	}
}

func TestPrepareLaunch_CustomEngineAndArgs(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	customBinDir := filepath.Join(tmpDir, "custom_bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0755)
	_ = os.MkdirAll(customBinDir, 0755)
	_ = os.MkdirAll(wadsDir, 0755)

	customBinPath := filepath.Join(customBinDir, "woof")
	_ = os.WriteFile(customBinPath, []byte("#!/bin/sh\n"), 0755)

	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0644)
	_ = os.WriteFile(filepath.Join(wadsDir, "sunlust.wad"), []byte("wad"), 0644)
	_ = os.WriteFile(filepath.Join(wadsDir, "sunlust.deh"), []byte("deh"), 0644)

	customEngines := map[string]preset.EngineConfig{
		"woof": {
			Name:        "woof",
			Binary:      customBinPath,
			ArgsStyle:   "boom",
			DefaultArgs: []string{"-geometry", "1920x1080"},
		},
	}

	p := preset.Preset{
		Name:           "Sunlust",
		Engine:         "woof",
		IWAD:           "DOOM2.WAD",
		Mappacks:       []string{"sunlust.wad", "sunlust.deh"},
		AdditionalArgs: "-skill 4",
	}

	opts := LaunchOptions{
		BinDir:    binDir,
		WadsDir:   wadsDir,
		Engines:   customEngines,
		DryRun:    true,
		ExtraArgs: []string{"-warp", "15"},
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}

	if plan.EngineBin != customBinPath {
		t.Errorf("expected engine binary %s, got %s", customBinPath, plan.EngineBin)
	}

	joined := strings.Join(plan.Args, " ")
	expectedTokens := []string{
		"-file " + filepath.Join(wadsDir, "sunlust.wad"),
		"-deh " + filepath.Join(wadsDir, "sunlust.deh"),
		"-geometry 1920x1080",
		"-skill 4",
		"-warp 15",
	}
	for _, tok := range expectedTokens {
		if !strings.Contains(joined, tok) {
			t.Errorf("expected args to contain %q, got: %s", tok, joined)
		}
	}
}
