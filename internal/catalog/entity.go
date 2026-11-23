// Package catalog manages declarative engine, preset, and override entities,
// layered catalog loading, query resolution, user persistence, and WAD indexing.
package catalog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// SchemaVersion is the supported entity schema version for catalog JSON files.
const SchemaVersion = 1

// ComplevelUnset is an override sentinel value (-1) that clears an inherited preset complevel.
const ComplevelUnset = -1

// ValidateComplevel verifies that cl is a supported Boom/PrBoom+/MBF21 compatibility level (0..17 or 21).
func ValidateComplevel(cl int) error {
	if (cl >= 0 && cl <= 17) || cl == 21 {
		return nil
	}
	return fmt.Errorf("invalid complevel %d (expected 0..17 or 21)", cl)
}

// ComplevelLabel formats a compatibility level integer with its common standard name when known.
func ComplevelLabel(cl int) string {
	switch cl {
	case 2:
		return "2: Vanilla"
	case 3:
		return "3: Ult. Doom"
	case 4:
		return "4: Final Doom"
	case 9:
		return "9: Boom"
	case 11:
		return "11: MBF"
	case 21:
		return "21: MBF21"
	default:
		return strconv.Itoa(cl)
	}
}

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
	SchemaVersion int            `json:"schema-version,omitempty"`
	Kind          Kind           `json:"kind"`
	ID            string         `json:"id"`
	DisplayName   string         `json:"display-name,omitempty"`
	Description   string         `json:"description,omitempty"`
	Family        string         `json:"family,omitempty"`
	ArgsStyle     string         `json:"args-style,omitempty"` // "boom" or "zdoom"
	Binary        string         `json:"binary,omitempty"`
	PrefixArgs    []string       `json:"prefix-args,omitempty"`
	DefaultArgs   []string       `json:"default-args,omitempty"`
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
	Disabled bool     `json:"disabled,omitempty"`
}

// Source describes a downloadable archive or file mirror for a preset.
type Source struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
}

// IsSupportedSourceURL reports whether url has a supported downloadable extension (.zip, .wad, .pk3).
func IsSupportedSourceURL(url string) bool {
	lower := strings.ToLower(strings.TrimSpace(url))
	if idx := strings.IndexAny(lower, "?#"); idx >= 0 {
		lower = lower[:idx]
	}
	return strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".wad") ||
		strings.HasSuffix(lower, ".pk3")
}

// Link describes a human-facing informational URL (such as an idgames page).
type Link struct {
	URL string `json:"url"`
	Rel string `json:"rel,omitempty"`
}

// Preset defines a single launchable Doom mapset or game configuration.
type Preset struct {
	SchemaVersion     int                 `json:"schema-version,omitempty"`
	Kind              Kind                `json:"kind"`
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Game              string              `json:"game,omitempty"`
	Engine            string              `json:"engine"`
	IWAD              string              `json:"iwad"`
	Files             []FileSpec          `json:"files,omitempty"`
	Complevel         *int                `json:"complevel,omitempty"`
	EngineArgs        map[string][]string `json:"engine-args,omitempty"`
	Args              []string            `json:"args,omitempty"`
	Category          string              `json:"category,omitempty"`
	Genre             string              `json:"genre,omitempty"`
	Difficulty        string              `json:"difficulty,omitempty"`
	Quality           string              `json:"quality,omitempty"`
	Compatibility     string              `json:"compatibility,omitempty"`
	Description       string              `json:"description,omitempty"`
	Author            string              `json:"author,omitempty"`
	ReleaseDate       string              `json:"release-date,omitempty"`
	Sources           []Source            `json:"sources,omitempty"`
	Links             []Link              `json:"links,omitempty"`
	LoadMapsAfterMods bool                `json:"load-maps-after-mods,omitempty"`
	Sort              int                 `json:"sort,omitempty"`

	Layer            Layer  `json:"-"`
	Overridden       bool   `json:"-"`
	EngineOverridden bool   `json:"-"`
	Path             string `json:"-"`
}

