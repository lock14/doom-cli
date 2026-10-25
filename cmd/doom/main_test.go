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

func TestExtractEngineArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		subcommand string
		rawArgs    []string
		expected   []string
	}{
		{
			name:       "play with dash-dash arguments",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--", "-nomonsters", "-fast"},
			expected:   []string{"-nomonsters", "-fast"},
		},
		{
			name:       "play with trailing dash-dash and no extra args",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--"},
			expected:   nil,
		},
		{
			name:       "play with --wads-dir flag",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--wads-dir", "/tmp/wads", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "launch skips preset name argument",
			subcommand: "launch",
			rawArgs:    []string{"doom", "launch", "Eviternity II", "-fast"},
			expected:   []string{"-fast"},
		},
		{
			name:       "launch with --engine, -e, and --dry-run flags",
			subcommand: "launch",
			rawArgs:    []string{"doom", "launch", "--dry-run", "-e", "uzdoom", "Eviternity II", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "launch with global --wads-dir value named launch before subcommand",
			subcommand: "launch",
			rawArgs:    []string{"doom", "--wads-dir", "launch", "launch", "Eviternity II", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "bare doom without extra args",
			subcommand: "play",
			rawArgs:    []string{"doom"},
			expected:   nil,
		},
		{
			name:       "play with --theme flag space separated",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--theme", "plasma", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "play with --theme= flag syntax",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--theme=toxic", "-fast"},
			expected:   []string{"-fast"},
		},
		{
			name:       "play with --nerd-fonts flag",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--nerd-fonts", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "play with --nerd-fonts=true flag syntax",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--nerd-fonts=true", "-fast"},
			expected:   []string{"-fast"},
		},
		{
			name:       "play with --wads-dir= flag syntax",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--wads-dir=/tmp/wads", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "bare doom with engine args",
			subcommand: "play",
			rawArgs:    []string{"doom", "-fast"},
			expected:   []string{"-fast"},
		},
		{
			name:       "bare doom with engine arg value named play",
			subcommand: "play",
			rawArgs:    []string{"doom", "-record", "play"},
			expected:   []string{"-record", "play"},
		},
		{
			name:       "bare doom with --wads-dir value named play",
			subcommand: "play",
			rawArgs:    []string{"doom", "--wads-dir", "play", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "bare doom with known flags and engine args",
			subcommand: "play",
			rawArgs:    []string{"doom", "--theme", "blood", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "bare doom with dash-dash arguments",
			subcommand: "play",
			rawArgs:    []string{"doom", "--theme", "blood", "--", "-nomonsters", "-fast"},
			expected:   []string{"-nomonsters", "-fast"},
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

func TestRunThemesList(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	if err := runThemesList(nil, nil); err != nil {
		t.Fatalf("runThemesList() returned unexpected error: %v", err)
	}
}

func TestRunThemesSet(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	// Test setting valid builtin theme
	if err := runThemesSet(nil, []string{"blood"}); err != nil {
		t.Fatalf("runThemesSet(blood) error = %v", err)
	}

	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Theme != "blood" {
		t.Errorf("cfg.Theme = %q, expected 'blood'", cfg.Theme)
	}

	// Test setting unknown theme
	if err := runThemesSet(nil, []string{"nonexistent-theme"}); err == nil {
		t.Errorf("runThemesSet(nonexistent-theme) expected error, got nil")
	}

	// Test setting custom theme file in themes directory
	customThemeDir := paths.ThemesDir
	if err := os.MkdirAll(customThemeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(themes) error = %v", err)
	}
	customThemeFile := filepath.Join(customThemeDir, "custom.json")
	customContent := `{"name": "custom", "description": "Custom theme"}`
	if err := os.WriteFile(customThemeFile, []byte(customContent), 0o600); err != nil {
		t.Fatalf("WriteFile(custom.json) error = %v", err)
	}

	if err := runThemesSet(nil, []string{"custom"}); err != nil {
		t.Fatalf("runThemesSet(custom) error = %v", err)
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

	// 1. Show with empty config and verify default theme via config get
	if err := runConfigShow(nil, nil); err != nil {
		t.Fatalf("runConfigShow() error = %v", err)
	}
	var getOut bytes.Buffer
	cfgCmd := newConfigCmd()
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
		"play", "launch", "setup", "wads", "engines",
		"soundfont", "config", "presets", "themes",
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

	// Verify persistent flags
	for _, flag := range []string{"wads-dir", "theme", "nerd-fonts"} {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected persistent flag %q on root command", flag)
		}
	}
	for _, removed := range []string{
		"engine", "wads-dirs", "bin-dir", "soundfonts-dir", "dry-run", "presets-file", "once", "force",
	} {
		if cmd.PersistentFlags().Lookup(removed) != nil || cmd.Flags().Lookup(removed) != nil {
			t.Errorf("expected flag %q to be removed from root command", removed)
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
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", tempDir)
	t.Setenv("DOOM_WADS_DIR", tempDir)
	t.Setenv("PATH", tempDir)

	enginesCmd := newEnginesCmd()

	// 1. engines list
	enginesCmd.SetArgs([]string{"list"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines list failed: %v", err)
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

	// Verify added in catalog.d
	_, _, cat, err := loadAppState()
	if err != nil {
		t.Fatalf("loadAppState failed: %v", err)
	}
	if eng, ok := cat.Engines["woof"]; !ok || eng.ID != "woof" ||
		!reflect.DeepEqual(eng.PrefixArgs, []string{"flatpak", "run", "org.doom.Woof"}) {
		t.Fatalf("expected woof in cat.Engines with prefix_args, got %+v", cat.Engines)
	}

	// 2b. engines config on built-in dsda-doom across multiple calls, clearing fields, and reset
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{
		"config", "dsda-doom", "--binary", "/custom/dsda-doom", "--badge", "DSDA+", "--args", "-fast -nomonsters",
	})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines config dsda-doom failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	eng, ok := cat.FindEngine("dsda-doom")
	if !ok || eng.Binary != "/custom/dsda-doom" || eng.Badge != "DSDA+" ||
		!reflect.DeepEqual(eng.DefaultArgs, []string{"-fast", "-nomonsters"}) {
		t.Fatalf("expected overridden dsda-doom with args, got %+v", eng)
	}

	// Clear badge and args back to empty via CLI flags
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"config", "dsda-doom", "--badge", "", "--args", ""})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines config clear flags failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	eng, ok = cat.FindEngine("dsda-doom")
	if !ok || eng.Badge != "" || len(eng.DefaultArgs) != 0 {
		t.Fatalf("expected cleared Badge and DefaultArgs on dsda-doom, got %+v", eng)
	}

	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"config", "dsda-doom", "--reset"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines config dsda-doom --reset failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	if eng, ok := cat.FindEngine("dsda-doom"); !ok || eng.Binary != "dsda-doom" {
		t.Fatalf("expected reset dsda-doom binary, got %+v", eng)
	}

	// 3. engines remove built-in (fails)
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"remove", "uzdoom"})
	if err := enginesCmd.Execute(); err == nil {
		t.Errorf("expected error removing built-in uzdoom, got nil")
	}

	// 4. engines remove custom (succeeds)
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"remove", "woof"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines remove woof failed: %v", err)
	}

	_, _, cat, _ = loadAppState()
	if _, ok := cat.Engines["woof"]; ok {
		t.Errorf("expected woof removed from cat.Engines, got %+v", cat.Engines)
	}
}

func TestPresetsCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", tempDir)
	t.Setenv("DOOM_WADS_DIR", tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. presets list shows [✓ Ready] for Doom II and [✗ Missing] for unextracted megawads
	var listOut bytes.Buffer
	presetsCmd := newPresetsCmd()
	presetsCmd.SetOut(&listOut)
	presetsCmd.SetArgs([]string{"list"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets list failed: %v", err)
	}
	if !strings.Contains(listOut.String(), "[✓ Ready]") || !strings.Contains(listOut.String(), "[✗ Missing]") {
		t.Errorf("expected presets list to contain [✓ Ready] and [✗ Missing], got:\n%s", listOut.String())
	}

	// 2. presets show built-in
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"show", "Alien Vendetta"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets show Alien Vendetta failed: %v", err)
	}

	// 3. presets add custom with a custom engine that has DisplayName but no Description
	enginesCmd := newEnginesCmd()
	enginesCmd.SetArgs([]string{
		"add", "Woof!", "--id", "woof", "--binary", "woof", "--style", "boom",
	})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines add woof failed: %v", err)
	}

	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{
		"add", "CustomMap",
		"-e", "woof",
		"--iwad", "DOOM2.WAD",
		"--files", "custom.wad",
		"--description", "My Custom Map",
	})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets add failed: %v", err)
	}

	// 4. presets show custom
	var showOut bytes.Buffer
	presetsCmd = newPresetsCmd()
	presetsCmd.SetOut(&showOut)
	presetsCmd.SetArgs([]string{"show", "CustomMap"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets show CustomMap failed: %v", err)
	}
	if !strings.Contains(showOut.String(), "Engine:        Woof!") {
		t.Errorf("expected presets show CustomMap to display 'Engine:        Woof!', got:\n%s", showOut.String())
	}

	// 5. presets config across multiple calls preserves prior fields
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"config", "Sunlust", "-e", "uzdoom"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets config -e failed: %v", err)
	}
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"config", "Sunlust", "--args", "-skill 4"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets config --args failed: %v", err)
	}

	_, _, cat, err := loadAppState()
	if err != nil {
		t.Fatalf("loadAppState failed: %v", err)
	}
	sunlust := cat.Find("sunlust")
	if sunlust == nil || sunlust.Engine != "uzdoom" || !reflect.DeepEqual(sunlust.Args, []string{"-skill", "4"}) {
		t.Fatalf("expected Sunlust override in catalog, got %+v", sunlust)
	}

	// 6. presets config reset
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"config", "Sunlust", "--reset"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets config --reset failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	sunlust = cat.Find("sunlust")
	if sunlust == nil || sunlust.Engine != "dsda-doom" || len(sunlust.Args) != 0 {
		t.Fatalf("expected Sunlust launch options cleared, got %+v", sunlust)
	}

	// 7. presets remove custom
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"remove", "CustomMap"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets remove failed: %v", err)
	}
	_, _, cat, _ = loadAppState()
	if cat.Find("custommap") != nil {
		t.Fatalf("expected CustomMap removed")
	}

	// 8. presets lint on repo root
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"lint", filepath.Join("..", "..")})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets lint failed: %v", err)
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

func TestWadsCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_WADS_DIR", tempDir)

	wadsCmd := newWadsCmd()
	subCommands := wadsCmd.Commands()

	tests := []struct {
		name        string
		commandName string
	}{
		{
			name:        "fetch subcommand exists",
			commandName: "fetch",
		},
		{
			name:        "list subcommand exists",
			commandName: "list",
		},
		{
			name:        "extract subcommand exists",
			commandName: "extract",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found bool
			for _, cmd := range subCommands {
				if cmd.Name() == tt.commandName {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("subcommand %q not found in wads command", tt.commandName)
			}
		})
	}

	// Create sunlust.wad on disk and verify `wads list` reports [✓ Found: ...] and [✗ Missing]
	sunlustPath := filepath.Join(tempDir, "sunlust.wad")
	if err := os.WriteFile(sunlustPath, []byte("PWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	wadsCmd = newWadsCmd()
	wadsCmd.SetOut(&out)
	wadsCmd.SetArgs([]string{"list"})
	if err := wadsCmd.Execute(); err != nil {
		t.Fatalf("wads list failed: %v", err)
	}
	if !strings.Contains(out.String(), "sunlust.wad [✓ Found: "+sunlustPath+"]") ||
		!strings.Contains(out.String(), "[✗ Missing]") {
		t.Errorf("unexpected wads list output:\n%s", out.String())
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

	// Custom theme in themes list
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
	themesCmd := newThemesCmd()
	themesCmd.SetOut(&out)
	themesCmd.SetArgs([]string{"list"})
	if err := themesCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "neon") {
		t.Errorf("expected themes list to include custom theme 'neon', got:\n%s", out.String())
	}
}

func TestSetupCommandPartialFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", filepath.Join(tempDir, "bin"))
	t.Setenv("DOOM_WADS_DIR", filepath.Join(tempDir, "wads"))

	// Create a regular file at SoundFontsDir so Step 3 (InstallSoundFont) fails deterministically without network IO,
	// and pre-create engine binaries and WAD files so Steps 2 and 5 succeed via idempotency checks.
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
	t.Parallel()

	// 1. play command has no local --once, --dry-run, or --engine flags
	playCmd := newPlayCmd()
	for _, f := range []string{"once", "dry-run", "engine"} {
		if playCmd.Flags().Lookup(f) != nil {
			t.Errorf("expected playCmd not to have local flag %q", f)
		}
	}

	// 2. launch command has local --engine (-e) and --dry-run flags
	launchCmd := newLaunchCmd()
	engFlag := launchCmd.Flags().Lookup("engine")
	if engFlag == nil || engFlag.Shorthand != "e" {
		t.Errorf("expected launchCmd to have --engine (-e), got %+v", engFlag)
	}
	if launchCmd.Flags().Lookup("dry-run") == nil {
		t.Error("expected launchCmd to have --dry-run flag")
	}

	// 3. engines add and engines config flags
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
	for _, removed := range []string{"family", "display-name"} {
		if addEngCmd.Flags().Lookup(removed) != nil {
			t.Errorf("expected engines add not to have removed flag %q", removed)
		}
	}

	cfgEngCmd, _, err := enginesCmd.Find([]string{"config"})
	if err != nil {
		t.Fatalf("enginesCmd.Find(config): %v", err)
	}
	for _, want := range []string{"binary", "style", "args", "prefix-args", "badge", "display-name", "reset"} {
		if cfgEngCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected engines config to have flag %q", want)
		}
	}
	for _, removed := range []string{"default-args", "append-arg"} {
		if cfgEngCmd.Flags().Lookup(removed) != nil {
			t.Errorf("expected engines config not to have removed flag %q", removed)
		}
	}

	// 4. presets add and presets config flags
	presetsCmd := newPresetsCmd()
	addPreCmd, _, err := presetsCmd.Find([]string{"add"})
	if err != nil {
		t.Fatalf("presetsCmd.Find(add): %v", err)
	}
	for _, want := range []string{"id", "engine", "iwad", "files", "args", "description"} {
		if addPreCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected presets add to have flag %q", want)
		}
	}
	if f := addPreCmd.Flags().Lookup("engine"); f == nil || f.Shorthand != "e" {
		t.Errorf("expected presets add --engine to have shorthand 'e', got %+v", f)
	}
	for _, removed := range []string{
		"source-url", "source-sha256", "optional-files", "category", "author", "release",
	} {
		if addPreCmd.Flags().Lookup(removed) != nil {
			t.Errorf("expected presets add not to have removed flag %q", removed)
		}
	}

	cfgPreCmd, _, err := presetsCmd.Find([]string{"config"})
	if err != nil {
		t.Fatalf("presetsCmd.Find(config): %v", err)
	}
	if f := cfgPreCmd.Flags().Lookup("engine"); f == nil || f.Shorthand != "e" {
		t.Errorf("expected presets config --engine to have shorthand 'e', got %+v", f)
	}

	// 5. --force is scoped locally to setup, wads fetch, wads extract, engines install, and soundfont install
	setupCmd := newSetupCmd()
	if setupCmd.Flags().Lookup("force") == nil {
		t.Error("expected setupCmd to have local --force flag")
	}
	wadsCmd := newWadsCmd()
	for _, sub := range []string{"fetch", "extract"} {
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
	sfCmd := newSoundfontCmd()
	instSfCmd, _, err := sfCmd.Find([]string{"install"})
	if err != nil {
		t.Fatalf("sfCmd.Find(install): %v", err)
	}
	if instSfCmd.Flags().Lookup("force") == nil {
		t.Error("expected soundfont install to have local --force flag")
	}
	if playCmd.Flags().Lookup("force") != nil || launchCmd.Flags().Lookup("force") != nil {
		t.Error("expected playCmd and launchCmd not to have --force flag")
	}
}

