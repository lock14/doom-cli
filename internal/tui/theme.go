// Package tui provides terminal styling, theming, and interactive launcher views.
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme defines the semantic color palette for the launcher TUI.
type Theme struct {
	Name           string
	Description    string
	Type           string // "ANSI-16" or "TrueColor"
	BrandIcon      string
	BrandCap       lipgloss.TerminalColor
	BrandText      lipgloss.TerminalColor
	BrandBg        lipgloss.TerminalColor
	StatsCap       lipgloss.TerminalColor
	StatsText      lipgloss.TerminalColor
	StatsBg        lipgloss.TerminalColor
	Prompt         lipgloss.TerminalColor
	CursorBar      lipgloss.TerminalColor
	CursorText     lipgloss.TerminalColor
	BorderActive   lipgloss.TerminalColor
	BorderInactive lipgloss.TerminalColor
	TitleActive    lipgloss.TerminalColor
	TitleInactive  lipgloss.TerminalColor
	Label          lipgloss.TerminalColor
	Keycap         lipgloss.TerminalColor
	KeyDesc        lipgloss.TerminalColor
	TagDSDA        lipgloss.TerminalColor
	TagUZDoom      lipgloss.TerminalColor
	StatusReady    lipgloss.TerminalColor
	StatusMissing  lipgloss.TerminalColor
	Muted          lipgloss.TerminalColor
}

// ThemeStyles holds pre-compiled lipgloss.Style instances derived from a Theme.
type ThemeStyles struct {
	BrandIcon        string
	BrandCap         lipgloss.Style
	BrandBody        lipgloss.Style
	StatsCap         lipgloss.Style
	StatsBody        lipgloss.Style
	FilterPrompt     lipgloss.Style
	CursorBar        lipgloss.Style
	CursorText       lipgloss.Style
	BoxActive        lipgloss.Style
	BoxInactive      lipgloss.Style
	BorderFgActive   lipgloss.Style
	BorderFgInactive lipgloss.Style
	TitleActive      lipgloss.Style
	TitleInactive    lipgloss.Style
	Label            lipgloss.Style
	ValueBold        lipgloss.Style
	KeyHelpKey       lipgloss.Style
	KeyHelpDesc      lipgloss.Style
	Bullet           lipgloss.Style
	TagDSDA          lipgloss.Style
	TagUZDoom        lipgloss.Style
	StatusReady      lipgloss.Style
	StatusMissing    lipgloss.Style
	Placeholder      lipgloss.Style
	Help             lipgloss.Style
}

// CustomThemeFile defines the JSON serialization format for user-authored custom theme files.
type CustomThemeFile struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	BrandIcon      string `json:"brand_icon,omitempty"`
	BrandCap       string `json:"brand_cap,omitempty"`
	BrandText      string `json:"brand_text,omitempty"`
	BrandBg        string `json:"brand_bg,omitempty"`
	StatsCap       string `json:"stats_cap,omitempty"`
	StatsText      string `json:"stats_text,omitempty"`
	StatsBg        string `json:"stats_bg,omitempty"`
	Prompt         string `json:"prompt,omitempty"`
	CursorBar      string `json:"cursor_bar,omitempty"`
	CursorText     string `json:"cursor_text,omitempty"`
	BorderActive   string `json:"border_active,omitempty"`
	BorderInactive string `json:"border_inactive,omitempty"`
	TitleActive    string `json:"title_active,omitempty"`
	TitleInactive  string `json:"title_inactive,omitempty"`
	Label          string `json:"label,omitempty"`
	Keycap         string `json:"keycap,omitempty"`
	KeyDesc        string `json:"key_desc,omitempty"`
	TagDSDA        string `json:"tag_dsda,omitempty"`
	TagUZDoom      string `json:"tag_uzdoom,omitempty"`
	StatusReady    string `json:"status_ready,omitempty"`
	StatusMissing  string `json:"status_missing,omitempty"`
	Muted          string `json:"muted,omitempty"`
}

// DefaultTheme is the standard semantic ANSI-16 theme tailored for classic Doom.
var DefaultTheme = Theme{
	Name:           "default",
	Description:    "Classic Doom Semantic ANSI (Adaptive)",
	Type:           "ANSI-16",
	BrandCap:       lipgloss.Color("1"),
	BrandText:      lipgloss.Color("7"),
	BrandBg:        lipgloss.Color("1"),
	StatsCap:       lipgloss.Color("8"),
	StatsText:      lipgloss.Color("7"),
	StatsBg:        lipgloss.Color("8"),
	Prompt:         lipgloss.Color("6"),
	CursorBar:      lipgloss.Color("6"),
	CursorText:     lipgloss.Color("6"),
	BorderActive:   lipgloss.Color("8"),
	BorderInactive: lipgloss.Color("8"),
	TitleActive:    lipgloss.Color("6"),
	TitleInactive:  lipgloss.Color("8"),
	Label:          lipgloss.Color("6"),
	Keycap:         lipgloss.Color("3"),
	KeyDesc:        lipgloss.Color("8"),
	TagDSDA:        lipgloss.Color("3"),
	TagUZDoom:      lipgloss.Color("2"),
	StatusReady:    lipgloss.Color("2"),
	StatusMissing:  lipgloss.Color("1"),
	Muted:          lipgloss.Color("8"),
}

