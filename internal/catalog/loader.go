package catalog

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lock14/doom-cli"
)

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
		ents, _, err := ScanFS(sub, ScanOptions{
			DisplayPrefix: "catalog",
			Strict:        true,
			Severity:      "error",
		})
		if err != nil {
			cachedCuratedErr = err
			return
		}
		cachedEnts = ents
		cachedCurated = Build(ents, nil)
	})
	return cachedEnts, cachedCurated, cachedCuratedErr
}

// LoadCurated returns a fresh clone of the embedded curated catalog entities.
func LoadCurated() (*Catalog, error) {
	_, cat, err := loadEmbeddedCurated()
	if err != nil {
		return nil, err
	}
	return cat.Clone(), nil
}

// LoadFromDir loads a curated catalog from an on-disk directory (such as "./catalog").
func LoadFromDir(dir string) (*Catalog, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	curatedEnts, _, err := ScanFS(os.DirFS(dir), ScanOptions{
		DisplayPrefix: filepath.ToSlash(dir),
		Strict:        true,
		Severity:      "error",
	})
	if err != nil {
		return nil, err
	}
	return Build(curatedEnts, nil), nil
}

// LoadLayered loads the embedded curated catalog and merges user entities from userPath.
// When explicitPath is true, a non-existent userPath returns an error instead of being treated as empty.
func LoadLayered(userPath string, explicitPath bool) (*Catalog, error) {
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

	userEnts, diags, loadErr := ScanFS(os.DirFS(userPath), ScanOptions{
		DisplayPrefix:    filepath.ToSlash(userPath),
		Strict:           false,
		Severity:         "warn",
		QuarantinePrefix: true,
	})
	if loadErr != nil {
		return nil, loadErr
	}

	if len(userEnts) == 0 && len(diags) == 0 {
		return curatedCat.Clone(), nil
	}

	cat := Build(curatedEnts, userEnts)
	cat.Diagnostics = append(diags, cat.Diagnostics...)
	return cat, nil
}

