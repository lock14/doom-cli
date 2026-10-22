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
	SHA256   string   `json:"sha256,omitempty"`
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

// FindEngine resolves an engine by ID or alias (case-insensitive).
func (c *Catalog) FindEngine(key string) (EngineConfig, bool) {
	if c == nil {
		return EngineConfig{}, false
	}
	clean := strings.ToLower(strings.TrimSpace(key))
	if clean == "" {
		return EngineConfig{}, false
	}
	if eng, ok := c.Engines[clean]; ok {
		return eng, true
	}
	for k, eng := range c.Engines {
		if strings.EqualFold(k, clean) || strings.EqualFold(eng.ID, clean) {
			return eng, true
		}
		for _, alias := range eng.Aliases {
			if strings.EqualFold(alias, clean) {
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

// Find searches for a preset by exact ID, exact case-insensitive name, or name or ID prefix.
func (c *Catalog) Find(target string) *Preset {
	if c == nil {
		return nil
	}
	clean := strings.TrimSpace(strings.ToLower(target))
	if clean == "" {
		return nil
	}

	for i := range c.Presets {
		if strings.ToLower(c.Presets[i].ID) == clean {
			return &c.Presets[i]
		}
	}
	for i := range c.Presets {
		if strings.ToLower(c.Presets[i].Name) == clean {
			return &c.Presets[i]
		}
	}
	for i := range c.Presets {
		if strings.HasPrefix(strings.ToLower(c.Presets[i].Name), clean) ||
			strings.HasPrefix(strings.ToLower(c.Presets[i].ID), clean) {
			return &c.Presets[i]
		}
	}
	return nil
}

// FindExact resolves a preset by exact ID or unambiguous case-insensitive Name.
// Prefix matches are not accepted, making FindExact safe for mutating commands.
func (c *Catalog) FindExact(target string) (*Preset, error) {
	if c == nil {
		return nil, fmt.Errorf("preset %q not found", target)
	}
	clean := strings.TrimSpace(strings.ToLower(target))
	if clean == "" {
		return nil, fmt.Errorf("empty preset name")
	}

	for i := range c.Presets {
		if strings.ToLower(c.Presets[i].ID) == clean {
			return &c.Presets[i], nil
		}
	}

	var matches []int
	for i := range c.Presets {
		if strings.ToLower(c.Presets[i].Name) == clean {
			matches = append(matches, i)
		}
	}
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

// LoadCatalog loads the embedded curated catalog and layers any user entities from
// userPath (either a directory of *.json entity files or a single entity JSON file).
// If userPath is empty, only the embedded curated catalog is returned.
func LoadCatalog(userPath string) (*Catalog, error) {
	return LoadLayeredCatalog(userPath, false)
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

	var userEnts []Entity
	var diags []Diagnostic
	if info.IsDir() {
		var loadErr error
		userEnts, diags, loadErr = scanEntityFS(os.DirFS(userPath), dirScanOptions{
			displayPrefix:    filepath.ToSlash(userPath),
			strict:           false,
			severity:         "warn",
			quarantinePrefix: true,
		})
		if loadErr != nil {
			return nil, loadErr
		}
	} else {
		raw, readErr := os.ReadFile(userPath)
		if readErr != nil {
			return nil, readErr
		}
		ent, decErr := DecodeEntity(raw)
		if decErr != nil {
			return nil, fmt.Errorf("%s: %w", userPath, decErr)
		}
		ent.Path = userPath
		userEnts = []Entity{ent}
	}

	if len(userEnts) == 0 && len(diags) == 0 {
		return curatedCat.Clone(), nil
	}

	cat := buildCatalog(curatedEnts, userEnts)
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
			if eng, ok := cat.FindEngine(p.Engine); ok {
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
			if eng, ok := cat.FindEngine(engName); ok {
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
	if len(ov.AppendArgs) > 0 {
		p.Args = append(append([]string(nil), p.Args...), ov.AppendArgs...)
	}
	p.Overridden = true
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
