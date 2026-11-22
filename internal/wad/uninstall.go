package wad

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/fsutil"
)

// UninstallPreset removes preset-exclusive PWAD, PK3, DeHackEd, and companion README files
// (along with any .bak.* backups) from primaryWadsDir while protecting base IWADs, assets shared
// by other presets in cat, and files residing in secondary read-only extraWadRoots.
func UninstallPreset(
	p catalog.Preset,
	cat *catalog.Catalog,
	primaryWadsDir string,
	extraWadRoots []string,
	out io.Writer,
) (int, error) {
	if out == nil {
		out = io.Discard
	}
	if !p.HasRequiredFiles() {
		return 0, fmt.Errorf(
			"preset %q is a base IWAD game (%s); base IWADs are protected from 'doom wads uninstall'",
			p.Name,
			p.IWAD,
		)
	}
	if strings.TrimSpace(primaryWadsDir) == "" {
		return 0, errors.New("primary WADs directory is not configured")
	}

	sharedNorms := collectSharedFileNorms(p, cat)
	roots := append([]string{primaryWadsDir}, extraWadRoots...)
	idx := catalog.NewDirIndex(roots...).WithCatalog(cat).WithPresets(p)

	var candidates []string
	seen := make(map[string]bool)
	addCandidate := func(cand string) {
		clean := filepath.Clean(cand)
		if clean == "" || seen[clean] {
			return
		}
		seen[clean] = true
		candidates = append(candidates, clean)
	}

	var exclusiveReadme string
	if readmePath, ok := idx.ResolveReadme(p); ok {
		if !isSharedReadme(p, readmePath, cat, idx) {
			exclusiveReadme = readmePath
		}
	}

	for _, f := range p.Files {
		if catalog.IsIWADName(f.Name) {
			continue
		}
		baseNorm := catalog.NormalizeFilename(catalog.SlashBase(f.Name))
		if baseNorm == "" || sharedNorms[baseNorm] {
			continue
		}
		resolvedPath, ok := idx.ResolveFileSpec(f)
		if !ok {
			continue
		}
		resBase := filepath.Base(resolvedPath)
		resNorm := catalog.NormalizeFilename(resBase)
		if catalog.IsIWADName(resBase) || sharedNorms[resNorm] {
			continue
		}
		addCandidate(resolvedPath)
	}

	if exclusiveReadme != "" {
		addCandidate(exclusiveReadme)
	}

	cleanPrimary := filepath.Clean(primaryWadsDir)
	removed := 0
	readOnlySkipped := 0
	for _, cand := range candidates {
		if !isContainedRegularFile(cand, cleanPrimary) {
			fmt.Fprintf(out, "ℹ Skipping %s (located in secondary read-only WAD directory)\n", cand)
			readOnlySkipped++
			continue
		}
		if err := os.Remove(cand); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		fsutil.RemoveBackups(cand)
		parent := filepath.Clean(filepath.Dir(cand))
		if parent != cleanPrimary {
			_ = os.Remove(parent)
		}
		fmt.Fprintf(out, "✓ Removed %s\n", cand)
		removed++
	}

	if removed == 0 {
		if readOnlySkipped > 0 {
			return 0, fmt.Errorf("cannot uninstall %s: files reside in read-only wads-dirs", p.Name)
		}
		fmt.Fprintf(out, "ℹ %s is not installed in %s\n", p.Name, primaryWadsDir)
		return 0, nil
	}

	fmt.Fprintf(out, "✓ Uninstalled %s (%d file(s) removed)\n", p.Name, removed)
	return removed, nil
}

func collectSharedFileNorms(target catalog.Preset, cat *catalog.Catalog) map[string]bool {
	shared := make(map[string]bool)
	add := func(raw string) {
		norm := catalog.NormalizeFilename(catalog.SlashBase(raw))
		if norm != "" {
			shared[norm] = true
		}
	}
	add(target.IWAD)
	if cat == nil {
		return shared
	}
	for _, other := range cat.Presets {
		if isSamePresetIdentity(target, other) {
			continue
		}
		add(other.IWAD)
		for _, f := range other.Files {
			add(f.Name)
			for _, al := range f.Aliases {
				add(al)
			}
		}
	}
	return shared
}

func isSamePresetIdentity(a, b catalog.Preset) bool {
	if a.ID != "" && b.ID != "" {
		return strings.EqualFold(a.ID, b.ID)
	}
	return strings.EqualFold(a.Name, b.Name)
}

func isSharedReadme(
	target catalog.Preset,
	readmePath string,
	cat *catalog.Catalog,
	idx *catalog.DirIndex,
) bool {
	base := filepath.Base(readmePath)
	normBase := catalog.NormalizeFilename(base)
	iwadStem := strings.TrimSuffix(
		catalog.SlashBase(target.IWAD),
		filepath.Ext(catalog.SlashBase(target.IWAD)),
	)
	if iwadStem != "" && normBase == catalog.NormalizeFilename(iwadStem+".txt") {
		return true
	}
	if cat == nil || idx == nil {
		return false
	}
	cleanReadme := filepath.Clean(readmePath)
	for _, other := range cat.Presets {
		if isSamePresetIdentity(target, other) {
			continue
		}
		if otherReadme, ok := idx.ResolveReadme(other); ok && filepath.Clean(otherReadme) == cleanReadme {
			return true
		}
	}
	return false
}

func isContainedRegularFile(filePath, cleanPrimary string) bool {
	fi, err := os.Lstat(filePath)
	if err != nil || fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return false
	}

	realPrimary := cleanPrimary
	if resolved, evalErr := filepath.EvalSymlinks(cleanPrimary); evalErr == nil {
		realPrimary = filepath.Clean(resolved)
	}

	realParent, evalErr := filepath.EvalSymlinks(filepath.Dir(filePath))
	if evalErr != nil {
		return false
	}
	realParent = filepath.Clean(realParent)
	if realParent == realPrimary {
		return true
	}
	prefix := realPrimary
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(realParent, prefix)
}
