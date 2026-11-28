package steam

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const regProbeTimeout = 1500 * time.Millisecond

var (
	vdfPathRegex   = regexp.MustCompile(`^\s*"path"\s*"([^"]+)"`)
	regPathRegex   = regexp.MustCompile(`(?i)\bREG_(?:EXPAND_)?SZ\s+(.+)$`)
	windowsDrives  = []string{"C", "D", "E", "F", "G", "H", "I", "J"}
	macAppKeywords = []string{"doom", "heretic", "hexen", "strife", "gog"}
	protonAppIDs   = []string{"2280", "3286930"}
)

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

func parseWindowsRegPaths(out string) []string {
	var paths []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		matches := regPathRegex.FindStringSubmatch(scanner.Text())
		if len(matches) < 2 {
			continue
		}
		val := strings.Trim(strings.TrimSpace(matches[1]), `"`)
		if val == "" {
			continue
		}
		paths = append(paths, filepath.FromSlash(val))
	}
	return paths
}

func queryWindowsRegPaths(args ...string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), regProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "reg", args...).Output()
	if err != nil {
		return nil
	}
	return parseWindowsRegPaths(string(out))
}

func findMacAppBundles(appDirs ...string) []string {
	var matches []string
	for _, appDir := range appDirs {
		if strings.TrimSpace(appDir) == "" {
			continue
		}
		entries, err := os.ReadDir(appDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			lower := strings.ToLower(entry.Name())
			matched := false
			for _, kw := range macAppKeywords {
				if strings.Contains(lower, kw) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			full := filepath.Join(appDir, entry.Name())
			if entry.IsDir() {
				matches = append(matches, full)
			} else if entry.Type()&os.ModeSymlink != 0 {
				if fi, statErr := os.Stat(full); statErr == nil && fi.IsDir() {
					matches = append(matches, full)
				}
			}
		}
	}
	return matches
}

func collectPlatformRoots(
	goos, home, xdgDataHome, prog, prog86 string,
	regQuery func(args ...string) []string,
) (steamRoots, directRoots []string) {
	switch goos {
	case "darwin":
		appDirs := []string{"/Applications"}
		if home != "" {
			appSupport := filepath.Join(home, "Library", "Application Support")
			steamRoots = append(steamRoots,
				filepath.Join(appSupport, "Steam"),
			)
			directRoots = append(directRoots,
				filepath.Join(appSupport, "GOG.com"),
				filepath.Join(appSupport, "CrossOver", "Bottles"),
				filepath.Join(home, "Library", "Containers", "com.isaacmarovitz.Whisky", "Bottles"),
				filepath.Join(home, "GOG Games"),
				filepath.Join(home, "Games", "Heroic"),
			)
			appDirs = append(appDirs, filepath.Join(home, "Applications"))
		}
		directRoots = append(directRoots, filepath.Join("/", "Users", "Shared", "Epic Games"))
		directRoots = append(directRoots, findMacAppBundles(appDirs...)...)

	case "windows":
		if prog86 == "" {
			prog86 = `C:\Program Files (x86)`
		}
		if prog == "" {
			prog = `C:\Program Files`
		}
		steamRoots = append(steamRoots,
			filepath.Join(prog86, "Steam"),
			filepath.Join(prog, "Steam"),
		)
		if regQuery != nil {
			steamRoots = append(steamRoots,
				regQuery("query", `HKLM\SOFTWARE\WOW6432Node\Valve\Steam`, "/v", "InstallPath")...,
			)
			steamRoots = append(steamRoots,
				regQuery("query", `HKLM\SOFTWARE\Valve\Steam`, "/v", "InstallPath")...,
			)
			steamRoots = append(steamRoots,
				regQuery("query", `HKCU\Software\Valve\Steam`, "/v", "SteamPath")...,
			)
			steamRoots = append(steamRoots,
				regQuery("query", `HKCU\Software\Valve\Steam`, "/v", "InstallPath")...,
			)
		}
		for _, drive := range windowsDrives {
			steamRoots = append(steamRoots,
				drive+`:\Steam`,
				drive+`:\SteamLibrary`,
			)
		}

		directRoots = append(directRoots,
			filepath.Join(prog86, "GOG Galaxy", "Games"),
			filepath.Join(prog, "GOG Galaxy", "Games"),
			filepath.Join(prog, "Epic Games"),
			filepath.Join(prog86, "Epic Games"),
		)
		if regQuery != nil {
			directRoots = append(directRoots,
				regQuery("query", `HKLM\SOFTWARE\WOW6432Node\GOG.com\Games`, "/s", "/v", "path")...,
			)
			directRoots = append(directRoots,
				regQuery("query", `HKLM\SOFTWARE\GOG.com\Games`, "/s", "/v", "path")...,
			)
		}
		if home != "" {
			directRoots = append(directRoots,
				filepath.Join(home, "Saved Games", "Nightdive Studios", "DOOM"),
				filepath.Join(home, "Saved Games", "Nightdive Studios", "Heretic + Hexen"),
				filepath.Join(home, "Saved Games", "id Software"),
			)
		}
		for _, drive := range windowsDrives {
			directRoots = append(directRoots,
				drive+`:\GOG Games`,
				drive+`:\GOG Galaxy\Games`,
				drive+`:\XboxGames`,
				drive+`:\Epic Games`,
				drive+`:\Games`,
			)
		}

	default: // Linux and BSD
		if xdgDataHome != "" {
			steamRoots = append(steamRoots, filepath.Join(xdgDataHome, "Steam"))
		}
		if home != "" {
			flatpakApp := filepath.Join(home, ".var", "app")
			steamRoots = append(steamRoots,
				filepath.Join(home, ".local", "share", "Steam"),
				filepath.Join(home, ".steam", "steam"),
				filepath.Join(home, ".steam", "root"),
				filepath.Join(flatpakApp, "com.valvesoftware.Steam", "data", "Steam"),
				filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"),
			)
			directRoots = append(directRoots,
				filepath.Join(home, "GOG Games"),
				filepath.Join(home, "Games", "Heroic"),
				filepath.Join(home, "Games"),
				filepath.Join(home, ".local", "share", "lutris"),
				filepath.Join(home, ".local", "share", "bottles"),
				filepath.Join(flatpakApp, "com.heroicgameslauncher.hgl", "Games"),
				filepath.Join(flatpakApp, "net.lutris.Lutris", "Games"),
				filepath.Join(flatpakApp, "net.lutris.Lutris", "data", "lutris"),
				filepath.Join(flatpakApp, "net.lutris.Lutris", ".local", "share", "lutris"),
				filepath.Join(flatpakApp, "com.usebottles.bottles", "data", "bottles"),
				filepath.Join(home, ".wine"),
			)
		}
		directRoots = append(directRoots,
			"/usr/share/games/doom",
			"/usr/local/share/games/doom",
			"/usr/share/doom",
		)
	}

	return steamRoots, directRoots
}

func steamVDFCandidates(root string) []string {
	var libs []string
	for _, steamappsName := range []string{"steamapps", "SteamApps"} {
		vdf := filepath.Join(root, steamappsName, "libraryfolders.vdf")
		libs = append(libs, ParseLibraryFolders(vdf)...)
	}
	return libs
}

func steamCommonDirs(root string) ([]string, bool) {
	var commons []string
	hasSteamApps := false
	for _, steamappsName := range []string{"steamapps", "SteamApps"} {
		saDir := filepath.Join(root, steamappsName)
		if fi, err := os.Stat(saDir); err == nil && fi.IsDir() {
			hasSteamApps = true
			commonDir := filepath.Join(saDir, "common")
			if cfi, cErr := os.Stat(commonDir); cErr == nil && cfi.IsDir() {
				commons = append(commons, commonDir)
			}
		}
	}
	return commons, hasSteamApps
}

func steamCompatSavedGamesDirs(root string) []string {
	var dirs []string
	for _, steamappsName := range []string{"steamapps", "SteamApps"} {
		for _, appID := range protonAppIDs {
			savedGames := filepath.Join(
				root, steamappsName, "compatdata", appID,
				"pfx", "drive_c", "users", "steamuser", "Saved Games",
			)
			for _, sub := range []string{
				filepath.Join(savedGames, "Nightdive Studios", "DOOM"),
				filepath.Join(savedGames, "Nightdive Studios", "Heretic + Hexen"),
				filepath.Join(savedGames, "id Software"),
			} {
				if fi, err := os.Stat(sub); err == nil && fi.IsDir() {
					dirs = append(dirs, sub)
				}
			}
		}
	}
	return dirs
}

func resolveSteamCommonDirs(steamRoots []string) []string {
	var allLibs []string
	for _, r := range normalizeExistingDirs(steamRoots) {
		allLibs = append(allLibs, r)
		allLibs = append(allLibs, steamVDFCandidates(r)...)
	}

	var resolved []string
	for _, lib := range normalizeExistingDirs(allLibs) {
		commons, hasSteamApps := steamCommonDirs(lib)
		if len(commons) > 0 {
			resolved = append(resolved, commons...)
		} else if !hasSteamApps {
			resolved = append(resolved, lib)
		}
		resolved = append(resolved, steamCompatSavedGamesDirs(lib)...)
	}
	return normalizeExistingDirs(resolved)
}

// ExpandSearchRoots normalizes explicit search roots, expands any Steam libraryfolders.vdf
// references, narrows Steam library roots to their steamapps/common directory when present,
// and prunes nested child directories already covered by a parent root.
func ExpandSearchRoots(roots []string) []string {
	var candidates []string
	for _, r := range normalizeExistingDirs(roots) {
		if commons, _ := steamCommonDirs(r); len(commons) > 0 {
			candidates = append(candidates, commons...)
		} else {
			candidates = append(candidates, r)
		}
		candidates = append(candidates, steamCompatSavedGamesDirs(r)...)
		for _, lib := range normalizeExistingDirs(steamVDFCandidates(r)) {
			if commons, hasSteamApps := steamCommonDirs(lib); len(commons) > 0 {
				candidates = append(candidates, commons...)
			} else if !hasSteamApps {
				candidates = append(candidates, lib)
			}
			candidates = append(candidates, steamCompatSavedGamesDirs(lib)...)
		}
	}
	return pruneNestedDirs(normalizeExistingDirs(candidates))
}

// GetSearchRoots returns platform-specific potential game directory roots, narrowing
// Steam installations and secondary VDF libraries to their steamapps/common directories
// and pruning nested child directories already covered by a parent root.
func GetSearchRoots() []string {
	home, _ := os.UserHomeDir()
	var regFn func(args ...string) []string
	if runtime.GOOS == "windows" {
		regFn = queryWindowsRegPaths
	}
	steamRoots, directRoots := collectPlatformRoots(
		runtime.GOOS,
		home,
		os.Getenv("XDG_DATA_HOME"),
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		regFn,
	)

	var candidates []string
	candidates = append(candidates, resolveSteamCommonDirs(steamRoots)...)
	candidates = append(candidates, directRoots...)
	return pruneNestedDirs(normalizeExistingDirs(candidates))
}

func dedupDirKey(clean string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(clean)
	}
	return clean
}

