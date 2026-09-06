// Package tui provides terminal styling, theming, and interactive launcher views.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
