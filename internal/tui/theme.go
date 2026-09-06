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

// ClassicTheme (aliased as DefaultTheme) is the standard semantic ANSI-16 theme tailored for classic Doom.
var ClassicTheme = Theme{
	Name:           "classic",
	Description:    "Classic Doom Semantic ANSI (Adaptive)",
	Type:           "ANSI-16",
	BrandCap:       lipgloss.Color("1"),
	BrandText:      lipgloss.Color("7"),
	BrandBg:        lipgloss.Color("1"),
	StatsCap:       lipgloss.Color("8"),
	StatsText:      lipgloss.Color("7"),
	StatsBg:        lipgloss.Color("8"),
	Prompt:         lipgloss.Color("3"),
	CursorBar:      lipgloss.Color("1"),
	CursorText:     lipgloss.Color("3"),
	BorderActive:   lipgloss.Color("7"),
	BorderInactive: lipgloss.Color("8"),
	TitleActive:    lipgloss.Color("1"),
	TitleInactive:  lipgloss.Color("8"),
	Label:          lipgloss.Color("8"),
	Keycap:         lipgloss.Color("3"),
	KeyDesc:        lipgloss.Color("8"),
	TagDSDA:        lipgloss.Color("3"),
	TagUZDoom:      lipgloss.Color("2"),
	StatusReady:    lipgloss.Color("2"),
	StatusMissing:  lipgloss.Color("1"),
	Muted:          lipgloss.Color("8"),
}

// DefaultTheme is retained as an alias to ClassicTheme for backward compatibility.
var DefaultTheme = ClassicTheme

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

// ToxicTheme is a radioactive Nukage Green and Hazard Amber theme inspired by Phobos techbases.
var ToxicTheme = Theme{
	Name:           "toxic",
	Description:    "Radioactive Nukage Green & Hazard Amber",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#FFB703"),
	BrandText:      lipgloss.Color("#0D1B2A"),
	BrandBg:        lipgloss.Color("#FFB703"),
	StatsCap:       lipgloss.Color("#1A2830"),
	StatsText:      lipgloss.Color("#70E000"),
	StatsBg:        lipgloss.Color("#1A2830"),
	Prompt:         lipgloss.Color("#FB8500"),
	CursorBar:      lipgloss.Color("#70E000"),
	CursorText:     lipgloss.Color("#70E000"),
	BorderActive:   lipgloss.Color("#38B000"),
	BorderInactive: lipgloss.Color("#142129"),
	TitleActive:    lipgloss.Color("#70E000"),
	TitleInactive:  lipgloss.Color("#2D4A3E"),
	Label:          lipgloss.Color("#FFB703"),
	Keycap:         lipgloss.Color("#FFB703"),
	KeyDesc:        lipgloss.Color("#5C7D73"),
	TagDSDA:        lipgloss.Color("#FFB703"),
	TagUZDoom:      lipgloss.Color("#70E000"),
	StatusReady:    lipgloss.Color("#70E000"),
	StatusMissing:  lipgloss.Color("#E63946"),
	Muted:          lipgloss.Color("#5C7D73"),
}

// InfernoTheme is a volcanic molten magma and charred basalt theme inspired by Episode 3 and Plutonia.
var InfernoTheme = Theme{
	Name:           "inferno",
	Description:    "Volcanic Molten Magma & Charred Basalt",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#FF5400"),
	BrandText:      lipgloss.Color("#120805"),
	BrandBg:        lipgloss.Color("#FF5400"),
	StatsCap:       lipgloss.Color("#1E0F09"),
	StatsText:      lipgloss.Color("#FF7700"),
	StatsBg:        lipgloss.Color("#1E0F09"),
	Prompt:         lipgloss.Color("#FFAA00"),
	CursorBar:      lipgloss.Color("#FF7700"),
	CursorText:     lipgloss.Color("#FFAA00"),
	BorderActive:   lipgloss.Color("#9E2A00"),
	BorderInactive: lipgloss.Color("#24120B"),
	TitleActive:    lipgloss.Color("#FF5400"),
	TitleInactive:  lipgloss.Color("#5E2B18"),
	Label:          lipgloss.Color("#FFAA00"),
	Keycap:         lipgloss.Color("#FFAA00"),
	KeyDesc:        lipgloss.Color("#8F5C4A"),
	TagDSDA:        lipgloss.Color("#FFB703"),
	TagUZDoom:      lipgloss.Color("#FF5400"),
	StatusReady:    lipgloss.Color("#2EC4B6"),
	StatusMissing:  lipgloss.Color("#D90429"),
	Muted:          lipgloss.Color("#8F5C4A"),
}

