package wad

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lock14/doom-cli/internal/fsutil"
	"github.com/lock14/doom-cli/internal/preset"
)

const maxEmbeddedLumpBytes = 64 << 20

// ExtractWADLump extracts the raw payload of the first non-empty lump matching lumpName
// (case-insensitive, up to 8 ASCII characters) from an IWAD or PWAD byte slice.
func ExtractWADLump(wadData []byte, lumpName string) ([]byte, bool) {
	return ExtractWADLumpFromReader(bytes.NewReader(wadData), int64(len(wadData)), lumpName)
}

// ExtractWADLumpFromReader extracts the raw payload of the first non-empty lump matching
// lumpName from an io.ReaderAt without reading the entire WAD file into memory.
func ExtractWADLumpFromReader(r io.ReaderAt, dataLen int64, lumpName string) ([]byte, bool) {
	if r == nil || dataLen < 12 {
		return nil, false
	}
	var hdr [12]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil {
		return nil, false
	}
	ident := string(hdr[0:4])
	if ident != "PWAD" && ident != "IWAD" {
		return nil, false
	}
	numLumps := int64(binary.LittleEndian.Uint32(hdr[4:8]))
	dirOffset := int64(binary.LittleEndian.Uint32(hdr[8:12]))
	if numLumps <= 0 || numLumps > 65536 || dirOffset < 12 || dirOffset+numLumps*16 > dataLen {
		return nil, false
	}

	dirBuf := make([]byte, numLumps*16)
	if _, err := r.ReadAt(dirBuf, dirOffset); err != nil {
		return nil, false
	}

	want := strings.ToUpper(strings.TrimSpace(lumpName))
	for i := int64(0); i < numLumps; i++ {
		entry := dirBuf[i*16 : (i+1)*16]
		lumpPos := int64(binary.LittleEndian.Uint32(entry[0:4]))
		lumpSize := int64(binary.LittleEndian.Uint32(entry[4:8]))
		if lumpSize <= 0 || lumpSize > maxEmbeddedLumpBytes || lumpPos < 12 || lumpPos+lumpSize > dataLen {
			continue
		}
		nameBytes := entry[8:16]
		if nul := bytes.IndexByte(nameBytes, 0); nul >= 0 {
			nameBytes = nameBytes[:nul]
		}
		if strings.EqualFold(string(nameBytes), want) {
			out := make([]byte, lumpSize)
			if _, err := r.ReadAt(out, lumpPos); err != nil {
				return nil, false
			}
			return out, true
		}
	}
	return nil, false
}

func isZip(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	var buf [4]byte
	if _, err := io.ReadFull(f, buf[:]); err != nil {
		return false
	}
	return bytes.Equal(buf[:], []byte{'P', 'K', 0x03, 0x04})
}

