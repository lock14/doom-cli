package catalog

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lock14/doom-cli/internal/fsutil"
)

// MarshalEntity serializes an entity value as indented JSON without HTML escaping.
func MarshalEntity(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ValidateSHA256 verifies that digest is either empty or a valid 64-character hex string.
func ValidateSHA256(digest string) error {
	clean := strings.TrimSpace(digest)
	if clean == "" {
		return nil
	}
	if len(clean) != 64 {
		return fmt.Errorf("invalid sha256 %q (expected 64 hex characters)", digest)
	}
	if _, err := hex.DecodeString(clean); err != nil {
		return fmt.Errorf("invalid sha256 %q: %w", digest, err)
	}
	return nil
}

func validateCatalogDirPath(userDir string) error {
	if strings.TrimSpace(userDir) == "" {
		return errors.New("empty user catalog directory")
	}
	if info, err := os.Stat(userDir); err == nil && !info.IsDir() {
		return fmt.Errorf("catalog path %q is a single file; pass a directory to write entities", userDir)
	}
	return nil
}

// AddEngine writes or updates a user engine entity file in userDir (<userDir>/engine.<id>.json).
// It refuses to shadow a curated engine ID or alias.
func AddEngine(userDir string, curated *Catalog, eng EngineConfig) (string, error) {
	if err := validateCatalogDirPath(userDir); err != nil {
		return "", err
	}
	if eng.ID == "" {
		eng.ID = Slug(eng.DisplayName)
	} else {
		eng.ID = Slug(eng.ID)
	}
	if err := ValidateID(eng.ID); err != nil {
		return "", fmt.Errorf("invalid engine id (pass --id <slug>): %w", err)
	}

	if curated != nil {
		if existing, ok := curated.FindEngine(eng.ID); ok && existing.Layer == LayerCurated {
			return "", fmt.Errorf(
				"engine id %q collides with curated engine %q; "+
					"pass a different --id or use `doom engines set %s`",
				eng.ID, existing.ID, existing.ID,
			)
		}
	}
	for _, al := range eng.Aliases {
		cleanAl := strings.ToLower(strings.TrimSpace(al))
		if cleanAl == "" {
			continue
		}
		if err := ValidateID(cleanAl); err != nil {
			return "", fmt.Errorf("invalid engine alias %q: %w", al, err)
		}
		if curated != nil {
			if existing, ok := curated.FindEngine(cleanAl); ok && existing.Layer == LayerCurated {
				return "", fmt.Errorf("engine alias %q collides with curated engine %q", al, existing.ID)
			}
		}
	}

	eng.SchemaVersion = SchemaVersion
	eng.Kind = KindEngine
	eng.ArgsStyle = eng.EffectiveArgsStyle()
	if eng.ArgsStyle != "boom" && eng.ArgsStyle != "zdoom" {
		return "", fmt.Errorf("invalid args-style %q (must be \"boom\" or \"zdoom\")", eng.ArgsStyle)
	}
	if strings.TrimSpace(eng.Binary) == "" {
		eng.Binary = eng.ID
	}

	data, err := MarshalEntity(eng)
	if err != nil {
		return "", err
	}
	targetPath := filepath.Join(userDir, ExpectedFilename(Entity{Engine: &eng}))
	if _, err := fsutil.WriteFileAtomic(targetPath, data, 0o644); err != nil {
		return "", err
	}
	return targetPath, nil
}

// EngineOverrideOptions specifies the fields to update on an engine override.
type EngineOverrideOptions struct {
	Binary      *string
	ArgsStyle   *string
	DefaultArgs *[]string
	AppendArgs  *[]string
	PrefixArgs  *[]string
	Badge       *string
	DisplayName *string
}

// ConfigEngine creates or updates <userDir>/override.engine.<id>.json for the
// target engine, preserving any previously configured override fields not touched
// by opts.
func ConfigEngine(userDir string, layered *Catalog, key string, opts EngineOverrideOptions) (string, error) {
	if err := validateCatalogDirPath(userDir); err != nil {
		return "", err
	}
	if layered == nil {
		return "", errors.New("nil catalog")
	}
	eng, ok := layered.FindEngine(key)
	if !ok {
		return "", fmt.Errorf("unknown engine %q (register it first with `doom engines add`)", key)
	}
	if err := ValidateID(eng.ID); err != nil {
		return "", fmt.Errorf("invalid engine id %q: %w", eng.ID, err)
	}

	if opts.ArgsStyle != nil {
		style := strings.ToLower(strings.TrimSpace(*opts.ArgsStyle))
		if style != "boom" && style != "zdoom" {
			return "", fmt.Errorf("invalid args-style %q (must be \"boom\" or \"zdoom\")", *opts.ArgsStyle)
		}
		opts.ArgsStyle = &style
	}

	overridePath := filepath.Join(userDir, fmt.Sprintf("override.engine.%s.json", eng.ID))
	ov := Override{
		SchemaVersion: SchemaVersion,
		Kind:          KindOverride,
		Target: OverrideTarget{
			Kind: KindEngine,
			ID:   eng.ID,
		},
	}

	if existingRaw, readErr := os.ReadFile(overridePath); readErr == nil {
		if existingEnt, decErr := DecodeEntity(existingRaw); decErr == nil && existingEnt.Override != nil {
			ov = *existingEnt.Override
			ov.SchemaVersion = SchemaVersion
			ov.Kind = KindOverride
			ov.Target = OverrideTarget{Kind: KindEngine, ID: eng.ID}
		}
	}

	if opts.Binary != nil ||
		opts.ArgsStyle != nil ||
		opts.DefaultArgs != nil ||
		opts.PrefixArgs != nil ||
		opts.Badge != nil ||
		opts.DisplayName != nil {
		if ov.Set == nil {
			ov.Set = &OverrideSet{}
		}
		if opts.Binary != nil {
			ov.Set.Binary = opts.Binary
		}
		if opts.ArgsStyle != nil {
			ov.Set.ArgsStyle = opts.ArgsStyle
		}
		if opts.DefaultArgs != nil {
			s := make([]string, len(*opts.DefaultArgs))
			copy(s, *opts.DefaultArgs)
			ov.Set.DefaultArgs = &s
		}
		if opts.PrefixArgs != nil {
			s := make([]string, len(*opts.PrefixArgs))
			copy(s, *opts.PrefixArgs)
			ov.Set.PrefixArgs = &s
		}
		if opts.Badge != nil {
			ov.Set.Badge = opts.Badge
		}
		if opts.DisplayName != nil {
			ov.Set.DisplayName = opts.DisplayName
		}
	}
	if opts.AppendArgs != nil {
		ov.AppendArgs = append([]string(nil), (*opts.AppendArgs)...)
	}

	data, err := MarshalEntity(ov)
	if err != nil {
		return "", err
	}
	if _, err := fsutil.WriteFileAtomic(overridePath, data, 0o644); err != nil {
		return "", err
	}
	return overridePath, nil
}

// ResetEngineOverride removes <userDir>/override.engine.<id>.json for the given engine.
func ResetEngineOverride(userDir string, layered *Catalog, key string) error {
	if err := validateCatalogDirPath(userDir); err != nil {
		return err
	}
	if layered == nil {
		return errors.New("nil catalog")
	}
	eng, ok := layered.FindEngine(key)
	if !ok {
		return fmt.Errorf("unknown engine %q", key)
	}
	if err := ValidateID(eng.ID); err != nil {
		return fmt.Errorf("invalid engine id %q: %w", eng.ID, err)
	}
	overridePath := filepath.Join(userDir, fmt.Sprintf("override.engine.%s.json", eng.ID))
	if err := os.Remove(overridePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	fsutil.RemoveBackups(overridePath)
	return nil
}

// RemoveEngine deletes a user engine entity from userDir by ID or alias.
// Built-in curated engines cannot be removed. Returns the removed file path and
// any preset names in layered that still reference the removed engine.
func RemoveEngine(userDir string, curated, layered *Catalog, key string) (string, []string, error) {
	if err := validateCatalogDirPath(userDir); err != nil {
		return "", nil, err
	}
	clean := strings.TrimSpace(key)
	if clean == "" {
		return "", nil, errors.New("empty engine name")
	}

	var targetID string
	if layered != nil {
		if eng, ok := layered.FindEngine(clean); ok {
			targetID = eng.ID
		}
	}
	if targetID == "" {
		targetID = Slug(clean)
	}
	if err := ValidateID(targetID); err != nil {
		return "", nil, fmt.Errorf("invalid engine id %q: %w", targetID, err)
	}

	if curated != nil {
		if _, ok := curated.Engines[targetID]; ok {
			userFile := filepath.Join(userDir, fmt.Sprintf("engine.%s.json", targetID))
			if _, statErr := os.Stat(userFile); os.IsNotExist(statErr) {
				return "", nil, fmt.Errorf("cannot remove built-in curated engine %q", targetID)
			}
		}
	}

	userFile := filepath.Join(userDir, fmt.Sprintf("engine.%s.json", targetID))
	if err := os.Remove(userFile); err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("custom engine %q not found", key)
		}
		return "", nil, err
	}
	fsutil.RemoveBackups(userFile)

	overrideFile := filepath.Join(userDir, fmt.Sprintf("override.engine.%s.json", targetID))
	_ = os.Remove(overrideFile)
	fsutil.RemoveBackups(overrideFile)

	var dependents []string
	if layered != nil {
		for _, p := range layered.Presets {
			if strings.EqualFold(p.Engine, targetID) {
				dependents = append(dependents, p.Name)
			}
		}
	}
	return userFile, dependents, nil
}

