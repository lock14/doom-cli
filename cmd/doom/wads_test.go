package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/repolint"
	"github.com/lock14/doom-cli/internal/wadfmt"
)

func TestWadsCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	for _, kv := range [][2]string{
		{"bin-dir", tempDir},
		{"wads-dir", tempDir},
	} {
		if err := runConfigSet(nil, []string{kv[0], kv[1]}); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(tempDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify expected wads subcommands exist and legacy extract/config do not
	wadsCmd := newWadsCmd()
	subCommands := make(map[string]*bool)
	for _, want := range []string{"list", "show", "fetch", "scan", "add", "set", "reset", "remove", "lint", "build"} {
		b := false
		subCommands[want] = &b
	}
	for _, sub := range wadsCmd.Commands() {
		if ptr, ok := subCommands[sub.Name()]; ok {
			*ptr = true
		}
		if sub.Name() == "extract" || sub.Name() == "config" {
			t.Errorf("unexpected legacy subcommand %q on wadsCmd", sub.Name())
		}
	}
	for name, found := range subCommands {
		if !*found {
			t.Errorf("expected subcommand %q on wadsCmd", name)
		}
	}

	// 1. wads list shows [✓ Ready] for Doom II, [✗ Missing] for unextracted megawads, and category tags
	var listOut bytes.Buffer
	wadsCmd = newWadsCmd()
	wadsCmd.SetOut(&listOut)
	wadsCmd.SetArgs([]string{"list"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads list failed: %v", err)
	}
	for _, want := range []string{"[✓ Ready]", "[✗ Missing]", "[Official IWAD", "[Community Megawad"} {
		if !strings.Contains(listOut.String(), want) {
			t.Errorf("expected wads list to contain %q, got:\n%s", want, listOut.String())
		}
	}

	// 2. wads show built-in includes Launch Cmd preview
	var avOut bytes.Buffer
	wadsCmd = newWadsCmd()
	wadsCmd.SetOut(&avOut)
	wadsCmd.SetArgs([]string{"show", "Alien Vendetta"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads show Alien Vendetta failed: %v", err)
	}
	if !strings.Contains(avOut.String(), "Launch Cmd:") || !strings.Contains(avOut.String(), "AV.WAD") {
		t.Errorf("expected wads show to include Launch Cmd preview, got:\n%s", avOut.String())
	}

	// 3. wads add custom with a custom engine that has DisplayName and Description
	enginesCmd := newEnginesCmd()
	enginesCmd.SetArgs([]string{
		"add", "Woof!", "--id", "woof", "--binary", "woof", "--style", "boom", "--description", "MBF21 port",
	})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines add woof failed: %v", err)
	}

	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{
		"add", "CustomMap",
		"-e", "woof",
		"--iwad", "DOOM2.WAD",
		"--files", "custom.wad",
		"--description", "My Custom Map",
	})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads add failed: %v", err)
	}

	// 4. wads show custom displays both DisplayName and Description via FormatLabel
	var showOut bytes.Buffer
	wadsCmd = newWadsCmd()
	wadsCmd.SetOut(&showOut)
	wadsCmd.SetArgs([]string{"show", "CustomMap"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads show CustomMap failed: %v", err)
	}
	if !strings.Contains(showOut.String(), "Engine:        Woof! (MBF21 port)") {
		t.Errorf(
			"expected wads show CustomMap to display 'Engine:        Woof! (MBF21 port)', got:\n%s",
			showOut.String(),
		)
	}

	// 5. wads set across multiple calls preserves prior fields (and supports Roman numeral / filename query doom2)
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"set", "Sunlust", "-e", "uzdoom"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads set -e failed: %v", err)
	}
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"set", "Sunlust", "--args", "-skill 4"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads set --args failed: %v", err)
	}
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"set", "doom2", "-e", "uzdoom"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads set doom2 failed: %v", err)
	}
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"reset", "doom2"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads reset doom2 failed: %v", err)
	}

	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"set", "evilution", "-e", "uzdoom"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads set evilution failed: %v", err)
	}
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"reset", "evilution"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads reset evilution failed: %v", err)
	}

	// Verify wads list displays (custom) and (overridden)
	listOut.Reset()
	wadsCmd = newWadsCmd()
	wadsCmd.SetOut(&listOut)
	wadsCmd.SetArgs([]string{"list"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads list after add/set failed: %v", err)
	}
	if !strings.Contains(listOut.String(), "(custom)") || !strings.Contains(listOut.String(), "(overridden)") {
		t.Errorf("expected wads list to show (custom) and (overridden), got:\n%s", listOut.String())
	}

	_, _, cat, err := loadAppState()
	if err != nil {
		t.Fatalf("loadAppState failed: %v", err)
	}
	sunlust := cat.Find("sunlust")
	if sunlust == nil || sunlust.Engine != "uzdoom" || !reflect.DeepEqual(sunlust.Args, []string{"-skill", "4"}) {
		t.Fatalf("expected Sunlust override in catalog, got %+v", sunlust)
	}

	// 6. wads reset
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"reset", "Sunlust"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads reset failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	sunlust = cat.Find("sunlust")
	if sunlust == nil || sunlust.Engine != "dsda-doom" || len(sunlust.Args) != 0 {
		t.Fatalf("expected Sunlust launch options cleared, got %+v", sunlust)
	}

	// 7. wads remove rejects built-in curated presets (including via substring lookup) and removes custom presets
	for _, curatedQuery := range []string{"doom2", "plutonia"} {
		wadsCmd = newWadsCmd()
		wadsCmd.SetArgs([]string{"remove", curatedQuery})
		if err := wadsCmd.Execute(); err == nil ||
			!strings.Contains(err.Error(), "cannot remove built-in curated preset") {
			t.Errorf("expected wads remove %s to reject built-in curated preset, got %v", curatedQuery, err)
		}
	}

	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"remove", "CustomMap"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads remove failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	if cat.Find("custommap") != nil {
		t.Fatalf("expected CustomMap removed")
	}

	// 8. wads lint on repo root and outside repo checkout
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"lint", filepath.Join("..", "..")})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads lint failed: %v", err)
	}

	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"lint", tempDir})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads lint outside repo checkout failed: %v", err)
	}

	// Verify lint fails on invalid user entity without double-printing quarantine warning
	paths := resolveAppPaths()
	if err := os.MkdirAll(paths.CatalogDir, 0o755); err != nil {
		t.Fatal(err)
	}
	badFile := filepath.Join(paths.CatalogDir, "preset.bad.json")
	if err := os.WriteFile(badFile, []byte(`{"kind":"preset","id":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	diags, lintErr := repolint.LintCatalog(filepath.Join("..", ".."), paths.CatalogDir)
	if lintErr != nil {
		t.Fatalf("LintCatalog error: %v", lintErr)
	}
	if !repolint.HasErrors(diags) {
		t.Errorf("expected LintCatalog to report errors for invalid user entity")
	}
}

func TestRunWadsTUISingleShot(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("PATH", tempDir)
	for _, kv := range [][2]string{
		{"bin-dir", tempDir},
		{"wads-dir", tempDir},
	} {
		if err := runConfigSet(nil, []string{kv[0], kv[1]}); err != nil {
			t.Fatal(err)
		}
	}

	origArgs := os.Args
	os.Args = []string{"doom", "wads"}
	defer func() { os.Args = origArgs }()

	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	// 1. Quitting immediately via 'q' in non-TTY numbered menu returns nil
	rQuit, wQuit, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = wQuit.WriteString("q\n")
	_ = wQuit.Close()
	os.Stdin = rQuit
	if err := runWadsTUI(newWadsCmd(), nil); err != nil {
		t.Fatalf("runWadsTUI on quit returned unexpected error: %v", err)
	}
	_ = rQuit.Close()

	if runtime.GOOS == "windows" {
		return
	}

	// 2. Selecting a preset executes the engine once and exits immediately without looping
	if err := os.WriteFile(filepath.Join(tempDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(tempDir, "launches.log")
	script := "#!/bin/sh\necho launched >> \"" + logPath + "\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(tempDir, "dsda-doom"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, cat, err := loadAppState()
	if err != nil {
		t.Fatalf("loadAppState: %v", err)
	}
	doom2Idx := -1
	for i, p := range cat.Presets {
		if p.ID == "doom-ii" {
			doom2Idx = i + 1
			break
		}
	}
	if doom2Idx == -1 {
		t.Fatal("doom-ii preset not found in catalog")
	}

	rPlay, wPlay, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprintf(wPlay, "%d\n\n%d\n", doom2Idx, doom2Idx)
	_ = wPlay.Close()
	os.Stdin = rPlay
	defer func() { _ = rPlay.Close() }()

	if err := runWadsTUI(newWadsCmd(), nil); err != nil {
		t.Fatalf("runWadsTUI single-shot execution failed: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected fake engine to write %s: %v", logPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 || lines[0] != "launched" {
		t.Errorf("expected exactly 1 engine launch in single-shot mode, got %d (%v)", len(lines), lines)
	}
}

func TestWadsListAndShowTaxonomyMetadata(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOOM_CONFIG_DIR", tempDir)

	var listBuf bytes.Buffer
	listCmd := newRootCmd()
	listCmd.SetOut(&listBuf)
	listCmd.SetErr(&listBuf)
	listCmd.SetArgs([]string{"wads", "list"})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("wads list failed: %v", err)
	}
	listOut := listBuf.String()
	for _, want := range []string{
		"AUGER;ZENITH",
		"Cyberpunk Adventure | Moderate | S-Tier (9.5/10)",
		"MyHouse.pk3",
		"Strife: Quest for the Sigil",
	} {
		if !strings.Contains(listOut, want) {
			t.Errorf("expected wads list output to contain %q, got:\n%s", want, listOut)
		}
	}

	var showBuf bytes.Buffer
	showCmd := newRootCmd()
	showCmd.SetOut(&showBuf)
	showCmd.SetErr(&showBuf)
	showCmd.SetArgs([]string{"wads", "show", "auger-zenith"})
	if err := showCmd.Execute(); err != nil {
		t.Fatalf("wads show auger-zenith failed: %v", err)
	}
	showOut := showBuf.String()
	for _, want := range []string{
		"Genre:         Cyberpunk Adventure",
		"Difficulty:    Moderate",
		"Quality:       S-Tier (9.5/10)",
		"https://www.doomworld.com/idgames/levels/doom2/Ports/megawads/dbp37_augzen",
	} {
		if !strings.Contains(showOut, want) {
			t.Errorf("expected wads show output to contain %q, got:\n%s", want, showOut)
		}
	}
}

func TestRootExecFlag_BuildLauncherNilCfg_ThemeDirReject_And_FetchBehavior(t *testing.T) {
	tempDir := t.TempDir()
	wadsDir := filepath.Join(tempDir, "wads")
	binDir := filepath.Join(tempDir, "bin")
	for _, d := range []string{wadsDir, binDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	for _, kv := range [][2]string{
		{"bin-dir", binDir},
		{"wads-dir", wadsDir},
	} {
		if err := runConfigSet(nil, []string{kv[0], kv[1]}); err != nil {
			t.Fatal(err)
		}
	}

	// 1. buildLauncherOptions does not panic when cfg or paths is nil.
	paths := resolveAppPaths()
	theme, _, opts := buildLauncherOptions(paths, nil, nil, 0)
	if theme.Name == "" || opts.ThemeSource != "default" {
		t.Errorf("unexpected buildLauncherOptions(paths, nil) result: theme=%q src=%q", theme.Name, opts.ThemeSource)
	}
	theme2, _, _ := buildLauncherOptions(nil, nil, nil, 0)
	if theme2.Name == "" {
		t.Errorf("unexpected empty theme for buildLauncherOptions(nil, nil)")
	}

	// 2. doom config set theme <directory> rejects directory paths.
	someDir := filepath.Join(tempDir, "not-a-theme-file")
	if err := os.MkdirAll(someDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runConfigSet(nil, []string{"theme", someDir}); err == nil {
		t.Errorf("expected runConfigSet(theme, directory) to fail, got nil")
	}

	// 3. doom wads fetch <commercial> returns non-zero error when missing from local and Steam libraries.
	var fetchOut bytes.Buffer
	wadsCmd := newWadsCmd()
	wadsCmd.SetOut(&fetchOut)
	wadsCmd.SetErr(&fetchOut)
	wadsCmd.SetArgs([]string{"fetch", "doom-ii"})
	if err := wadsCmd.Execute(); err == nil {
		t.Fatal("expected wads fetch doom-ii to return error when DOOM2.WAD is missing")
	}
	if !strings.Contains(fetchOut.String(), "is a commercial game") {
		t.Errorf("expected commercial game message for missing doom-ii, got:\n%s", fetchOut.String())
	}

	// 4. doom wads fetch <custom> returns non-zero error with non-commercial message when required file is missing.
	addCmd := newWadsCmd()
	addCmd.SetArgs([]string{"add", "LocalMap", "--files", "missing-local.wad"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("wads add LocalMap failed: %v", err)
	}
	fetchOut.Reset()
	wadsCmd = newWadsCmd()
	wadsCmd.SetOut(&fetchOut)
	wadsCmd.SetErr(&fetchOut)
	wadsCmd.SetArgs([]string{"fetch", "LocalMap"})
	if err := wadsCmd.Execute(); err == nil {
		t.Fatal("expected wads fetch LocalMap to return error when required files are missing")
	}
	if strings.Contains(fetchOut.String(), "is a commercial game") ||
		!strings.Contains(fetchOut.String(), "has no download sources configured") {
		t.Errorf("expected custom no-sources message for LocalMap, got:\n%s", fetchOut.String())
	}

	// 5. doom -exec autoexec.cfg -errorlog err.txt --dry-run forwards -exec and -errorlog cleanly.
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, binName := range []string{"dsda-doom", "dsda-doom.exe"} {
		if err := os.WriteFile(filepath.Join(binDir, binName), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var rootOut bytes.Buffer
	rootCmd := newRootCmd()
	rootCmd.SetOut(&rootOut)
	rootCmd.SetErr(&rootOut)
	rootCmd.SetArgs([]string{"-exec", "autoexec.cfg", "-errorlog", "err.txt", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom -exec autoexec.cfg -errorlog err.txt --dry-run failed: %v", err)
	}
	if !strings.Contains(rootOut.String(), "-exec autoexec.cfg -errorlog err.txt") {
		t.Errorf("expected -exec and -errorlog forwarded to engine, got:\n%s", rootOut.String())
	}

	// 6. doom wads fetch doom-ii runs Steam discovery to synthesize optional idkfa 2024.wad
	// even when DOOM2.WAD is already present in wads-dir.
	oggPayload := []byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00")
	extrasBuf, err := wadfmt.BuildBytes("PWAD", []wadfmt.Lump{{Name: "H_RUNNIN", Data: oggPayload}})
	if err != nil {
		t.Fatalf("wadfmt.BuildBytes: %v", err)
	}
	for _, steamSub := range []string{
		filepath.Join(tempDir, "GOG Games", "rerelease"),
		filepath.Join(tempDir, "Saved Games", "Nightdive Studios", "DOOM", "rerelease"),
	} {
		if err := os.MkdirAll(steamSub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(steamSub, "extras.wad"), extrasBuf, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fetchOut.Reset()
	wadsCmd = newWadsCmd()
	wadsCmd.SetOut(&fetchOut)
	wadsCmd.SetErr(&fetchOut)
	wadsCmd.SetArgs([]string{"fetch", "doom-ii"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads fetch doom-ii with existing DOOM2.WAD failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "idkfa 2024.wad")); err != nil {
		t.Errorf("expected wads fetch doom-ii to synthesize optional idkfa 2024.wad, stat err: %v", err)
	}

	// 7. Malformed config warning routes through cmd.ErrOrStderr() via loadAppStateWithErr.
	if err := os.WriteFile(paths.ConfigFile, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	var listOut, listErr bytes.Buffer
	listCmd := newWadsCmd()
	listCmd.SetOut(&listOut)
	listCmd.SetErr(&listErr)
	listCmd.SetArgs([]string{"list"})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("wads list with malformed config failed: %v", err)
	}
	if !strings.Contains(listErr.String(), "Warning: could not parse") {
		t.Errorf("expected malformed config warning on cmd stderr, got:\n%s", listErr.String())
	}
}