func zipBase(entryName string) string {
	slash := strings.ReplaceAll(entryName, `\`, `/`)
	return path.Base(path.Clean(slash))
}

func isSkippableZipEntry(zf *zip.File) bool {
	if zf.FileInfo().IsDir() || zf.Mode()&os.ModeSymlink != 0 {
		return true
	}
	slash := strings.ReplaceAll(zf.Name, `\`, `/`)
	clean := path.Clean(slash)
	return clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(slash, "/../")
}

func (d *Downloader) extractFromZip(
	zipPath string,
	expectedFiles []string,
	presets ...preset.Preset,
) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed opening zip: %w", err)
	}
	defer zr.Close()

	cleanWadsDir := filepath.Clean(d.WadsDir)
	aliasCat := &preset.Catalog{Presets: presets}

	var wadFiles []*zip.File
	var pk3Files []*zip.File
	var dehFiles []*zip.File
	var txtFiles []*zip.File
	for _, zf := range zr.File {
		if isSkippableZipEntry(zf) {
			continue
		}
		baseLower := strings.ToLower(zipBase(zf.Name))
		if strings.HasSuffix(baseLower, ".wad") {
			wadFiles = append(wadFiles, zf)
		} else if strings.HasSuffix(baseLower, ".pk3") {
			pk3Files = append(pk3Files, zf)
		} else if strings.HasSuffix(baseLower, ".deh") || strings.HasSuffix(baseLower, ".bex") {
			dehFiles = append(dehFiles, zf)
		} else if strings.HasSuffix(baseLower, ".txt") {
			txtFiles = append(txtFiles, zf)
		}
	}

	var wadExpCount, pk3ExpCount, dehExpCount int
	for _, exp := range expectedFiles {
		el := strings.ToLower(exp)
		switch {
		case strings.HasSuffix(el, ".wad"):
			wadExpCount++
		case strings.HasSuffix(el, ".pk3"):
			pk3ExpCount++
		case strings.HasSuffix(el, ".deh") || strings.HasSuffix(el, ".bex"):
			dehExpCount++
		}
	}

	for _, exp := range expectedFiles {
		destPath, joinErr := fsutil.SafeJoin(cleanWadsDir, exp)
		if joinErr != nil {
			return fmt.Errorf("illegal file destination for %q: %w", exp, joinErr)
		}

		expBase := filepath.Base(destPath)
		expLower := strings.ToLower(expBase)
		expNorm := preset.NormalizeFilename(expBase)

		var match *zip.File

		// 1. Exact case-insensitive match on base filename
		for _, zf := range zr.File {
			if isSkippableZipEntry(zf) {
				continue
			}
			if strings.ToLower(zipBase(zf.Name)) == expLower {
				match = zf
				break
			}
		}

		// 2. Normalized match (ignoring spaces, underscores, dashes)
		if match == nil {
			for _, zf := range zr.File {
				if isSkippableZipEntry(zf) {
					continue
				}
				if preset.NormalizeFilename(zipBase(zf.Name)) == expNorm {
					match = zf
					break
				}
			}
		}

		// 3. Known aliases (curated and preset-defined)
		if match == nil {
			for _, alias := range aliasCat.FileAliases(expBase) {
				aliasNorm := preset.NormalizeFilename(alias)
				for _, zf := range zr.File {
					if isSkippableZipEntry(zf) {
						continue
					}
					zfBase := zipBase(zf.Name)
					if strings.EqualFold(zfBase, alias) || preset.NormalizeFilename(zfBase) == aliasNorm {
						match = zf
						break
					}
				}
				if match != nil {
					break
				}
			}
		}

		// 4. Single-file fallback when the preset expects exactly one file of that type
		if match == nil {
			if strings.HasSuffix(expLower, ".wad") && wadExpCount == 1 && len(wadFiles) == 1 {
				match = wadFiles[0]
			} else if strings.HasSuffix(expLower, ".pk3") && pk3ExpCount == 1 && len(pk3Files) == 1 {
				match = pk3Files[0]
			} else if (strings.HasSuffix(expLower, ".deh") || strings.HasSuffix(expLower, ".bex")) &&
				dehExpCount == 1 && len(dehFiles) == 1 {
				match = dehFiles[0]
			}
		}

		if match == nil {
			return fmt.Errorf("could not find matching file in archive for %s", exp)
		}

		if err := extractZipFile(match, cleanWadsDir, destPath); err != nil {
			return fmt.Errorf("error extracting %s: %w", exp, err)
		}
		fmt.Fprintf(d.Out, "    Installed: %s -> %s\n", zipBase(match.Name), destPath)
	}

	// Extract accompanying readme (.txt) if present in the archive, preferring entries
	// whose normalized stem matches an expected file stem, alias, or preset name.
	// Generic fallback files (such as readme.txt) are saved as <primaryStem>.txt so
	// DirIndex.ResolveReadme resolves them and archives never overwrite each other.
	for _, tf := range orderReadmeCandidates(txtFiles, expectedFiles, aliasCat, presets) {
		cleanName := zipBase(tf.Name)
		if strings.EqualFold(cleanName, "license.txt") {
			continue
		}
		if cleanName == "." || cleanName == ".." || strings.Contains(cleanName, "..") {
			continue
		}
		destName := readmeDestName(cleanName, expectedFiles, aliasCat, presets)
		destPath, joinErr := fsutil.SafeJoin(cleanWadsDir, destName)
		if joinErr != nil {
			continue
		}

		if err := extractZipFile(tf, cleanWadsDir, destPath); err == nil {
			fmt.Fprintf(d.Out, "    Readme:    %s -> %s\n", cleanName, destPath)
			break
		}
	}

	return nil
}

func collectReadmeStems(
	expectedFiles []string,
	aliasCat *preset.Catalog,
	presets []preset.Preset,
) []string {
	var stems []string
	seen := make(map[string]bool)
	addStem := func(raw string) {
		norm := preset.NormalizeFilename(raw)
		if norm == "" || seen[norm] {
			return
		}
		seen[norm] = true
		stems = append(stems, norm)
	}

	for _, exp := range expectedFiles {
		expBase := path.Base(strings.ReplaceAll(exp, `\`, `/`))
		addStem(strings.TrimSuffix(expBase, path.Ext(expBase)))
		for _, al := range aliasCat.FileAliases(expBase) {
			alBase := path.Base(strings.ReplaceAll(al, `\`, `/`))
			addStem(strings.TrimSuffix(alBase, path.Ext(alBase)))
		}
	}
	for _, p := range presets {
		addStem(p.Name)
	}
	return stems
}

func readmeDestName(
	cleanName string,
	expectedFiles []string,
	aliasCat *preset.Catalog,
	presets []preset.Preset,
) string {
	stems := collectReadmeStems(expectedFiles, aliasCat, presets)
	txtStem := preset.NormalizeFilename(strings.TrimSuffix(cleanName, path.Ext(cleanName)))
	for _, stem := range stems {
		if txtStem == stem {
			return cleanName
		}
		if strings.HasPrefix(txtStem, stem) && preset.IsReadmeSuffix(strings.TrimPrefix(txtStem, stem)) {
			return cleanName
		}
	}

	for _, exp := range expectedFiles {
		expBase := path.Base(strings.ReplaceAll(strings.TrimSpace(exp), `\`, `/`))
		ext := path.Ext(expBase)
		if strings.EqualFold(ext, ".wad") || strings.EqualFold(ext, ".pk3") {
			if stem := strings.TrimSuffix(expBase, ext); stem != "" {
				return stem + ".txt"
			}
		}
	}

	for _, exp := range expectedFiles {
		expBase := path.Base(strings.ReplaceAll(strings.TrimSpace(exp), `\`, `/`))
		if stem := strings.TrimSuffix(expBase, path.Ext(expBase)); stem != "" {
			return stem + ".txt"
		}
	}
	return cleanName
}

func orderReadmeCandidates(
	txtFiles []*zip.File,
	expectedFiles []string,
	aliasCat *preset.Catalog,
	presets []preset.Preset,
) []*zip.File {
	stems := collectReadmeStems(expectedFiles, aliasCat, presets)

	var exact, readmePrefix, otherPrefix, rest []*zip.File
	for _, tf := range txtFiles {
		base := zipBase(tf.Name)
		txtStem := preset.NormalizeFilename(strings.TrimSuffix(base, path.Ext(base)))
		if txtStem == "" || txtStem == "license" {
			continue
		}
		matchedExact := false
		matchedReadmePrefix := false
		matchedOtherPrefix := false
		for _, stem := range stems {
			if txtStem == stem {
				matchedExact = true
				break
			}
			if strings.HasPrefix(txtStem, stem) {
				if preset.IsReadmeSuffix(strings.TrimPrefix(txtStem, stem)) {
					matchedReadmePrefix = true
				} else {
					matchedOtherPrefix = true
				}
			}
		}
		switch {
		case matchedExact:
			exact = append(exact, tf)
		case matchedReadmePrefix:
			readmePrefix = append(readmePrefix, tf)
		case matchedOtherPrefix:
			otherPrefix = append(otherPrefix, tf)
		default:
			rest = append(rest, tf)
		}
	}

	out := make([]*zip.File, 0, len(exact)+len(readmePrefix)+len(otherPrefix)+len(rest))
	out = append(out, exact...)
	out = append(out, readmePrefix...)
	out = append(out, otherPrefix...)
	out = append(out, rest...)
	return out
}

func extractZipFile(zf *zip.File, targetDir, destPath string) error {
	cleanTargetDir := filepath.Clean(targetDir)
	cleanDest := filepath.Clean(destPath)
	prefix := cleanTargetDir
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	if cleanDest == cleanTargetDir || !strings.HasPrefix(cleanDest, prefix) {
		return fmt.Errorf("illegal file path in archive: %s", zf.Name)
	}

	if err := os.MkdirAll(filepath.Dir(cleanDest), 0o755); err != nil {
		return err
	}

	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, _, err = fsutil.WriteLimitedReaderAtomic(cleanDest, rc, fsutil.MaxExtractFileBytes, 0o644)
	return err
}
