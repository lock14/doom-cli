package preset

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lock14/doom-cli/internal/fsutil"
)

var filenameReplacer = strings.NewReplacer(" ", "", "-", "", "_", "")

var knownIWADNames = map[string]bool{
	"doom.wad":      true,
	"doom1.wad":     true,
	"doom2.wad":     true,
	"tnt.wad":       true,
	"plutonia.wad":  true,
	"heretic.wad":   true,
	"hexen.wad":     true,
	"strife1.wad":   true,
	"freedoom1.wad": true,
	"freedoom2.wad": true,
}

func isIWADName(name string) bool {
	return knownIWADNames[strings.ToLower(strings.TrimSpace(name))]
}

var (
	curatedAliasesOnce sync.Once
	curatedAliasMap    map[string][]string
)

func buildAliasMap(presets []Preset) map[string][]string {
	groups := [][]string{
		{"DOOM.WAD", "DOOM1.WAD"},
	}
	for _, p := range presets {
		for _, f := range p.Files {
			if len(f.Aliases) == 0 {
				continue
			}
			base := filepath.Base(strings.TrimSpace(f.Name))
			if base == "" || isIWADName(base) {
				continue
			}
			grp := make([]string, 0, 1+len(f.Aliases))
			grp = append(grp, base)
			for _, al := range f.Aliases {
				trimmed := strings.TrimSpace(al)
				if trimmed == "" || strings.ContainsAny(trimmed, `/\`) || isIWADName(trimmed) {
					continue
				}
				grp = append(grp, trimmed)
			}
			if len(grp) > 1 {
				groups = append(groups, grp)
			}
		}
	}

	out := make(map[string][]string)
	for _, grp := range groups {
		for _, member := range grp {
			key := NormalizeFilename(member)
			if key == "" {
				continue
			}
			existing := out[key]
			for _, other := range grp {
				otherNorm := NormalizeFilename(other)
				if otherNorm == "" || otherNorm == key {
					continue
				}
				dup := false
				for _, prev := range existing {
					if NormalizeFilename(prev) == otherNorm {
						dup = true
						break
					}
				}
				if !dup {
					existing = append(existing, other)
				}
			}
			out[key] = existing
		}
	}
	return out
}

func mergeAliasMaps(maps ...map[string][]string) map[string][]string {
	out := make(map[string][]string)
	for _, m := range maps {
		for key, aliases := range m {
			existing := out[key]
			for _, al := range aliases {
				alNorm := NormalizeFilename(al)
				if alNorm == "" || alNorm == key {
					continue
				}
				dup := false
				for _, prev := range existing {
					if NormalizeFilename(prev) == alNorm {
						dup = true
						break
					}
				}
				if !dup {
					existing = append(existing, al)
				}
			}
			out[key] = existing
		}
	}
	return out
}

func getCuratedAliasMap() map[string][]string {
	curatedAliasesOnce.Do(func() {
		_, cat, err := loadEmbeddedCurated()
		if err != nil || cat == nil {
			curatedAliasMap = buildAliasMap(nil)
			return
		}
		curatedAliasMap = buildAliasMap(cat.Presets)
	})
	return curatedAliasMap
}

// FileAliases returns any recognized alternate filenames for name across the catalog's presets.
func (c *Catalog) FileAliases(name string) []string {
	key := NormalizeFilename(filepath.Base(strings.TrimSpace(name)))
	if key == "" {
		return nil
	}
	if c == nil {
		return append([]string(nil), getCuratedAliasMap()[key]...)
	}
	merged := mergeAliasMaps(getCuratedAliasMap(), buildAliasMap(c.Presets))
	return append([]string(nil), merged[key]...)
}

// KnownAliases returns any recognized alternate filenames for a given target filename
// derived from the curated preset catalog and base IWAD mappings.
func KnownAliases(name string) []string {
	key := NormalizeFilename(filepath.Base(strings.TrimSpace(name)))
	if key == "" {
		return nil
	}
	return append([]string(nil), getCuratedAliasMap()[key]...)
}

type rootCache struct {
	dir     string
	entries []os.DirEntry
	subOnce sync.Once
	subdirs []subDirCache
}

type subDirCache struct {
	dir     string
	entries []os.DirEntry
}

// DirIndex caches directory entries across one or more WAD root directories for
// fast case-insensitive, normalized, alias-tolerant, and 1-level subdirectory resolution.
type DirIndex struct {
	roots    []*rootCache
	aliasMap map[string][]string
}

// NewDirIndex creates a DirIndex for the provided WAD root directories (root 0 is primary).
func NewDirIndex(roots ...string) *DirIndex {
	idx := &DirIndex{}
	seen := make(map[string]bool)
	for _, r := range roots {
		trimmed := strings.TrimSpace(r)
		if trimmed == "" {
			continue
		}
		clean := filepath.Clean(trimmed)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		rc := &rootCache{dir: trimmed}
		if entries, err := os.ReadDir(trimmed); err == nil {
			rc.entries = entries
		}
		idx.roots = append(idx.roots, rc)
	}
	return idx
}

// WithCatalog configures the DirIndex to resolve file aliases declared across cat
// in addition to the embedded curated catalog. It returns idx for method chaining.
func (idx *DirIndex) WithCatalog(cat *Catalog) *DirIndex {
	if idx == nil || cat == nil {
		return idx
	}
	return idx.WithPresets(cat.Presets...)
}

// WithPresets configures the DirIndex to resolve file aliases declared on presets
// in addition to the embedded curated catalog. It returns idx for method chaining.
func (idx *DirIndex) WithPresets(presets ...Preset) *DirIndex {
	if idx == nil || len(presets) == 0 {
		return idx
	}
	base := idx.aliasMap
	if base == nil {
		base = getCuratedAliasMap()
	}
	idx.aliasMap = mergeAliasMaps(base, buildAliasMap(presets))
	return idx
}

func (idx *DirIndex) aliasesFor(targetName string) []string {
	key := NormalizeFilename(filepath.Base(strings.TrimSpace(targetName)))
	if key == "" {
		return nil
	}
	if idx != nil && idx.aliasMap != nil {
		return idx.aliasMap[key]
	}
	return getCuratedAliasMap()[key]
}

func (idx *DirIndex) fileAliases(f FileSpec) []string {
	var out []string
	seen := make(map[string]bool)
	add := func(al string) {
		trimmed := strings.TrimSpace(al)
		if trimmed == "" {
			return
		}
		norm := NormalizeFilename(trimmed)
		if norm == "" || seen[norm] {
			return
		}
		seen[norm] = true
		out = append(out, trimmed)
	}
	for _, al := range f.Aliases {
		add(al)
	}
	for _, al := range idx.aliasesFor(f.Name) {
		add(al)
	}
	return out
}

func (rc *rootCache) loadSubdirs() []subDirCache {
	rc.subOnce.Do(func() {
		var names []string
		for _, entry := range rc.entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			subPath := filepath.Join(rc.dir, name)
			if subEntries, err := os.ReadDir(subPath); err == nil {
				rc.subdirs = append(rc.subdirs, subDirCache{
					dir:     subPath,
					entries: subEntries,
				})
			}
		}
	})
	return rc.subdirs
}

// ResolveFile searches for targetName across indexed WAD roots with case-insensitivity,
// normalization, known aliases, safe relative sub-paths, and 1-level subdirectory fallback
// for non-IWAD files.
func (idx *DirIndex) ResolveFile(targetName string) (string, bool) {
	if idx == nil || len(idx.roots) == 0 || strings.TrimSpace(targetName) == "" {
		return "", false
	}

	// Relative sub-path (such as "sub/foo.wad" or "sub\foo.wad")
	if strings.ContainsAny(targetName, `/\`) {
		return idx.resolveRelativeSubpath(targetName)
	}

	// 1. Direct match across root 0..N top-level entries
	for _, rc := range idx.roots {
		if p, ok := resolveSingle(rc.dir, rc.entries, targetName); ok {
			return p, true
		}
	}

	// 2. Alias match across root 0..N top-level entries
	aliases := idx.aliasesFor(targetName)
	for _, alias := range aliases {
		for _, rc := range idx.roots {
			if p, ok := resolveSingle(rc.dir, rc.entries, alias); ok {
				return p, true
			}
		}
	}

	// 3. For non-IWAD files, scan 1-level child directories across root 0..N
	if !isIWADName(targetName) {
		for _, rc := range idx.roots {
			for _, sub := range rc.loadSubdirs() {
				if p, ok := resolveSingle(sub.dir, sub.entries, targetName); ok {
					return p, true
				}
			}
		}
		for _, alias := range aliases {
			for _, rc := range idx.roots {
				for _, sub := range rc.loadSubdirs() {
					if p, ok := resolveSingle(sub.dir, sub.entries, alias); ok {
						return p, true
					}
				}
			}
		}
	}

	return "", false
}

func (idx *DirIndex) resolveRelativeSubpath(targetName string) (string, bool) {
	cleanRel, err := fsutil.CleanRel(targetName)
	if err != nil {
		return "", false
	}
	parts := strings.Split(cleanRel, "/")
	baseName := parts[len(parts)-1]
	subDirRel := strings.Join(parts[:len(parts)-1], "/")

	for _, rc := range idx.roots {
		parentDir := rc.dir
		if subDirRel != "" {
			var parentErr error
			parentDir, parentErr = fsutil.SafeJoin(rc.dir, subDirRel)
			if parentErr != nil {
				continue
			}
		}
		entries, readErr := os.ReadDir(parentDir)
		if readErr != nil {
			continue
		}
		if p, ok := resolveSingle(parentDir, entries, baseName); ok {
			return p, true
		}
		for _, alias := range idx.aliasesFor(baseName) {
			if p, ok := resolveSingle(parentDir, entries, alias); ok {
				return p, true
			}
		}
	}
	return "", false
}

// ResolveReadme locates an accompanying documentation or text file for the preset.
// Base IWAD presets (those with no non-optional files) fall back to <iwad>.txt,
// whereas mapsets with required PWADs never borrow the base IWAD's text file.
func (idx *DirIndex) ResolveReadme(p Preset) (string, bool) {
	if idx == nil || len(idx.roots) == 0 {
		return "", false
	}
	for _, f := range p.Files {
		ext := filepath.Ext(f.Name)
		base := strings.TrimSuffix(f.Name, ext)
		if path, ok := idx.ResolveFile(base + ".txt"); ok {
			return path, true
		}
		for _, al := range idx.fileAliases(f) {
			alExt := filepath.Ext(al)
			alBase := strings.TrimSuffix(al, alExt)
			if alBase != "" {
				if path, ok := idx.ResolveFile(alBase + ".txt"); ok {
					return path, true
				}
			}
		}
	}
	nameClean := strings.ReplaceAll(p.Name, " ", "") + ".txt"
	if path, ok := idx.ResolveFile(nameClean); ok {
		return path, true
	}
	if path, ok := idx.resolveReadmePrefix(p); ok {
		return path, true
	}
	if !p.HasRequiredFiles() {
		iwadBase := strings.TrimSuffix(p.IWAD, filepath.Ext(p.IWAD))
		if path, ok := idx.ResolveFile(iwadBase + ".txt"); ok {
			return path, true
		}
	}
	return "", false
}

func (idx *DirIndex) resolveReadmePrefix(p Preset) (string, bool) {
	var stems []string
	seen := make(map[string]bool)
	addStem := func(raw string) {
		norm := NormalizeFilename(raw)
		if norm == "" || seen[norm] {
			return
		}
		seen[norm] = true
		stems = append(stems, norm)
	}
	for _, f := range p.Files {
		if f.Optional {
			continue
		}
		base := filepath.Base(strings.TrimSpace(f.Name))
		addStem(strings.TrimSuffix(base, filepath.Ext(base)))
		for _, al := range idx.fileAliases(f) {
			addStem(strings.TrimSuffix(al, filepath.Ext(al)))
		}
	}
	addStem(p.Name)

	matchEntries := func(dir string, entries []os.DirEntry, prefix string) (string, bool) {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			norm := NormalizeFilename(entry.Name())
			if strings.HasSuffix(norm, ".txt") && strings.HasPrefix(norm, prefix) {
				return filepath.Join(dir, entry.Name()), true
			}
		}
		return "", false
	}

	for _, stem := range stems {
		prefix := stem + "readme"
		for _, rc := range idx.roots {
			if path, ok := matchEntries(rc.dir, rc.entries, prefix); ok {
				return path, true
			}
		}
		for _, rc := range idx.roots {
			for _, sub := range rc.loadSubdirs() {
				if path, ok := matchEntries(sub.dir, sub.entries, prefix); ok {
					return path, true
				}
			}
		}
	}
	return "", false
}

// NormalizeFilename strips spaces, dashes, underscores, and lowers case for fuzzy matching.
func NormalizeFilename(name string) string {
	return filenameReplacer.Replace(strings.ToLower(name))
}

// ResolveFile searches for a required file within wadsDir with case-insensitivity,
// normalization, safe sub-path handling, and known aliases.
func ResolveFile(wadsDir string, targetName string) (string, bool) {
	return NewDirIndex(wadsDir).ResolveFile(targetName)
}

func resolveSingle(wadsDir string, entries []os.DirEntry, targetName string) (string, bool) {
	if wadsDir == "" || targetName == "" {
		return "", false
	}

	// 1. Exact match against cached entries
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() == targetName {
			return filepath.Join(wadsDir, entry.Name()), true
		}
	}

	targetLower := strings.ToLower(targetName)
	targetNorm := NormalizeFilename(targetName)

	// 2. Case-insensitive match
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.ToLower(name) == targetLower {
			return filepath.Join(wadsDir, name), true
		}
	}

	// 3. Normalized match (ignoring spaces, dashes, underscores)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if NormalizeFilename(name) == targetNorm {
			return filepath.Join(wadsDir, name), true
		}
	}

	return "", false
}

// ResolveReadme locates an accompanying documentation or text file for the preset.
func ResolveReadme(wadsDir string, p Preset) (string, bool) {
	return NewDirIndex(wadsDir).ResolveReadme(p)
}
