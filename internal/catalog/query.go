package catalog

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// FindEngine resolves an engine by ID or alias (case-insensitive), falling back
// to the embedded curated catalog when c is nil or does not contain a match.
func (c *Catalog) FindEngine(key string) (EngineConfig, bool) {
	clean := strings.ToLower(strings.TrimSpace(key))
	if clean == "" {
		return EngineConfig{}, false
	}
	if c != nil && len(c.Engines) > 0 {
		if eng, ok := findEngineInMap(c.Engines, clean); ok {
			return eng, true
		}
	}
	_, curated, err := loadEmbeddedCurated()
	if err == nil && curated != nil && curated != c {
		if eng, ok := findEngineInMap(curated.Engines, clean); ok {
			return eng, true
		}
	}
	return EngineConfig{}, false
}

func findEngineInMap(engines map[string]EngineConfig, clean string) (EngineConfig, bool) {
	if eng, ok := engines[clean]; ok {
		return eng, true
	}
	keys := make([]string, 0, len(engines))
	for k := range engines {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	slug := Slug(clean)
	for _, k := range keys {
		eng := engines[k]
		if strings.EqualFold(k, clean) ||
			strings.EqualFold(eng.ID, clean) ||
			strings.EqualFold(eng.DisplayName, clean) ||
			(slug != "" && (strings.EqualFold(eng.ID, slug) || strings.EqualFold(Slug(eng.DisplayName), slug))) {
			return eng, true
		}
		for _, alias := range eng.Aliases {
			if strings.EqualFold(alias, clean) || (slug != "" && strings.EqualFold(Slug(alias), slug)) {
				return eng, true
			}
		}
	}
	return EngineConfig{}, false
}

// EngineDisplayName returns the human-friendly display name for engineKey.
func (c *Catalog) EngineDisplayName(engineKey string) string {
	if eng, ok := c.FindEngine(engineKey); ok {
		return eng.EffectiveDisplayName()
	}
	if strings.TrimSpace(engineKey) == "" {
		return "UZDoom"
	}
	return engineKey
}

// EngineBadge returns the short badge label for engineKey.
func (c *Catalog) EngineBadge(engineKey string) string {
	if eng, ok := c.FindEngine(engineKey); ok {
		return eng.BadgeText()
	}
	return (EngineConfig{ID: engineKey}).BadgeText()
}

func stripPresetExt(s string) string {
	lower := strings.TrimSpace(strings.ToLower(s))
	for _, ext := range []string{".wad", ".pk3", ".deh", ".bex"} {
		if strings.HasSuffix(lower, ext) && len(lower) > len(ext) {
			return strings.TrimSuffix(lower, ext)
		}
	}
	return lower
}

func tokenizePresetQuery(s string) []string {
	stripped := stripPresetExt(s)
	raw := strings.FieldsFunc(stripped, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(raw) == 0 {
		return nil
	}
	tokens := make([]string, len(raw))
	for i, tok := range raw {
		switch tok {
		case "iv":
			tokens[i] = "4"
		case "iii":
			tokens[i] = "3"
		case "ii":
			tokens[i] = "2"
		case "i":
			if len(raw) == 1 || i > 0 {
				tokens[i] = "1"
			} else {
				tokens[i] = tok
			}
		default:
			if len(raw) == 1 {
				switch {
				case strings.HasSuffix(tok, "iii") && len(tok) > 3:
					tok = strings.TrimSuffix(tok, "iii") + "3"
				case strings.HasSuffix(tok, "ii") && len(tok) > 2:
					tok = strings.TrimSuffix(tok, "ii") + "2"
				}
			}
			tokens[i] = tok
		}
	}
	return tokens
}

// NormalizePresetQuery normalizes a preset query string by stripping optional
// .wad, .pk3, .deh, or .bex extensions, tokenizing on word boundaries,
// converting Roman numerals ("iv", "iii", "ii", "i") to Arabic ("4", "3", "2", "1"),
// and joining tokens into a lowercase alphanumeric key.
func NormalizePresetQuery(s string) string {
	tokens := tokenizePresetQuery(s)
	if len(tokens) > 0 {
		return strings.Join(tokens, "")
	}
	return NormalizeFilename(stripPresetExt(s))
}

func presetWadStems(p Preset) []string {
	var stems []string
	add := func(raw string) {
		base := stripPresetExt(SlashBase(raw))
		if base == "" || base == "extras" {
			return
		}
		for _, candidate := range []string{NormalizeFilename(base), NormalizePresetQuery(base)} {
			if candidate == "" {
				continue
			}
			dup := false
			for _, prev := range stems {
				if prev == candidate {
					dup = true
					break
				}
			}
			if !dup {
				stems = append(stems, candidate)
			}
		}
	}
	if !p.HasRequiredFiles() {
		if strings.TrimSpace(p.IWAD) != "" {
			add(p.IWAD)
			if NormalizeFilename(stripPresetExt(SlashBase(p.IWAD))) == "doom" {
				add("doom1")
			}
		}
		return stems
	}
	for _, f := range p.Files {
		if f.Optional {
			continue
		}
		add(f.Name)
		for _, al := range f.Aliases {
			add(al)
		}
	}
	return stems
}

func firstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

func presetInitialisms(p Preset) (exact []string, prefix []string) {
	tokens := tokenizePresetQuery(p.Name)
	if len(tokens) < 2 {
		return nil, nil
	}
	var full strings.Builder
	for _, tok := range tokens {
		if r, ok := firstRune(tok); ok {
			full.WriteRune(r)
		}
	}
	exact = append(exact, full.String())

	// Episode shorthand (e.g. "Back to Saturn X: Episode 1" -> "btsx1" and series prefix "btsx")
	if len(tokens) >= 3 && tokens[len(tokens)-2] == "episode" {
		var epShort strings.Builder
		for i, tok := range tokens {
			if i == len(tokens)-2 {
				continue
			}
			if r, ok := firstRune(tok); ok {
				epShort.WriteRune(r)
			}
		}
		exact = append(exact, epShort.String())

		var basePrefix strings.Builder
		for _, tok := range tokens[:len(tokens)-2] {
			if r, ok := firstRune(tok); ok {
				basePrefix.WriteRune(r)
			}
		}
		if basePrefix.Len() >= 2 {
			prefix = append(prefix, basePrefix.String())
		}
	}

	// Edition suffix shorthand (e.g. "Going Down Turbo" -> "gd")
	if len(tokens) >= 3 && tokens[len(tokens)-1] == "turbo" {
		var baseShort strings.Builder
		for _, tok := range tokens[:len(tokens)-1] {
			if r, ok := firstRune(tok); ok {
				baseShort.WriteRune(r)
			}
		}
		if baseShort.Len() >= 2 {
			exact = append(exact, baseShort.String())
		}
	}

	return exact, prefix
}

func sliceContainsAny(haystack []string, a, b string) bool {
	for _, item := range haystack {
		if item != "" && (item == a || item == b) {
			return true
		}
	}
	return false
}

func containsTokenSequence(haystack, needle []string) bool {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return false
	}
	for start := 0; start <= len(haystack)-len(needle); start++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[start+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func bestPresetMatch(presets []Preset, matches []int) int {
	bestIdx := matches[0]
	bestTokens := len(tokenizePresetQuery(presets[bestIdx].Name))
	bestLen := len(NormalizeFilename(presets[bestIdx].Name))
	for _, idx := range matches[1:] {
		tokCount := len(tokenizePresetQuery(presets[idx].Name))
		normLen := len(NormalizeFilename(presets[idx].Name))
		if tokCount < bestTokens || (tokCount == bestTokens && normLen < bestLen) {
			bestIdx = idx
			bestTokens = tokCount
			bestLen = normLen
		}
	}
	return bestIdx
}

func (c *Catalog) findMatches(target string, allowFuzzy bool) []int {
	if c == nil {
		return nil
	}
	clean := strings.TrimSpace(strings.ToLower(target))
	if clean == "" {
		return nil
	}

	// Tier 1a: Exact ID match.
	for i := range c.Presets {
		if strings.ToLower(c.Presets[i].ID) == clean {
			return []int{i}
		}
	}

	// Tier 1b: Exact case-insensitive Name match.
	var matches []int
	for i := range c.Presets {
		if strings.ToLower(c.Presets[i].Name) == clean {
			matches = append(matches, i)
		}
	}
	if len(matches) > 0 {
		return matches
	}

	stripped := stripPresetExt(clean)
	normTarget := NormalizeFilename(stripped)
	romanTarget := NormalizePresetQuery(clean)
	if normTarget == "" {
		return nil
	}

	// Tier 2a: Exact normalized or Roman-normalized Name or ID.
	for i := range c.Presets {
		normName := NormalizeFilename(c.Presets[i].Name)
		normID := NormalizeFilename(c.Presets[i].ID)
		if normName == normTarget || normID == normTarget ||
			NormalizePresetQuery(c.Presets[i].Name) == romanTarget ||
			NormalizePresetQuery(c.Presets[i].ID) == romanTarget {
			matches = append(matches, i)
		}
	}
	if len(matches) > 0 {
		return matches
	}

	// Tier 2b: Canonical WAD filename stem or FileSpec alias.
	for i := range c.Presets {
		if sliceContainsAny(presetWadStems(c.Presets[i]), normTarget, romanTarget) {
			matches = append(matches, i)
		}
	}
	if len(matches) > 0 {
		return matches
	}

	// Tier 2c: Implicit "doom" prefix (e.g. "II", "ii", "2" -> "doom-ii"; "1", "i" -> "doom"; "zero" -> "doom-zero").
	doomNorm := "doom" + normTarget
	doomRoman := "doom" + romanTarget
	for i := range c.Presets {
		if romanTarget == "1" && strings.ToLower(c.Presets[i].ID) == "doom" {
			matches = append(matches, i)
			continue
		}
		normName := NormalizeFilename(c.Presets[i].Name)
		normID := NormalizeFilename(c.Presets[i].ID)
		if normName == doomNorm || normID == doomNorm ||
			NormalizePresetQuery(c.Presets[i].Name) == doomRoman ||
			NormalizePresetQuery(c.Presets[i].ID) == doomRoman {
			matches = append(matches, i)
		}
	}
	if len(matches) > 0 {
		return matches
	}

	// Tier 2d: Multi-word initialisms / acronyms (e.g. "av", "aa", "btsx1", "btsx2", "nrftl", "sod", "lor", "gd").
	for i := range c.Presets {
		exactInit, _ := presetInitialisms(c.Presets[i])
		if sliceContainsAny(exactInit, normTarget, romanTarget) {
			matches = append(matches, i)
		}
	}
	if len(matches) > 0 {
		return matches
	}

	// Tier 3: Whole-word token sequence or series-prefix initialism (e.g. "evilution", "revilution", "btsx").
	queryTokens := tokenizePresetQuery(clean)
	if len(queryTokens) > 0 && len(normTarget) >= 3 {
		for i := range c.Presets {
			nameTokens := tokenizePresetQuery(c.Presets[i].Name)
			idTokens := tokenizePresetQuery(c.Presets[i].ID)
			_, prefixInit := presetInitialisms(c.Presets[i])
			if containsTokenSequence(nameTokens, queryTokens) ||
				containsTokenSequence(idTokens, queryTokens) ||
				sliceContainsAny(prefixInit, normTarget, romanTarget) {
				matches = append(matches, i)
			}
		}
		if len(matches) > 0 {
			if allowFuzzy && len(matches) > 1 {
				return []int{bestPresetMatch(c.Presets, matches)}
			}
			return matches
		}
	}

	if !allowFuzzy || len(normTarget) < 3 {
		return nil
	}

	// Tier 4a: Prefix match on Name, ID, or normalized Name/ID (preferring shortest preset name).
	for i := range c.Presets {
		normName := NormalizeFilename(c.Presets[i].Name)
		normID := NormalizeFilename(c.Presets[i].ID)
		if strings.HasPrefix(strings.ToLower(c.Presets[i].Name), clean) ||
			strings.HasPrefix(strings.ToLower(c.Presets[i].ID), clean) ||
			strings.HasPrefix(normName, normTarget) ||
			strings.HasPrefix(normID, normTarget) ||
			strings.HasPrefix(NormalizePresetQuery(c.Presets[i].Name), romanTarget) {
			matches = append(matches, i)
		}
	}
	if len(matches) > 0 {
		return []int{bestPresetMatch(c.Presets, matches)}
	}

	// Tier 4b: Substring match on normalized Name/ID (preferring shortest preset name).
	for i := range c.Presets {
		normName := NormalizeFilename(c.Presets[i].Name)
		normID := NormalizeFilename(c.Presets[i].ID)
		if strings.Contains(normName, normTarget) ||
			strings.Contains(normID, normTarget) ||
			strings.Contains(NormalizePresetQuery(c.Presets[i].Name), romanTarget) {
			matches = append(matches, i)
		}
	}
	if len(matches) > 0 {
		return []int{bestPresetMatch(c.Presets, matches)}
	}

	return nil
}

// Find searches for a preset using deterministic tiered resolution: exact ID or name,
// normalized or Roman-numeral match, canonical WAD filename stem, implicit "doom" prefix,
// multi-word initialism, whole-word token sequence, or prefix/substring fallback.
func (c *Catalog) Find(target string) *Preset {
	matches := c.findMatches(target, true)
	if len(matches) == 0 {
		return nil
	}
	return &c.Presets[matches[0]]
}

// FindExact resolves a preset by exact ID, unambiguous case-insensitive Name,
// unambiguous normalized/Roman-numeral match, canonical WAD stem, implicit "doom" prefix,
// multi-word initialism, or unambiguous whole-word token match.
// Partial-word prefix and substring matches are not accepted, making FindExact safe for mutating commands.
func (c *Catalog) FindExact(target string) (*Preset, error) {
	if c == nil {
		return nil, fmt.Errorf("preset %q not found", target)
	}
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("empty preset name")
	}
	matches := c.findMatches(target, false)
	if len(matches) == 1 {
		return &c.Presets[matches[0]], nil
	}
	if len(matches) > 1 {
		ids := make([]string, len(matches))
		for i, idx := range matches {
			ids[i] = c.Presets[idx].ID
		}
		return nil, fmt.Errorf("preset name %q is ambiguous; specify id (%s)", target, strings.Join(ids, ", "))
	}
	return nil, fmt.Errorf("preset %q not found", target)
}