// MergeUserDir merges user entities and overrides from userDir onto a clone of base
// (or the embedded curated catalog when base is nil).
func MergeUserDir(base *Catalog, userDir string) (*Catalog, error) {
	if base == nil {
		return LoadLayered(userDir, false)
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
			userEnts, diags, _ = ScanFS(os.DirFS(userDir), ScanOptions{
				DisplayPrefix:    filepath.ToSlash(userDir),
				Strict:           false,
				Severity:         "warn",
				QuarantinePrefix: true,
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
			ec := cloneEngineConfig(*ent.Engine)
			baseEnts = append(baseEnts, Entity{Engine: &ec, Path: ent.Path})
			seenEngines[ec.ID] = true
		}
	}
	engineKeys := make([]string, 0, len(base.Engines))
	for id := range base.Engines {
		engineKeys = append(engineKeys, id)
	}
	sort.Strings(engineKeys)
	for _, id := range engineKeys {
		eng := base.Engines[id]
		ec := cloneEngineConfig(eng)
		if seenEngines[id] {
			if !eng.Overridden && eng.Layer == LayerCurated {
				for i := range baseEnts {
					if baseEnts[i].Engine != nil && baseEnts[i].Engine.ID == id {
						baseEnts[i].Engine = &ec
						break
					}
				}
			}
		} else if eng.Layer == LayerUser {
			// Preserve synthetic in-memory fixtures (Path == "") while letting disk userDir govern disk entities.
			if eng.Path == "" && !diskUserEngines[id] && !eng.Overridden {
				userEnts = append(userEnts, Entity{Engine: &ec, Path: eng.Path})
			}
		} else {
			baseEnts = append(baseEnts, Entity{Engine: &ec, Path: eng.Path})
			seenEngines[id] = true
		}
	}

	for i, p := range base.Presets {
		if p.Layer == LayerUser {
			if p.Path == "" && !diskUserPresets[p.ID] && !p.Overridden {
				pc := *clonePreset(p)
				userEnts = append(userEnts, Entity{Preset: &pc, Path: p.Path})
			} else if p.Path != "" && !diskUserPresets[p.ID] && curatedCat != nil {
				// Restore any curated preset previously shadowed by a now-deleted user preset file.
				if cp, findErr := curatedCat.FindExact(p.ID); findErr == nil && cp != nil && cp.ID == p.ID {
					pc := *clonePreset(*cp)
					baseEnts = append(baseEnts, Entity{Preset: &pc, Path: pc.Path})
				}
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

	cat := Build(baseEnts, userEnts)
	cat.Diagnostics = append(diags, cat.Diagnostics...)
	return cat, nil
}

// Build merges curated and user entity slices into a single layered Catalog.
func Build(curatedEnts, userEnts []Entity) *Catalog {
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
				eng := cloneEngineConfig(*ent.Engine)
				eng.Layer = b.layer
				eng.Path = ent.Path
				eng.ArgsStyle = eng.EffectiveArgsStyle()
				if b.layer == LayerUser {
					if existing, exists := cat.Engines[eng.ID]; exists && existing.Layer == LayerCurated {
						cat.Diagnostics = append(cat.Diagnostics, Diagnostic{
							Path:     ent.Path,
							Severity: "warn",
							Message: fmt.Sprintf(
								"user engine %s cannot shadow built-in curated engine %s",
								eng.ID, existing.ID,
							),
						})
						continue
					}
					if owner, exists := aliasOwner[strings.ToLower(eng.ID)]; exists && owner != eng.ID {
						cat.Diagnostics = append(cat.Diagnostics, Diagnostic{
							Path:     ent.Path,
							Severity: "warn",
							Message:  fmt.Sprintf("engine id %s conflicts with alias on engine %s", eng.ID, owner),
						})
						continue
					}
				}
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
					if al := strings.ToLower(strings.TrimSpace(alias)); al != "" {
						aliasOwner[al] = eng.ID
					}
				}
				cat.Engines[eng.ID] = eng

			case ent.Preset != nil:
				p := *clonePreset(*ent.Preset)
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
			if baseNorm := NormalizeFilename(SlashBase(ex)); baseNorm != "" {
				excluded[baseNorm] = true
			}
		}
		var kept []FileSpec
		for _, f := range p.Files {
			norm := NormalizeFilename(f.Name)
			baseNorm := NormalizeFilename(SlashBase(f.Name))
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

// LoadPresetBaseAndOverride loads the unmerged base Preset (from the curated catalog or
// <catalogDir>/preset.<id>.json) and any existing user Override from
// <catalogDir>/override.preset.<id>.json.
func LoadPresetBaseAndOverride(catalogDir string, id string) (*Preset, *Override, error) {
	clean := strings.TrimSpace(id)
	if clean == "" {
		return nil, nil, fmt.Errorf("empty preset id")
	}

	curated, err := LoadCurated()
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
			userPresetName := fmt.Sprintf("preset.%s.json", candidateID)
			userPresetPath := filepath.Join(dir, userPresetName)
			if raw, readErr := os.ReadFile(userPresetPath); readErr == nil {
				if ent, decErr := DecodeEntity(raw); decErr == nil &&
					ent.Preset != nil && ExpectedFilename(ent) == userPresetName {
					up := clonePreset(*ent.Preset)
					up.Layer = LayerUser
					up.Path = filepath.ToSlash(userPresetPath)
					base = up
				}
			} else if base == nil {
				ents, _, _ := ScanFS(os.DirFS(dir), ScanOptions{
					DisplayPrefix: filepath.ToSlash(dir),
					Strict:        false,
					Severity:      "warn",
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
				ovName := fmt.Sprintf("override.preset.%s.json", base.ID)
				ovPath := filepath.Join(dir, ovName)
				if raw, readErr := os.ReadFile(ovPath); readErr == nil {
					if ent, decErr := DecodeEntity(raw); decErr == nil &&
						ent.Override != nil && ExpectedFilename(ent) == ovName {
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
		e.DefaultArgs = append(append([]string(nil), e.DefaultArgs...), ov.AppendArgs...)
	}
	e.Overridden = true
}
