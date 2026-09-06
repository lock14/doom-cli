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
