package catalog

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lock14/doom-cli/internal/fsutil"
)

var filenameReplacer = strings.NewReplacer(" ", "", "-", "", "_", "")

// SlashBase returns the final slash-separated path element of raw after normalizing backslashes.
func SlashBase(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return path.Base(strings.ReplaceAll(trimmed, `\`, `/`))
}

func slashExt(raw string) string {
	return path.Ext(SlashBase(raw))
}

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

// IsIWADName reports whether name normalizes to a standard base game IWAD filename.
func IsIWADName(name string) bool {
	norm := NormalizeFilename(SlashBase(name))
	return norm != "" && knownIWADNames[norm]
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
			base := SlashBase(f.Name)
			if base == "" || IsIWADName(base) {
				continue
			}
			grp := make([]string, 0, 1+len(f.Aliases))
			grp = append(grp, base)
			for _, al := range f.Aliases {
				trimmed := strings.TrimSpace(al)
				if trimmed == "" || strings.ContainsAny(trimmed, `/\`) || IsIWADName(trimmed) {
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
	key := NormalizeFilename(SlashBase(name))
	if key == "" {
		return nil
	}
	if c == nil {
		return append([]string(nil), getCuratedAliasMap()[key]...)
	}
	merged := mergeAliasMaps(getCuratedAliasMap(), buildAliasMap(c.Presets))
	return append([]string(nil), merged[key]...)
}

type rootCache struct {
	dir     string
	entries []os.DirEntry
	subOnce sync.Once
	subdirs []subDirCache
	relMu   sync.Mutex
	relDirs map[string][]os.DirEntry
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
	key := NormalizeFilename(SlashBase(targetName))
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

func isResolvableFile(dir string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return false
	}
	if entry.Type()&os.ModeSymlink != 0 {
		fi, err := os.Stat(filepath.Join(dir, entry.Name()))
		return err == nil && !fi.IsDir()
	}
	return true
}

func (rc *rootCache) loadSubdirs() []subDirCache {
	rc.subOnce.Do(func() {
		var names []string
		for _, entry := range rc.entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if entry.IsDir() {
				names = append(names, entry.Name())
			} else if entry.Type()&os.ModeSymlink != 0 {
				if fi, err := os.Stat(filepath.Join(rc.dir, entry.Name())); err == nil && fi.IsDir() {
					names = append(names, entry.Name())
				}
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

func (idx *DirIndex) resolveCandidates(primary string, extraAliases []string) (string, bool) {
	if idx == nil || len(idx.roots) == 0 || strings.TrimSpace(primary) == "" {
		return "", false
	}
	if strings.ContainsAny(primary, `/\`) {
		if p, ok := idx.resolveRelativeSubpath(primary); ok {
			return p, true
		}
		if len(extraAliases) == 0 {
			return "", false
		}
	}

	var topCandidates []string
	if !strings.ContainsAny(primary, `/\`) {
		topCandidates = append(topCandidates, primary)
	}
	seen := make(map[string]bool)
	if len(topCandidates) > 0 {
		seen[NormalizeFilename(primary)] = true
	}
	for _, al := range extraAliases {
		trimmed := strings.TrimSpace(al)
		if trimmed == "" || strings.ContainsAny(trimmed, `/\`) {
			continue
		}
		norm := NormalizeFilename(trimmed)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		topCandidates = append(topCandidates, trimmed)
	}
	if len(topCandidates) == 0 {
		return "", false
	}

	// 1. Check top-level entries root-by-root (primary WadsDir first, canonical before aliases).
	for _, rc := range idx.roots {
		for _, cand := range topCandidates {
			if p, ok := resolveSingle(rc.dir, rc.entries, cand); ok {
				return p, true
			}
		}
	}

	// 2. For non-IWAD files, scan 1-level child directories root-by-root.
	if !IsIWADName(primary) {
		for _, rc := range idx.roots {
			for _, sub := range rc.loadSubdirs() {
				for _, cand := range topCandidates {
					if p, ok := resolveSingle(sub.dir, sub.entries, cand); ok {
						return p, true
					}
				}
			}
		}
	}

	return "", false
}

// ResolveFile searches for targetName across indexed WAD roots with case-insensitivity,
// normalization, known aliases, safe relative sub-paths, and 1-level subdirectory fallback
// for non-IWAD files.
func (idx *DirIndex) ResolveFile(targetName string) (string, bool) {
	if idx == nil || len(idx.roots) == 0 || strings.TrimSpace(targetName) == "" {
		return "", false
	}
	if strings.ContainsAny(targetName, `/\`) {
		return idx.resolveRelativeSubpath(targetName)
	}
	return idx.resolveCandidates(targetName, idx.aliasesFor(targetName))
}

func (rc *rootCache) readRelDir(subDirRel string) ([]os.DirEntry, string, bool) {
	if subDirRel == "" {
		return rc.entries, rc.dir, true
	}
	parentDir, err := fsutil.SafeJoin(rc.dir, subDirRel)
	if err != nil {
		return nil, "", false
	}
	rc.relMu.Lock()
	defer rc.relMu.Unlock()
	if rc.relDirs != nil {
		if entries, cached := rc.relDirs[subDirRel]; cached {
			return entries, parentDir, entries != nil
		}
	} else {
		rc.relDirs = make(map[string][]os.DirEntry)
	}
	entries, readErr := os.ReadDir(parentDir)
	if readErr != nil {
		rc.relDirs[subDirRel] = nil
		return nil, "", false
	}
	rc.relDirs[subDirRel] = entries
	return entries, parentDir, true
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
		entries, parentDir, ok := rc.readRelDir(subDirRel)
		if !ok {
			continue
		}
		if p, found := resolveSingle(parentDir, entries, baseName); found {
			return p, true
		}
		for _, alias := range idx.aliasesFor(baseName) {
			if p, found := resolveSingle(parentDir, entries, alias); found {
				return p, true
			}
		}
	}
	return "", false
}

// ResolveFileSpec resolves f.Name or any of its declared or catalog aliases across indexed WAD roots.
func (idx *DirIndex) ResolveFileSpec(f FileSpec) (string, bool) {
	if idx == nil {
		return "", false
	}
	return idx.resolveCandidates(f.Name, idx.fileAliases(f))
}

// IsPresetInstalled reports whether the preset's base IWAD (when specified) and all
// non-optional PWAD, PK3, and DeHackEd files resolve in the indexed WAD directories.
func (idx *DirIndex) IsPresetInstalled(p Preset) bool {
	if idx == nil {
		return false
	}
	if strings.TrimSpace(p.IWAD) != "" {
		if _, ok := idx.ResolveFile(p.IWAD); !ok {
			return false
		}
	}
	return idx.ArePresetFilesInstalled(p)
}

// ArePresetFilesInstalled reports whether all non-optional PWAD, PK3, and DeHackEd
// files declared in p.Files resolve in the indexed WAD directories.
func (idx *DirIndex) ArePresetFilesInstalled(p Preset) bool {
	if idx == nil {
		return false
	}
	for _, f := range p.Files {
		if f.Optional {
			continue
		}
		if _, ok := idx.ResolveFileSpec(f); !ok {
			return false
		}
	}
	return true
}

// NeedsSteamDiscovery reports whether fetching p may require Steam or GOG discovery
// because p has no download sources, its base IWAD is missing, or any optional
// companion file (such as idkfa 2024.wad) is missing from the indexed directories.
func (idx *DirIndex) NeedsSteamDiscovery(p Preset) bool {
	if idx == nil || len(p.Sources) == 0 {
		return true
	}
	if strings.TrimSpace(p.IWAD) != "" {
		if _, ok := idx.ResolveFile(p.IWAD); !ok {
			return true
		}
	}
	for _, f := range p.Files {
		if f.Optional {
			if _, ok := idx.ResolveFileSpec(f); !ok {
				return true
			}
		}
	}
	return false
}

// ResolveReadme locates an accompanying documentation or text file for the preset.
// Base IWAD presets (those with no non-optional files) fall back to <iwad>.txt,
// whereas mapsets with required PWADs never borrow the base IWAD's text file.
func (idx *DirIndex) ResolveReadme(p Preset) (string, bool) {
	if idx == nil || len(idx.roots) == 0 {
		return "", false
	}
	for _, f := range p.Files {
		if f.Optional {
			continue
		}
		ext := slashExt(f.Name)
		base := strings.TrimSuffix(f.Name, ext)
		if resolved, ok := idx.ResolveFile(base + ".txt"); ok {
			return resolved, true
		}
		if strings.ContainsAny(base, `/\`) {
			if resolved, ok := idx.ResolveFile(SlashBase(base) + ".txt"); ok {
				return resolved, true
			}
		}
		for _, al := range idx.fileAliases(f) {
			alExt := slashExt(al)
			alBase := strings.TrimSuffix(al, alExt)
			if alBase != "" {
				if resolved, ok := idx.ResolveFile(alBase + ".txt"); ok {
					return resolved, true
				}
			}
		}
	}
	nameClean := strings.ReplaceAll(p.Name, " ", "") + ".txt"
	if resolved, ok := idx.ResolveFile(nameClean); ok {
		return resolved, true
	}
	if resolved, ok := idx.resolveReadmePrefix(p); ok {
		return resolved, true
	}
	if !p.HasRequiredFiles() {
		iwadBase := strings.TrimSuffix(p.IWAD, slashExt(p.IWAD))
		if resolved, ok := idx.ResolveFile(iwadBase + ".txt"); ok {
			return resolved, true
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
		base := SlashBase(f.Name)
		addStem(strings.TrimSuffix(base, slashExt(base)))
		for _, al := range idx.fileAliases(f) {
			alBase := SlashBase(al)
			addStem(strings.TrimSuffix(alBase, slashExt(alBase)))
		}
	}
	addStem(p.Name)

	matchEntries := func(dir string, entries []os.DirEntry, stem string) (string, bool) {
		for _, entry := range entries {
			if !isResolvableFile(dir, entry) {
				continue
			}
			norm := NormalizeFilename(entry.Name())
			if !strings.HasSuffix(norm, ".txt") || !strings.HasPrefix(norm, stem) {
				continue
			}
			rest := strings.TrimSuffix(strings.TrimPrefix(norm, stem), ".txt")
			if IsReadmeSuffix(rest) {
				return filepath.Join(dir, entry.Name()), true
			}
		}
		return "", false
	}

	for _, stem := range stems {
		for _, rc := range idx.roots {
			if resolved, ok := matchEntries(rc.dir, rc.entries, stem); ok {
				return resolved, true
			}
		}
		for _, rc := range idx.roots {
			for _, sub := range rc.loadSubdirs() {
				if resolved, ok := matchEntries(sub.dir, sub.entries, stem); ok {
					return resolved, true
				}
			}
		}
	}
	return "", false
}

// IsReadmeSuffix reports whether a normalized suffix after a preset or WAD stem
// represents a README or versioned documentation suffix (such as "readme*",
// "v<digit>*", or "<digit>.<digit>*") without matching sequel numbers ("2" or "2v1.0").
func IsReadmeSuffix(rest string) bool {
	if strings.HasPrefix(rest, "readme") {
		return true
	}
	if len(rest) >= 2 && rest[0] == 'v' && rest[1] >= '0' && rest[1] <= '9' {
		return true
	}
	if len(rest) >= 3 && rest[0] >= '0' && rest[0] <= '9' {
		dot := strings.IndexByte(rest, '.')
		if dot > 0 && dot+1 < len(rest) && rest[dot+1] >= '0' && rest[dot+1] <= '9' {
			for i := 0; i < dot; i++ {
				if rest[i] < '0' || rest[i] > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
}

// NormalizeFilename strips spaces, dashes, underscores, and lowers case for fuzzy matching.
func NormalizeFilename(name string) string {
	return filenameReplacer.Replace(strings.ToLower(name))
}

func resolveSingle(wadsDir string, entries []os.DirEntry, targetName string) (string, bool) {
	if wadsDir == "" || targetName == "" {
		return "", false
	}

	for _, entry := range entries {
		if entry.Name() == targetName && isResolvableFile(wadsDir, entry) {
			return filepath.Join(wadsDir, entry.Name()), true
		}
	}

	targetLower := strings.ToLower(targetName)
	targetNorm := NormalizeFilename(targetName)

	for _, entry := range entries {
		name := entry.Name()
		if strings.ToLower(name) == targetLower && isResolvableFile(wadsDir, entry) {
			return filepath.Join(wadsDir, name), true
		}
	}

	for _, entry := range entries {
		name := entry.Name()
		if NormalizeFilename(name) == targetNorm && isResolvableFile(wadsDir, entry) {
			return filepath.Join(wadsDir, name), true
		}
	}

	return "", false
}
