// Package preset manages the declarative entity catalog, layering, lookup,
// file resolution, linting, and README table synchronization.
package preset

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/lock14/doom-cli"
)

// SchemaVersion is the supported entity schema version for catalog JSON files.
const SchemaVersion = 1

// Kind discriminates the entity types stored in catalog files.
type Kind string

// Supported entity kinds.
const (
	KindEngine   Kind = "engine"
	KindPreset   Kind = "preset"
	KindOverride Kind = "override"
)

// Layer identifies the precedence layer an entity originated from.
type Layer int

// Catalog precedence layers, lowest precedence first.
const (
	LayerCurated Layer = iota
	LayerUser
)

var idSlugRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateID verifies that id is a valid lowercase ASCII slug.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("empty id")
	}
	if !idSlugRegex.MatchString(id) {
		return fmt.Errorf("invalid id %q (must match %s)", id, idSlugRegex.String())
	}
	return nil
}

// Slug normalizes a display name into a lowercase ASCII hyphenated identifier.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// PinnedRelease describes a fallback release asset with optional SHA-256 verification.
type PinnedRelease struct {
	Version string `json:"version,omitempty"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256,omitempty"`
}

// PlatformRecipe defines how to locate or pin a release asset for a specific GOOS/GOARCH.
type PlatformRecipe struct {
	Assets      []string       `json:"assets,omitempty"`
	Pinned      *PinnedRelease `json:"pinned,omitempty"`
	Unsupported bool           `json:"unsupported,omitempty"`
}

// InstallRecipe defines declarative GitHub release installation rules for an engine.
type InstallRecipe struct {
	Repo      string                    `json:"repo,omitempty"`
	Platforms map[string]PlatformRecipe `json:"platforms,omitempty"`
}

// EngineConfig describes a Doom source port engine entity.
type EngineConfig struct {
	SchemaVersion int            `json:"schema_version,omitempty"`
	Kind          Kind           `json:"kind"`
	ID            string         `json:"id"`
	DisplayName   string         `json:"display_name,omitempty"`
	Description   string         `json:"description,omitempty"`
	Family        string         `json:"family,omitempty"`
	ArgsStyle     string         `json:"args_style,omitempty"` // "boom" or "zdoom"
	Binary        string         `json:"binary,omitempty"`
	PrefixArgs    []string       `json:"prefix_args,omitempty"`
	DefaultArgs   []string       `json:"default_args,omitempty"`
	Aliases       []string       `json:"aliases,omitempty"`
	Badge         string         `json:"badge,omitempty"`
	Install       *InstallRecipe `json:"install,omitempty"`

	Layer      Layer  `json:"-"`
	Overridden bool   `json:"-"`
	Path       string `json:"-"`
}

// EffectiveDisplayName returns DisplayName when set, falling back to ID.
func (e EngineConfig) EffectiveDisplayName() string {
	if strings.TrimSpace(e.DisplayName) != "" {
		return e.DisplayName
	}
	return e.ID
}

// EffectiveBinary returns Binary when set, falling back to ID.
func (e EngineConfig) EffectiveBinary() string {
	if strings.TrimSpace(e.Binary) != "" {
		return e.Binary
	}
	return e.ID
}

// BadgeText returns the short badge text for TUI and CLI listings.
// If Badge is empty, it derives an uppercase prefix of up to 6 runes from ID.
func (e EngineConfig) BadgeText() string {
	if strings.TrimSpace(e.Badge) != "" {
		return e.Badge
	}
	base := e.ID
	if idx := strings.IndexByte(base, '-'); idx > 0 && idx <= 6 {
		base = base[:idx]
	}
	upper := strings.ToUpper(base)
	runes := []rune(upper)
	if len(runes) > 6 {
		return string(runes[:6])
	}
	return upper
}

// EffectiveArgsStyle returns "boom" or "zdoom", inferring from engine ID or family if unset.
func (e EngineConfig) EffectiveArgsStyle() string {
	style := strings.ToLower(strings.TrimSpace(e.ArgsStyle))
	if style == "boom" || style == "zdoom" {
		return style
	}
	return GuessArgsStyle(e.ID, e.Family)
}

// FormatLabel returns a human-readable engine label combining EffectiveDisplayName
// and Description when present.
func (e EngineConfig) FormatLabel() string {
	disp := e.EffectiveDisplayName()
	desc := strings.TrimSpace(e.Description)
	if desc == "" || strings.EqualFold(desc, disp) {
		return disp
	}
	if openIdx := strings.Index(desc, " ("); openIdx >= 0 && strings.HasSuffix(desc, ")") {
		subtitle := strings.TrimSpace(desc[openIdx+2 : len(desc)-1])
		if subtitle == "" || strings.EqualFold(subtitle, disp) {
			return disp
		}
		return fmt.Sprintf("%s (%s)", disp, subtitle)
	}
	return fmt.Sprintf("%s (%s)", disp, desc)
}