// CyberpunkTheme is a high-tech neon 24-bit TrueColor theme.
var CyberpunkTheme = Theme{
	Name:           "cyberpunk",
	Description:    "High-tech neon 24-bit TrueColor",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#FF2A6D"),
	BrandText:      lipgloss.Color("#FFFFFF"),
	BrandBg:        lipgloss.Color("#FF2A6D"),
	StatsCap:       lipgloss.Color("#1F1A3A"),
	StatsText:      lipgloss.Color("#05D9E8"),
	StatsBg:        lipgloss.Color("#1F1A3A"),
	Prompt:         lipgloss.Color("#05D9E8"),
	CursorBar:      lipgloss.Color("#05D9E8"),
	CursorText:     lipgloss.Color("#05D9E8"),
	BorderActive:   lipgloss.Color("#7A22A8"),
	BorderInactive: lipgloss.Color("#2C1654"),
	TitleActive:    lipgloss.Color("#05D9E8"),
	TitleInactive:  lipgloss.Color("#5B4278"),
	Label:          lipgloss.Color("#FF2A6D"),
	Keycap:         lipgloss.Color("#FFE600"),
	KeyDesc:        lipgloss.Color("#7A7593"),
	TagDSDA:        lipgloss.Color("#FFE600"),
	TagUZDoom:      lipgloss.Color("#05D9E8"),
	StatusReady:    lipgloss.Color("#00F5D4"),
	StatusMissing:  lipgloss.Color("#FF2A6D"),
	Muted:          lipgloss.Color("#7A7593"),
}

// BloodTheme is a gothic Crimson & Bone aesthetic inspired by Nightdive.
var BloodTheme = Theme{
	Name:           "blood",
	Description:    "Gothic Crimson & Bone Nightdive aesthetic",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#9B111E"),
	BrandText:      lipgloss.Color("#FFFFFF"),
	BrandBg:        lipgloss.Color("#9B111E"),
	StatsCap:       lipgloss.Color("#2B1114"),
	StatsText:      lipgloss.Color("#D4AF37"),
	StatsBg:        lipgloss.Color("#2B1114"),
	Prompt:         lipgloss.Color("#C92A2A"),
	CursorBar:      lipgloss.Color("#D4AF37"),
	CursorText:     lipgloss.Color("#E8E8E8"),
	BorderActive:   lipgloss.Color("#8B0000"),
	BorderInactive: lipgloss.Color("#3E181D"),
	TitleActive:    lipgloss.Color("#D4AF37"),
	TitleInactive:  lipgloss.Color("#6B3B42"),
	Label:          lipgloss.Color("#D4AF37"),
	Keycap:         lipgloss.Color("#D4AF37"),
	KeyDesc:        lipgloss.Color("#8E7578"),
	TagDSDA:        lipgloss.Color("#D4AF37"),
	TagUZDoom:      lipgloss.Color("#4EAA25"),
	StatusReady:    lipgloss.Color("#4EAA25"),
	StatusMissing:  lipgloss.Color("#C92A2A"),
	Muted:          lipgloss.Color("#8E7578"),
}

// MatrixTheme is a retro monochrome Phosphor Green aesthetic.
var MatrixTheme = Theme{
	Name:           "matrix",
	Description:    "Retro monochrome Phosphor Green",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#00FF66"),
	BrandText:      lipgloss.Color("#000000"),
	BrandBg:        lipgloss.Color("#00FF66"),
	StatsCap:       lipgloss.Color("#003B00"),
	StatsText:      lipgloss.Color("#00FF66"),
	StatsBg:        lipgloss.Color("#003B00"),
	Prompt:         lipgloss.Color("#00FF66"),
	CursorBar:      lipgloss.Color("#33FF33"),
	CursorText:     lipgloss.Color("#33FF33"),
	BorderActive:   lipgloss.Color("#008F11"),
	BorderInactive: lipgloss.Color("#003B00"),
	TitleActive:    lipgloss.Color("#00FF66"),
	TitleInactive:  lipgloss.Color("#005511"),
	Label:          lipgloss.Color("#00FF66"),
	Keycap:         lipgloss.Color("#33FF33"),
	KeyDesc:        lipgloss.Color("#008F11"),
	TagDSDA:        lipgloss.Color("#66FF66"),
	TagUZDoom:      lipgloss.Color("#00FF66"),
	StatusReady:    lipgloss.Color("#33FF33"),
	StatusMissing:  lipgloss.Color("#FF0033"),
	Muted:          lipgloss.Color("#008F11"),
}

