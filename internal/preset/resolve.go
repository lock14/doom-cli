package preset

import (
	"os"
	"path/filepath"
	"strings"
)

var filenameReplacer = strings.NewReplacer(" ", "", "-", "", "_", "")

// IsOptionalFile reports whether the given filename is an optional enhancement file (e.g. idkfa 2024.wad).
func IsOptionalFile(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "idkfa 2024.wad")
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
	default:
		return nil
	}
}

// DirIndex caches directory entries for fast case-insensitive and normalized filename resolution.
type DirIndex struct {
	wadsDir string
	entries []os.DirEntry
}

// NewDirIndex creates a DirIndex for the provided directory, caching its entries.
func NewDirIndex(wadsDir string) *DirIndex {
	idx := &DirIndex{wadsDir: wadsDir}
	if wadsDir != "" {
		if entries, err := os.ReadDir(wadsDir); err == nil {
			idx.entries = entries
		}
	}
	return idx
}

// ResolveFile searches for targetName in the indexed directory with case-insensitivity, normalization, and aliases.
func (idx *DirIndex) ResolveFile(targetName string) (string, bool) {
	if idx == nil || idx.wadsDir == "" || targetName == "" {
		return "", false
	}

	if path, ok := resolveSingle(idx.wadsDir, idx.entries, targetName); ok {
		return path, true
	}

	for _, alias := range KnownAliases(targetName) {
		if path, ok := resolveSingle(idx.wadsDir, idx.entries, alias); ok {
			return path, true
		}
	}

	return "", false
}

// ResolveReadme locates an accompanying documentation or text file for the preset using the indexed directory.
func (idx *DirIndex) ResolveReadme(p Preset) (string, bool) {
	if idx == nil || idx.wadsDir == "" {
		return "", false
	}
	for _, m := range p.Mappacks {
		ext := filepath.Ext(m)
		base := strings.TrimSuffix(m, ext)
		if path, ok := idx.ResolveFile(base + ".txt"); ok {
			return path, true
		}
	}
	nameClean := strings.ReplaceAll(p.Name, " ", "") + ".txt"
	if path, ok := idx.ResolveFile(nameClean); ok {
		return path, true
	}
	if len(p.Mappacks) == 0 {
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

// ResolveFile searches for a required file within wadsDir with case-insensitivity, normalization, and known aliases.
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
