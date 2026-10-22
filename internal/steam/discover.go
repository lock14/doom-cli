// Package steam handles auto-discovery and extraction of official game files from Steam and GOG libraries.
package steam

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/lock14/doom-cli/internal/fsutil"
	"github.com/lock14/doom-cli/internal/preset"
)

// TargetPattern maps a candidate source filename to its canonical destination name in WADs directory.
type TargetPattern struct {
	SourcePattern string
	DestName      string
}

// DefaultTargetPatterns returns all official commercial Doom files, expansions, and catalog WADs
// (including declared filename aliases) to extract from Steam and GOG installations.
func DefaultTargetPatterns() []TargetPattern {
	cat, err := preset.LoadCuratedCatalog()
	if err != nil {
		return nil
	}
	return TargetPatternsForCatalog(cat)
}

// TargetPatternsForCatalog synthesizes extraction target patterns from the given preset catalog,
// mapping both canonical filenames and their declared aliases to each preset's canonical filename.
func TargetPatternsForCatalog(cat *preset.Catalog) []TargetPattern {
	if cat == nil {
		return nil
	}

	var patterns []TargetPattern
	seen := make(map[string]bool)

	addPattern := func(sourcePattern, destName string) {
		src := strings.TrimSpace(sourcePattern)
		dst := strings.TrimSpace(destName)
		if src == "" || dst == "" {
			return
		}
		key := preset.NormalizeFilename(src) + "->" + strings.ToLower(dst)
		if seen[key] {
			return
		}
		seen[key] = true
		patterns = append(patterns, TargetPattern{
			SourcePattern: src,
			DestName:      dst,
		})
	}

	// 1. Base IWADs referenced across the catalog (plus official Hexen expansion HEXDD.WAD)
	seenIWADs := make(map[string]bool)
	for _, p := range cat.Presets {
		iwad := filepath.Base(strings.TrimSpace(p.IWAD))
		if iwad == "" || seenIWADs[strings.ToLower(iwad)] {
			continue
		}
		seenIWADs[strings.ToLower(iwad)] = true
		addPattern(iwad, iwad)
		for _, alias := range cat.FileAliases(iwad) {
			addPattern(alias, iwad)
		}
		if strings.EqualFold(iwad, "HEXEN.WAD") {
			addPattern("HEXDD.WAD", "HEXDD.WAD")
		}
	}

	// 2. PWADs, DeHackEd patches, and optional soundtracks declared across catalog presets
	for _, p := range cat.Presets {
		for _, f := range p.Files {
			dest := filepath.Base(strings.TrimSpace(f.Name))
			if dest == "" {
				continue
			}
			addPattern(dest, dest)
			for _, alias := range f.Aliases {
				addPattern(alias, dest)
			}
		}
	}

	return patterns
}

var vdfPathRegex = regexp.MustCompile(`^\s*"path"\s*"([^"]+)"`)