// GuessArgsStyle infers "boom" or "zdoom" from an engine identifier and family string.
func GuessArgsStyle(idOrName, family string) string {
	name := strings.ToLower(idOrName)
	fam := strings.ToLower(family)
	if strings.Contains(name, "dsda") ||
		strings.Contains(name, "boom") ||
		strings.Contains(name, "woof") ||
		strings.Contains(name, "crispy") ||
		strings.Contains(name, "chocolate") ||
		strings.Contains(name, "mbf") ||
		strings.Contains(fam, "boom") ||
		strings.Contains(fam, "mbf") {
		return "boom"
	}
	return "zdoom"
}

// FileSpec describes a single WAD, PK3, or DeHackEd file in a preset's load order.
type FileSpec struct {
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

// Source describes a downloadable archive or file mirror for a preset.
type Source struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
}

// Link describes a human-facing informational URL (such as an idgames page).
type Link struct {
	URL string `json:"url"`
	Rel string `json:"rel,omitempty"`
}

// Preset defines a single launchable Doom mapset or game configuration.
type Preset struct {
	SchemaVersion     int        `json:"schema_version,omitempty"`
	Kind              Kind       `json:"kind"`
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Engine            string     `json:"engine"`
	IWAD              string     `json:"iwad"`
	Files             []FileSpec `json:"files,omitempty"`
	Args              []string   `json:"args,omitempty"`
	Category          string     `json:"category,omitempty"`
	Genre             string     `json:"genre,omitempty"`
	Difficulty        string     `json:"difficulty,omitempty"`
	Quality           string     `json:"quality,omitempty"`
	Compatibility     string     `json:"compatibility,omitempty"`
	Description       string     `json:"description,omitempty"`
	Author            string     `json:"author,omitempty"`
	ReleaseDate       string     `json:"release_date,omitempty"`
	Sources           []Source   `json:"sources,omitempty"`
	Links             []Link     `json:"links,omitempty"`
	LoadMapsAfterMods bool       `json:"load_maps_after_mods,omitempty"`
	Sort              int        `json:"sort,omitempty"`

	Layer            Layer  `json:"-"`
	Overridden       bool   `json:"-"`
	EngineOverridden bool   `json:"-"`
	Path             string `json:"-"`
}

// IsCustom reports whether the preset originated from the user layer.
func (p Preset) IsCustom() bool {
	return p.Layer == LayerUser
}

// HasRequiredFiles reports whether the preset declares at least one non-optional file.
func (p Preset) HasRequiredFiles() bool {
	for _, f := range p.Files {
		if !f.Optional {
			return true
		}
	}
	return false
}

// FileNames returns the ordered slice of file names declared on the preset.
func (p Preset) FileNames() []string {
	if len(p.Files) == 0 {
		return nil
	}
	out := make([]string, len(p.Files))
	for i, f := range p.Files {
		out[i] = f.Name
	}
	return out
}

// PrimaryURL returns the first non-empty informational link URL on the preset, or "".
func (p Preset) PrimaryURL() string {
	for _, l := range p.Links {
		if u := strings.TrimSpace(l.URL); u != "" {
			return u
		}
	}
	return ""
}

// OverrideTarget identifies the entity an Override applies to.
type OverrideTarget struct {
	Kind Kind   `json:"kind"`
	ID   string `json:"id"`
}

// OverrideSet holds optional replacement fields for a preset or engine override.
type OverrideSet struct {
	Engine            *string     `json:"engine,omitempty"`
	IWAD              *string     `json:"iwad,omitempty"`
	Files             *[]FileSpec `json:"files,omitempty"`
	Args              *[]string   `json:"args,omitempty"`
	Category          *string     `json:"category,omitempty"`
	Genre             *string     `json:"genre,omitempty"`
	Difficulty        *string     `json:"difficulty,omitempty"`
	Quality           *string     `json:"quality,omitempty"`
	Compatibility     *string     `json:"compatibility,omitempty"`
	Description       *string     `json:"description,omitempty"`
	Author            *string     `json:"author,omitempty"`
	ReleaseDate       *string     `json:"release_date,omitempty"`
	Sources           *[]Source   `json:"sources,omitempty"`
	Links             *[]Link     `json:"links,omitempty"`
	LoadMapsAfterMods *bool       `json:"load_maps_after_mods,omitempty"`
	Binary            *string     `json:"binary,omitempty"`
	ArgsStyle         *string     `json:"args_style,omitempty"`
	DefaultArgs       *[]string   `json:"default_args,omitempty"`
	PrefixArgs        *[]string   `json:"prefix_args,omitempty"`
	Badge             *string     `json:"badge,omitempty"`
	DisplayName       *string     `json:"display_name,omitempty"`
}

