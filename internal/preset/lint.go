package preset

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lock14/doom-cli"
)

var personalPathRegex = regexp.MustCompile(`(?i)(/home/[a-z0-9_-]+|/Users/[a-z0-9_-]+|C:\\Users\\|%USERPROFILE%)`)

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

// HasErrors reports whether any diagnostic in diags has severity "error".
func HasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// LintCatalog validates curated entities (from <rootDir>/catalog if present,
// otherwise embedded), optional user entities in userDir, template files, and
// README.md table synchronization when <rootDir>/README.md exists.
func LintCatalog(rootDir, userDir string) ([]Diagnostic, error) {
	var diags []Diagnostic
	addErr := func(where, format string, a ...any) {
		diags = append(diags, Diagnostic{
			Path:     where,
			Severity: "error",
			Message:  fmt.Sprintf(format, a...),
		})
	}
	addWarn := func(where, format string, a ...any) {
		diags = append(diags, Diagnostic{
			Path:     where,
			Severity: "warn",
			Message:  fmt.Sprintf(format, a...),
		})
	}

	var curatedFS fs.FS
	curatedPrefix := "catalog"
	if strings.TrimSpace(rootDir) != "" {
		diskCat := filepath.Join(rootDir, "catalog")
		if info, err := os.Stat(diskCat); err == nil && info.IsDir() {
			curatedFS = os.DirFS(diskCat)
			curatedPrefix = filepath.ToSlash(diskCat)
		}
	}
	if curatedFS == nil {
		sub, err := fs.Sub(doomcli.FS, "catalog")
		if err != nil {
			return nil, err
		}
		curatedFS = sub
	}

	curatedEnts, curatedDiags, _ := scanEntityFS(curatedFS, dirScanOptions{
		displayPrefix:      curatedPrefix,
		strict:             false,
		checkPersonalPaths: true,
		severity:           "error",
	})
	diags = append(diags, curatedDiags...)

	var userEnts []Entity
	if strings.TrimSpace(userDir) != "" {
		if info, err := os.Stat(userDir); err == nil && info.IsDir() {
			uEnts, uDiags, _ := scanEntityFS(os.DirFS(userDir), dirScanOptions{
				displayPrefix:      filepath.ToSlash(userDir),
				strict:             false,
				checkPersonalPaths: true,
				severity:           "error",
			})
			userEnts = uEnts
			diags = append(diags, uDiags...)
		}
	}

	engineIDs := make(map[string]string)
	presetIDs := make(map[string]string)
	presetIWADs := make(map[string]string)
	sortOwners := make(map[int]string)
	fileAliasTargets := make(map[string]string)
	canonicalFiles := make(map[string]string)

	type taggedEnt struct {
		ent     Entity
		curated bool
	}
	var all []taggedEnt
	for _, e := range curatedEnts {
		all = append(all, taggedEnt{ent: e, curated: true})
	}
	for _, e := range userEnts {
		all = append(all, taggedEnt{ent: e, curated: false})
	}

	for _, te := range all {
		if te.ent.Preset == nil {
			continue
		}
		for _, f := range te.ent.Preset.Files {
			base := filepath.Base(strings.TrimSpace(f.Name))
			if norm := NormalizeFilename(base); norm != "" {
				if _, exists := canonicalFiles[norm]; !exists {
					canonicalFiles[norm] = base
				}
			}
		}
	}

	validateFiles := func(where, ownerID, iwad string, files []FileSpec) {
		iwadNorm := NormalizeFilename(iwad)
		for _, f := range files {
			if strings.TrimSpace(f.Name) == "" {
				addErr(where, "preset %s has an empty file name", ownerID)
				continue
			}
			fileNorm := NormalizeFilename(filepath.Base(f.Name))
			if iwadNorm != "" && fileNorm == iwadNorm {
				addErr(where, "iwad %q must not be repeated in files", iwad)
			}
			if err := ValidateSHA256(f.SHA256); err != nil {
				addErr(where, "file %s: %v", f.Name, err)
			}
			seenAliases := make(map[string]bool)
			for _, al := range f.Aliases {
				trimmedAl := strings.TrimSpace(al)
				if trimmedAl == "" {
					addErr(where, "file %s has an empty alias", f.Name)
					continue
				}
				if strings.ContainsAny(trimmedAl, `/\`) {
					addErr(where, "file %s alias %q must not contain path separators", f.Name, al)
					continue
				}
				alNorm := NormalizeFilename(trimmedAl)
				if (iwadNorm != "" && alNorm == iwadNorm) || isIWADName(trimmedAl) {
					matchedIWAD := iwad
					if matchedIWAD == "" || alNorm != iwadNorm {
						matchedIWAD = trimmedAl
					}
					addErr(where, "file %s alias %q must not match iwad %q", f.Name, al, matchedIWAD)
				}
				if alNorm == fileNorm {
					addErr(
						where,
						"file %s alias %q is redundant with normalized filename",
						f.Name, al,
					)
				}
				if seenAliases[alNorm] {
					addErr(where, "file %s has duplicate normalized alias %q", f.Name, al)
				}
				seenAliases[alNorm] = true
				if canonName, exists := canonicalFiles[alNorm]; exists && alNorm != fileNorm {
					addErr(
						where,
						"file %s alias %q conflicts with canonical target %q",
						f.Name, al, canonName,
					)
				} else if prevTarget, exists := fileAliasTargets[alNorm]; exists && prevTarget != fileNorm {
					addErr(
						where,
						"file %s alias %q conflicts with canonical target %q",
						f.Name, al, prevTarget,
					)
				} else {
					fileAliasTargets[alNorm] = fileNorm
				}
			}
		}
	}

	for _, te := range all {
		if te.ent.Engine == nil {
			continue
		}
		eng := te.ent.Engine
		where := te.ent.Path
		if prev, exists := engineIDs[eng.ID]; exists && te.curated {
			addErr(where, "duplicate engine id %q (already in %s)", eng.ID, prev)
		}
		engineIDs[eng.ID] = where
		for _, al := range eng.Aliases {
			cleanAl := strings.ToLower(strings.TrimSpace(al))
			if err := ValidateID(cleanAl); err != nil {
				addErr(where, "invalid engine alias %q: %v", al, err)
				continue
			}
			if prev, exists := engineIDs[cleanAl]; exists && prev != where {
				addErr(where, "engine alias %q conflicts with %s", al, prev)
			}
			engineIDs[cleanAl] = where
		}
		if eng.Install != nil {
			for plat, pr := range eng.Install.Platforms {
				if !strings.Contains(plat, "/") {
					addErr(where, "invalid platform key %q (want goos/goarch)", plat)
				}
				for _, pat := range pr.Assets {
					if _, err := regexp.Compile(pat); err != nil {
						addErr(where, "invalid asset regex %q for %s: %v", pat, plat, err)
					}
				}
				if pr.Pinned != nil {
					if !strings.HasPrefix(pr.Pinned.URL, "https://") {
						addErr(where, "pinned URL for %s must start with https://", plat)
					}
					if err := ValidateSHA256(pr.Pinned.SHA256); err != nil {
						addErr(where, "pinned sha256 for %s: %v", plat, err)
					}
				}
			}
		}
	}

	for _, te := range all {
		if te.ent.Preset == nil {
			continue
		}
		p := te.ent.Preset
		where := te.ent.Path
		if prev, exists := presetIDs[p.ID]; exists && te.curated {
			addErr(where, "duplicate preset id %q (already in %s)", p.ID, prev)
		}
		presetIDs[p.ID] = where
		presetIWADs[p.ID] = p.IWAD

		engKey := strings.ToLower(strings.TrimSpace(p.Engine))
		if _, ok := engineIDs[engKey]; !ok {
			addErr(where, "preset %s references unknown engine %q", p.ID, p.Engine)
		}

		validateFiles(where, p.ID, p.IWAD, p.Files)

		for _, s := range p.Sources {
			if !strings.HasPrefix(s.URL, "https://") {
				addErr(where, "source URL %q must use https://", s.URL)
			}
			if !IsSupportedSourceURL(s.URL) {
				addErr(where, "source URL %q must end with .zip, .wad, or .pk3", s.URL)
			}
			if err := ValidateSHA256(s.SHA256); err != nil {
				addErr(where, "source %s: %v", s.URL, err)
			}
		}

		for _, l := range p.Links {
			if !strings.HasPrefix(l.URL, "https://") && !strings.HasPrefix(l.URL, "http://") {
				addErr(where, "link URL %q must use http:// or https://", l.URL)
			}
		}

		if te.curated {
			if strings.TrimSpace(p.Author) == "" ||
				strings.TrimSpace(p.ReleaseDate) == "" ||
				strings.TrimSpace(p.Category) == "" {
				addErr(where, "curated preset %s requires author, release_date, and category", p.ID)
			}
			if p.Sort <= 0 {
				addWarn(where, "curated preset %s has no positive sort order", p.ID)
			} else if prev, dup := sortOwners[p.Sort]; dup {
				addErr(where, "sort %d on preset %s collides with %s", p.Sort, p.ID, prev)
			} else {
				sortOwners[p.Sort] = p.ID
			}
		}
	}

	for _, te := range all {
		if te.ent.Override == nil {
			continue
		}
		ov := te.ent.Override
		where := te.ent.Path
		switch ov.Target.Kind {
		case KindPreset:
			if _, ok := presetIDs[ov.Target.ID]; !ok {
				addErr(where, "override target preset/%s does not exist", ov.Target.ID)
			}
			ovIWAD := presetIWADs[ov.Target.ID]
			if ov.Set != nil {
				if ov.Set.IWAD != nil && *ov.Set.IWAD != "" {
					ovIWAD = *ov.Set.IWAD
				}
				if ov.Set.Files != nil {
					validateFiles(where, ov.Target.ID, ovIWAD, *ov.Set.Files)
				}
			}
			if len(ov.AppendFiles) > 0 {
				validateFiles(where, ov.Target.ID, ovIWAD, ov.AppendFiles)
			}
		case KindEngine:
			if _, ok := engineIDs[ov.Target.ID]; !ok {
				addErr(where, "override target engine/%s does not exist", ov.Target.ID)
			}
		}
		if ov.Set != nil && ov.Set.Engine != nil && *ov.Set.Engine != "" {
			if _, ok := engineIDs[strings.ToLower(strings.TrimSpace(*ov.Set.Engine))]; !ok {
				addErr(where, "override engine %q does not exist", *ov.Set.Engine)
			}
		}
	}

	if strings.TrimSpace(rootDir) != "" {
		for _, relCfg := range []string{
			filepath.Join("dsda-doom", "dsda-doom.cfg"),
			filepath.Join("uzdoom", "autoexec.cfg"),
		} {
			cfgPath := filepath.Join(rootDir, relCfg)
			if raw, err := os.ReadFile(cfgPath); err == nil {
				if match := personalPathRegex.FindString(string(raw)); match != "" {
					addErr(filepath.ToSlash(cfgPath), "contains personal path %q", match)
				}
			}
		}

		readmePath := filepath.Join(rootDir, "README.md")
		if readmeBytes, err := os.ReadFile(readmePath); err == nil && !HasErrors(curatedDiags) {
			curatedCat := buildCatalog(curatedEnts, nil)
			wantTable := GenerateReadmeTable(curatedCat)
			matches := readmeTableRegex.FindStringSubmatch(string(readmeBytes))
			if len(matches) < 3 {
				addErr(filepath.ToSlash(readmePath), "could not locate '## Preconfigured Presets' markdown table")
			} else if strings.TrimSpace(matches[2]) != strings.TrimSpace(wantTable) {
				addErr(
					filepath.ToSlash(readmePath),
					"README.md preset table is out of sync with catalog/; run 'doom presets build'",
				)
			}
		}
	}

	sort.SliceStable(diags, func(i, j int) bool {
		if diags[i].Path != diags[j].Path {
			return diags[i].Path < diags[j].Path
		}
		return diags[i].Message < diags[j].Message
	})
	return diags, nil
}
