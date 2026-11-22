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

	if len(searchRoots) == 0 {
		searchRoots = GetSearchRoots()
	}

	existingRoots := normalizeExistingDirs(searchRoots)

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
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		walkErr := filepath.WalkDir(root, func(walkPath string, d os.DirEntry, err error) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err != nil || d == nil {
				return nil
			}
			cleanCur := filepath.Clean(walkPath)
			if d.IsDir() {
				if cleanCur == cleanWadsDir && cleanCur != root {
					return filepath.SkipDir
				}
				if skippedSteamSubdirs[strings.ToLower(d.Name())] {
					return filepath.SkipDir
				}
			}
			rel, relErr := filepath.Rel(root, walkPath)
			if relErr == nil && strings.Count(rel, string(filepath.Separator)) > 15 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Dir(cleanCur) == cleanWadsDir {
				return nil
			}

			lowerName := strings.ToLower(d.Name())
			if strings.HasSuffix(lowerName, ".wad") ||
				strings.HasSuffix(lowerName, ".pk3") ||
				strings.HasSuffix(lowerName, ".deh") ||
				strings.HasSuffix(lowerName, ".bex") {
				indexed[lowerName] = append(indexed[lowerName], walkPath)
				normName := catalog.NormalizeFilename(d.Name())
				indexedNorm[normName] = append(indexedNorm[normName], walkPath)
			}
			return nil
		})
		if errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded) {
			return 0, walkErr
		}
	}

	if cat == nil {
		cat, _ = catalog.LoadCurated()
	}
	patterns := TargetPatternsForCatalog(cat)
	wadRoots := append([]string{wadsDir}, extraWadRoots...)
	dirIdx := catalog.NewDirIndex(wadRoots...).WithCatalog(cat)
	foundCount := 0
	installedTargets := make(map[string]bool)
	processedTargets := make(map[string]bool)
	var idkfaDestName string

	patternsByDest := make(map[string][]TargetPattern, len(patterns))
	for _, tp := range patterns {
		destLower := strings.ToLower(tp.DestName)
		patternsByDest[destLower] = append(patternsByDest[destLower], tp)
	}

	for _, tp := range patterns {
		if err := ctx.Err(); err != nil {
			return foundCount, err
		}
		if idkfaDestName == "" && catalog.NormalizeFilename(tp.DestName) == "idkfa2024.wad" {
			idkfaDestName = tp.DestName
		}
		destLower := strings.ToLower(tp.DestName)
		if processedTargets[destLower] {
			continue
		}
		processedTargets[destLower] = true

		destFile, joinErr := fsutil.SafeJoin(wadsDir, tp.DestName)
		if joinErr != nil {
			continue
		}

		destPatterns := patternsByDest[destLower]
		if !force {
			if fi, err := os.Stat(destFile); err == nil && fi.Size() > 0 {
				continue
			}
			if _, ok := dirIdx.ResolveFile(tp.DestName); ok {
				continue
			}
			alreadyResolved := false
			for _, p := range destPatterns {
				if _, ok := dirIdx.ResolveFile(p.SourcePattern); ok {
					alreadyResolved = true
					break
				}
			}
			if alreadyResolved {
				continue
			}
		}

		var candidates []string
		seenCand := make(map[string]bool)
		addCandidate := func(c string) {
			if filepath.Clean(c) == filepath.Clean(destFile) {
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
				fmt.Fprintf(out, "✓ Found & Installed: %s\n    Source: %s\n", tp.DestName, src)
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
		if destFile, err := fsutil.SafeJoin(wadsDir, idkfaDestName); err == nil {
			alreadyExists := false
			if !force {
				if fi, statErr := os.Stat(destFile); statErr == nil && fi.Size() > 0 {
					alreadyExists = true
				} else if _, ok := dirIdx.ResolveFile(idkfaDestName); ok {
					alreadyExists = true
				}
			}
			if !alreadyExists {
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
						installedTargets[strings.ToLower(idkfaDestName)] = true
						break
					}
				}
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