// FrostTheme is a soothing glacial ice and midnight navy theme inspired by Cocytus and Nordic palettes.
var FrostTheme = Theme{
	Name:           "frost",
	Description:    "Glacial Cyan & Midnight Polar Navy",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#56CFE1"),
	BrandText:      lipgloss.Color("#0A1128"),
	BrandBg:        lipgloss.Color("#56CFE1"),
	StatsCap:       lipgloss.Color("#1B2A4A"),
	StatsText:      lipgloss.Color("#72EFDD"),
	StatsBg:        lipgloss.Color("#1B2A4A"),
	Prompt:         lipgloss.Color("#48CAE4"),
	CursorBar:      lipgloss.Color("#72EFDD"),
	CursorText:     lipgloss.Color("#F8FAFC"),
	BorderActive:   lipgloss.Color("#1E3A5F"),
	BorderInactive: lipgloss.Color("#0F172A"),
	TitleActive:    lipgloss.Color("#56CFE1"),
	TitleInactive:  lipgloss.Color("#2B4C6F"),
	Label:          lipgloss.Color("#48CAE4"),
	Keycap:         lipgloss.Color("#72EFDD"),
	KeyDesc:        lipgloss.Color("#64748B"),
	TagDSDA:        lipgloss.Color("#F4A261"),
	TagUZDoom:      lipgloss.Color("#48CAE4"),
	StatusReady:    lipgloss.Color("#72EFDD"),
	StatusMissing:  lipgloss.Color("#F43F5E"),
	Muted:          lipgloss.Color("#64748B"),
}

