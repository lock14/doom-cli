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

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/preset"
)

func TestParseRootLaunchArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rawArgs    []string
		wantPreset string
		wantExtra  []string
	}{
		{
			name:       "naked doom defaults to doom preset",
			rawArgs:    []string{"doom"},
			wantPreset: "doom",
			wantExtra:  nil,
		},
		{
			name:       "naked doom with single-dash engine flags defaults to doom preset",
			rawArgs:    []string{"doom", "-skill", "4", "-warp", "1", "1"},
			wantPreset: "doom",
			wantExtra:  []string{"-skill", "4", "-warp", "1", "1"},
		},
		{
			name:       "naked doom with CLI flags only defaults to doom preset",
			rawArgs:    []string{"doom", "-e", "uzdoom", "--dry-run", "--theme", "blood", "--nerd-fonts"},
			wantPreset: "doom",
			wantExtra:  nil,
		},
		{
			name:       "explicit wad preset with engine flags",
			rawArgs:    []string{"doom", "sunlust", "-skill", "4", "-pistolstart"},
			wantPreset: "sunlust",
			wantExtra:  []string{"-skill", "4", "-pistolstart"},
		},
		{
			name:       "CLI flags before explicit wad preset and engine flags",
			rawArgs:    []string{"doom", "--dry-run", "-e", "uzdoom", "Eviternity II", "-skill", "4"},
			wantPreset: "Eviternity II",
			wantExtra:  []string{"-skill", "4"},
		},
		{
			name: "CLI flags with equals syntax and dash-dash separator",
			rawArgs: []string{
				"doom", "--wads-dir=/tmp/wads", "--theme=toxic",
				"--nerd-fonts=true", "sunlust", "--", "-nomonsters", "-fast",
			},
			wantPreset: "sunlust",
			wantExtra:  []string{"-nomonsters", "-fast"},
		},
		{
			name:       "naked doom with dash-dash separator",
			rawArgs:    []string{"doom", "--theme", "blood", "--", "-nomonsters", "-fast"},
			wantPreset: "doom",
			wantExtra:  []string{"-nomonsters", "-fast"},
		},
		{
			name:       "trailing dash-dash with no extra args",
			rawArgs:    []string{"doom", "--"},
			wantPreset: "doom",
			wantExtra:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotPreset, gotExtra := parseRootLaunchArgs(tt.rawArgs)
			if gotPreset != tt.wantPreset {
				t.Errorf("parseRootLaunchArgs(%v) preset = %q, want %q", tt.rawArgs, gotPreset, tt.wantPreset)
			}
			if len(gotExtra) == 0 && len(tt.wantExtra) == 0 {
				return
			}
			if !reflect.DeepEqual(gotExtra, tt.wantExtra) {
				t.Errorf("parseRootLaunchArgs(%v) extra = %v, want %v", tt.rawArgs, gotExtra, tt.wantExtra)
			}
		})
	}
}

func TestExtractEngineArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		subcommand string
		rawArgs    []string
		expected   []string
	}{
		{
			name:       "wads with dash-dash arguments",
			subcommand: "wads",
			rawArgs:    []string{"doom", "wads", "--", "-nomonsters", "-fast"},
			expected:   []string{"-nomonsters", "-fast"},
		},
		{
			name:       "wads with trailing dash-dash and no extra args",
			subcommand: "wads",
			rawArgs:    []string{"doom", "wads", "--"},
			expected:   nil,
		},
		{
			name:       "wads with --wads-dir flag and engine args",
			subcommand: "wads",
			rawArgs:    []string{"doom", "wads", "--wads-dir", "/tmp/wads", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "root command with preset and engine args",
			subcommand: "",
			rawArgs:    []string{"doom", "--dry-run", "-e", "uzdoom", "Eviternity II", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "bare doom with engine args",
			subcommand: "",
			rawArgs:    []string{"doom", "-fast"},
			expected:   []string{"-fast"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractEngineArgs(tt.subcommand, tt.rawArgs)
			if len(result) == 0 && len(tt.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("extractEngineArgs(%q, %v) = %v, expected %v",
					tt.subcommand, tt.rawArgs, result, tt.expected)
			}
		})
	}
}

func TestConfigThemesAndThemeValidation(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	if err := runConfigThemes(nil, nil); err != nil {
		t.Fatalf("runConfigThemes() returned unexpected error: %v", err)
	}

	// 1. Set valid built-in theme via doom config set theme <name>
	if err := runConfigSet(nil, []string{"theme", "blood"}); err != nil {
		t.Fatalf("runConfigSet(theme, blood) error = %v", err)
	}

	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != "blood" {
		t.Errorf("cfg.Theme = %q, expected 'blood'", cfg.Theme)
	}

	// 2. Setting unknown theme fails with helpful hint
	if err := runConfigSet(nil, []string{"theme", "nonexistent-theme"}); err == nil {
		t.Errorf("runConfigSet(theme, nonexistent-theme) expected error, got nil")
	}

	// 3. Setting custom theme file in themes directory succeeds
	customThemeDir := paths.ThemesDir
	if err := os.MkdirAll(customThemeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(themes) error = %v", err)
	}
	customThemeFile := filepath.Join(customThemeDir, "custom.json")
	customContent := `{"name": "custom", "description": "Custom theme"}`
	if err := os.WriteFile(customThemeFile, []byte(customContent), 0o600); err != nil {
		t.Fatalf("WriteFile(custom.json) error = %v", err)
	}

	if err := runConfigSet(nil, []string{"theme", "custom"}); err != nil {
		t.Fatalf("runConfigSet(theme, custom) error = %v", err)
	}

	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != "custom" {
		t.Errorf("cfg.Theme = %q, expected 'custom'", cfg.Theme)
	}
}

func TestParseBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bool
		wantErr  bool
	}{
		{input: "true", expected: true, wantErr: false},
		{input: "t", expected: true, wantErr: false},
		{input: "1", expected: true, wantErr: false},
		{input: "on", expected: true, wantErr: false},
		{input: "yes", expected: true, wantErr: false},
		{input: "y", expected: true, wantErr: false},
		{input: "enable", expected: true, wantErr: false},
		{input: "enabled", expected: true, wantErr: false},
		{input: "false", expected: false, wantErr: false},
		{input: "f", expected: false, wantErr: false},
		{input: "0", expected: false, wantErr: false},
		{input: "off", expected: false, wantErr: false},
		{input: "no", expected: false, wantErr: false},
		{input: "n", expected: false, wantErr: false},
		{input: "disable", expected: false, wantErr: false},
		{input: "disabled", expected: false, wantErr: false},
		{input: "maybe", expected: false, wantErr: true},
		{input: "2", expected: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := parseBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBool(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parseBool(%q) = %v, expected %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRunConfigCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	paths := getPaths()

	// 1. Naked `doom config` in non-TTY falls back to runConfigShow
	var showOut bytes.Buffer
	cfgCmd := newConfigCmd()
	cfgCmd.SetOut(&showOut)
	if err := cfgCmd.Execute(); err != nil {
		t.Fatalf("naked config command error = %v", err)
	}
	if !strings.Contains(showOut.String(), "=== Doom CLI Configuration ===") {
		t.Errorf("expected naked config in non-TTY to print configuration, got:\n%s", showOut.String())
	}

	var getOut bytes.Buffer
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&getOut)
	cfgCmd.SetArgs([]string{"get", "theme"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatalf("config get theme error = %v", err)
	}
	if strings.TrimSpace(getOut.String()) != "classic" {
		t.Errorf("config get theme with empty config = %q, want %q", strings.TrimSpace(getOut.String()), "classic")
	}

	// 2. Set nerd-fonts on
	if err := runConfigSet(nil, []string{"nerd-fonts", "on"}); err != nil {
		t.Fatalf("runConfigSet(nerd-fonts, on) error = %v", err)
	}
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !cfg.NerdFonts {
		t.Errorf("expected NerdFonts true, got false")
	}

	// 3. Set nerd-fonts off
	if err := runConfigSet(nil, []string{"nerd-fonts", "off"}); err != nil {
		t.Fatalf("runConfigSet(nerd-fonts, off) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.NerdFonts {
		t.Errorf("expected NerdFonts false, got true")
	}

	// 4. Toggle nerd-fonts
	if err := runConfigToggle(nil, []string{"nerd-fonts"}); err != nil {
		t.Fatalf("runConfigToggle(nerd-fonts) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !cfg.NerdFonts {
		t.Errorf("expected NerdFonts true after toggle, got false")
	}

	// 5. Set theme via config set
	if err := runConfigSet(nil, []string{"theme", "blood"}); err != nil {
		t.Fatalf("runConfigSet(theme, blood) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != "blood" {
		t.Errorf("expected Theme blood, got %q", cfg.Theme)
	}

	// 5b. Set wads-dirs via config set
	if err := runConfigSet(nil, []string{"wads-dirs", "/tmp/w1,/tmp/w2"}); err != nil {
		t.Fatalf("runConfigSet(wads-dirs) error = %v", err)
	}
	cfg, err = config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !reflect.DeepEqual(cfg.WadsDirs, []string{"/tmp/w1", "/tmp/w2"}) {
		t.Errorf("expected WadsDirs [/tmp/w1 /tmp/w2], got %v", cfg.WadsDirs)
	}

	// 6. Get settings
	if err := runConfigGet(nil, []string{"theme"}); err != nil {
		t.Errorf("runConfigGet(theme) error = %v", err)
	}
	if err := runConfigGet(nil, []string{"nerd-fonts"}); err != nil {
		t.Errorf("runConfigGet(nerd-fonts) error = %v", err)
	}
	if err := runConfigGet(nil, []string{"wads-dirs"}); err != nil {
		t.Errorf("runConfigGet(wads-dirs) error = %v", err)
	}
	if err := runConfigGet(nil, nil); err != nil {
		t.Errorf("runConfigGet() without args error = %v", err)
	}

	// 7. Error cases
	if err := runConfigSet(nil, []string{"invalid-key", "val"}); err == nil {
		t.Errorf("runConfigSet(invalid-key) expected error, got nil")
	}
	if err := runConfigSet(nil, []string{"nerd-fonts", "invalid-bool"}); err == nil {
		t.Errorf("runConfigSet(nerd-fonts, invalid-bool) expected error, got nil")
	}
	if err := runConfigToggle(nil, []string{"theme"}); err == nil {
		t.Errorf("runConfigToggle(theme) expected error for non-bool key, got nil")
	}
	if err := runConfigGet(nil, []string{"invalid-key"}); err == nil {
		t.Errorf("runConfigGet(invalid-key) expected error, got nil")
	}
}

func TestGetPathsMalformedConfigPreservesFlags(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	p0 := config.GetPaths()
	if err := os.MkdirAll(p0.ConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p0.ConfigFile, []byte(`{invalid json`), 0o600); err != nil {
		t.Fatal(err)
	}

	customWads := filepath.Join(tempDir, "custom-wads")
	flagWadsDir = customWads
	defer func() { flagWadsDir = "" }()

	got := getPaths()
	if got.WadsDir != customWads {
		t.Errorf("getPaths().WadsDir = %q, want %q even on malformed config", got.WadsDir, customWads)
	}
}

func TestCanonicalConfigKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "theme exact", input: "theme", expected: "theme", wantErr: false},
		{name: "theme with spaces", input: "  THEME  ", expected: "theme", wantErr: false},
		{name: "nerd-fonts standard", input: "nerd-fonts", expected: "nerd-fonts", wantErr: false},
		{name: "nerd_fonts underscore", input: "nerd_fonts", expected: "nerd-fonts", wantErr: false},
		{name: "wads-dir dash", input: "wads-dir", expected: "wads-dir", wantErr: false},
		{name: "wads_dir underscore", input: "wads_dir", expected: "wads-dir", wantErr: false},
		{name: "wads-dirs dash", input: "wads-dirs", expected: "wads-dirs", wantErr: false},
		{name: "wads_dirs underscore", input: "wads_dirs", expected: "wads-dirs", wantErr: false},
		{name: "bin-dir dash", input: "bin-dir", expected: "bin-dir", wantErr: false},
		{name: "bin_dir underscore", input: "bin_dir", expected: "bin-dir", wantErr: false},
		{name: "soundfonts-dir dash", input: "soundfonts-dir", expected: "soundfonts-dir", wantErr: false},
		{name: "soundfonts_dir underscore", input: "soundfonts_dir", expected: "soundfonts-dir", wantErr: false},
		{name: "unknown key", input: "unknown", expected: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalConfigKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("canonicalConfigKey(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if got != tt.expected {
				t.Errorf("canonicalConfigKey(%q) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNewRootCmd(t *testing.T) {
	cmd := newRootCmd()
	if cmd == nil {
		t.Fatal("expected non-nil root command")
	}

	expectedSubcommands := []string{
		"wads", "engines", "config", "setup",
	}

	foundSubcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		foundSubcommands[sub.Name()] = true
	}

	for _, expected := range expectedSubcommands {
		if !foundSubcommands[expected] {
			t.Errorf("expected subcommand %q not found in root command", expected)
		}
	}

	removedSubcommands := []string{
		"play", "launch", "presets", "themes", "soundfont",
	}
	for _, removed := range removedSubcommands {
		if foundSubcommands[removed] {
			t.Errorf("expected legacy subcommand %q to be removed from root command", removed)
		}
	}

	// Verify persistent flags
	for _, flag := range []string{"wads-dir", "theme", "nerd-fonts"} {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected persistent flag %q on root command", flag)
		}
	}

	// Verify local root flags (-e/--engine and --dry-run)
	engFlag := cmd.Flags().Lookup("engine")
	if engFlag == nil || engFlag.Shorthand != "e" {
		t.Errorf("expected rootCmd to have local --engine (-e) flag, got %+v", engFlag)
	}
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Error("expected rootCmd to have local --dry-run flag")
	}

	for _, removed := range []string{
		"wads-dirs", "bin-dir", "soundfonts-dir", "presets-file", "once", "force",
	} {
		if cmd.PersistentFlags().Lookup(removed) != nil || cmd.Flags().Lookup(removed) != nil {
			t.Errorf("expected flag %q to be absent from root command", removed)
		}
	}
	if !cmd.FParseErrWhitelist.UnknownFlags {
		t.Error("expected rootCmd.FParseErrWhitelist.UnknownFlags to be true")
	}

	// Verify bare `doom -skill 4` routes to rootCmd and passes argument validation
	targetCmd, flags, findErr := cmd.Find([]string{"-skill", "4"})
	if findErr != nil {
		t.Fatalf("cmd.Find([-skill 4]) unexpected error: %v", findErr)
	}
	if targetCmd != cmd {
		t.Fatalf("expected rootCmd to be matched for bare engine flags, got %q", targetCmd.Name())
	}
	if err := targetCmd.ParseFlags(flags); err != nil {
		t.Fatalf("targetCmd.ParseFlags([-skill 4]) unexpected error: %v", err)
	}
	if err := targetCmd.ValidateArgs(targetCmd.Flags().Args()); err != nil {
		t.Fatalf("targetCmd.ValidateArgs([4]) unexpected error: %v", err)
	}
}

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
	t.Setenv("DOOM_BIN_DIR", binDir)
	t.Setenv("DOOM_WADS_DIR", wadsDir)
	t.Setenv("DOOM_SF_DIR", sfDir)
	t.Setenv("PATH", binDir)

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

	// 2. engines add
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{
		"add", "woof",
		"--binary", "woof",
		"--style", "boom",
		"--prefix-args", "flatpak run org.doom.Woof",
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
		!reflect.DeepEqual(eng.PrefixArgs, []string{"flatpak", "run", "org.doom.Woof"}) {
		t.Fatalf("expected woof in cat.Engines with prefix_args, got %+v", cat.Engines)
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
	enginesCmd.SetArgs([]string{"remove", "woof"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines remove woof failed: %v", err)
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

	// 6. engines install (with pre-created binaries and SoundFont) deploys configs and verifies SoundFont
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
	if !strings.Contains(instOut.String(), "SoundFont already installed") {
		t.Errorf("expected engines install to also ensure SoundFont installation, got:\n%s", instOut.String())
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

func TestWadsCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", tempDir)
	t.Setenv("DOOM_WADS_DIR", tempDir)

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

	// 3. wads add custom with a custom engine that has DisplayName
	enginesCmd := newEnginesCmd()
	enginesCmd.SetArgs([]string{
		"add", "Woof!", "--id", "woof", "--binary", "woof", "--style", "boom",
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

	// 4. wads show custom
	var showOut bytes.Buffer
	wadsCmd = newWadsCmd()
	wadsCmd.SetOut(&showOut)
	wadsCmd.SetArgs([]string{"show", "CustomMap"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads show CustomMap failed: %v", err)
	}
	if !strings.Contains(showOut.String(), "Engine:        Woof!") {
		t.Errorf("expected wads show CustomMap to display 'Engine:        Woof!', got:\n%s", showOut.String())
	}

	// 5. wads set across multiple calls preserves prior fields
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

	// 7. wads remove custom
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"remove", "CustomMap"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads remove failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	if cat.Find("custommap") != nil {
		t.Fatalf("expected CustomMap removed")
	}

	// 8. wads lint on repo root
	wadsCmd = newWadsCmd()
	wadsCmd.SetArgs([]string{"lint", filepath.Join("..", "..")})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads lint failed: %v", err)
	}

	// Verify lint fails on invalid user entity
	paths := getPaths()
	if err := os.MkdirAll(paths.CatalogDir, 0o755); err != nil {
		t.Fatal(err)
	}
	badFile := filepath.Join(paths.CatalogDir, "preset.bad.json")
	if err := os.WriteFile(badFile, []byte(`{"kind":"preset","id":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	diags, lintErr := preset.LintCatalog(filepath.Join("..", ".."), paths.CatalogDir)
	if lintErr != nil {
		t.Fatalf("LintCatalog error: %v", lintErr)
	}
	if !preset.HasErrors(diags) {
		t.Errorf("expected LintCatalog to report errors for invalid user entity")
	}
}

func TestRootCmd_Version(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd()
	if cmd.Version == "" {
		t.Fatalf("expected rootCmd.Version to be non-empty")
	}
	if !strings.Contains(cmd.Version, "commit:") || !strings.Contains(cmd.Version, "built:") {
		t.Errorf("expected rootCmd.Version to contain commit and built metadata, got %q", cmd.Version)
	}
}

func TestConfigFourTierHierarchyAndCustomThemesList(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	// Set config.json values including explicit nerd-fonts = off
	if err := runConfigSet(nil, []string{"wads-dir", "/cfg/wads"}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigSet(nil, []string{"theme", "amber"}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigSet(nil, []string{"nerd-fonts", "off"}); err != nil {
		t.Fatal(err)
	}

	// Verify explicit nerd_fonts: false in config.json reports (config) source when env/flag are unset
	var out bytes.Buffer
	cfgCmd := newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"show"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Nerd Fonts:     disabled (universal rectangular badges) (config)") {
		t.Errorf("expected explicit nerd_fonts=false to report (config), got:\n%s", out.String())
	}

	// Override via env vars
	t.Setenv("DOOM_WADS_DIR", "/env/wads")
	t.Setenv("DOOM_WADS_DIRS", "/env/extra1,/env/extra2")
	t.Setenv("DOOM_THEME", "plasma")
	t.Setenv("DOOM_NERD_FONTS", "true")

	out.Reset()
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"get", "wads-dir"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "/env/wads" {
		t.Errorf("config get wads-dir with env override = %q, want /env/wads", got)
	}

	out.Reset()
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"get", "nerd-fonts"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "true" {
		t.Errorf("config get nerd-fonts with DOOM_NERD_FONTS=true = %q, want true", got)
	}

	// Override via flag
	flagWadsDir = "/flag/wads"
	defer func() {
		flagWadsDir = ""
	}()

	out.Reset()
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"show"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	showStr := out.String()
	for _, want := range []string{
		"Theme:          plasma (env)",
		"Nerd Fonts:     enabled (Powerlevel10k rounded capsules) (env)",
		"WADs Dir:       /flag/wads (flag)",
		"WADs Dirs:      /env/extra1, /env/extra2 (env)",
	} {
		if !strings.Contains(showStr, want) {
			t.Errorf("expected config show to contain %q, got:\n%s", want, showStr)
		}
	}

	// Custom theme in doom config themes
	paths := getPaths()
	if err := os.MkdirAll(paths.ThemesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(paths.ThemesDir, "neon.json"),
		[]byte(`{"name":"neon","description":"Custom neon"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	cfgCmd = newConfigCmd()
	cfgCmd.SetOut(&out)
	cfgCmd.SetArgs([]string{"themes"})
	if err := cfgCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "neon") {
		t.Errorf("expected config themes to include custom theme 'neon', got:\n%s", out.String())
	}
}

func TestSetupCommandPartialFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", filepath.Join(tempDir, "bin"))
	t.Setenv("DOOM_WADS_DIR", filepath.Join(tempDir, "wads"))

	sfConflictFile := filepath.Join(tempDir, "sf-conflict-file")
	if err := os.WriteFile(sfConflictFile, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOOM_SF_DIR", sfConflictFile)

	binDir := filepath.Join(tempDir, "bin")
	wadsDir := filepath.Join(tempDir, "wads")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, binName := range []string{"uzdoom", "dsda-doom", "uzdoom.exe", "dsda-doom.exe"} {
		if err := os.WriteFile(filepath.Join(binDir, binName), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cat, err := preset.LoadCuratedCatalog()
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

func TestSubcommandFlagScoping(t *testing.T) {
	// 1. root command has local --engine (-e) and --dry-run flags, and wads command does not
	rootCmd := newRootCmd()
	engFlag := rootCmd.Flags().Lookup("engine")
	if engFlag == nil || engFlag.Shorthand != "e" {
		t.Errorf("expected rootCmd to have --engine (-e), got %+v", engFlag)
	}
	if rootCmd.Flags().Lookup("dry-run") == nil {
		t.Error("expected rootCmd to have --dry-run flag")
	}

	wadsCmd := newWadsCmd()
	for _, f := range []string{"once", "dry-run", "engine"} {
		if wadsCmd.Flags().Lookup(f) != nil {
			t.Errorf("expected wadsCmd not to have local flag %q", f)
		}
	}

	// 2. engines add and engines set flags
	enginesCmd := newEnginesCmd()
	addEngCmd, _, err := enginesCmd.Find([]string{"add"})
	if err != nil {
		t.Fatalf("enginesCmd.Find(add): %v", err)
	}
	for _, want := range []string{"id", "binary", "style", "args", "prefix-args", "badge", "description"} {
		if addEngCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected engines add to have flag %q", want)
		}
	}

	setEngCmd, _, err := enginesCmd.Find([]string{"set"})
	if err != nil {
		t.Fatalf("enginesCmd.Find(set): %v", err)
	}
	for _, want := range []string{"binary", "style", "args", "prefix-args", "badge", "display-name"} {
		if setEngCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected engines set to have flag %q", want)
		}
	}
	if setEngCmd.Flags().Lookup("reset") != nil {
		t.Error("expected engines set not to have --reset flag (reset is a separate subcommand)")
	}

	// 3. wads add and wads set flags
	addWadCmd, _, err := wadsCmd.Find([]string{"add"})
	if err != nil {
		t.Fatalf("wadsCmd.Find(add): %v", err)
	}
	for _, want := range []string{"id", "engine", "iwad", "files", "args", "description"} {
		if addWadCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected wads add to have flag %q", want)
		}
	}
	if f := addWadCmd.Flags().Lookup("engine"); f == nil || f.Shorthand != "e" {
		t.Errorf("expected wads add --engine to have shorthand 'e', got %+v", f)
	}

	setWadCmd, _, err := wadsCmd.Find([]string{"set"})
	if err != nil {
		t.Fatalf("wadsCmd.Find(set): %v", err)
	}
	if f := setWadCmd.Flags().Lookup("engine"); f == nil || f.Shorthand != "e" {
		t.Errorf("expected wads set --engine to have shorthand 'e', got %+v", f)
	}
	if setWadCmd.Flags().Lookup("reset") != nil {
		t.Error("expected wads set not to have --reset flag (reset is a separate subcommand)")
	}

	// 4. --force is scoped locally to setup, wads fetch, wads scan, and engines install
	setupCmd := newSetupCmd()
	if setupCmd.Flags().Lookup("force") == nil {
		t.Error("expected setupCmd to have local --force flag")
	}
	for _, sub := range []string{"fetch", "scan"} {
		subCmd, _, findErr := wadsCmd.Find([]string{sub})
		if findErr != nil {
			t.Fatalf("wadsCmd.Find(%s): %v", sub, findErr)
		}
		if subCmd.Flags().Lookup("force") == nil {
			t.Errorf("expected wads %s to have local --force flag", sub)
		}
	}
	instEngCmd, _, err := enginesCmd.Find([]string{"install"})
	if err != nil {
		t.Fatalf("enginesCmd.Find(install): %v", err)
	}
	if instEngCmd.Flags().Lookup("force") == nil {
		t.Error("expected engines install to have local --force flag")
	}
	if rootCmd.Flags().Lookup("force") != nil || wadsCmd.Flags().Lookup("force") != nil {
		t.Error("expected rootCmd and wadsCmd not to have --force flag")
	}
}

func TestRootDirectLauncherDryRunAndEngineOverride(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", tempDir)
	t.Setenv("DOOM_WADS_DIR", tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "DOOM.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "TNT.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, binName := range []string{"uzdoom", "uzdoom.exe", "dsda-doom", "dsda-doom.exe"} {
		if err := os.WriteFile(filepath.Join(tempDir, binName), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Naked `doom --dry-run` launches The Ultimate Doom (DOOM.WAD) in dsda-doom
	var out bytes.Buffer
	rootCmd := newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom --dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "dsda-doom") || !strings.Contains(out.String(), "DOOM.WAD") {
		t.Errorf("expected naked doom --dry-run to launch DOOM.WAD in dsda-doom, got:\n%s", out.String())
	}

	// 2. `doom --dry-run -e uzdoom "Doom II" -skill 4` launches DOOM2.WAD in uzdoom with -skill 4
	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"--dry-run", "-e", "uzdoom", "Doom II", "-skill", "4"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom --dry-run -e uzdoom 'Doom II' -skill 4 failed: %v", err)
	}
	if !strings.Contains(out.String(), "uzdoom") ||
		!strings.Contains(out.String(), "DOOM2.WAD") ||
		!strings.Contains(out.String(), "-skill 4") {
		t.Errorf("expected dry-run output to contain uzdoom, DOOM2.WAD, and -skill 4, got:\n%s", out.String())
	}

	// 3. `doom -skill 4 -warp 1 1 --dry-run` launches DOOM.WAD with forwarded engine flags and without --dry-run leaked
	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"-skill", "4", "-warp", "1", "1", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom -skill 4 -warp 1 1 --dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "DOOM.WAD") ||
		!strings.Contains(out.String(), "-skill 4 -warp 1 1") ||
		strings.Contains(out.String(), "--dry-run") {
		t.Errorf(
			"expected naked doom with engine flags to launch DOOM.WAD with -skill 4 -warp 1 1 (no --dry-run), got:\n%s",
			out.String(),
		)
	}

	// 4. Single-dash Doom engine flags containing 'e', 'h', or 'v', or ending in 'e'
	// (-nomonsters, -complevel 21, -shorttics, -viddump out.mkv, -episode 2, -respawn, -deh foo.deh, -nomouse)
	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{
		"--dry-run",
		"-nomonsters",
		"-complevel", "21",
		"-shorttics",
		"-viddump", "out.mkv",
		"-episode", "2",
		"-respawn",
		"-deh", "foo.deh",
		"-nomouse",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom with single-dash e/h/v flags failed: %v", err)
	}
	wantEngineFlags := "-nomonsters -complevel 21 -shorttics -viddump out.mkv -episode 2 -respawn -deh foo.deh -nomouse"
	if !strings.Contains(out.String(), wantEngineFlags) {
		t.Errorf("expected dry-run output to contain %q, got:\n%s", wantEngineFlags, out.String())
	}

	// 5. Preset resolution via findPreset (substring and Roman numeral / numeric matching)
	for _, query := range []string{"doom2", "doom-ii", "evilution"} {
		out.Reset()
		rootCmd = newRootCmd()
		rootCmd.SetOut(&out)
		rootCmd.SetArgs([]string{query, "--dry-run"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("doom %s --dry-run failed: %v", query, err)
		}
	}
}

func TestRunWadsTUISingleShot(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", tempDir)
	t.Setenv("DOOM_WADS_DIR", tempDir)
	t.Setenv("PATH", tempDir)

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