// Override patches a lower-layer preset or engine without replacing its base file.
type Override struct {
	SchemaVersion int            `json:"schema_version,omitempty"`
	Kind          Kind           `json:"kind"`
	Target        OverrideTarget `json:"target"`
	Set           *OverrideSet   `json:"set,omitempty"`
	AppendFiles   []FileSpec     `json:"append_files,omitempty"`
	AppendArgs    []string       `json:"append_args,omitempty"`
	ExcludeFiles  []string       `json:"exclude_files,omitempty"`

	Path string `json:"-"`
}

// Entity wraps a decoded catalog entity; exactly one pointer is non-nil.
type Entity struct {
	Engine   *EngineConfig
	Preset   *Preset
	Override *Override
	Path     string
}

// Diagnostic represents a validation or quarantine message produced during loading or linting.
type Diagnostic struct {
	Path     string
	Severity string // "error" or "warn"
	Message  string
}

func (d Diagnostic) String() string {
	if d.Path != "" {
		return fmt.Sprintf("%s: %s: %s", d.Severity, d.Path, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Severity, d.Message)
}

// Catalog holds the merged engines, ordered presets, and any load diagnostics.
type Catalog struct {
	Engines     map[string]EngineConfig
	Presets     []Preset
	Diagnostics []Diagnostic
}

func cloneFileSpecs(files []FileSpec) []FileSpec {
	if len(files) == 0 {
		return nil
	}
	out := make([]FileSpec, len(files))
	for i, f := range files {
		fc := f
		fc.Aliases = append([]string(nil), f.Aliases...)
		out[i] = fc
	}
	return out
}

// Clone returns a deep copy of the Catalog.
func (c *Catalog) Clone() *Catalog {
	if c == nil {
		return nil
	}
	out := &Catalog{
		Engines:     make(map[string]EngineConfig, len(c.Engines)),
		Presets:     make([]Preset, len(c.Presets)),
		Diagnostics: append([]Diagnostic(nil), c.Diagnostics...),
	}
	for k, e := range c.Engines {
		ec := e
		ec.PrefixArgs = append([]string(nil), e.PrefixArgs...)
		ec.DefaultArgs = append([]string(nil), e.DefaultArgs...)
		ec.Aliases = append([]string(nil), e.Aliases...)
		out.Engines[k] = ec
	}
	for i, p := range c.Presets {
		pc := p
		pc.Files = cloneFileSpecs(p.Files)
		pc.Args = append([]string(nil), p.Args...)
		pc.Sources = append([]Source(nil), p.Sources...)
		pc.Links = append([]Link(nil), p.Links...)
		out.Presets[i] = pc
	}
	return out
}

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
	slug := Slug(clean)
	for k, eng := range engines {
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
		base := stripPresetExt(filepath.Base(strings.TrimSpace(raw)))
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
			if NormalizeFilename(stripPresetExt(filepath.Base(p.IWAD))) == "doom" {
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

// DecodeEntity strictly decodes a single JSON entity document.
// Unknown fields, trailing tokens, and unsupported schema versions are rejected.
func DecodeEntity(data []byte) (Entity, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return Entity{}, fmt.Errorf("empty entity file")
	}
	if data[0] != '{' {
		return Entity{}, fmt.Errorf("expected JSON object")
	}

	var head struct {
		SchemaVersion *int `json:"schema_version"`
		Kind          Kind `json:"kind"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return Entity{}, err
	}
	if head.SchemaVersion != nil && *head.SchemaVersion != SchemaVersion {
		return Entity{}, fmt.Errorf("unsupported schema_version %d (want %d)", *head.SchemaVersion, SchemaVersion)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var ent Entity
	var err error
	switch head.Kind {
	case KindEngine:
		e := &EngineConfig{}
		err = dec.Decode(e)
		if err == nil {
			if e.SchemaVersion == 0 {
				e.SchemaVersion = SchemaVersion
			}
			ent.Engine = e
		}
	case KindPreset:
		p := &Preset{}
		err = dec.Decode(p)
		if err == nil {
			if p.SchemaVersion == 0 {
				p.SchemaVersion = SchemaVersion
			}
			ent.Preset = p
		}
	case KindOverride:
		o := &Override{}
		err = dec.Decode(o)
		if err == nil {
			if o.SchemaVersion == 0 {
				o.SchemaVersion = SchemaVersion
			}
			ent.Override = o
		}
	case "":
		return Entity{}, fmt.Errorf("missing required field \"kind\"")
	default:
		return Entity{}, fmt.Errorf("unknown entity kind %q", head.Kind)
	}
	if err != nil {
		return Entity{}, err
	}
	if tok, tailErr := dec.Token(); tailErr != io.EOF {
		return Entity{}, fmt.Errorf("unexpected trailing JSON token %v", tok)
	}
	if err := validateDecodedEntity(ent); err != nil {
		return Entity{}, err
	}
	return ent, nil
}

func validateDecodedEntity(ent Entity) error {
	switch {
	case ent.Engine != nil:
		if err := ValidateID(ent.Engine.ID); err != nil {
			return fmt.Errorf("engine id: %w", err)
		}
		if ent.Engine.ArgsStyle != "" && ent.Engine.ArgsStyle != "boom" && ent.Engine.ArgsStyle != "zdoom" {
			return fmt.Errorf(
				"engine %s: args_style %q must be \"boom\" or \"zdoom\"",
				ent.Engine.ID, ent.Engine.ArgsStyle,
			)
		}
	case ent.Preset != nil:
		if err := ValidateID(ent.Preset.ID); err != nil {
			return fmt.Errorf("preset id: %w", err)
		}
		if strings.TrimSpace(ent.Preset.Name) == "" {
			return fmt.Errorf("preset %s: missing name", ent.Preset.ID)
		}
		if strings.TrimSpace(ent.Preset.IWAD) == "" {
			return fmt.Errorf("preset %s: missing iwad", ent.Preset.ID)
		}
		if strings.TrimSpace(ent.Preset.Engine) == "" {
			return fmt.Errorf("preset %s: missing engine", ent.Preset.ID)
		}
	case ent.Override != nil:
		if ent.Override.Target.Kind != KindPreset && ent.Override.Target.Kind != KindEngine {
			return fmt.Errorf("override target kind %q must be \"preset\" or \"engine\"", ent.Override.Target.Kind)
		}
		if err := ValidateID(ent.Override.Target.ID); err != nil {
			return fmt.Errorf("override target id: %w", err)
		}
		if ent.Override.Set != nil && ent.Override.Set.ArgsStyle != nil {
			style := *ent.Override.Set.ArgsStyle
			if style != "" && style != "boom" && style != "zdoom" {
				return fmt.Errorf(
					"override %s/%s: args_style %q must be \"boom\" or \"zdoom\"",
					ent.Override.Target.Kind, ent.Override.Target.ID, style,
				)
			}
		}
	}
	return nil
}

// ExpectedFilename returns the canonical filename (<kind>.<id>.json or
// override.<target.kind>.<target.id>.json) for the given entity.
func ExpectedFilename(ent Entity) string {
	switch {
	case ent.Engine != nil:
		return fmt.Sprintf("engine.%s.json", ent.Engine.ID)
	case ent.Preset != nil:
		return fmt.Sprintf("preset.%s.json", ent.Preset.ID)
	case ent.Override != nil:
		return fmt.Sprintf("override.%s.%s.json", ent.Override.Target.Kind, ent.Override.Target.ID)
	default:
		return ""
	}
}

type dirScanOptions struct {
	displayPrefix      string
	strict             bool
	checkPersonalPaths bool
	severity           string
	quarantinePrefix   bool
}

func scanEntityFS(fsys fs.FS, opts dirScanOptions) ([]Entity, []Diagnostic, error) {
	severity := opts.severity
	if severity == "" {
		severity = "warn"
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		if opts.strict {
			return nil, nil, err
		}
		return nil, []Diagnostic{{
			Path:     opts.displayPrefix,
			Severity: severity,
			Message:  err.Error(),
		}}, nil
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var entities []Entity
	var diags []Diagnostic
	for _, name := range names {
		where := name
		if opts.displayPrefix != "" && opts.displayPrefix != "." {
			where = path.Join(opts.displayPrefix, name)
		}
		raw, readErr := fs.ReadFile(fsys, name)
		if readErr != nil {
			if opts.strict {
				return nil, nil, fmt.Errorf("%s: %w", where, readErr)
			}
			diags = append(diags, Diagnostic{Path: where, Severity: severity, Message: readErr.Error()})
			continue
		}
		if opts.checkPersonalPaths {
			if match := personalPathRegex.FindString(string(raw)); match != "" {
				diags = append(diags, Diagnostic{
					Path:     where,
					Severity: severity,
					Message:  fmt.Sprintf("contains hardcoded personal path %q", match),
				})
			}
		}
		ent, decErr := DecodeEntity(raw)
		if decErr != nil {
			if opts.strict {
				return nil, nil, fmt.Errorf("%s: %w", where, decErr)
			}
			msg := decErr.Error()
			if opts.quarantinePrefix {
				msg = fmt.Sprintf("quarantined invalid entity file (%v)", decErr)
			}
			diags = append(diags, Diagnostic{Path: where, Severity: severity, Message: msg})
			continue
		}
		wantName := ExpectedFilename(ent)
		if name != wantName {
			msg := fmt.Sprintf("filename %q does not match entity identity (want %q)", name, wantName)
			if opts.strict {
				return nil, nil, fmt.Errorf("%s: %s", where, msg)
			}
			if opts.quarantinePrefix {
				msg = "quarantined: " + msg
			}
			diags = append(diags, Diagnostic{Path: where, Severity: severity, Message: msg})
			continue
		}
		ent.Path = where
		entities = append(entities, ent)
	}
	return entities, diags, nil
}

var (
	curatedOnce      sync.Once
	cachedCurated    *Catalog
	cachedEnts       []Entity
	cachedCuratedErr error
)

func loadEmbeddedCurated() ([]Entity, *Catalog, error) {
	curatedOnce.Do(func() {
		sub, err := fs.Sub(doomcli.FS, "catalog")
		if err != nil {
			cachedCuratedErr = err
			return
		}
		ents, _, err := scanEntityFS(sub, dirScanOptions{
			displayPrefix: "catalog",
			strict:        true,
			severity:      "error",
		})
		if err != nil {
			cachedCuratedErr = err
			return
		}
		cachedEnts = ents
		cachedCurated = buildCatalog(ents, nil)
	})
	return cachedEnts, cachedCurated, cachedCuratedErr
}

// LoadCuratedCatalog returns a fresh clone of the embedded curated catalog entities.
func LoadCuratedCatalog() (*Catalog, error) {
	_, cat, err := loadEmbeddedCurated()
	if err != nil {
		return nil, err
	}
	return cat.Clone(), nil
}

// LoadCatalogFromDir loads a curated catalog from an on-disk directory (such as "./catalog").
func LoadCatalogFromDir(dir string) (*Catalog, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	curatedEnts, _, err := scanEntityFS(os.DirFS(dir), dirScanOptions{
		displayPrefix: filepath.ToSlash(dir),
		strict:        true,
		severity:      "error",
	})
	if err != nil {
		return nil, err
	}
	return buildCatalog(curatedEnts, nil), nil
}

// LoadLayeredCatalog loads the embedded curated catalog and merges user entities from userPath.
// When explicitPath is true, a non-existent userPath returns an error instead of being treated as empty.
func LoadLayeredCatalog(userPath string, explicitPath bool) (*Catalog, error) {
	curatedEnts, curatedCat, err := loadEmbeddedCurated()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userPath) == "" {
		return curatedCat.Clone(), nil
	}

	info, statErr := os.Stat(userPath)
	if statErr != nil {
		if os.IsNotExist(statErr) && !explicitPath {
			return curatedCat.Clone(), nil
		}
		return nil, statErr
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", userPath)
	}

	userEnts, diags, loadErr := scanEntityFS(os.DirFS(userPath), dirScanOptions{
		displayPrefix:    filepath.ToSlash(userPath),
		strict:           false,
		severity:         "warn",
		quarantinePrefix: true,
	})
	if loadErr != nil {
		return nil, loadErr
	}

	if len(userEnts) == 0 && len(diags) == 0 {
		return curatedCat.Clone(), nil
	}

	cat := buildCatalog(curatedEnts, userEnts)
	cat.Diagnostics = append(diags, cat.Diagnostics...)
	return cat, nil
}

// MergeUserDir merges user entities and overrides from userDir onto a clone of base
// (or the embedded curated catalog when base is nil).
func MergeUserDir(base *Catalog, userDir string) (*Catalog, error) {
	if base == nil {
		return LoadLayeredCatalog(userDir, false)
	}
	curatedEnts, curatedCat, err := loadEmbeddedCurated()
	if err != nil {
		return nil, err
	}

	var userEnts []Entity
	var diags []Diagnostic
	if strings.TrimSpace(userDir) != "" {
		if info, statErr := os.Stat(userDir); statErr == nil {
			if !info.IsDir() {
				return nil, fmt.Errorf("%s is not a directory", userDir)
			}
			userEnts, diags, _ = scanEntityFS(os.DirFS(userDir), dirScanOptions{
				displayPrefix:    filepath.ToSlash(userDir),
				strict:           false,
				severity:         "warn",
				quarantinePrefix: true,
			})
		}
	}

	diskUserEngines := make(map[string]bool)
	diskUserPresets := make(map[string]bool)
	for _, ent := range userEnts {
		if ent.Engine != nil {
			diskUserEngines[ent.Engine.ID] = true
		}
		if ent.Preset != nil {
			diskUserPresets[ent.Preset.ID] = true
		}
	}

	var baseEnts []Entity
	seenEngines := make(map[string]bool)
	for _, ent := range curatedEnts {
		if ent.Engine != nil {
			ec := *ent.Engine
			baseEnts = append(baseEnts, Entity{Engine: &ec, Path: ent.Path})
			seenEngines[ec.ID] = true
		}
	}
	for id, eng := range base.Engines {
		ec := eng
		if seenEngines[id] {
			if !eng.Overridden {
				for i := range baseEnts {
					if baseEnts[i].Engine != nil && baseEnts[i].Engine.ID == id {
						baseEnts[i].Engine = &ec
						break
					}
				}
			}
		} else if eng.Layer == LayerUser {
			if !diskUserEngines[id] && !eng.Overridden {
				userEnts = append(userEnts, Entity{Engine: &ec, Path: eng.Path})
			}
		} else {
			baseEnts = append(baseEnts, Entity{Engine: &ec, Path: eng.Path})
		}
	}
	for i, p := range base.Presets {
		if p.Layer == LayerUser {
			if !diskUserPresets[p.ID] && !p.Overridden {
				pc := *clonePreset(p)
				userEnts = append(userEnts, Entity{Preset: &pc, Path: p.Path})
			}
			continue
		}
		var pc Preset
		if p.Overridden && curatedCat != nil {
			if cp, findErr := curatedCat.FindExact(p.ID); findErr == nil && cp != nil {
				pc = *clonePreset(*cp)
			} else {
				pc = *clonePreset(p)
				pc.Overridden = false
				pc.EngineOverridden = false
			}
		} else {
			pc = *clonePreset(p)
		}
		if pc.Sort == 0 {
			pc.Sort = i + 1
		}
		baseEnts = append(baseEnts, Entity{Preset: &pc, Path: pc.Path})
	}

	cat := buildCatalog(baseEnts, userEnts)
	cat.Diagnostics = append(diags, cat.Diagnostics...)
	return cat, nil
}

func buildCatalog(curatedEnts, userEnts []Entity) *Catalog {
	cat := &Catalog{
		Engines: make(map[string]EngineConfig),
	}

	type batch struct {
		layer Layer
		ents  []Entity
	}
	batches := []batch{
		{layer: LayerCurated, ents: curatedEnts},
		{layer: LayerUser, ents: userEnts},
	}

	presetIndex := make(map[string]int)
	aliasOwner := make(map[string]string)

	for _, b := range batches {
		var layerPresets []Preset
		for _, ent := range b.ents {
			switch {
			case ent.Engine != nil:
				eng := *ent.Engine
				eng.Layer = b.layer
				eng.Path = ent.Path
				eng.ArgsStyle = eng.EffectiveArgsStyle()
				conflict := false
				for _, alias := range eng.Aliases {
					al := strings.ToLower(strings.TrimSpace(alias))
					if al == "" {
						continue
					}
					if existingEngine, exists := cat.Engines[al]; exists && existingEngine.ID != eng.ID {
						cat.Diagnostics = append(cat.Diagnostics, Diagnostic{
							Path:     ent.Path,
							Severity: "warn",
							Message: fmt.Sprintf(
								"engine %s alias %q conflicts with engine %s",
								eng.ID, alias, existingEngine.ID,
							),
						})
						conflict = true
						break
					}
					if owner, exists := aliasOwner[al]; exists && owner != eng.ID {
						cat.Diagnostics = append(cat.Diagnostics, Diagnostic{
							Path:     ent.Path,
							Severity: "warn",
							Message:  fmt.Sprintf("engine %s alias %q already used by engine %s", eng.ID, alias, owner),
						})
						conflict = true
						break
					}
				}
				if conflict && b.layer == LayerUser {
					continue
				}
				for _, alias := range eng.Aliases {
					aliasOwner[strings.ToLower(strings.TrimSpace(alias))] = eng.ID
				}
				cat.Engines[eng.ID] = eng

			case ent.Preset != nil:
				p := *ent.Preset
				p.Layer = b.layer
				p.Path = ent.Path
				layerPresets = append(layerPresets, p)
			}
		}

		sort.SliceStable(layerPresets, func(i, j int) bool {
			si, sj := presetSortKey(layerPresets[i]), presetSortKey(layerPresets[j])
			if si != sj {
				return si < sj
			}
			if b.layer == LayerUser {
				ni, nj := strings.ToLower(layerPresets[i].Name), strings.ToLower(layerPresets[j].Name)
				if ni != nj {
					return ni < nj
				}
			}
			return layerPresets[i].ID < layerPresets[j].ID
		})

		for _, p := range layerPresets {
			if eng, ok := findEngineInMap(cat.Engines, strings.ToLower(strings.TrimSpace(p.Engine))); ok {
				p.Engine = eng.ID
			}
			if idx, exists := presetIndex[p.ID]; exists {
				cat.Presets[idx] = p
			} else {
				presetIndex[p.ID] = len(cat.Presets)
				cat.Presets = append(cat.Presets, p)
			}
		}

		for _, ent := range b.ents {
			if ent.Override == nil {
				continue
			}
			ov := ent.Override
			switch ov.Target.Kind {
			case KindPreset:
				idx, ok := presetIndex[ov.Target.ID]
				if !ok {
					cat.Diagnostics = append(cat.Diagnostics, Diagnostic{
						Path:     ent.Path,
						Severity: "warn",
						Message:  fmt.Sprintf("override target preset/%s not found", ov.Target.ID),
					})
					continue
				}
				applyPresetOverride(cat, &cat.Presets[idx], ov)
			case KindEngine:
				eng, ok := cat.Engines[ov.Target.ID]
				if !ok {
					cat.Diagnostics = append(cat.Diagnostics, Diagnostic{
						Path:     ent.Path,
						Severity: "warn",
						Message:  fmt.Sprintf("override target engine/%s not found", ov.Target.ID),
					})
					continue
				}
				applyEngineOverride(&eng, ov)
				cat.Engines[ov.Target.ID] = eng
			}
		}
	}

	return cat
}

func presetSortKey(p Preset) int {
	if p.Sort > 0 {
		return p.Sort
	}
	return math.MaxInt
}

func applyPresetOverride(cat *Catalog, p *Preset, ov *Override) {
	if ov.Set != nil {
		s := ov.Set
		if s.Engine != nil && *s.Engine != "" {
			engName := *s.Engine
			if eng, ok := findEngineInMap(cat.Engines, strings.ToLower(strings.TrimSpace(engName))); ok {
				engName = eng.ID
			}
			p.Engine = engName
			p.EngineOverridden = true
		}
		if s.IWAD != nil && *s.IWAD != "" {
			p.IWAD = *s.IWAD
		}
		if s.Files != nil {
			p.Files = cloneFileSpecs(*s.Files)
		}
		if s.Args != nil {
			p.Args = append([]string(nil), (*s.Args)...)
		}
		if s.Category != nil {
			p.Category = *s.Category
		}
		if s.Genre != nil {
			p.Genre = *s.Genre
		}
		if s.Difficulty != nil {
			p.Difficulty = *s.Difficulty
		}
		if s.Quality != nil {
			p.Quality = *s.Quality
		}
		if s.Compatibility != nil {
			p.Compatibility = *s.Compatibility
		}
		if s.Description != nil {
			p.Description = *s.Description
		}
		if s.Author != nil {
			p.Author = *s.Author
		}
		if s.ReleaseDate != nil {
			p.ReleaseDate = *s.ReleaseDate
		}
		if s.Sources != nil {
			p.Sources = append([]Source(nil), (*s.Sources)...)
		}
		if s.Links != nil {
			p.Links = append([]Link(nil), (*s.Links)...)
		}
		if s.LoadMapsAfterMods != nil {
			p.LoadMapsAfterMods = *s.LoadMapsAfterMods
		}
	}
	if len(ov.AppendFiles) > 0 {
		p.Files = append(cloneFileSpecs(p.Files), cloneFileSpecs(ov.AppendFiles)...)
	}
	if len(ov.ExcludeFiles) > 0 && len(p.Files) > 0 {
		excluded := make(map[string]bool, len(ov.ExcludeFiles)*2)
		for _, ex := range ov.ExcludeFiles {
			if norm := NormalizeFilename(ex); norm != "" {
				excluded[norm] = true
			}
			if baseNorm := NormalizeFilename(filepath.Base(ex)); baseNorm != "" {
				excluded[baseNorm] = true
			}
		}
		var kept []FileSpec
		for _, f := range p.Files {
			norm := NormalizeFilename(f.Name)
			baseNorm := NormalizeFilename(filepath.Base(f.Name))
			if (norm != "" && excluded[norm]) || (baseNorm != "" && excluded[baseNorm]) {
				continue
			}
			kept = append(kept, f)
		}
		p.Files = kept
	}
	if len(ov.AppendArgs) > 0 {
		p.Args = append(append([]string(nil), p.Args...), ov.AppendArgs...)
	}
	p.Overridden = true
}

func clonePreset(p Preset) *Preset {
	pc := p
	pc.Files = cloneFileSpecs(p.Files)
	pc.Args = append([]string(nil), p.Args...)
	pc.Sources = append([]Source(nil), p.Sources...)
	pc.Links = append([]Link(nil), p.Links...)
	return &pc
}

// LoadPresetBaseAndOverride loads the unmerged base Preset (from the curated catalog or
// <catalogDir>/preset.<id>.json) and any existing user Override from
// <catalogDir>/override.preset.<id>.json.
func LoadPresetBaseAndOverride(catalogDir string, id string) (*Preset, *Override, error) {
	clean := strings.TrimSpace(id)
	if clean == "" {
		return nil, nil, fmt.Errorf("empty preset id")
	}

	curated, err := LoadCuratedCatalog()
	if err != nil {
		return nil, nil, err
	}

	var base *Preset
	if cp, findErr := curated.FindExact(clean); findErr == nil && cp != nil {
		base = clonePreset(*cp)
	}

	dir := strings.TrimSpace(catalogDir)
	if dir != "" {
		if info, statErr := os.Stat(dir); statErr == nil {
			if !info.IsDir() {
				return nil, nil, fmt.Errorf("%s is not a directory", dir)
			}
			candidateID := Slug(clean)
			if base != nil {
				candidateID = base.ID
			}
			userPresetPath := filepath.Join(dir, fmt.Sprintf("preset.%s.json", candidateID))
			if raw, readErr := os.ReadFile(userPresetPath); readErr == nil {
				if ent, decErr := DecodeEntity(raw); decErr == nil && ent.Preset != nil {
					up := clonePreset(*ent.Preset)
					up.Layer = LayerUser
					up.Path = filepath.ToSlash(userPresetPath)
					base = up
				}
			} else if base == nil {
				ents, _, _ := scanEntityFS(os.DirFS(dir), dirScanOptions{
					displayPrefix: filepath.ToSlash(dir),
					strict:        false,
					severity:      "warn",
				})
				for _, ent := range ents {
					if ent.Preset != nil &&
						(strings.EqualFold(ent.Preset.ID, clean) || strings.EqualFold(ent.Preset.Name, clean)) {
						up := clonePreset(*ent.Preset)
						up.Layer = LayerUser
						up.Path = ent.Path
						base = up
						break
					}
				}
			}

			if base != nil {
				ovPath := filepath.Join(dir, fmt.Sprintf("override.preset.%s.json", base.ID))
				if raw, readErr := os.ReadFile(ovPath); readErr == nil {
					if ent, decErr := DecodeEntity(raw); decErr == nil && ent.Override != nil {
						ov := *ent.Override
						ov.Path = filepath.ToSlash(ovPath)
						ov.AppendFiles = cloneFileSpecs(ent.Override.AppendFiles)
						ov.AppendArgs = append([]string(nil), ent.Override.AppendArgs...)
						ov.ExcludeFiles = append([]string(nil), ent.Override.ExcludeFiles...)
						return base, &ov, nil
					}
				}
			}
		}
	}

	if base == nil {
		return nil, nil, fmt.Errorf("preset %q not found", id)
	}
	return base, nil, nil
}

func applyEngineOverride(e *EngineConfig, ov *Override) {
	if ov.Set != nil {
		s := ov.Set
		if s.Binary != nil {
			e.Binary = *s.Binary
		}
		if s.ArgsStyle != nil {
			e.ArgsStyle = *s.ArgsStyle
		}
		if s.DefaultArgs != nil {
			e.DefaultArgs = append([]string(nil), (*s.DefaultArgs)...)
		}
		if s.PrefixArgs != nil {
			e.PrefixArgs = append([]string(nil), (*s.PrefixArgs)...)
		}
		if s.Badge != nil {
			e.Badge = *s.Badge
		}
		if s.DisplayName != nil {
			e.DisplayName = *s.DisplayName
		}
		if s.Description != nil {
			e.Description = *s.Description
		}
	}
	if len(ov.AppendArgs) > 0 {
		e.DefaultArgs = append(e.DefaultArgs, ov.AppendArgs...)
	}
	e.Overridden = true
}
