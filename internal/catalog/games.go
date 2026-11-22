package catalog

import (
	"sort"
	"strings"
	"unicode"
)

// CoreGame identifies the primary Doom franchise family (Ultimate Doom, Doom II, and Final Doom).
const CoreGame = "doom"

// InferGameFromIWAD derives the canonical lowercase game slug from an IWAD filename.
// Doom-family IWADs (DOOM.WAD, DOOM2.WAD, TNT.WAD, PLUTONIA.WAD, Freedoom) and empty strings
// resolve to CoreGame ("doom").
func InferGameFromIWAD(iwad string) string {
	base := SlashBase(iwad)
	norm := NormalizeFilename(base)
	switch norm {
	case "", "doom.wad", "doom1.wad", "doom2.wad", "tnt.wad", "plutonia.wad",
		"freedoom1.wad", "freedoom2.wad":
		return CoreGame
	case "heretic.wad":
		return "heretic"
	case "hexen.wad":
		return "hexen"
	case "strife1.wad", "strife.wad":
		return "strife"
	default:
		stem := strings.TrimSuffix(base, slashExt(base))
		if slug := Slug(stem); slug != "" {
			return slug
		}
		return CoreGame
	}
}

// EffectiveGame returns the preset's explicit lowercase game slug when set,
// falling back to InferGameFromIWAD(p.IWAD).
func (p Preset) EffectiveGame() string {
	if g := strings.ToLower(strings.TrimSpace(p.Game)); g != "" {
		return g
	}
	return InferGameFromIWAD(p.IWAD)
}

// IsCoreGame reports whether p belongs to the core Doom game family ("doom").
func (p Preset) IsCoreGame() bool {
	return p.EffectiveGame() == CoreGame
}

// GameDisplayName returns a human-friendly title for a game slug.
func GameDisplayName(game string) string {
	clean := strings.ToLower(strings.TrimSpace(game))
	switch clean {
	case "", CoreGame:
		return "Doom"
	case "heretic":
		return "Heretic"
	case "hexen":
		return "Hexen"
	case "strife":
		return "Strife"
	default:
		parts := strings.Split(clean, "-")
		for i, part := range parts {
			runes := []rune(part)
			if len(runes) > 0 {
				runes[0] = unicode.ToUpper(runes[0])
				parts[i] = string(runes)
			}
		}
		return strings.Join(parts, " ")
	}
}

// OptionalGames returns the sorted distinct non-core game slugs declared across c.Presets.
func (c *Catalog) OptionalGames() []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]bool)
	var games []string
	for _, p := range c.Presets {
		g := p.EffectiveGame()
		if g == "" || g == CoreGame || seen[g] {
			continue
		}
		seen[g] = true
		games = append(games, g)
	}
	sort.Strings(games)
	return games
}

// InstalledOptionalGames returns the sorted distinct optional game slugs whose base IWAD
// resolves on disk in idx.
func (c *Catalog) InstalledOptionalGames(idx *DirIndex) []string {
	if c == nil || idx == nil {
		return nil
	}
	seen := make(map[string]bool)
	var games []string
	for _, p := range c.Presets {
		g := p.EffectiveGame()
		if g == "" || g == CoreGame || seen[g] || strings.TrimSpace(p.IWAD) == "" {
			continue
		}
		if _, ok := idx.ResolveFile(p.IWAD); ok {
			seen[g] = true
			games = append(games, g)
		}
	}
	sort.Strings(games)
	return games
}

// FilterByGames returns a deep-cloned Catalog retaining all engines, diagnostics, custom user
// presets, and curated presets whose EffectiveGame is marked true in allowed.
func (c *Catalog) FilterByGames(allowed map[string]bool) *Catalog {
	if c == nil {
		return nil
	}
	out := &Catalog{
		Engines:     make(map[string]EngineConfig, len(c.Engines)),
		Diagnostics: append([]Diagnostic(nil), c.Diagnostics...),
	}
	for k, e := range c.Engines {
		out.Engines[k] = cloneEngineConfig(e)
	}
	for _, p := range c.Presets {
		if p.IsCustom() || allowed[p.EffectiveGame()] {
			out.Presets = append(out.Presets, *clonePreset(p))
		}
	}
	return out
}

// ForPresets constructs a Catalog containing deep clones of presets alongside c's engines.
func (c *Catalog) ForPresets(presets ...Preset) *Catalog {
	out := &Catalog{
		Engines: make(map[string]EngineConfig),
		Presets: make([]Preset, len(presets)),
	}
	if c != nil {
		for k, e := range c.Engines {
			out.Engines[k] = cloneEngineConfig(e)
		}
	}
	for i, p := range presets {
		out.Presets[i] = *clonePreset(p)
	}
	return out
}

// IsPresetInLibrary reports whether p belongs in the default installed-only library view:
// custom user presets are always shown, and curated presets are shown when installed in idx.
func IsPresetInLibrary(p Preset, idx *DirIndex) bool {
	return p.IsCustom() || (idx != nil && idx.IsPresetInstalled(p))
}

// LibraryPresets returns the ordered slice of presets visible in the default installed-only
// library view and the count of hidden uninstalled curated presets.
func (c *Catalog) LibraryPresets(idx *DirIndex) ([]Preset, int) {
	if c == nil {
		return nil, 0
	}
	var visible []Preset
	hidden := 0
	for _, p := range c.Presets {
		if IsPresetInLibrary(p, idx) {
			visible = append(visible, *clonePreset(p))
		} else {
			hidden++
		}
	}
	return visible, hidden
}

// FilterLibrary returns a cloned Catalog restricted to LibraryPresets(idx) along with
// the hidden uninstalled preset count.
func (c *Catalog) FilterLibrary(idx *DirIndex) (*Catalog, int) {
	if c == nil {
		return nil, 0
	}
	visible, hidden := c.LibraryPresets(idx)
	out := &Catalog{
		Engines:     make(map[string]EngineConfig, len(c.Engines)),
		Presets:     visible,
		Diagnostics: append([]Diagnostic(nil), c.Diagnostics...),
	}
	for k, e := range c.Engines {
		out.Engines[k] = cloneEngineConfig(e)
	}
	return out, hidden
}
