package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/config"
)

func TestExtractEngineArgs(t *testing.T) {
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
			name:       "play with --once and --dry-run flags",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--once", "--dry-run", "-fast"},
			expected:   []string{"-fast"},
		},
		{
			name:       "play with known flags and values",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--engine", "dsda-doom", "--wads-dir", "/tmp/wads", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "launch skips preset name argument",
			subcommand: "launch",
			rawArgs:    []string{"doom", "launch", "Eviternity II", "-fast"},
			expected:   []string{"-fast"},
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
			rawArgs:    []string{"doom", "play", "--theme", "cyberpunk", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "play with --theme= flag syntax",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--theme=matrix", "-fast"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestWaitForEnter(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "newline immediately unblocks",
			input: "\n",
		},
		{
			name:  "text with newline",
			input: "hello\n",
		},
		{
			name:  "empty input / EOF unblocks without panic",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			waitForEnter(reader)
		})
	}
}

func TestRunThemesList(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	if err := runThemesList(nil, nil); err != nil {
		t.Fatalf("runThemesList() returned unexpected error: %v", err)
	}
}

func TestRunThemesSet(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

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
	customContent := `{"name": "custom", "type": "dark", "description": "Custom theme"}`
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
	paths := getPaths()

	// 1. Show with empty config
	if err := runConfigShow(nil, nil); err != nil {
		t.Fatalf("runConfigShow() error = %v", err)
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

	// 6. Get settings
	if err := runConfigGet(nil, []string{"theme"}); err != nil {
		t.Errorf("runConfigGet(theme) error = %v", err)
	}
	if err := runConfigGet(nil, []string{"nerd-fonts"}); err != nil {
		t.Errorf("runConfigGet(nerd-fonts) error = %v", err)
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
	for _, flag := range []string{"engine", "wads-dir", "bin-dir", "dry-run", "presets-file", "theme", "nerd-fonts"} {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected persistent flag %q on root command", flag)
		}
	}
}

func TestEnginesCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("BIN_DIR", tempDir)
	t.Setenv("WADS_DIR", tempDir)
	t.Setenv("PATH", tempDir)

	enginesCmd := newEnginesCmd()

	// 1. engines list
	enginesCmd.SetArgs([]string{"list"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines list failed: %v", err)
	}

	// 2. engines add
	enginesCmd.SetArgs([]string{"add", "woof", "--bin", "woof", "--args-style", "boom", "--desc", "Woof MBF21"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines add failed: %v", err)
	}

	// Verify added in config
	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if eng, ok := cfg.Engines["woof"]; !ok || eng.Name != "woof" {
		t.Fatalf("expected woof in cfg.Engines, got %+v", cfg.Engines)
	}

	// 3. engines remove built-in (fails)
	enginesCmd.SetArgs([]string{"remove", "uzdoom"})
	if err := enginesCmd.Execute(); err == nil {
		t.Errorf("expected error removing built-in uzdoom, got nil")
	}

	// 4. engines remove custom (succeeds)
	enginesCmd.SetArgs([]string{"remove", "woof"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines remove woof failed: %v", err)
	}

	cfg, _ = config.LoadConfig(paths)
	if _, ok := cfg.Engines["woof"]; ok {
		t.Errorf("expected woof removed from cfg.Engines, got %+v", cfg.Engines)
	}
}

func TestPresetsCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("BIN_DIR", tempDir)
	t.Setenv("WADS_DIR", tempDir)

	presetsCmd := newPresetsCmd()

	// 1. presets list
	presetsCmd.SetArgs([]string{"list"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets list failed: %v", err)
	}

	// 2. presets show built-in
	presetsCmd.SetArgs([]string{"show", "Alien Vendetta"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets show Alien Vendetta failed: %v", err)
	}

	// 3. presets add custom
	presetsCmd.SetArgs([]string{
		"add", "CustomMap",
		"--engine", "uzdoom",
		"--iwad", "DOOM2.WAD",
		"--files", "custom.wad",
		"--desc", "My Custom Map",
	})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets add failed: %v", err)
	}

	// 4. presets show custom
	presetsCmd.SetArgs([]string{"show", "CustomMap"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets show CustomMap failed: %v", err)
	}

	// 5. presets config
	presetsCmd.SetArgs([]string{
		"config", "Sunlust",
		"--engine", "uzdoom",
		"--args", "-skill 4",
	})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets config failed: %v", err)
	}

	paths := getPaths()
	cfg, _ := config.LoadConfig(paths)
	if opt, ok := cfg.LaunchOptions["Sunlust"]; !ok || opt.Engine != "uzdoom" || opt.AdditionalArgs != "-skill 4" {
		t.Fatalf("expected Sunlust launch options in config, got %+v", cfg.LaunchOptions)
	}

	// 6. presets config reset
	presetsCmd.SetArgs([]string{"config", "Sunlust", "--reset"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets config --reset failed: %v", err)
	}
	cfg, _ = config.LoadConfig(paths)
	if _, ok := cfg.LaunchOptions["Sunlust"]; ok {
		t.Fatalf("expected Sunlust launch options cleared, got %+v", cfg.LaunchOptions)
	}

	// 7. presets remove custom
	presetsCmd.SetArgs([]string{"remove", "CustomMap"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets remove failed: %v", err)
	}
	cfg, _ = config.LoadConfig(paths)
	if len(cfg.Presets) != 0 {
		t.Fatalf("expected 0 custom presets, got %d", len(cfg.Presets))
	}
}