// MonochromeTheme is a minimalist high-contrast Black & White ANSI theme.
var MonochromeTheme = Theme{
	Name:           "monochrome",
	Description:    "Minimalist high-contrast Black & White",
	Type:           "ANSI",
	BrandIcon:      "💀",
	BrandCap:       lipgloss.Color("8"),
	BrandText:      lipgloss.Color("15"),
	BrandBg:        lipgloss.Color("8"),
	StatsCap:       lipgloss.Color("7"),
	StatsText:      lipgloss.Color("0"),
	StatsBg:        lipgloss.Color("7"),
	Prompt:         lipgloss.Color("15"),
	CursorBar:      lipgloss.Color("15"),
	CursorText:     lipgloss.Color("15"),
	BorderActive:   lipgloss.Color("15"),
	BorderInactive: lipgloss.Color("8"),
	TitleActive:    lipgloss.Color("15"),
	TitleInactive:  lipgloss.Color("8"),
	Label:          lipgloss.Color("15"),
	Keycap:         lipgloss.Color("15"),
	KeyDesc:        lipgloss.Color("8"),
	TagDSDA:        lipgloss.Color("15"),
	TagUZDoom:      lipgloss.Color("7"),
	StatusReady:    lipgloss.Color("15"),
	StatusMissing:  lipgloss.Color("8"),
	Muted:          lipgloss.Color("8"),
}

// BuiltinThemes maps lowercase theme names to their built-in Theme definitions.
var BuiltinThemes = map[string]Theme{
	"default":    DefaultTheme,
	"doom":       DefaultTheme,
	"cyberpunk":  CyberpunkTheme,
	"blood":      BloodTheme,
	"matrix":     MatrixTheme,
	"monochrome": MonochromeTheme,
}

// GetBuiltinTheme retrieves a built-in theme by case-insensitive name.
func GetBuiltinTheme(name string) (Theme, bool) {
	t, ok := BuiltinThemes[strings.ToLower(strings.TrimSpace(name))]
	return t, ok
}

// ListBuiltinThemes returns an ordered list of all built-in themes.
func ListBuiltinThemes() []Theme {
	order := []string{"default", "cyberpunk", "blood", "matrix", "monochrome"}
	var result []Theme
	for _, name := range order {
		if t, ok := BuiltinThemes[name]; ok {
			result = append(result, t)
		}
	}
	return result
}

// CompileStyles constructs pre-compiled lipgloss.Style instances from a Theme.
func CompileStyles(t Theme) ThemeStyles {
	icon := t.BrandIcon
	if icon == "" {
		icon = "💀"
	}

	return ThemeStyles{
		BrandIcon:    icon,
		BrandCap:     lipgloss.NewStyle().Foreground(t.BrandCap),
		BrandBody:    lipgloss.NewStyle().Background(t.BrandBg).Foreground(t.BrandText).Bold(true),
		StatsCap:     lipgloss.NewStyle().Foreground(t.StatsCap),
		StatsBody:    lipgloss.NewStyle().Background(t.StatsBg).Foreground(t.StatsText).Bold(true),
		FilterPrompt: lipgloss.NewStyle().Foreground(t.Prompt).Bold(true),
		CursorBar:    lipgloss.NewStyle().Foreground(t.CursorBar).Bold(true),
		CursorText:   lipgloss.NewStyle().Foreground(t.CursorText).Bold(true),
		BoxActive: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderActive).
			Padding(0, 1),
		BoxInactive: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderInactive).
			Padding(0, 1),
		BorderFgActive:   lipgloss.NewStyle().Foreground(t.BorderActive),
		BorderFgInactive: lipgloss.NewStyle().Foreground(t.BorderInactive),
		TitleActive:      lipgloss.NewStyle().Foreground(t.TitleActive).Bold(true),
		TitleInactive:    lipgloss.NewStyle().Foreground(t.TitleInactive).Bold(true),
		Label:            lipgloss.NewStyle().Foreground(t.Label).Bold(true),
		ValueBold:        lipgloss.NewStyle().Bold(true),
		KeyHelpKey:       lipgloss.NewStyle().Foreground(t.Keycap).Bold(true),
		KeyHelpDesc:      lipgloss.NewStyle().Foreground(t.KeyDesc),
		Bullet:           lipgloss.NewStyle().Foreground(t.Muted),
		TagDSDA:          lipgloss.NewStyle().Foreground(t.TagDSDA),
		TagUZDoom:        lipgloss.NewStyle().Foreground(t.TagUZDoom),
		StatusReady:      lipgloss.NewStyle().Foreground(t.StatusReady).Bold(true),
		StatusMissing:    lipgloss.NewStyle().Foreground(t.StatusMissing).Bold(true),
		Placeholder:      lipgloss.NewStyle().Foreground(t.Muted),
		Help:             lipgloss.NewStyle().Foreground(t.Muted),
	}
}

