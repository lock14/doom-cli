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
)

// TargetPattern maps a candidate source filename to its canonical destination name in WADs directory.
type TargetPattern struct {
	SourcePattern string
	DestName      string
}

// DefaultTargetPatterns returns all official commercial Doom files and re-release add-ons to extract.
func DefaultTargetPatterns() []TargetPattern {
	return []TargetPattern{
		{"DOOM.WAD", "DOOM.WAD"},
		{"DOOM1.WAD", "DOOM.WAD"},
		{"DOOM2.WAD", "DOOM2.WAD"},
		{"PLUTONIA.WAD", "PLUTONIA.WAD"},
		{"TNT.WAD", "TNT.WAD"},
		{"HERETIC.WAD", "HERETIC.WAD"},
		{"HEXEN.WAD", "HEXEN.WAD"},
		{"HEXDD.WAD", "HEXDD.WAD"},
		{"NERVE.WAD", "NERVE.WAD"},
		{"MASTERLEVELS.WAD", "MASTERLEVELS.WAD"},
		{"idkfa 2024.wad", "idkfa 2024.wad"},
		{"idkfa_2024.wad", "idkfa 2024.wad"},
		{"id24res.wad", "id24res.wad"},
		{"id1-res.wad", "id1-res.wad"},
		{"id1-weap.wad", "id1-weap.wad"},
		{"id1.wad", "id1.wad"},
		{"extras.wad", "extras.wad"},
		{"sigil.wad", "SIGIL_V1_23.wad"},
		{"SIGIL_V1_23.wad", "SIGIL_V1_23.wad"},
		{"sigil_ii.wad", "SIGIL_II_V1_0.WAD"},
		{"SIGIL_II_V1_0.WAD", "SIGIL_II_V1_0.WAD"},
		{"DoomZero.wad", "DoomZero.wad"},
		{"doomzero.wad", "DoomZero.wad"},
		{"DOOMZERO.DEH", "DOOMZERO.DEH"},
		{"doomzero.deh", "DOOMZERO.DEH"},
		{"gdturbo.wad", "gdturbo.wad"},
		{"gd.wad", "gdturbo.wad"},
		{"SCYTHE.WAD", "SCYTHE.WAD"},
		{"scythe.wad", "SCYTHE.WAD"},
		{"scythe2.wad", "scythe2.wad"},
		{"SUNDER_V2512.wad", "SUNDER_V2512.wad"},
		{"sunder.wad", "SUNDER_V2512.wad"},
	}
}

// ParseLibraryFolders extracts secondary library folder paths from Steam's libraryfolders.vdf.
func ParseLibraryFolders(vdfPath string) []string {
	f, err := os.Open(vdfPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var paths []string
	re := regexp.MustCompile(`^\s*"path"\s*"([^"]+)"`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			p := strings.TrimSpace(matches[1])
			// Handle Windows escaped backslashes in VDF
			p = strings.ReplaceAll(p, `\\`, `\`)
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				paths = append(paths, p)
			}
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

	default: // Linux / BSD
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

	// Build indexed map: lowercase filename -> slice of paths
	indexed := make(map[string][]string)
	for _, root := range existingRoots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			// Limit depth to avoid scanning irrelevant deep trees
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && strings.Count(rel, string(filepath.Separator)) > 8 {
				return filepath.SkipDir
			}

			lowerName := strings.ToLower(info.Name())
			if strings.HasSuffix(lowerName, ".wad") ||
				strings.HasSuffix(lowerName, ".pk3") ||
				strings.HasSuffix(lowerName, ".deh") {
				indexed[lowerName] = append(indexed[lowerName], path)
			}
			return nil
		})
	}

	patterns := DefaultTargetPatterns()
	foundCount := 0
	installedTargets := make(map[string]bool)

	for _, tp := range patterns {
		destFile := filepath.Join(wadsDir, tp.DestName)
		if installedTargets[strings.ToLower(tp.DestName)] {
			continue
		}

		if !force {
			if fi, err := os.Stat(destFile); err == nil && fi.Size() > 0 {
				continue
			}
		}

		candidates := indexed[strings.ToLower(tp.SourcePattern)]
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
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