func TestLaunchCommandDryRunAndEngineOverride(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", tempDir)
	t.Setenv("DOOM_WADS_DIR", tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, binName := range []string{"uzdoom", "uzdoom.exe"} {
		if err := os.WriteFile(filepath.Join(tempDir, binName), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	launchCmd := newLaunchCmd()
	launchCmd.SetOut(&out)
	launchCmd.SetArgs([]string{"--dry-run", "-e", "uzdoom", "Doom II"})
	if err := launchCmd.Execute(); err != nil {
		t.Fatalf("launch --dry-run -e uzdoom failed: %v", err)
	}
	if !strings.Contains(out.String(), "uzdoom") || !strings.Contains(out.String(), "DOOM2.WAD") {
		t.Errorf("expected dry-run output to contain uzdoom and DOOM2.WAD, got:\n%s", out.String())
	}
}

func TestRunPlaySingleShot(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("DOOM_BIN_DIR", tempDir)
	t.Setenv("DOOM_WADS_DIR", tempDir)
	t.Setenv("PATH", tempDir)

	origArgs := os.Args
	os.Args = []string{"doom", "play"}
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
	if err := runPlay(newPlayCmd(), nil); err != nil {
		t.Fatalf("runPlay on quit returned unexpected error: %v", err)
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
	// Provide multiple selections on stdin to verify runPlay only consumes one and does not loop
	_, _ = fmt.Fprintf(wPlay, "%d\n\n%d\n", doom2Idx, doom2Idx)
	_ = wPlay.Close()
	os.Stdin = rPlay
	defer func() { _ = rPlay.Close() }()

	if err := runPlay(newPlayCmd(), nil); err != nil {
		t.Fatalf("runPlay single-shot execution failed: %v", err)
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