// LoadThemeFile parses a JSON custom theme file, falling back to DefaultTheme for missing properties.
func LoadThemeFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("reading theme file: %w", err)
	}

	var c CustomThemeFile
	if err := json.Unmarshal(data, &c); err != nil {
		return Theme{}, fmt.Errorf("parsing theme JSON: %w", err)
	}

	theme := DefaultTheme
	if c.Name != "" {
		theme.Name = c.Name
	} else {
		theme.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if c.Description != "" {
		theme.Description = c.Description
	} else {
		theme.Description = "Custom user theme"
	}
	theme.Type = "Custom"
	if c.BrandIcon != "" {
		theme.BrandIcon = c.BrandIcon
	}

	assignColor(&theme.BrandCap, c.BrandCap)
	assignColor(&theme.BrandText, c.BrandText)
	assignColor(&theme.BrandBg, c.BrandBg)
	assignColor(&theme.StatsCap, c.StatsCap)
	assignColor(&theme.StatsText, c.StatsText)
	assignColor(&theme.StatsBg, c.StatsBg)
	assignColor(&theme.Prompt, c.Prompt)
	assignColor(&theme.CursorBar, c.CursorBar)
	assignColor(&theme.CursorText, c.CursorText)
	assignColor(&theme.BorderActive, c.BorderActive)
	assignColor(&theme.BorderInactive, c.BorderInactive)
	assignColor(&theme.TitleActive, c.TitleActive)
	assignColor(&theme.TitleInactive, c.TitleInactive)
	assignColor(&theme.Label, c.Label)
	assignColor(&theme.Keycap, c.Keycap)
	assignColor(&theme.KeyDesc, c.KeyDesc)
	assignColor(&theme.TagDSDA, c.TagDSDA)
	assignColor(&theme.TagUZDoom, c.TagUZDoom)
	assignColor(&theme.StatusReady, c.StatusReady)
	assignColor(&theme.StatusMissing, c.StatusMissing)
	assignColor(&theme.Muted, c.Muted)

	return theme, nil
}

func assignColor(target *lipgloss.TerminalColor, val string) {
	if val != "" {
		*target = lipgloss.Color(val)
	}
}

// ResolveTheme resolves a Theme based on CLI flag, DOOM_THEME env, user config, and custom themes directory.
func ResolveTheme(flagTheme, envTheme, configTheme, themesDir string) Theme {
	for _, candidate := range []string{flagTheme, envTheme, configTheme} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}

		if t, ok := GetBuiltinTheme(candidate); ok {
			return t
		}

		if strings.HasSuffix(strings.ToLower(candidate), ".json") {
			if t, err := LoadThemeFile(candidate); err == nil {
				return t
			}
		}

		if themesDir != "" {
			themePath := filepath.Join(themesDir, candidate+".json")
			if t, err := LoadThemeFile(themePath); err == nil {
				return t
			}
		}
	}

	return DefaultTheme
}

// RenderBrandPill renders the DOOM pill using this theme's brand styles.
func (s ThemeStyles) RenderBrandPill() string {
	icon := s.BrandIcon
	if icon == "" {
		icon = "💀"
	}
	text := fmt.Sprintf(" %s DOOM ", icon)
	if strings.TrimSpace(icon) == "" {
		text = " DOOM "
	}
	return renderCapsule(s.BrandCap, s.BrandBody, text)
}

// RenderStatsPill renders the preset count pill using this theme's stats styles.
func (s ThemeStyles) RenderStatsPill(count, total int) string {
	return renderCapsule(s.StatsCap, s.StatsBody, fmt.Sprintf(" %d / %d presets ", count, total))
}

// RenderScrollPill renders the scroll percentage pill for the README viewer.
func (s ThemeStyles) RenderScrollPill(percent float64) string {
	pct := int(percent * 100)
	pctText := fmt.Sprintf(" %d%% ", pct)
	if pct <= 0 {
		pctText = " Top "
	} else if pct >= 100 {
		pctText = " End "
	}
	return renderCapsule(s.StatsCap, s.StatsBody, pctText)
}

// FormatKeyHelp renders two-tone keycap help instructions.
func (s ThemeStyles) FormatKeyHelp(items []keyHelp) string {
	var parts []string
	for _, item := range items {
		parts = append(parts, s.KeyHelpKey.Render(item.key)+" "+s.KeyHelpDesc.Render(item.desc))
	}
	sep := s.Bullet.Render("  •  ")
	return "\n\n" + strings.Join(parts, sep)
}
