package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

var personalPathRegex = regexp.MustCompile(
	`(?i)(/home/[a-z0-9_-]+|/Users/[a-z0-9_-]+|C:(?:\\+|/)Users(?:\\+|/)|%USERPROFILE%)`,
)

// FindPersonalPath returns the first personal-path substring matched in s, or "".
func FindPersonalPath(s string) string {
	return personalPathRegex.FindString(s)
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
		SchemaVersion *int `json:"schema-version"`
		Kind          Kind `json:"kind"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return Entity{}, err
	}
	if head.SchemaVersion != nil && *head.SchemaVersion != SchemaVersion {
		return Entity{}, fmt.Errorf("unsupported schema-version %d (want %d)", *head.SchemaVersion, SchemaVersion)
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

func validateEngineArgsKeys(m map[string][]string) error {
	for k := range m {
		clean := strings.ToLower(strings.TrimSpace(k))
		if err := ValidateID(clean); err != nil {
			return fmt.Errorf("engine-args key %q: %w", k, err)
		}
	}
	return nil
}

func validateFileSpecs(files []FileSpec) error {
	for _, f := range files {
		if f.Disabled && !f.Optional {
			return fmt.Errorf("file %q cannot set \"disabled\": true without \"optional\": true", f.Name)
		}
	}
	return nil
}

func validateDecodedEntity(ent Entity) error {
	switch {
	case ent.Engine != nil:
		if err := ValidateID(ent.Engine.ID); err != nil {
			return fmt.Errorf("engine id: %w", err)
		}
		if ent.Engine.ArgsStyle != "" && ent.Engine.ArgsStyle != "boom" && ent.Engine.ArgsStyle != "zdoom" {
			return fmt.Errorf(
				"engine %s: args-style %q must be \"boom\" or \"zdoom\"",
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
		if ent.Preset.Game != "" {
			if err := ValidateID(ent.Preset.Game); err != nil {
				return fmt.Errorf("preset %s game: %w", ent.Preset.ID, err)
			}
		}
		if strings.TrimSpace(ent.Preset.IWAD) == "" {
			return fmt.Errorf("preset %s: missing iwad", ent.Preset.ID)
		}
		if strings.TrimSpace(ent.Preset.Engine) == "" {
			return fmt.Errorf("preset %s: missing engine", ent.Preset.ID)
		}
		if ent.Preset.Complevel != nil {
			if err := ValidateComplevel(*ent.Preset.Complevel); err != nil {
				return fmt.Errorf("preset %s: %w", ent.Preset.ID, err)
			}
		}
		if err := validateEngineArgsKeys(ent.Preset.EngineArgs); err != nil {
			return fmt.Errorf("preset %s: %w", ent.Preset.ID, err)
		}
		if err := validateFileSpecs(ent.Preset.Files); err != nil {
			return fmt.Errorf("preset %s: %w", ent.Preset.ID, err)
		}
	case ent.Override != nil:
		if ent.Override.Target.Kind != KindPreset && ent.Override.Target.Kind != KindEngine {
			return fmt.Errorf("override target kind %q must be \"preset\" or \"engine\"", ent.Override.Target.Kind)
		}
		if err := ValidateID(ent.Override.Target.ID); err != nil {
			return fmt.Errorf("override target id: %w", err)
		}
		if ent.Override.Set != nil && ent.Override.Set.Game != nil && *ent.Override.Set.Game != "" {
			if err := ValidateID(*ent.Override.Set.Game); err != nil {
				return fmt.Errorf(
					"override %s/%s game: %w",
					ent.Override.Target.Kind, ent.Override.Target.ID, err,
				)
			}
		}
		if ent.Override.Set != nil && ent.Override.Set.Complevel != nil {
			cl := *ent.Override.Set.Complevel
			if cl != ComplevelUnset {
				if err := ValidateComplevel(cl); err != nil {
					return fmt.Errorf(
						"override %s/%s: %w",
						ent.Override.Target.Kind, ent.Override.Target.ID, err,
					)
				}
			}
		}
		if ent.Override.Set != nil && ent.Override.Set.EngineArgs != nil {
			if err := validateEngineArgsKeys(*ent.Override.Set.EngineArgs); err != nil {
				return fmt.Errorf(
					"override %s/%s: %w",
					ent.Override.Target.Kind, ent.Override.Target.ID, err,
				)
			}
		}
		if ent.Override.Set != nil && ent.Override.Set.Files != nil {
			if err := validateFileSpecs(*ent.Override.Set.Files); err != nil {
				return fmt.Errorf(
					"override %s/%s: %w",
					ent.Override.Target.Kind, ent.Override.Target.ID, err,
				)
			}
		}
		if err := validateFileSpecs(ent.Override.AppendFiles); err != nil {
			return fmt.Errorf(
				"override %s/%s: %w",
				ent.Override.Target.Kind, ent.Override.Target.ID, err,
			)
		}
		if err := validateEngineArgsKeys(ent.Override.AppendEngineArgs); err != nil {
			return fmt.Errorf(
				"override %s/%s: %w",
				ent.Override.Target.Kind, ent.Override.Target.ID, err,
			)
		}
		if ent.Override.Set != nil && ent.Override.Set.ArgsStyle != nil {
			style := *ent.Override.Set.ArgsStyle
			if style != "" && style != "boom" && style != "zdoom" {
				return fmt.Errorf(
					"override %s/%s: args-style %q must be \"boom\" or \"zdoom\"",
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

// ScanOptions configures entity directory scanning and diagnostic reporting.
type ScanOptions struct {
	DisplayPrefix      string
	Strict             bool
	CheckPersonalPaths bool
	Severity           string
	QuarantinePrefix   bool
}

// ScanFS reads and decodes every non-hidden .json entity file in the root of fsys.
func ScanFS(fsys fs.FS, opts ScanOptions) ([]Entity, []Diagnostic, error) {
	severity := opts.Severity
	if severity == "" {
		severity = "warn"
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		if opts.Strict {
			return nil, nil, err
		}
		return nil, []Diagnostic{{
			Path:     opts.DisplayPrefix,
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
		if opts.DisplayPrefix != "" && opts.DisplayPrefix != "." {
			where = path.Join(opts.DisplayPrefix, name)
		}
		raw, readErr := fs.ReadFile(fsys, name)
		if readErr != nil {
			if opts.Strict {
				return nil, nil, fmt.Errorf("%s: %w", where, readErr)
			}
			diags = append(diags, Diagnostic{Path: where, Severity: severity, Message: readErr.Error()})
			continue
		}
		if opts.CheckPersonalPaths {
			if match := FindPersonalPath(string(raw)); match != "" {
				diags = append(diags, Diagnostic{
					Path:     where,
					Severity: severity,
					Message:  fmt.Sprintf("contains hardcoded personal path %q", match),
				})
			}
		}
		ent, decErr := DecodeEntity(raw)
		if decErr != nil {
			if opts.Strict {
				return nil, nil, fmt.Errorf("%s: %w", where, decErr)
			}
			msg := decErr.Error()
			if opts.QuarantinePrefix {
				msg = fmt.Sprintf("quarantined invalid entity file (%v)", decErr)
			}
			diags = append(diags, Diagnostic{Path: where, Severity: severity, Message: msg})
			continue
		}
		wantName := ExpectedFilename(ent)
		if name != wantName {
			msg := fmt.Sprintf("filename %q does not match entity identity (want %q)", name, wantName)
			if opts.Strict {
				return nil, nil, fmt.Errorf("%s: %s", where, msg)
			}
			if opts.QuarantinePrefix {
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
