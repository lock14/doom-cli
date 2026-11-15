package steam

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

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
				filepath.Join(home, "GOG Games"),
			)
		}
		for _, pat := range []string{
			"/Applications/*Doom*",
			"/Applications/*DOOM*",
			"/Applications/*Heretic*",
			"/Applications/*Hexen*",
			"/Applications/*Strife*",
			"/Applications/*GOG*",
		} {
			if matches, err := filepath.Glob(pat); err == nil {
				roots = append(roots, matches...)
			}
		}

	case "windows":
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
		if home != "" {
			roots = append(roots,
				filepath.Join(home, "Saved Games", "Nightdive Studios", "DOOM"),
				filepath.Join(home, "Saved Games", "id Software"),
			)
		}
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
	var candidates []string
	for _, r := range roots {
		candidates = append(candidates, r)
		vdf := filepath.Join(r, "steamapps", "libraryfolders.vdf")
		candidates = append(candidates, ParseLibraryFolders(vdf)...)
	}

	return normalizeExistingDirs(candidates)
}

func normalizeExistingDirs(candidates []string) []string {
	var existing []string
	seen := make(map[string]bool)
	for _, r := range candidates {
		clean := filepath.Clean(r)
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			clean = filepath.Clean(resolved)
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		if fi, err := os.Stat(clean); err == nil && fi.IsDir() {
			existing = append(existing, clean)
		}
	}
	return existing
}
