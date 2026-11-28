// Package steam handles auto-discovery and extraction of official game files from Steam and GOG libraries.
package steam

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/fsutil"
)

const maxSearchDepth = 15

// TargetPattern maps a candidate source filename to its canonical destination name in WADs directory.
type TargetPattern struct {
	SourcePattern string
	DestName      string
}

func baseSlash(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return ""
	}
	clean := path.Base(strings.ReplaceAll(trimmed, `\`, `/`))
	if clean == "." || clean == "/" {
		return ""
	}
	return clean
}

// DefaultTargetPatterns returns all official commercial Doom files, expansions, and catalog WADs
// (including declared filename aliases) to extract from Steam and GOG installations.
func DefaultTargetPatterns() []TargetPattern {
	cat, err := catalog.LoadCurated()
	if err != nil {
		return nil
	}
	return TargetPatternsForCatalog(cat)
}

// TargetPatternsForCatalog synthesizes extraction target patterns from the given preset catalog,
// mapping both canonical filenames and their declared aliases to each preset's canonical filename.
func TargetPatternsForCatalog(cat *catalog.Catalog) []TargetPattern {
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
		key := catalog.NormalizeFilename(src) + "->" + strings.ToLower(dst)
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
		iwad := baseSlash(p.IWAD)
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
			dest := baseSlash(f.Name)
			if dest == "" {
				continue
			}
			addPattern(dest, dest)
			for _, alias := range f.Aliases {
				addPattern(alias, dest)
			}
			if catalog.NormalizeFilename(dest) == "idkfa2024.wad" {
				addPattern("extras.wad", "extras.wad")
			}
		}
	}

	return patterns
}

var skippedSteamSubdirs = map[string]bool{
	"shadercache": true,
	"workshop":    true,
	"downloading": true,
	"temp":        true,
	"depotcache":  true,
	"appcache":    true,
	"dosdevices":  true,
}

func isIndexableGameFile(lowerName string) bool {
	return strings.HasSuffix(lowerName, ".wad") ||
		strings.HasSuffix(lowerName, ".pk3") ||
		strings.HasSuffix(lowerName, ".deh") ||
		strings.HasSuffix(lowerName, ".bex")
}

func walkSearchRoot(
	ctx context.Context,
	root, cleanWadsDir string,
	visitFile func(filePath, entryName string),
) error {
	visited := make(map[string]bool)
	var walk func(dir string, depth int) error
	walk = func(dir string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > maxSearchDepth {
			return nil
		}
		realDir := filepath.Clean(dir)
		if resolved, err := filepath.EvalSymlinks(realDir); err == nil {
			realDir = filepath.Clean(resolved)
		}
		if depth > 0 && dedupDirKey(realDir) == dedupDirKey(cleanWadsDir) {
			return nil
		}
		dirKey := dedupDirKey(realDir)
		if visited[dirKey] {
			return nil
		}
		visited[dirKey] = true

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, d := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := d.Name()
			entryPath := filepath.Join(dir, name)
			isDir := d.IsDir()
			if !isDir && d.Type()&os.ModeSymlink != 0 {
				fi, statErr := os.Stat(entryPath)
				if statErr != nil {
					continue
				}
				isDir = fi.IsDir()
			}
			if isDir {
				if skippedSteamSubdirs[strings.ToLower(name)] {
					continue
				}
				if err := walk(entryPath, depth+1); err != nil {
					return err
				}
				continue
			}
			if dedupDirKey(realDir) == dedupDirKey(cleanWadsDir) {
				continue
			}
			if isIndexableGameFile(strings.ToLower(name)) {
				visitFile(entryPath, name)
			}
		}
		return nil
	}
	return walk(root, 0)
}

// DiscoverAndExtract searches for official game files and catalog WADs across search roots
// using target patterns synthesized from cat (or the curated catalog when cat is nil).
// When force is false, files already present in wadsDir or any extraWadRoots are skipped.
func DiscoverAndExtract(
	cat *catalog.Catalog,
	searchRoots []string,
	wadsDir string,
	force bool,
	out io.Writer,
	extraWadRoots ...string,
) (int, error) {
	return DiscoverAndExtractContext(
		context.Background(), cat, searchRoots, wadsDir, force, out, extraWadRoots...,
	)
}

// DiscoverAndExtractContext searches for official game files and catalog WADs under ctx.
// Passing searchRoots == nil auto-discovers platform Steam, GOG, Xbox, and Epic directories,
// whereas a non-nil slice restricts scanning to those directories (after VDF/common expansion).
func DiscoverAndExtractContext(
	ctx context.Context,
	cat *catalog.Catalog,
	searchRoots []string,
	wadsDir string,
	force bool,
	out io.Writer,
	extraWadRoots ...string,
) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if out == nil {
		out = io.Discard
	}

	if err := os.MkdirAll(wadsDir, 0o755); err != nil {
		return 0, fmt.Errorf("failed to create destination directory %s: %w", wadsDir, err)
	}

	cleanWadsDir := filepath.Clean(wadsDir)
	if resolved, err := filepath.EvalSymlinks(cleanWadsDir); err == nil {
		cleanWadsDir = filepath.Clean(resolved)
	}

	if cat == nil {
		cat, _ = catalog.LoadCurated()
	}
	patterns := TargetPatternsForCatalog(cat)
	wadRoots := append([]string{wadsDir}, extraWadRoots...)
	dirIdx := catalog.NewDirIndex(wadRoots...).WithCatalog(cat)

	var destOrder []string
	destNames := make(map[string]string, len(patterns))
	patternsByDest := make(map[string][]TargetPattern, len(patterns))
	var idkfaDestName string
	for _, tp := range patterns {
		if idkfaDestName == "" && catalog.NormalizeFilename(tp.DestName) == "idkfa2024.wad" {
			idkfaDestName = tp.DestName
		}
		destLower := strings.ToLower(tp.DestName)
		if _, exists := destNames[destLower]; !exists {
			destNames[destLower] = tp.DestName
			destOrder = append(destOrder, destLower)
		}
		patternsByDest[destLower] = append(patternsByDest[destLower], tp)
	}

	isDestInstalled := func(destLower string) bool {
		destName := destNames[destLower]
		destFile, joinErr := fsutil.SafeJoin(wadsDir, destName)
		if joinErr != nil {
			return true
		}
		if fi, err := os.Stat(destFile); err == nil && fi.Size() > 0 {
			return true
		}
		if _, ok := dirIdx.ResolveFile(destName); ok {
			return true
		}
		for _, p := range patternsByDest[destLower] {
			if _, ok := dirIdx.ResolveFile(p.SourcePattern); ok {
				return true
			}
		}
		return false
	}

	existingCount := 0
	alreadyInstalled := make(map[string]bool, len(destOrder))
	if !force {
		for _, destLower := range destOrder {
			if isDestInstalled(destLower) {
				alreadyInstalled[destLower] = true
				existingCount++
			}
		}
		if len(destOrder) > 0 && existingCount == len(destOrder) {
			fmt.Fprintf(out, "=== Doom IWAD & Commercial Expansion Extractor ===\n")
			fmt.Fprintf(out, "Target WADs directory: %s\n\n", wadsDir)
			fmt.Fprintf(
				out,
				"All %d target game file(s) are already installed (pass --force to re-extract).\n",
				existingCount,
			)
			return 0, nil
		}
	}

	var existingRoots []string
	if searchRoots == nil {
		existingRoots = GetSearchRoots()
	} else {
		existingRoots = ExpandSearchRoots(searchRoots)
	}

	if len(existingRoots) == 0 {
		if !force && existingCount > 0 {
			fmt.Fprintf(
				out,
				"No Steam or GOG directories found on this system "+
					"(%d existing file(s) already installed).\n",
				existingCount,
			)
		} else {
			fmt.Fprintf(out, "No Steam or GOG directories found on this system.\n")
		}
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
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		walkErr := walkSearchRoot(ctx, root, cleanWadsDir, func(filePath, entryName string) {
			lowerName := strings.ToLower(entryName)
			indexed[lowerName] = append(indexed[lowerName], filePath)
			normName := catalog.NormalizeFilename(entryName)
			indexedNorm[normName] = append(indexedNorm[normName], filePath)
		})
		if errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded) {
			return 0, walkErr
		}
	}

	foundCount := 0
	installedTargets := make(map[string]bool)
	for _, destLower := range destOrder {
		if err := ctx.Err(); err != nil {
			return foundCount, err
		}
		if !force && alreadyInstalled[destLower] {
			continue
		}

		destName := destNames[destLower]
		destFile, joinErr := fsutil.SafeJoin(wadsDir, destName)
		if joinErr != nil {
			continue
		}

		destPatterns := patternsByDest[destLower]
		var candidates []string
		seenCand := make(map[string]bool)
		addCandidate := func(c string) {
			if dedupDirKey(filepath.Clean(c)) == dedupDirKey(filepath.Clean(destFile)) {
				return
			}
			if !seenCand[c] {
				seenCand[c] = true
				candidates = append(candidates, c)
			}
		}
		for _, p := range destPatterns {
			for _, c := range indexed[strings.ToLower(p.SourcePattern)] {
				addCandidate(c)
			}
		}
		for _, p := range destPatterns {
			for _, c := range indexedNorm[catalog.NormalizeFilename(p.SourcePattern)] {
				addCandidate(c)
			}
		}
		if len(candidates) > 1 {
			candidates = rankRereleaseCandidates(candidates)
		}
		for _, src := range candidates {
			if err := copyFile(src, destFile); err == nil {
				fmt.Fprintf(out, "✓ Found & Installed: %s\n    Source: %s\n", destName, src)
				foundCount++
				installedTargets[destLower] = true
				break
			}
		}
	}

	if idkfaDestName != "" && !installedTargets[strings.ToLower(idkfaDestName)] {
		if err := ctx.Err(); err != nil {
			return foundCount, err
		}
		idkfaLower := strings.ToLower(idkfaDestName)
		if destFile, err := fsutil.SafeJoin(wadsDir, idkfaDestName); err == nil {
			if force || !alreadyInstalled[idkfaLower] {
				extrasCandidates := rankRereleaseCandidates(indexed["extras.wad"])
				if existingExtras, ok := dirIdx.ResolveFile("extras.wad"); ok {
					extrasCandidates = append(extrasCandidates, existingExtras)
				} else if existingExtras, joinErr := fsutil.SafeJoin(wadsDir, "extras.wad"); joinErr == nil {
					extrasCandidates = append(extrasCandidates, existingExtras)
				}
				for _, extrasSrc := range extrasCandidates {
					tracks, synthErr := SynthesizeIDKFAWAD(extrasSrc, destFile)
					if synthErr == nil && tracks > 0 {
						fmt.Fprintf(
							out,
							"✓ Synthesized & Installed: %s (%d tracks)\n    Source: %s\n",
							idkfaDestName, tracks, extrasSrc,
						)
						foundCount++
						installedTargets[idkfaLower] = true
						break
					}
				}
			}
		}
	}

	fmt.Fprintln(out)
	switch {
	case foundCount > 0:
		fmt.Fprintf(out, "Extracted %d game file(s) into %s\n", foundCount, wadsDir)
	case !force && existingCount > 0:
		fmt.Fprintf(
			out,
			"No new game files found to extract "+
				"(%d existing file(s) already installed; pass --force to re-extract).\n",
			existingCount,
		)
	default:
		fmt.Fprintf(
			out,
			"No matching game files found across searched directories "+
				"(pass --from <dir> to scan a custom location).\n",
		)
	}

	return foundCount, nil
}

func copyFile(src, dst string) error {
	_, err := fsutil.CopyFileAtomic(src, dst, 0o644)
	return err
}

func rereleasePriority(filePath string) int {
	dir := filepath.ToSlash(filepath.Dir(filePath))
	parts := strings.Split(dir, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		seg := parts[i]
		if strings.EqualFold(seg, "rerelease") {
			return 1
		}
		if strings.EqualFold(seg, "base") || strings.EqualFold(seg, "dosbox") {
			return -1
		}
	}
	return 0
}

func rankRereleaseCandidates(paths []string) []string {
	ranked := append([]string(nil), paths...)
	sort.SliceStable(ranked, func(i, j int) bool {
		iPrio := rereleasePriority(ranked[i])
		jPrio := rereleasePriority(ranked[j])
		if iPrio != jPrio {
			return iPrio > jPrio
		}
		var iSize, jSize int64
		if fi, err := os.Stat(ranked[i]); err == nil {
			iSize = fi.Size()
		}
		if fj, err := os.Stat(ranked[j]); err == nil {
			jSize = fj.Size()
		}
		if iSize != jSize {
			return iSize > jSize
		}
		return len(filepath.Dir(ranked[i])) < len(filepath.Dir(ranked[j]))
	})
	return ranked
}
