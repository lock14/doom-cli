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

// CompileStyles constructs pre-compiled lipgloss.Style instances from a Theme.
func CompileStyles(t Theme) ThemeStyles {
	return ThemeStyles{
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

// RenderBrandPill renders the DOOM badge using this theme's brand styles.
// If nerdFonts is true, it renders Powerlevel10k rounded capsule ends ( and ).
// Otherwise, it renders a universal solid rectangular badge.
func (s ThemeStyles) RenderBrandPill(nerdFonts bool) string {
	if nerdFonts {
		return renderCapsule(s.BrandCap, s.BrandBody, " DOOM ")
	}
	return s.BrandBody.Render(" DOOM ")
}

// RenderStatsPill renders the preset count badge using this theme's stats styles.
func (s ThemeStyles) RenderStatsPill(count, total int, nerdFonts bool) string {
	text := fmt.Sprintf(" %d / %d presets ", count, total)
	if nerdFonts {
		return renderCapsule(s.StatsCap, s.StatsBody, text)
	}
	return s.StatsBody.Render(text)
}

// RenderScrollPill renders the scroll percentage badge for the README viewer.
func (s ThemeStyles) RenderScrollPill(percent float64, nerdFonts bool) string {
	pct := int(percent * 100)
	pctText := fmt.Sprintf(" %d%% ", pct)
	if pct <= 0 {
		pctText = " Top "
	} else if pct >= 100 {
		pctText = " End "
	}
	if nerdFonts {
		return renderCapsule(s.StatsCap, s.StatsBody, pctText)
	}
	return s.StatsBody.Render(pctText)
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
