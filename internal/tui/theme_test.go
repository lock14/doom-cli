package tui

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGetBuiltinTheme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
		found    bool
	}{
		{
			name:     "exact classic",
			input:    "classic",
			expected: "classic",
			found:    true,
		},
		{
			name:     "blood theme",
			input:    "blood",
			expected: "blood",
			found:    true,
		},
		{
			name:     "toxic theme",
			input:    "toxic",
			expected: "toxic",
			found:    true,
		},
		{
			name:     "inferno theme",
			input:    "inferno",
			expected: "inferno",
			found:    true,
		},
		{
			name:     "frost theme",
			input:    "frost",
			expected: "frost",
			found:    true,
		},
		{
			name:     "plasma theme",
			input:    "plasma",
			expected: "plasma",
			found:    true,
		},
		{
			name:     "heretic theme",
			input:    "heretic",
			expected: "heretic",
			found:    true,
		},
		{
			name:     "amber theme",
			input:    "amber",
			expected: "amber",
			found:    true,
		},
		{
			name:     "sigil theme",
			input:    "sigil",
			expected: "sigil",
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
	t.Parallel()

	list := ListBuiltinThemes()
	if len(list) != 10 {
		t.Fatalf("expected 10 built-in themes, got %d", len(list))
	}
	expectedNames := []string{
		"classic", "blood", "toxic", "inferno", "frost",
		"plasma", "heretic", "amber", "sigil", "monochrome",
	}
	for i, name := range expectedNames {
		if list[i].Name != name {
			t.Errorf("theme at index %d = %q, expected %q", i, list[i].Name, name)
		}
	}
}

func TestCompileStyles(t *testing.T) {
	t.Parallel()
	styles := CompileStyles(PlasmaTheme)
	rendered := styles.BrandBody.Render("TEST")
	if rendered == "" {
		t.Error("expected non-empty rendered string from compiled BrandBody style")
	}
}

func TestLoadThemeFile(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
			flagTheme: "plasma",
			envTheme:  "blood",
			expected:  "plasma",
		},
		{
			name:        "env precedence over config",
			flagTheme:   "",
			envTheme:    "blood",
			configTheme: "amber",
			expected:    "blood",
		},
		{
			name:        "config fallback when flag and env empty",
			flagTheme:   "",
			envTheme:    "",
			configTheme: "amber",
			expected:    "amber",
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
			name:        "all empty falls back to classic",
			flagTheme:   "",
			envTheme:    "",
			configTheme: "",
			expected:    "classic",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := ResolveTheme(tt.flagTheme, tt.envTheme, tt.configTheme, tt.themesDir)
			if res.Name != tt.expected {
				t.Errorf("ResolveTheme() = %q, expected %q", res.Name, tt.expected)
			}
		})
	}
}

func TestRenderBrandPill_NerdFonts(t *testing.T) {
	t.Parallel()
	styles := CompileStyles(ClassicTheme)

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
	t.Parallel()
	styles := CompileStyles(MonochromeTheme)
	brandPill := styles.RenderBrandPill(false)
	if !strings.Contains(brandPill, "DOOM") {
		t.Errorf("monochrome brand pill = %q, expected 'DOOM'", brandPill)
	}

	// Verify BrandBg and StatsBg are distinct for contrast
	if MonochromeTheme.BrandBg == MonochromeTheme.StatsBg {
		t.Errorf("expected BrandBg and StatsBg to differ in MonochromeTheme for visual contrast")
	}

	// Verify StatusMissing uses base-16 code 7 and reverse video for high prominence
	if fmt.Sprint(MonochromeTheme.StatusMissing) != "7" {
		t.Errorf("expected MonochromeTheme.StatusMissing to be '7', got %v", MonochromeTheme.StatusMissing)
	}
	if !styles.StatusMissing.GetReverse() {
		t.Errorf("expected MonochromeTheme styles.StatusMissing.GetReverse() == true")
	}
	if !styles.TitleActive.GetBold() {
		t.Errorf("expected styles.TitleActive.GetBold() == true")
	}
	if styles.TitleInactive.GetBold() {
		t.Errorf("expected styles.TitleInactive.GetBold() == false")
	}
}