func isStrictSubdir(child, parent string) bool {
	cClean := filepath.Clean(child)
	pClean := filepath.Clean(parent)
	if dedupDirKey(cClean) == dedupDirKey(pClean) {
		return false
	}
	sep := string(filepath.Separator)
	pPrefix := pClean
	if !strings.HasSuffix(pPrefix, sep) {
		pPrefix += sep
	}
	if strings.HasPrefix(cClean, pPrefix) {
		return true
	}
	pLower := strings.ToLower(pPrefix)
	if !strings.HasPrefix(strings.ToLower(cClean), pLower) ||
		len(cClean) <= len(pPrefix) ||
		cClean[len(pPrefix)-1] != filepath.Separator {
		return false
	}
	candParent := filepath.Clean(cClean[:len(pPrefix)])
	fiCand, err1 := os.Stat(candParent)
	fiParent, err2 := os.Stat(pClean)
	return err1 == nil && err2 == nil && os.SameFile(fiCand, fiParent)
}

func pruneNestedDirs(dirs []string) []string {
	if len(dirs) <= 1 {
		return dirs
	}
	out := make([]string, 0, len(dirs))
	for i, cand := range dirs {
		nested := false
		for j, other := range dirs {
			if i != j && isStrictSubdir(cand, other) {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, cand)
		}
	}
	return out
}

func normalizeExistingDirs(candidates []string) []string {
	var existing []string
	var existingInfos []os.FileInfo
	seen := make(map[string]bool)
	for _, r := range candidates {
		if strings.TrimSpace(r) == "" {
			continue
		}
		clean := filepath.Clean(r)
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			clean = filepath.Clean(resolved)
		}
		key := dedupDirKey(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		fi, err := os.Stat(clean)
		if err != nil || !fi.IsDir() {
			continue
		}
		same := false
		for _, prev := range existingInfos {
			if os.SameFile(prev, fi) {
				same = true
				break
			}
		}
		if same {
			continue
		}
		existing = append(existing, clean)
		existingInfos = append(existingInfos, fi)
	}
	return existing
}