// ParseVDFPaths parses folder path values from a VDF reader without checking filesystem existence.
func ParseVDFPaths(r io.Reader) []string {
	var paths []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		matches := vdfPathRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			p := strings.TrimSpace(matches[1])
			p = strings.ReplaceAll(p, `\\`, `\`)
			paths = append(paths, p)
		}
	}
	return paths
}

// ParseLibraryFolders extracts secondary library folder paths from Steam's libraryfolders.vdf.
func ParseLibraryFolders(vdfPath string) []string {
	f, err := os.Open(vdfPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var paths []string
	for _, p := range ParseVDFPaths(f) {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			paths = append(paths, p)
		}
	}
	return paths
}

// GetSearchRoots returns platform-specific potential game directory roots.
func GetSearchRoots() []string {
	home, _ := os.UserHomeDir()
	var roots []string

	switch runtime.GOOS {
	case "darwin":
		if home != "" {
			roots = append(roots,
				filepath.Join(home, "Library", "Application Support", "Steam"),
				filepath.Join(home, "Library", "Application Support", "GOG.com"),
			)
		}
		roots = append(roots, "/Applications")

	case "windows":
		// Standard Windows locations
		prog86 := os.Getenv("ProgramFiles(x86)")
		if prog86 == "" {
			prog86 = `C:\Program Files (x86)`
		}
		prog := os.Getenv("ProgramFiles")
		if prog == "" {
			prog = `C:\Program Files`
		}
		roots = append(roots,
			filepath.Join(prog86, "Steam"),
			filepath.Join(prog, "Steam"),
			`C:\GOG Games`,
		)
		// Check available drive letters
		for _, drive := range []string{"C", "D", "E", "F", "G"} {
			roots = append(roots, drive+`:\Steam`, drive+`:\GOG Games`, drive+`:\Games`)
		}

	default: // Linux and BSD
		if home != "" {
			roots = append(roots,
				filepath.Join(home, ".local", "share", "Steam"),
				filepath.Join(home, ".steam", "steam"),
				filepath.Join(home, ".steam", "root"),
				filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
				filepath.Join(home, "GOG Games"),
				filepath.Join(home, ".local", "share", "bottles"),
				filepath.Join(home, ".wine"),
			)
		}
	}

	// For any existing Steam directories, look for libraryfolders.vdf
	var expanded []string
	seen := make(map[string]bool)

	addRoot := func(r string) {
		clean := filepath.Clean(r)
		if seen[clean] {
			return
		}
		seen[clean] = true
		if fi, err := os.Stat(clean); err == nil && fi.IsDir() {
			expanded = append(expanded, clean)
		}
	}

	for _, r := range roots {
		addRoot(r)
		vdf := filepath.Join(r, "steamapps", "libraryfolders.vdf")
		for _, secondary := range ParseLibraryFolders(vdf) {
			addRoot(secondary)
		}
	}

	return expanded
}

// DiscoverAndExtract searches for official game files across search roots and copies them into wadsDir.
func DiscoverAndExtract(searchRoots []string, wadsDir string, force bool, out io.Writer) (int, error) {
	return DiscoverAndExtractForCatalog(nil, searchRoots, wadsDir, force, out)
}

// DiscoverAndExtractForCatalog searches for official game files and catalog WADs across search roots
// using target patterns synthesized from cat (or the curated catalog when cat is nil).
func DiscoverAndExtractForCatalog(
	cat *preset.Catalog,
	searchRoots []string,
	wadsDir string,
	force bool,
	out io.Writer,
) (int, error) {
	if out == nil {
		out = io.Discard
	}

	if err := os.MkdirAll(wadsDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create destination directory %s: %w", wadsDir, err)
	}

	if len(searchRoots) == 0 {
		searchRoots = GetSearchRoots()
	}

	var existingRoots []string
	for _, r := range searchRoots {
		if fi, err := os.Stat(r); err == nil && fi.IsDir() {
			existingRoots = append(existingRoots, r)
		}
	}

	if len(existingRoots) == 0 {
		fmt.Fprintf(out, "No Steam or GOG directories found on this system.\n")
		return 0, nil
	}

	fmt.Fprintf(out, "=== Doom IWAD & Commercial Expansion Extractor ===\n")
	fmt.Fprintf(out, "Target WADs directory: %s\n\n", wadsDir)
	fmt.Fprintf(out, "Searching for official game files across:\n")
	for _, r := range existingRoots {
		fmt.Fprintf(out, "  - %s\n", r)
	}
	fmt.Fprintln(out)

	// Build indexed maps from lowercase and normalized filenames to slices of paths
	indexed := make(map[string][]string)
	indexedNorm := make(map[string][]string)
	for _, root := range existingRoots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}
			// Limit depth to avoid scanning irrelevant deep directory trees, while
			// allowing Proton prefixes (compatdata/<id>/pfx/.../Saved Games/...)
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && strings.Count(rel, string(filepath.Separator)) > 15 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}

			lowerName := strings.ToLower(d.Name())
			if strings.HasSuffix(lowerName, ".wad") ||
				strings.HasSuffix(lowerName, ".pk3") ||
				strings.HasSuffix(lowerName, ".deh") {
				indexed[lowerName] = append(indexed[lowerName], path)
				normName := preset.NormalizeFilename(d.Name())
				indexedNorm[normName] = append(indexedNorm[normName], path)
			}
			return nil
		})
	}

	var patterns []TargetPattern
	if cat != nil {
		patterns = TargetPatternsForCatalog(cat)
	} else {
		patterns = DefaultTargetPatterns()
	}
	foundCount := 0
	installedTargets := make(map[string]bool)

	for _, tp := range patterns {
		destFile, joinErr := fsutil.SafeJoin(wadsDir, tp.DestName)
		if joinErr != nil {
			continue
		}
		if installedTargets[strings.ToLower(tp.DestName)] {
			continue
		}

		if !force {
			if fi, err := os.Stat(destFile); err == nil && fi.Size() > 0 {
				continue
			}
		}

		candidates := indexed[strings.ToLower(tp.SourcePattern)]
		if len(candidates) == 0 {
			candidates = indexedNorm[preset.NormalizeFilename(tp.SourcePattern)]
		}
		if len(candidates) > 0 {
			src := candidates[0]
			if err := copyFile(src, destFile); err == nil {
				fmt.Fprintf(out, "✓ Found & Installed: %s\n    Source: %s\n", tp.DestName, src)
				foundCount++
				installedTargets[strings.ToLower(tp.DestName)] = true
			}
		}
	}

	fmt.Fprintln(out)
	if foundCount > 0 {
		fmt.Fprintf(out, "Extracted %d game file(s) into %s\n", foundCount, wadsDir)
	} else {
		fmt.Fprintf(out, "No new game files found to extract. (Existing files were preserved)\n")
	}

	return foundCount, nil
}

func copyFile(src, dst string) error {
	_, err := fsutil.CopyFileAtomic(src, dst, 0o644)
	return err
}