func relativeLuminance(hex string) float64 {
	h := strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseUint(h[0:2], 16, 8)
	g, _ := strconv.ParseUint(h[2:4], 16, 8)
	b, _ := strconv.ParseUint(h[4:6], 16, 8)
	chanLum := func(v uint64) float64 {
		c := float64(v) / 255.0
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*chanLum(r) + 0.7152*chanLum(g) + 0.0722*chanLum(b)
}

func wcagContrastRatio(c1, c2 string) float64 {
	l1 := relativeLuminance(c1)
	l2 := relativeLuminance(c2)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func TestBuiltinThemesWCAGContrast(t *testing.T) {
	t.Parallel()
	const refCanvas = "#0D1117"
	for _, th := range ListBuiltinThemes() {
		th := th
		if th.Type != "TrueColor" {
			continue
		}
		t.Run(th.Name, func(t *testing.T) {
			t.Parallel()
			normalTokens := map[string]string{
				"TitleInactive": fmt.Sprint(th.TitleInactive),
				"Label":         fmt.Sprint(th.Label),
				"KeyDesc":       fmt.Sprint(th.KeyDesc),
				"TagDSDA":       fmt.Sprint(th.TagDSDA),
				"TagUZDoom":     fmt.Sprint(th.TagUZDoom),
				"StatusReady":   fmt.Sprint(th.StatusReady),
				"Muted":         fmt.Sprint(th.Muted),
			}
			for field, hex := range normalTokens {
				if r := wcagContrastRatio(hex, refCanvas); r < 4.5 {
					t.Errorf("%s.%s (%s vs %s) contrast = %.2f:1, want >= 4.5:1", th.Name, field, hex, refCanvas, r)
				}
			}
			boldTokens := map[string]string{
				"Prompt":        fmt.Sprint(th.Prompt),
				"CursorBar":     fmt.Sprint(th.CursorBar),
				"CursorText":    fmt.Sprint(th.CursorText),
				"TitleActive":   fmt.Sprint(th.TitleActive),
				"Keycap":        fmt.Sprint(th.Keycap),
				"StatusMissing": fmt.Sprint(th.StatusMissing),
			}
			for field, hex := range boldTokens {
				if r := wcagContrastRatio(hex, refCanvas); r < 3.0 {
					t.Errorf("%s.%s (%s vs %s) contrast = %.2f:1, want >= 3.0:1", th.Name, field, hex, refCanvas, r)
				}
			}
			if r := wcagContrastRatio(fmt.Sprint(th.BrandText), fmt.Sprint(th.BrandBg)); r < 3.0 {
				t.Errorf("%s.BrandText (%v vs %v) contrast = %.2f:1, want >= 3.0:1",
					th.Name, th.BrandText, th.BrandBg, r)
			}
			if r := wcagContrastRatio(fmt.Sprint(th.StatsText), fmt.Sprint(th.StatsBg)); r < 3.0 {
				t.Errorf("%s.StatsText (%v vs %v) contrast = %.2f:1, want >= 3.0:1",
					th.Name, th.StatsText, th.StatsBg, r)
			}
		})
	}
}

func TestReadmeCustomThemeJSONBlock(t *testing.T) {
	t.Parallel()
	readmePath := filepath.Join("..", "..", "README.md")
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}

	lines := strings.Split(string(raw), "\n")
	inCustomThemesSection := false
	inJSONFence := false
	var jsonBuf strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			inCustomThemesSection = strings.EqualFold(trimmed, "### Custom JSON themes")
			continue
		}
		if !inCustomThemesSection {
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			if !inJSONFence {
				if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")), "json") {
					inJSONFence = true
					jsonBuf.Reset()
				}
			} else {
				break
			}
			continue
		}
		if inJSONFence {
			jsonBuf.WriteString(line)
			jsonBuf.WriteByte('\n')
		}
	}

	block := strings.TrimSpace(jsonBuf.String())
	if block == "" {
		t.Fatal("could not find JSON code block under '### Custom JSON themes' in README.md")
	}

	dec := json.NewDecoder(strings.NewReader(block))
	dec.DisallowUnknownFields()
	var parsed CustomThemeFile
	if err := dec.Decode(&parsed); err != nil {
		t.Fatalf("README.md custom theme JSON failed strict CustomThemeFile decoding: %v\n%s", err, block)
	}
	if parsed.Name == "" || parsed.BrandCap == "" || parsed.StatusReady == "" {
		t.Errorf("expected populated CustomThemeFile fields in README.md example, got %+v", parsed)
	}
}

func TestListCustomThemes(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// 1. Valid custom theme
	if err := os.WriteFile(
		filepath.Join(tmpDir, "synthwave.json"),
		[]byte(`{"name":"synthwave","description":"Neon synthwave"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	// 2. Shadowing built-in theme name (should be skipped)
	if err := os.WriteFile(
		filepath.Join(tmpDir, "classic.json"),
		[]byte(`{"name":"classic","description":"Shadow built-in"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	// 3. Invalid JSON (should be skipped)
	if err := os.WriteFile(filepath.Join(tmpDir, "broken.json"), []byte(`{bad`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ListCustomThemes(tmpDir)
	if len(got) != 1 || got[0].Name != "synthwave" {
		t.Fatalf("ListCustomThemes = %+v, want single 'synthwave' theme", got)
	}
}