// ArgsForEngine returns a copy of the engine-specific arguments declared for engineID, or nil.
func (p Preset) ArgsForEngine(engineID string) []string {
	if len(p.EngineArgs) == 0 {
		return nil
	}
	clean := strings.ToLower(strings.TrimSpace(engineID))
	if clean == "" {
		return nil
	}
	if exact, ok := p.EngineArgs[clean]; ok && len(exact) > 0 {
		return append([]string(nil), exact...)
	}
	for k, v := range p.EngineArgs {
		if strings.EqualFold(strings.TrimSpace(k), clean) && len(v) > 0 {
			return append([]string(nil), v...)
		}
	}
	return nil
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
	Game              *string              `json:"game,omitempty"`
	Engine            *string              `json:"engine,omitempty"`
	IWAD              *string              `json:"iwad,omitempty"`
	Files             *[]FileSpec          `json:"files,omitempty"`
	Complevel         *int                 `json:"complevel,omitempty"`
	EngineArgs        *map[string][]string `json:"engine-args,omitempty"`
	Args              *[]string            `json:"args,omitempty"`
	Category          *string              `json:"category,omitempty"`
	Genre             *string              `json:"genre,omitempty"`
	Difficulty        *string              `json:"difficulty,omitempty"`
	Quality           *string              `json:"quality,omitempty"`
	Compatibility     *string              `json:"compatibility,omitempty"`
	Description       *string              `json:"description,omitempty"`
	Author            *string              `json:"author,omitempty"`
	ReleaseDate       *string              `json:"release-date,omitempty"`
	Sources           *[]Source            `json:"sources,omitempty"`
	Links             *[]Link              `json:"links,omitempty"`
	LoadMapsAfterMods *bool                `json:"load-maps-after-mods,omitempty"`
	Binary            *string              `json:"binary,omitempty"`
	ArgsStyle         *string              `json:"args-style,omitempty"`
	DefaultArgs       *[]string            `json:"default-args,omitempty"`
	PrefixArgs        *[]string            `json:"prefix-args,omitempty"`
	Badge             *string              `json:"badge,omitempty"`
	DisplayName       *string              `json:"display-name,omitempty"`
}

// IsEmpty reports whether every replacement pointer on s is nil.
func (s *OverrideSet) IsEmpty() bool {
	if s == nil {
		return true
	}
	return s.Game == nil &&
		s.Engine == nil &&
		s.IWAD == nil &&
		s.Files == nil &&
		s.Complevel == nil &&
		s.EngineArgs == nil &&
		s.Args == nil &&
		s.Category == nil &&
		s.Genre == nil &&
		s.Difficulty == nil &&
		s.Quality == nil &&
		s.Compatibility == nil &&
		s.Description == nil &&
		s.Author == nil &&
		s.ReleaseDate == nil &&
		s.Sources == nil &&
		s.Links == nil &&
		s.LoadMapsAfterMods == nil &&
		s.Binary == nil &&
		s.ArgsStyle == nil &&
		s.DefaultArgs == nil &&
		s.PrefixArgs == nil &&
		s.Badge == nil &&
		s.DisplayName == nil
}

// Override patches a lower-layer preset or engine without replacing its base file.
type Override struct {
	SchemaVersion    int                 `json:"schema-version,omitempty"`
	Kind             Kind                `json:"kind"`
	Target           OverrideTarget      `json:"target"`
	Set              *OverrideSet        `json:"set,omitempty"`
	AppendFiles      []FileSpec          `json:"append-files,omitempty"`
	AppendArgs       []string            `json:"append-args,omitempty"`
	AppendEngineArgs map[string][]string `json:"append-engine-args,omitempty"`
	IncludeFiles     []string            `json:"include-files,omitempty"`
	ExcludeFiles     []string            `json:"exclude-files,omitempty"`

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

func cloneEngineConfig(e EngineConfig) EngineConfig {
	ec := e
	ec.PrefixArgs = append([]string(nil), e.PrefixArgs...)
	ec.DefaultArgs = append([]string(nil), e.DefaultArgs...)
	ec.Aliases = append([]string(nil), e.Aliases...)
	return ec
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneEngineArgsMap(m map[string][]string) map[string][]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func clonePreset(p Preset) *Preset {
	pc := p
	pc.Files = cloneFileSpecs(p.Files)
	pc.Complevel = cloneIntPtr(p.Complevel)
	pc.EngineArgs = cloneEngineArgsMap(p.EngineArgs)
	pc.Args = append([]string(nil), p.Args...)
	pc.Sources = append([]Source(nil), p.Sources...)
	pc.Links = append([]Link(nil), p.Links...)
	return &pc
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
		out.Engines[k] = cloneEngineConfig(e)
	}
	for i, p := range c.Presets {
		out.Presets[i] = *clonePreset(p)
	}
	return out
}
