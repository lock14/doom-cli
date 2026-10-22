package wad

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lock14/doom-cli/internal/fsutil"
	"github.com/lock14/doom-cli/internal/preset"
)

// ExtractWADLump extracts the raw payload of the first non-empty lump matching lumpName
// (case-insensitive, up to 8 ASCII characters) from an IWAD or PWAD byte slice.
func ExtractWADLump(wadData []byte, lumpName string) ([]byte, bool) {
	if len(wadData) < 12 {
		return nil, false
	}
	ident := string(wadData[0:4])
	if ident != "PWAD" && ident != "IWAD" {
		return nil, false
	}
	numLumps := int64(binary.LittleEndian.Uint32(wadData[4:8]))
	dirOffset := int64(binary.LittleEndian.Uint32(wadData[8:12]))
	dataLen := int64(len(wadData))
	if numLumps <= 0 || numLumps > 65536 || dirOffset < 12 || dirOffset+numLumps*16 > dataLen {
		return nil, false
	}

	want := strings.ToUpper(strings.TrimSpace(lumpName))
	for i := int64(0); i < numLumps; i++ {
		entry := wadData[dirOffset+i*16 : dirOffset+(i+1)*16]
		lumpPos := int64(binary.LittleEndian.Uint32(entry[0:4]))
		lumpSize := int64(binary.LittleEndian.Uint32(entry[4:8]))
		if lumpSize <= 0 || lumpPos < 12 || lumpPos+lumpSize > dataLen {
			continue
		}
		nameBytes := entry[8:16]
		if nul := bytes.IndexByte(nameBytes, 0); nul >= 0 {
			nameBytes = nameBytes[:nul]
		}
		if strings.EqualFold(string(nameBytes), want) {
			out := make([]byte, lumpSize)
			copy(out, wadData[lumpPos:lumpPos+lumpSize])
			return out, true
		}
	}
	return nil, false
}

func isZip(path string) bool {
	f, err := os.Open(path)
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

func (d *Downloader) extractFromZip(
	zipPath string,
	_ int64,
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
	var dehFiles []*zip.File
	var txtFiles []*zip.File
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		// CodeQL and Zip Slip defense: skip entries containing path traversal
		cleanName := filepath.Clean(zf.Name)
		if strings.HasPrefix(cleanName, "..") ||
			strings.Contains(cleanName, "/../") ||
			strings.Contains(cleanName, "\\..\\") {
			continue
		}

		baseLower := strings.ToLower(filepath.Base(cleanName))
		if strings.HasSuffix(baseLower, ".wad") {
			wadFiles = append(wadFiles, zf)
		} else if strings.HasSuffix(baseLower, ".deh") || strings.HasSuffix(baseLower, ".bex") {
			dehFiles = append(dehFiles, zf)
		} else if strings.HasSuffix(baseLower, ".txt") {
			txtFiles = append(txtFiles, zf)
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
			if zf.FileInfo().IsDir() {
				continue
			}
			if strings.ToLower(filepath.Base(zf.Name)) == expLower {
				match = zf
				break
			}
		}

		// 2. Normalized match (ignoring spaces, underscores, dashes)
		if match == nil {
			for _, zf := range zr.File {
				if zf.FileInfo().IsDir() {
					continue
				}
				if preset.NormalizeFilename(filepath.Base(zf.Name)) == expNorm {
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
					if zf.FileInfo().IsDir() {
						continue
					}
					zfBase := filepath.Base(zf.Name)
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

		// 4. Single-file fallback
		if match == nil {
			if strings.HasSuffix(expLower, ".wad") && len(wadFiles) == 1 {
				match = wadFiles[0]
			} else if (strings.HasSuffix(expLower, ".deh") || strings.HasSuffix(expLower, ".bex")) &&
				len(dehFiles) == 1 {
				match = dehFiles[0]
			}
		}

		if match == nil {
			return fmt.Errorf("could not find matching file in archive for %s", exp)
		}

		if err := extractZipFile(match, cleanWadsDir, destPath); err != nil {
			return fmt.Errorf("error extracting %s: %w", exp, err)
		}
		fmt.Fprintf(d.Out, "    Installed: %s -> %s\n", filepath.Base(match.Name), destPath)
	}

	// Extract accompanying readme (.txt) if present in the archive
	for _, tf := range txtFiles {
		base := strings.ToLower(filepath.Base(tf.Name))
		if strings.EqualFold(base, "license.txt") {
			continue
		}
		cleanName := filepath.Base(filepath.Clean(tf.Name))
		if cleanName == "." || cleanName == ".." || strings.Contains(cleanName, "..") {
			continue
		}
		destPath, joinErr := fsutil.SafeJoin(cleanWadsDir, cleanName)
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

func extractZipFile(zf *zip.File, targetDir, destPath string) error {
	cleanTargetDir := filepath.Clean(targetDir)
	cleanDest := filepath.Clean(destPath)
	if !strings.HasPrefix(cleanDest, cleanTargetDir+string(filepath.Separator)) {
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

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	_, err = fsutil.WriteFileAtomic(cleanDest, data, 0o644)
	return err
}