// PlasmaTheme (aliased as CyberpunkTheme) is an electric cyan and neon magenta theme inspired by Plasma Rifle coils.
var PlasmaTheme = Theme{
	Name:           "plasma",
	Description:    "Plasma Rifle Coils & Electric Neon",
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

// CyberpunkTheme is retained as an alias to PlasmaTheme.
var CyberpunkTheme = PlasmaTheme

// HereticTheme is a dark fantasy amethyst and emerald theme inspired by Raven Software titles.
var HereticTheme = Theme{
	Name:           "heretic",
	Description:    "Raven Dark Fantasy Amethyst & Emerald",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#FF79C6"),
	BrandText:      lipgloss.Color("#282A36"),
	BrandBg:        lipgloss.Color("#FF79C6"),
	StatsCap:       lipgloss.Color("#44475A"),
	StatsText:      lipgloss.Color("#F1FA8C"),
	StatsBg:        lipgloss.Color("#44475A"),
	Prompt:         lipgloss.Color("#8BE9FD"),
	CursorBar:      lipgloss.Color("#BD93F9"),
	CursorText:     lipgloss.Color("#F8F8F2"),
	BorderActive:   lipgloss.Color("#6272A4"),
	BorderInactive: lipgloss.Color("#21222C"),
	TitleActive:    lipgloss.Color("#FF79C6"),
	TitleInactive:  lipgloss.Color("#4A435A"),
	Label:          lipgloss.Color("#8BE9FD"),
	Keycap:         lipgloss.Color("#F1FA8C"),
	KeyDesc:        lipgloss.Color("#6272A4"),
	TagDSDA:        lipgloss.Color("#F1FA8C"),
	TagUZDoom:      lipgloss.Color("#50FA7B"),
	StatusReady:    lipgloss.Color("#50FA7B"),
	StatusMissing:  lipgloss.Color("#FF5555"),
	Muted:          lipgloss.Color("#6272A4"),
}

// AmberTheme is a warm monochrome amber CRT theme inspired by vintage DEC VT220 monitors.
var AmberTheme = Theme{
	Name:           "amber",
	Description:    "Vintage Warm Amber Phosphor CRT",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#FFB000"),
	BrandText:      lipgloss.Color("#000000"),
	BrandBg:        lipgloss.Color("#FFB000"),
	StatsCap:       lipgloss.Color("#241700"),
	StatsText:      lipgloss.Color("#FFB000"),
	StatsBg:        lipgloss.Color("#241700"),
	Prompt:         lipgloss.Color("#FFB000"),
	CursorBar:      lipgloss.Color("#FFC000"),
	CursorText:     lipgloss.Color("#FFE066"),
	BorderActive:   lipgloss.Color("#CC8800"),
	BorderInactive: lipgloss.Color("#382400"),
	TitleActive:    lipgloss.Color("#FFB000"),
	TitleInactive:  lipgloss.Color("#6B4500"),
	Label:          lipgloss.Color("#DDAA33"),
	Keycap:         lipgloss.Color("#FFC000"),
	KeyDesc:        lipgloss.Color("#855800"),
	TagDSDA:        lipgloss.Color("#FFB000"),
	TagUZDoom:      lipgloss.Color("#FFC000"),
	StatusReady:    lipgloss.Color("#FFC000"),
	StatusMissing:  lipgloss.Color("#D90429"),
	Muted:          lipgloss.Color("#855800"),
}

// MatrixTheme is retained as an alias pointing to AmberTheme.
var MatrixTheme = AmberTheme

// SigilTheme is an occult velvet maroon and pentagram red theme inspired by Romero's Sigil episodes.
var SigilTheme = Theme{
	Name:           "sigil",
	Description:    "Romero Occult Velvet Maroon & Pentagram Red",
	Type:           "TrueColor",
	BrandCap:       lipgloss.Color("#D90429"),
	BrandText:      lipgloss.Color("#ECE2D0"),
	BrandBg:        lipgloss.Color("#D90429"),
	StatsCap:       lipgloss.Color("#20121A"),
	StatsText:      lipgloss.Color("#ECE2D0"),
	StatsBg:        lipgloss.Color("#20121A"),
	Prompt:         lipgloss.Color("#C9184A"),
	CursorBar:      lipgloss.Color("#D90429"),
	CursorText:     lipgloss.Color("#F0E6DF"),
	BorderActive:   lipgloss.Color("#5E0B15"),
	BorderInactive: lipgloss.Color("#170F14"),
	TitleActive:    lipgloss.Color("#D90429"),
	TitleInactive:  lipgloss.Color("#451E28"),
	Label:          lipgloss.Color("#C9184A"),
	Keycap:         lipgloss.Color("#DDA15E"),
	KeyDesc:        lipgloss.Color("#7A5260"),
	TagDSDA:        lipgloss.Color("#DDA15E"),
	TagUZDoom:      lipgloss.Color("#A70000"),
	StatusReady:    lipgloss.Color("#38B000"),
	StatusMissing:  lipgloss.Color("#D90429"),
	Muted:          lipgloss.Color("#7A5260"),
}

// MonochromeTheme is a minimalist high-contrast Black & White ANSI theme.
var MonochromeTheme = Theme{
	Name:           "monochrome",
	Description:    "Minimalist high-contrast Black & White",
	Type:           "ANSI",
	BrandCap:       lipgloss.Color("7"),
	BrandText:      lipgloss.Color("0"),
	BrandBg:        lipgloss.Color("7"),
	StatsCap:       lipgloss.Color("8"),
	StatsText:      lipgloss.Color("7"),
	StatsBg:        lipgloss.Color("8"),
	Prompt:         lipgloss.Color("7"),
	CursorBar:      lipgloss.Color("7"),
	CursorText:     lipgloss.Color("7"),
	BorderActive:   lipgloss.Color("7"),
	BorderInactive: lipgloss.Color("8"),
	TitleActive:    lipgloss.Color("7"),
	TitleInactive:  lipgloss.Color("8"),
	Label:          lipgloss.Color("7"),
	Keycap:         lipgloss.Color("7"),
	KeyDesc:        lipgloss.Color("8"),
	TagDSDA:        lipgloss.Color("7"),
	TagUZDoom:      lipgloss.Color("7"),
	StatusReady:    lipgloss.Color("7"),
	StatusMissing:  lipgloss.Color("8"),
	Muted:          lipgloss.Color("8"),
}

// BuiltinThemes maps lowercase theme names and aliases to their built-in Theme definitions.
var BuiltinThemes = map[string]Theme{
	"default":    ClassicTheme,
	"classic":    ClassicTheme,
	"doom":       ClassicTheme,
	"blood":      BloodTheme,
	"crimson":    BloodTheme,
	"toxic":      ToxicTheme,
	"phobos":     ToxicTheme,
	"nukage":     ToxicTheme,
	"inferno":    InfernoTheme,
	"magma":      InfernoTheme,
	"frost":      FrostTheme,
	"nord":       FrostTheme,
	"cocytus":    FrostTheme,
	"plasma":     PlasmaTheme,
	"cyberpunk":  PlasmaTheme,
	"neon":       PlasmaTheme,
	"heretic":    HereticTheme,
	"dracula":    HereticTheme,
	"hexen":      HereticTheme,
	"amber":      AmberTheme,
	"crt":        AmberTheme,
	"vt220":      AmberTheme,
	"matrix":     AmberTheme,
	"sigil":      SigilTheme,
	"romero":     SigilTheme,
	"monochrome": MonochromeTheme,
	"mono":       MonochromeTheme,
}

// GetBuiltinTheme retrieves a built-in theme by case-insensitive name or alias.
func GetBuiltinTheme(name string) (Theme, bool) {
	t, ok := BuiltinThemes[strings.ToLower(strings.TrimSpace(name))]
	return t, ok
}

// ListBuiltinThemes returns an ordered list of all 10 canonical built-in themes.
func ListBuiltinThemes() []Theme {
	order := []string{
		"classic", "blood", "toxic", "inferno", "frost",
		"plasma", "heretic", "amber", "sigil", "monochrome",
	}
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
