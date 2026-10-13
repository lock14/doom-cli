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

// KnownAliases returns any recognized alternate filenames for a given target filename.
func KnownAliases(name string) []string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "gdturbo.wad":
		return []string{"gd.wad"}
	case "gd.wad":
		return []string{"gdturbo.wad"}
	case "doom.wad":
		return []string{"doom1.wad"}
	case "sigil_ii_v1_0.wad":
		return []string{"sigil2.wad", "sigil_ii.wad"}
	case "sigil2.wad":
		return []string{"sigil_ii_v1_0.wad"}
	case "sigil_ii.wad":
		return []string{"sigil_ii_v1_0.wad"}
	case "sunder_v2512.wad":
		return []string{"sunder.wad"}
	case "sunder.wad":
		return []string{"sunder_v2512.wad"}
	default:
		return nil
	}
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
	roots []*rootCache
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

	// Relative sub-path (e.g. "sub/foo.wad" or "sub\foo.wad")
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
	for _, alias := range KnownAliases(targetName) {
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
		for _, alias := range KnownAliases(targetName) {
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
		if entries, readErr := os.ReadDir(parentDir); readErr == nil {
			if p, ok := resolveSingle(parentDir, entries, baseName); ok {
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
	}
	nameClean := strings.ReplaceAll(p.Name, " ", "") + ".txt"
	if path, ok := idx.ResolveFile(nameClean); ok {
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
