package preset

import (
	"os"
	"path/filepath"
	"strings"
)

// NormalizeFilename strips spaces, dashes, underscores, and lowers case for fuzzy matching.
func NormalizeFilename(name string) string {
	lower := strings.ToLower(name)
	r := strings.NewReplacer(" ", "", "-", "", "_", "")
	return r.Replace(lower)
}

// ResolveFile searches for a required file within wadsDir with case-insensitivity, normalization, and known aliases.
func ResolveFile(wadsDir string, targetName string) (string, bool) {
	if wadsDir == "" || targetName == "" {
		return "", false
	}

	exact := filepath.Join(wadsDir, targetName)
	if _, err := os.Stat(exact); err == nil {
		return exact, true
	}

	targetLower := strings.ToLower(targetName)
	targetNorm := NormalizeFilename(targetName)

	entries, err := os.ReadDir(wadsDir)
	if err != nil {
		return "", false
	}

	// 1. Case-insensitive match
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.ToLower(name) == targetLower {
			return filepath.Join(wadsDir, name), true
		}
	}

	// 2. Normalized match (ignoring spaces, dashes, underscores)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if NormalizeFilename(name) == targetNorm {
			return filepath.Join(wadsDir, name), true
		}
	}

	// 3. Known aliases
	switch targetLower {
	case "gdturbo.wad":
		return ResolveFile(wadsDir, "gd.wad")
	case "gd.wad":
		return ResolveFile(wadsDir, "gdturbo.wad")
	case "doom.wad":
		return ResolveFile(wadsDir, "doom1.wad")
	}

	return "", false
}

// ResolveReadme locates an accompanying documentation or text file for the preset.
func ResolveReadme(wadsDir string, p Preset) (string, bool) {
	for _, m := range p.Mappacks {
		ext := filepath.Ext(m)
		base := strings.TrimSuffix(m, ext)
		if path, ok := ResolveFile(wadsDir, base+".txt"); ok {
			return path, true
		}
	}
	nameClean := strings.ReplaceAll(p.Name, " ", "") + ".txt"
	if path, ok := ResolveFile(wadsDir, nameClean); ok {
		return path, true
	}
	iwadBase := strings.TrimSuffix(p.IWAD, filepath.Ext(p.IWAD))
	if path, ok := ResolveFile(wadsDir, iwadBase+".txt"); ok {
		return path, true
	}
	return "", false
}
