package preset

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

// AddEngine writes or updates a user engine entity file in userDir (<userDir>/engine.<id>.json).
// It refuses to shadow a curated engine ID or alias.
func AddEngine(userDir string, curated *Catalog, eng EngineConfig) (string, error) {
	if strings.TrimSpace(userDir) == "" {
		return "", errors.New("empty user catalog directory")
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
				"engine id %q collides with curated engine %q; pass a different --id or use an override",
				eng.ID, existing.ID,
			)
		}
	}

	eng.SchemaVersion = SchemaVersion
	eng.Kind = KindEngine
	eng.ArgsStyle = eng.EffectiveArgsStyle()
	if eng.ArgsStyle != "boom" && eng.ArgsStyle != "zdoom" {
		return "", fmt.Errorf("invalid args_style %q (must be \"boom\" or \"zdoom\")", eng.ArgsStyle)
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

// RemoveEngine deletes a user engine entity from userDir by ID or alias.
// Built-in curated engines cannot be removed. Returns the removed file path and
// any preset names in layered that still reference the removed engine.
func RemoveEngine(userDir string, curated, layered *Catalog, key string) (string, []string, error) {
	if strings.TrimSpace(userDir) == "" {
		return "", nil, errors.New("empty user catalog directory")
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
	if strings.TrimSpace(userDir) == "" {
		return "", errors.New("empty user catalog directory")
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
					"preset %q collides with curated preset %q (id %q); pass --id <slug> or use `doom presets config`",
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
	if strings.TrimSpace(p.Category) == "" {
		p.Category = "Custom"
	}
	for _, f := range p.Files {
		if err := ValidateSHA256(f.SHA256); err != nil {
			return "", fmt.Errorf("file %s: %w", f.Name, err)
		}
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

// OverridePatchRequest specifies the fields to update on a preset override.
type OverridePatchRequest struct {
	Engine      *string
	IWAD        *string
	AppendArgs  *[]string
	AppendFiles *[]FileSpec
}

// ConfigPreset creates or updates <userDir>/override.preset.<id>.json for the
// target preset, preserving any previously configured override fields not touched
// by req.
func ConfigPreset(userDir string, layered *Catalog, key string, req OverridePatchRequest) (string, *Preset, error) {
	if strings.TrimSpace(userDir) == "" {
		return "", nil, errors.New("empty user catalog directory")
	}
	target, err := layered.FindExact(key)
	if err != nil {
		return "", nil, err
	}

	if req.Engine != nil && *req.Engine != "" && layered != nil {
		eng, ok := layered.FindEngine(*req.Engine)
		if !ok {
			return "", nil, fmt.Errorf("unknown engine %q (register it first with `doom engines add`)", *req.Engine)
		}
		canonical := eng.ID
		req.Engine = &canonical
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
			ov.Set.Engine = req.Engine
		}
		if req.IWAD != nil {
			ov.Set.IWAD = req.IWAD
		}
	}
	if req.AppendArgs != nil {
		ov.AppendArgs = append([]string(nil), (*req.AppendArgs)...)
	}
	if req.AppendFiles != nil {
		ov.AppendFiles = append([]FileSpec(nil), (*req.AppendFiles)...)
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
	if strings.TrimSpace(userDir) == "" {
		return nil, false, errors.New("empty user catalog directory")
	}
	target, err := layered.FindExact(key)
	if err != nil {
		return nil, false, err
	}
	overridePath := filepath.Join(userDir, fmt.Sprintf("override.preset.%s.json", target.ID))
	if err := os.Remove(overridePath); err != nil {
		if os.IsNotExist(err) {
			return target, false, nil
		}
		return target, false, err
	}
	return target, true, nil
}

// RemovePreset deletes a user preset (<userDir>/preset.<id>.json) and/or any
// user override (<userDir>/override.preset.<id>.json) for the matching preset.
// If the target is a built-in curated preset with no user files, RemovePreset returns an error.
func RemovePreset(userDir string, curated, layered *Catalog, key string) (*Preset, error) {
	if strings.TrimSpace(userDir) == "" {
		return nil, errors.New("empty user catalog directory")
	}
	target, err := layered.FindExact(key)
	if err != nil {
		return nil, err
	}

	presetPath := filepath.Join(userDir, fmt.Sprintf("preset.%s.json", target.ID))
	overridePath := filepath.Join(userDir, fmt.Sprintf("override.preset.%s.json", target.ID))

	removedAny := false
	if err := os.Remove(presetPath); err == nil {
		removedAny = true
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.Remove(overridePath); err == nil {
		removedAny = true
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if !removedAny {
		if curated != nil {
			if cp, _ := curated.FindExact(target.ID); cp != nil {
				return nil, fmt.Errorf("cannot remove built-in curated preset %q", target.Name)
			}
		}
		return nil, fmt.Errorf("custom preset %q not found", key)
	}
	return target, nil
}