// AddPreset writes or updates a user preset entity file in userDir (<userDir>/preset.<id>.json).
// It refuses to collide with a curated preset ID or a user preset that has a different display name.
func AddPreset(userDir string, curated, layered *Catalog, p Preset) (string, error) {
	if err := validateCatalogDirPath(userDir); err != nil {
		return "", err
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return "", errors.New("preset name is required")
	}
	if strings.TrimSpace(p.ID) == "" {
		p.ID = Slug(p.Name)
	} else {
		p.ID = Slug(p.ID)
	}
	if err := ValidateID(p.ID); err != nil {
		return "", fmt.Errorf("invalid preset id (pass --id <slug>): %w", err)
	}

	if curated != nil {
		for _, cp := range curated.Presets {
			if cp.ID == p.ID || strings.EqualFold(cp.Name, p.Name) {
				return "", fmt.Errorf(
					"preset %q collides with curated preset %q (id %q); pass --id <slug> or use `doom wads set`",
					p.Name, cp.Name, cp.ID,
				)
			}
		}
	}

	targetPath := filepath.Join(userDir, fmt.Sprintf("preset.%s.json", p.ID))
	if existingRaw, err := os.ReadFile(targetPath); err == nil {
		if existingEnt, decErr := DecodeEntity(existingRaw); decErr == nil && existingEnt.Preset != nil {
			if !strings.EqualFold(existingEnt.Preset.Name, p.Name) {
				return "", fmt.Errorf(
					"id %q is already used by user preset %q; pass --id <slug> to disambiguate",
					p.ID, existingEnt.Preset.Name,
				)
			}
		}
	}

	if strings.TrimSpace(p.IWAD) == "" {
		p.IWAD = "DOOM2.WAD"
	}
	if _, err := fsutil.CleanRel(strings.TrimSpace(p.IWAD)); err != nil {
		return "", fmt.Errorf("invalid iwad path %q: %w", p.IWAD, err)
	}
	for _, f := range p.Files {
		if _, err := fsutil.CleanRel(strings.TrimSpace(f.Name)); err != nil {
			return "", fmt.Errorf("invalid file path %q: %w", f.Name, err)
		}
	}
	if strings.TrimSpace(p.Engine) == "" {
		p.Engine = "dsda-doom"
	}
	if layered != nil {
		eng, ok := layered.FindEngine(p.Engine)
		if !ok {
			return "", fmt.Errorf("unknown engine %q (register it first with `doom engines add`)", p.Engine)
		}
		p.Engine = eng.ID
	}
	if p.Complevel != nil {
		if err := ValidateComplevel(*p.Complevel); err != nil {
			return "", err
		}
	}
	if len(p.EngineArgs) > 0 {
		normalizedEngArgs, err := resolveStoreEngineArgs(layered, p.EngineArgs, false)
		if err != nil {
			return "", err
		}
		p.EngineArgs = normalizedEngArgs
	}
	if strings.TrimSpace(p.Category) == "" {
		p.Category = "Custom"
	}
	for _, s := range p.Sources {
		if err := ValidateSHA256(s.SHA256); err != nil {
			return "", fmt.Errorf("source %s: %w", s.URL, err)
		}
	}

	p.SchemaVersion = SchemaVersion
	p.Kind = KindPreset

	data, err := MarshalEntity(p)
	if err != nil {
		return "", err
	}
	if _, err := fsutil.WriteFileAtomic(targetPath, data, 0o644); err != nil {
		return "", err
	}
	return targetPath, nil
}

