package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetBuiltinTheme(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		found    bool
	}{
		{
			name:     "exact default",
			input:    "default",
			expected: "default",
			found:    true,
		},
		{
			name:     "alias doom",
			input:    "doom",
			expected: "default",
			found:    true,
		},
		{
			name:     "case-insensitive cyberpunk",
			input:    "CyberPunk",
			expected: "cyberpunk",
			found:    true,
		},
		{
			name:     "blood theme",
			input:    "blood",
			expected: "blood",
			found:    true,
		},
		{
			name:     "matrix theme",
			input:    "matrix",
			expected: "matrix",
			found:    true,
		},
		{
			name:     "monochrome theme",
			input:    "monochrome",
			expected: "monochrome",
			found:    true,
		},
		{
			name:     "unknown theme",
			input:    "nonexistent",
			expected: "",
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme, ok := GetBuiltinTheme(tt.input)
			if ok != tt.found {
				t.Fatalf("GetBuiltinTheme(%q) found = %v, expected %v", tt.input, ok, tt.found)
			}
			if ok && theme.Name != tt.expected {
				t.Errorf("GetBuiltinTheme(%q).Name = %q, expected %q", tt.input, theme.Name, tt.expected)
			}
		})
	}
}

func TestListBuiltinThemes(t *testing.T) {
	list := ListBuiltinThemes()
	if len(list) != 5 {
		t.Fatalf("expected 5 built-in themes, got %d", len(list))
	}
	expectedNames := []string{"default", "cyberpunk", "blood", "matrix", "monochrome"}
	for i, name := range expectedNames {
		if list[i].Name != name {
			t.Errorf("theme at index %d = %q, expected %q", i, list[i].Name, name)
		}
	}
}

func TestCompileStyles(t *testing.T) {
	styles := CompileStyles(CyberpunkTheme)
	rendered := styles.BrandBody.Render("TEST")
	if rendered == "" {
		t.Error("expected non-empty rendered string from compiled BrandBody style")
	}
}

func TestLoadThemeFile(t *testing.T) {
	tmpDir := t.TempDir()
	themeFile := filepath.Join(tmpDir, "custom.json")

	content := `{
		"name": "synthwave",
		"description": "Custom synthwave neon",
		"brand_cap": "#FF007F",
		"prompt": "#00F0FF"
	}`

	if err := os.WriteFile(themeFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write theme file: %v", err)
	}

	theme, err := LoadThemeFile(themeFile)
	if err != nil {
		t.Fatalf("failed to load custom theme file: %v", err)
	}

	if theme.Name != "synthwave" {
		t.Errorf("expected theme.Name 'synthwave', got %q", theme.Name)
	}
	if theme.Description != "Custom synthwave neon" {
		t.Errorf("expected theme.Description 'Custom synthwave neon', got %q", theme.Description)
	}
}

func TestResolveTheme(t *testing.T) {
	tmpDir := t.TempDir()
	customFile := filepath.Join(tmpDir, "violet.json")
	if err := os.WriteFile(customFile, []byte(`{"name":"violet"}`), 0o644); err != nil {
		t.Fatalf("failed to write custom theme: %v", err)
	}

	tests := []struct {
		name        string
		flagTheme   string
		envTheme    string
		configTheme string
		themesDir   string
		expected    string
	}{
		{
			name:      "flag precedence over env and config",
			flagTheme: "cyberpunk",
			envTheme:  "blood",
			expected:  "cyberpunk",
		},
		{
			name:        "env precedence over config",
			flagTheme:   "",
			envTheme:    "blood",
			configTheme: "matrix",
			expected:    "blood",
		},
		{
			name:        "config fallback when flag and env empty",
			flagTheme:   "",
			envTheme:    "",
			configTheme: "matrix",
			expected:    "matrix",
		},
		{
			name:        "custom file path via flag",
			flagTheme:   customFile,
			envTheme:    "",
			configTheme: "",
			expected:    "violet",
		},
		{
			name:        "custom theme from themesDir",
			flagTheme:   "violet",
			envTheme:    "",
			configTheme: "",
			themesDir:   tmpDir,
			expected:    "violet",
		},
		{
			name:        "all empty falls back to default",
			flagTheme:   "",
			envTheme:    "",
			configTheme: "",
			expected:    "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ResolveTheme(tt.flagTheme, tt.envTheme, tt.configTheme, tt.themesDir)
			if res.Name != tt.expected {
				t.Errorf("ResolveTheme() = %q, expected %q", res.Name, tt.expected)
			}
		})
	}
}

func TestRenderBrandPill_NerdFonts(t *testing.T) {
	styles := CompileStyles(DefaultTheme)

	// Solid badge without Nerd Fonts
	standard := styles.RenderBrandPill(false)
	if !strings.Contains(standard, "DOOM") || strings.Contains(standard, "") {
		t.Errorf("RenderBrandPill(false) = %q, expected plain DOOM without capsule caps", standard)
	}

	// Rounded capsule with Nerd Fonts
	nerd := styles.RenderBrandPill(true)
	if !strings.Contains(nerd, "DOOM") || !strings.Contains(nerd, "") || !strings.Contains(nerd, "") {
		t.Errorf("RenderBrandPill(true) = %q, expected DOOM with capsule caps", nerd)
	}
}

func TestMonochromeThemeContrast(t *testing.T) {
	styles := CompileStyles(MonochromeTheme)
	brandPill := styles.RenderBrandPill(false)
	if !strings.Contains(brandPill, "DOOM") {
		t.Errorf("monochrome brand pill = %q, expected 'DOOM'", brandPill)
	}

	// Verify BrandBg and StatsBg are distinct for contrast
	if MonochromeTheme.BrandBg == MonochromeTheme.StatsBg {
		t.Errorf("expected BrandBg and StatsBg to differ in MonochromeTheme for visual contrast")
	}
}
