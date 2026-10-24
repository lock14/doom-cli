// Package steam handles auto-discovery and extraction of official game files from Steam and GOG libraries.
package steam

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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
		if home != "" {
			roots = append(roots,
				filepath.Join(home, "Saved Games", "Nightdive Studios", "DOOM"),
				filepath.Join(home, "Saved Games", "id Software"),
			)
		}
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
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			clean = filepath.Clean(resolved)
		}
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

// DiscoverAndExtract searches for official game files and catalog WADs across search roots
// using target patterns synthesized from cat (or the curated catalog when cat is nil).
// When force is false, files already present in wadsDir or any extraWadRoots are skipped.
func DiscoverAndExtract(
	cat *preset.Catalog,
	searchRoots []string,
	wadsDir string,
	force bool,
	out io.Writer,
	extraWadRoots ...string,
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

	seenRoots := make(map[string]bool)
	var existingRoots []string
	for _, r := range searchRoots {
		clean := filepath.Clean(r)
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			clean = filepath.Clean(resolved)
		}
		if seenRoots[clean] {
			continue
		}
		if fi, err := os.Stat(clean); err == nil && fi.IsDir() {
			seenRoots[clean] = true
			existingRoots = append(existingRoots, clean)
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

	if cat == nil {
		cat, _ = preset.LoadCuratedCatalog()
	}
	patterns := TargetPatternsForCatalog(cat)
	wadRoots := append([]string{wadsDir}, extraWadRoots...)
	dirIdx := preset.NewDirIndex(wadRoots...).WithCatalog(cat)
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
		if idkfaDestName == "" && preset.NormalizeFilename(tp.DestName) == "idkfa2024.wad" {
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
			for _, c := range indexedNorm[preset.NormalizeFilename(p.SourcePattern)] {
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

func rereleasePriority(path string) int {
	dir := filepath.ToSlash(filepath.Dir(path))
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

// idkfaReusedTrackSlots maps an H_ lump name in extras.wad to additional D_ lump
// names that share the same composition in DOOM.WAD and DOOM2.WAD (per TRAKINFO).
// Multiple WAD directory entries share the same lump offset and size without
// duplicating audio payload bytes.
var idkfaReusedTrackSlots = map[string][]string{
	// Doom I reused music slots
	"H_INTRO": {"D_INTROA"},
	"H_E1M6":  {"D_E3M6"},
	"H_E1M7":  {"D_E2M5", "D_E3M5"},
	"H_E1M8":  {"D_E3M4"},
	"H_E1M9":  {"D_E3M9"},
	"H_E2M3":  {"D_INTER"},
	"H_E2M7":  {"D_E3M7"},
	"H_E2M9":  {"D_E3M1"},
	// Doom II reused music slots
	"H_RUNNIN": {"D_RUNNI2"},
	"H_STALKS": {"D_STLKS2", "D_STLKS3"},
	"H_COUNTD": {"D_COUNT2"},
	"H_DOOM":   {"D_DOOM2"},
	"H_THE_DA": {"D_THEDA2", "D_THEDA3"},
	"H_SHAWN":  {"D_SHAWN2", "D_SHAWN3"},
	"H_DDTBLU": {"D_DDTBL2", "D_DDTBL3"},
	"H_DEAD":   {"D_DEAD2"},
	"H_ROMERO": {"D_ROMER2"},
	"H_MESSAG": {"D_MESSG2"},
}

// SynthesizeIDKFAWAD extracts Andrew Hulshult's IDKFA soundtrack lumps (prefixed with H_)
// from a DOOM + DOOM II (2024) extras.wad file, renames each lump prefix from H_ to D_
// (including reused map music slots from TRAKINFO) so classic source ports play them
// automatically, and writes a valid PWAD atomically to dstPath.
func SynthesizeIDKFAWAD(srcExtrasPath, dstPath string) (int, error) {
	f, err := os.Open(srcExtrasPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	fileSize := fi.Size()
	if fileSize < 12 {
		return 0, errors.New("file too small to be a WAD")
	}

	var hdr [12]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return 0, err
	}
	ident := string(hdr[0:4])
	if ident != "PWAD" && ident != "IWAD" {
		return 0, fmt.Errorf("invalid WAD header %q", ident)
	}

	numLumps := int64(binary.LittleEndian.Uint32(hdr[4:8]))
	dirOffset := int64(binary.LittleEndian.Uint32(hdr[8:12]))
	if numLumps <= 0 || numLumps > 65536 || dirOffset < 12 || dirOffset+numLumps*16 > fileSize {
		return 0, errors.New("corrupt WAD directory bounds")
	}

	dirBuf := make([]byte, numLumps*16)
	if _, err := f.ReadAt(dirBuf, dirOffset); err != nil {
		return 0, err
	}

	type audioPayload struct {
		srcPos int64
		outPos uint32
		size   uint32
		hName  string
		dName  string
	}

	var payloads []audioPayload
	seenDNames := make(map[string]bool)
	curOutPos := int64(12)

	for i := int64(0); i < numLumps; i++ {
		entry := dirBuf[i*16 : (i+1)*16]
		lumpPos := int64(binary.LittleEndian.Uint32(entry[0:4]))
		lumpSize := int64(binary.LittleEndian.Uint32(entry[4:8]))
		if lumpSize <= 0 || lumpPos < 12 || lumpPos+lumpSize > fileSize {
			continue
		}
		nameSlice := entry[8:16]
		if nul := bytes.IndexByte(nameSlice, 0); nul >= 0 {
			nameSlice = nameSlice[:nul]
		}
		upperName := strings.ToUpper(string(nameSlice))
		if len(upperName) < 3 || !strings.HasPrefix(upperName, "H_") || lumpSize < 4 {
			continue
		}
		var magic [4]byte
		if _, err := f.ReadAt(magic[:], lumpPos); err != nil || string(magic[:]) != "OggS" {
			continue
		}
		if curOutPos+lumpSize > math.MaxUint32 {
			return 0, errors.New("synthesized WAD exceeds 4 GiB limit")
		}
		dName := "D_" + upperName[2:]
		seenDNames[dName] = true
		payloads = append(payloads, audioPayload{
			srcPos: lumpPos,
			outPos: uint32(curOutPos),
			size:   uint32(lumpSize),
			hName:  upperName,
			dName:  dName,
		})
		curOutPos += lumpSize
	}

	if len(payloads) == 0 {
		return 0, errors.New("no H_ soundtrack lumps found in extras.wad")
	}

	type wadDirEntry struct {
		outPos uint32
		size   uint32
		name   [8]byte
	}

	var dirEntries []wadDirEntry
	addDirEntry := func(outPos, size uint32, lumpName string) {
		var nameBytes [8]byte
		copy(nameBytes[:], lumpName)
		dirEntries = append(dirEntries, wadDirEntry{
			outPos: outPos,
			size:   size,
			name:   nameBytes,
		})
	}

	for _, p := range payloads {
		addDirEntry(p.outPos, p.size, p.dName)
		for _, alias := range idkfaReusedTrackSlots[p.hName] {
			if !seenDNames[alias] {
				seenDNames[alias] = true
				addDirEntry(p.outPos, p.size, alias)
			}
		}
	}

	var outHdr [12]byte
	copy(outHdr[0:4], "PWAD")
	binary.LittleEndian.PutUint32(outHdr[4:8], uint32(len(dirEntries)))
	binary.LittleEndian.PutUint32(outHdr[8:12], uint32(curOutPos))

	outDir := make([]byte, len(dirEntries)*16)
	for i, de := range dirEntries {
		dstEntry := outDir[i*16 : (i+1)*16]
		binary.LittleEndian.PutUint32(dstEntry[0:4], de.outPos)
		binary.LittleEndian.PutUint32(dstEntry[4:8], de.size)
		copy(dstEntry[8:16], de.name[:])
	}

	readers := make([]io.Reader, 0, len(payloads)+2)
	readers = append(readers, bytes.NewReader(outHdr[:]))
	for _, p := range payloads {
		readers = append(readers, io.NewSectionReader(f, p.srcPos, int64(p.size)))
	}
	readers = append(readers, bytes.NewReader(outDir))

	if _, _, err := fsutil.WriteReaderAtomic(dstPath, io.MultiReader(readers...), 0o644); err != nil {
		return 0, err
	}
	return len(dirEntries), nil
}
