package main

import (
	"os"
	"path/filepath"
	"reflect"
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
		{
			name:       "play with --soundfonts-dir= flag syntax",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--soundfonts-dir=/tmp/sf", "-fast"},
			expected:   []string{"-fast"},
		},
		{
			name:       "play with --bin-dir= and --wads-dir= flag syntax",
			subcommand: "play",
			rawArgs:    []string{"doom", "play", "--bin-dir=/tmp/bin", "--wads-dir=/tmp/wads", "-skill", "4"},
			expected:   []string{"-skill", "4"},
		},
		{
			name:       "bare doom with engine args",
			subcommand: "play",
			rawArgs:    []string{"doom", "-fast"},
			expected:   []string{"-fast"},
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

func TestWaitForEnter(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			reader := strings.NewReader(tt.input)
			waitForEnter(reader)
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

	customBin := filepath.Join(tempDir, "custom-bin")
	flagBinDir = customBin
	defer func() { flagBinDir = "" }()

	got := getPaths()
	if got.BinDir != customBin {
		t.Errorf("getPaths().BinDir = %q, want %q even on malformed config", got.BinDir, customBin)
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
		{name: "nerdfonts squashed", input: "nerdfonts", expected: "nerd-fonts", wantErr: false},
		{name: "wads-dir dash", input: "wads-dir", expected: "wads-dir", wantErr: false},
		{name: "wads_dir underscore", input: "wads_dir", expected: "wads-dir", wantErr: false},
		{name: "wads-dirs dash", input: "wads-dirs", expected: "wads-dirs", wantErr: false},
		{name: "wads_dirs underscore", input: "wads_dirs", expected: "wads-dirs", wantErr: false},
		{name: "bin-dir dash", input: "bin-dir", expected: "bin-dir", wantErr: false},
		{name: "bin_dir underscore", input: "bin_dir", expected: "bin-dir", wantErr: false},
		{name: "soundfonts-dir dash", input: "soundfonts-dir", expected: "soundfonts-dir", wantErr: false},
		{name: "sf-dir short", input: "sf-dir", expected: "soundfonts-dir", wantErr: false},
		{name: "sf_dir short underscore", input: "sf_dir", expected: "soundfonts-dir", wantErr: false},
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
	for _, flag := range []string{"engine", "wads-dir", "bin-dir", "dry-run", "presets-file", "theme", "nerd-fonts"} {
		if cmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected persistent flag %q on root command", flag)
		}
	}
}

func TestEnginesCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	t.Setenv("BIN_DIR", tempDir)
	t.Setenv("WADS_DIR", tempDir)
	t.Setenv("PATH", tempDir)

	enginesCmd := newEnginesCmd()

	// 1. engines list
	enginesCmd.SetArgs([]string{"list"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines list failed: %v", err)
	}

	// 2. engines add (also tests --bin / --args-style / --desc flag normalization)
	enginesCmd = newEnginesCmd()
	enginesCmd.SetArgs([]string{"add", "woof", "--bin", "woof", "--args-style", "boom", "--desc", "Woof MBF21"})
	if err := enginesCmd.Execute(); err != nil {
		t.Fatalf("engines add failed: %v", err)
	}

	// Verify added in catalog.d
	_, _, cat, err := loadAppState()
	if err != nil {
		t.Fatalf("loadAppState failed: %v", err)
	}
	if eng, ok := cat.Engines["woof"]; !ok || eng.ID != "woof" {
		t.Fatalf("expected woof in cat.Engines, got %+v", cat.Engines)
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
	t.Setenv("BIN_DIR", tempDir)
	t.Setenv("WADS_DIR", tempDir)

	// 1. presets list
	presetsCmd := newPresetsCmd()
	presetsCmd.SetArgs([]string{"list"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets list failed: %v", err)
	}

	// 2. presets show built-in
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"show", "Alien Vendetta"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets show Alien Vendetta failed: %v", err)
	}

	// 3. presets add custom
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{
		"add", "CustomMap",
		"--engine", "uzdoom",
		"--iwad", "DOOM2.WAD",
		"--files", "custom.wad",
		"--description", "My Custom Map",
	})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets add failed: %v", err)
	}

	// 4. presets show custom
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"show", "CustomMap"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets show CustomMap failed: %v", err)
	}

	// 5. presets config across multiple calls preserves prior fields
	presetsCmd = newPresetsCmd()
	presetsCmd.SetArgs([]string{"config", "Sunlust", "--engine", "uzdoom"})
	if err := presetsCmd.Execute(); err != nil {
		t.Fatalf("presets config --engine failed: %v", err)
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
	wadsCmd := newWadsCmd()
	subCommands := wadsCmd.Commands()

	tests := []struct {
		name        string
		commandName string
		wantAliases []string
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
			name:        "extract-steam subcommand has extract alias",
			commandName: "extract-steam",
			wantAliases: []string{"extract"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found bool
			for _, cmd := range subCommands {
				if cmd.Name() == tt.commandName {
					found = true
					if len(tt.wantAliases) > 0 {
						if !reflect.DeepEqual(cmd.Aliases, tt.wantAliases) {
							t.Errorf("cmd %q aliases = %v, want %v",
								cmd.Name(), cmd.Aliases, tt.wantAliases)
						}
					}
					break
				}
			}
			if !found {
				t.Fatalf("subcommand %q not found in wads command", tt.commandName)
			}
		})
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