func resolveStoreEngineArgs(
	layered *Catalog,
	raw map[string][]string,
	keepEmpty bool,
) (map[string][]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string][]string, len(raw))
	for k, slice := range raw {
		clean := strings.ToLower(strings.TrimSpace(k))
		if layered != nil {
			eng, ok := layered.FindEngine(clean)
			if !ok {
				return nil, fmt.Errorf(
					"unknown engine %q in engine-args (register it first with `doom engines add`)",
					k,
				)
			}
			clean = eng.ID
		} else if err := ValidateID(clean); err != nil {
			return nil, fmt.Errorf("invalid engine-args key %q: %w", k, err)
		}
		if len(slice) == 0 {
			if keepEmpty {
				out[clean] = []string{}
			}
			continue
		}
		out[clean] = append([]string(nil), slice...)
	}
	if len(out) == 0 && !keepEmpty {
		return nil, nil
	}
	return out, nil
}

func cleanFileList(in []string) []string {
	var cleaned []string
	for _, f := range in {
		if trimmed := strings.TrimSpace(f); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

func filterOutMatchingFiles(existing, remove []string) []string {
	if len(existing) == 0 || len(remove) == 0 {
		return existing
	}
	remSet := buildNormalizedFileSet(remove)
	var kept []string
	for _, item := range existing {
		if !matchesNormalizedFileSet(remSet, item) {
			kept = append(kept, item)
		}
	}
	return kept
}

// OverridePatchRequest specifies the fields to update on a preset override.
type OverridePatchRequest struct {
	Engine           *string
	IWAD             *string
	Complevel        *int
	ClearComplevel   bool
	SetEngineArgs    *map[string][]string
	AppendEngineArgs *map[string][]string
	SetArgs          *[]string
	AppendArgs       *[]string
	AppendFiles      *[]FileSpec
	IncludeFiles     *[]string
	ExcludeFiles     *[]string
}

// ConfigPreset creates or updates <userDir>/override.preset.<id>.json for the
// target preset, preserving any previously configured override fields not touched
// by req.
func ConfigPreset(userDir string, layered *Catalog, key string, req OverridePatchRequest) (string, *Preset, error) {
	if err := validateCatalogDirPath(userDir); err != nil {
		return "", nil, err
	}
	target, err := layered.FindExact(key)
	if err != nil {
		return "", nil, err
	}
	if err := ValidateID(target.ID); err != nil {
		return "", nil, fmt.Errorf("invalid preset id %q: %w", target.ID, err)
	}
	if req.IWAD != nil && strings.TrimSpace(*req.IWAD) != "" {
		if _, err := fsutil.CleanRel(strings.TrimSpace(*req.IWAD)); err != nil {
			return "", nil, fmt.Errorf("invalid iwad path %q: %w", *req.IWAD, err)
		}
	}
	if req.AppendFiles != nil {
		for _, f := range *req.AppendFiles {
			if _, err := fsutil.CleanRel(strings.TrimSpace(f.Name)); err != nil {
				return "", nil, fmt.Errorf("invalid file path %q: %w", f.Name, err)
			}
		}
	}

	if req.Engine != nil && *req.Engine != "" && layered != nil {
		eng, ok := layered.FindEngine(*req.Engine)
		if !ok {
			return "", nil, fmt.Errorf("unknown engine %q (register it first with `doom engines add`)", *req.Engine)
		}
		canonical := eng.ID
		req.Engine = &canonical
	}
	if req.Complevel != nil && *req.Complevel != ComplevelUnset {
		if err := ValidateComplevel(*req.Complevel); err != nil {
			return "", nil, err
		}
	}
	var resolvedSetEngArgs map[string][]string
	if req.SetEngineArgs != nil {
		var err error
		resolvedSetEngArgs, err = resolveStoreEngineArgs(layered, *req.SetEngineArgs, true)
		if err != nil {
			return "", nil, err
		}
	}
	var resolvedAppendEngArgs map[string][]string
	if req.AppendEngineArgs != nil {
		var err error
		resolvedAppendEngArgs, err = resolveStoreEngineArgs(layered, *req.AppendEngineArgs, true)
		if err != nil {
			return "", nil, err
		}
	}

	overridePath := filepath.Join(userDir, fmt.Sprintf("override.preset.%s.json", target.ID))
	ov := Override{
		SchemaVersion: SchemaVersion,
		Kind:          KindOverride,
		Target: OverrideTarget{
			Kind: KindPreset,
			ID:   target.ID,
		},
	}

	if existingRaw, readErr := os.ReadFile(overridePath); readErr == nil {
		if existingEnt, decErr := DecodeEntity(existingRaw); decErr == nil && existingEnt.Override != nil {
			ov = *existingEnt.Override
			ov.SchemaVersion = SchemaVersion
			ov.Kind = KindOverride
			ov.Target = OverrideTarget{Kind: KindPreset, ID: target.ID}
		}
	}

	if req.Engine != nil || req.IWAD != nil {
		if ov.Set == nil {
			ov.Set = &OverrideSet{}
		}
		if req.Engine != nil {
			if *req.Engine == "" {
				ov.Set.Engine = nil
			} else {
				ov.Set.Engine = req.Engine
			}
		}
		if req.IWAD != nil {
			if *req.IWAD == "" {
				ov.Set.IWAD = nil
			} else {
				ov.Set.IWAD = req.IWAD
			}
		}
	}
	if req.Complevel != nil || req.ClearComplevel {
		if ov.Set == nil {
			ov.Set = &OverrideSet{}
		}
		if req.ClearComplevel {
			ov.Set.Complevel = nil
		} else {
			ov.Set.Complevel = cloneIntPtr(req.Complevel)
		}
	}
	if req.SetEngineArgs != nil {
		if ov.Set == nil {
			ov.Set = &OverrideSet{}
		}
		mergedMap := make(map[string][]string)
		if ov.Set.EngineArgs != nil {
			for k, v := range *ov.Set.EngineArgs {
				mergedMap[k] = append([]string(nil), v...)
			}
		}
		for k, v := range resolvedSetEngArgs {
			mergedMap[k] = append([]string(nil), v...)
			if len(ov.AppendEngineArgs) > 0 {
				delete(ov.AppendEngineArgs, k)
			}
		}
		if len(mergedMap) == 0 {
			ov.Set.EngineArgs = nil
		} else {
			ov.Set.EngineArgs = &mergedMap
		}
		if len(ov.AppendEngineArgs) == 0 {
			ov.AppendEngineArgs = nil
		}
	}
	if req.AppendEngineArgs != nil {
		if ov.AppendEngineArgs == nil {
			ov.AppendEngineArgs = make(map[string][]string)
		}
		for k, v := range resolvedAppendEngArgs {
			if len(v) == 0 {
				delete(ov.AppendEngineArgs, k)
			} else {
				ov.AppendEngineArgs[k] = append([]string(nil), v...)
			}
		}
		if len(ov.AppendEngineArgs) == 0 {
			ov.AppendEngineArgs = nil
		}
	}
	if req.SetArgs != nil {
		if ov.Set == nil {
			ov.Set = &OverrideSet{}
		}
		s := append([]string(nil), (*req.SetArgs)...)
		ov.Set.Args = &s
		ov.AppendArgs = nil
	} else if req.AppendArgs != nil {
		ov.AppendArgs = append([]string(nil), (*req.AppendArgs)...)
		if ov.Set != nil {
			ov.Set.Args = nil
		}
	}
	if ov.Set != nil && ov.Set.IsEmpty() {
		ov.Set = nil
	}
	if req.AppendFiles != nil {
		ov.AppendFiles = cloneFileSpecs(*req.AppendFiles)
	}
	if req.IncludeFiles != nil {
		ov.IncludeFiles = cleanFileList(*req.IncludeFiles)
		if req.ExcludeFiles == nil && len(ov.IncludeFiles) > 0 {
			ov.ExcludeFiles = filterOutMatchingFiles(ov.ExcludeFiles, ov.IncludeFiles)
		}
	}
	if req.ExcludeFiles != nil {
		ov.ExcludeFiles = cleanFileList(*req.ExcludeFiles)
		if req.IncludeFiles == nil && len(ov.ExcludeFiles) > 0 {
			ov.IncludeFiles = filterOutMatchingFiles(ov.IncludeFiles, ov.ExcludeFiles)
		}
	}

	data, err := MarshalEntity(ov)
	if err != nil {
		return "", nil, err
	}
	if _, err := fsutil.WriteFileAtomic(overridePath, data, 0o644); err != nil {
		return "", nil, err
	}
	return overridePath, target, nil
}

// ResetPresetOverride removes <userDir>/override.preset.<id>.json for the given preset.
func ResetPresetOverride(userDir string, layered *Catalog, key string) (*Preset, bool, error) {
	if err := validateCatalogDirPath(userDir); err != nil {
		return nil, false, err
	}
	target, err := layered.FindExact(key)
	if err != nil {
		return nil, false, err
	}
	if err := ValidateID(target.ID); err != nil {
		return nil, false, fmt.Errorf("invalid preset id %q: %w", target.ID, err)
	}
	overridePath := filepath.Join(userDir, fmt.Sprintf("override.preset.%s.json", target.ID))
	if err := os.Remove(overridePath); err != nil {
		if os.IsNotExist(err) {
			return target, false, nil
		}
		return target, false, err
	}
	fsutil.RemoveBackups(overridePath)
	return target, true, nil
}

// RemovePreset deletes a custom user preset (<userDir>/preset.<id>.json) and any
// accompanying override (<userDir>/override.preset.<id>.json) for the matching preset.
// Built-in curated presets cannot be removed; use ResetPresetOverride instead.
func RemovePreset(userDir string, curated, layered *Catalog, key string) (*Preset, error) {
	if err := validateCatalogDirPath(userDir); err != nil {
		return nil, err
	}
	target, err := layered.FindExact(key)
	if err != nil {
		return nil, err
	}
	if err := ValidateID(target.ID); err != nil {
		return nil, fmt.Errorf("invalid preset id %q: %w", target.ID, err)
	}

	presetPath := filepath.Join(userDir, fmt.Sprintf("preset.%s.json", target.ID))
	isCurated := target.Layer == LayerCurated
	if !isCurated && curated != nil {
		if cp, _ := curated.FindExact(target.ID); cp != nil {
			isCurated = true
		}
	}
	if isCurated {
		if _, statErr := os.Stat(presetPath); os.IsNotExist(statErr) {
			return nil, fmt.Errorf(
				"cannot remove built-in curated preset %q "+
					"(use 'doom wads uninstall %s' to delete downloaded WAD files, "+
					"or 'doom wads reset %s' to clear custom overrides)",
				target.Name, target.ID, target.ID,
			)
		}
	}

	if err := os.Remove(presetPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("custom preset %q not found", key)
		}
		return nil, err
	}
	fsutil.RemoveBackups(presetPath)

	overridePath := filepath.Join(userDir, fmt.Sprintf("override.preset.%s.json", target.ID))
	_ = os.Remove(overridePath)
	fsutil.RemoveBackups(overridePath)

	return target, nil
}
