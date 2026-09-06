package wad

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lock14/doom-cli/internal/preset"
)

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

func (d *Downloader) extractFromZip(zipPath string, size int64, expectedFiles []string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed opening zip: %w", err)
	}
	defer zr.Close()

	cleanWadsDir := filepath.Clean(d.WadsDir)

	var wadFiles []*zip.File
	var dehFiles []*zip.File
	var txtFiles []*zip.File
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		// CodeQL / Zip Slip defense: skip entries containing path traversal
		cleanName := filepath.Clean(zf.Name)
		if strings.HasPrefix(cleanName, "..") ||
			strings.Contains(cleanName, "/../") ||
			strings.Contains(cleanName, "\\..\\") {
			continue
		}

		baseLower := strings.ToLower(filepath.Base(cleanName))
		if strings.HasSuffix(baseLower, ".wad") {
			wadFiles = append(wadFiles, zf)
		} else if strings.HasSuffix(baseLower, ".deh") {
			dehFiles = append(dehFiles, zf)
		} else if strings.HasSuffix(baseLower, ".txt") {
			txtFiles = append(txtFiles, zf)
		}
	}

	for _, exp := range expectedFiles {
		var match *zip.File
		expLower := strings.ToLower(exp)
		expNorm := preset.NormalizeFilename(exp)

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

		// 3. Known aliases
		if match == nil {
			switch expLower {
			case "gdturbo.wad":
				for _, zf := range zr.File {
					if strings.EqualFold(filepath.Base(zf.Name), "gd.wad") {
						match = zf
						break
					}
				}
			case "gd.wad":
				for _, zf := range zr.File {
					if strings.EqualFold(filepath.Base(zf.Name), "gdturbo.wad") {
						match = zf
						break
					}
				}
			}
		}

		// 4. Single-file fallback
		if match == nil {
			if strings.HasSuffix(expLower, ".wad") && len(wadFiles) == 1 {
				match = wadFiles[0]
			} else if strings.HasSuffix(expLower, ".deh") && len(dehFiles) == 1 {
				match = dehFiles[0]
			}
		}

		if match == nil {
			return fmt.Errorf("could not find matching file in archive for %s", exp)
		}

		cleanExp := filepath.Base(filepath.Clean(exp))
		destPath := filepath.Clean(filepath.Join(cleanWadsDir, cleanExp))
		prefix := cleanWadsDir + string(filepath.Separator)
		if !strings.HasPrefix(destPath, prefix) {
			return fmt.Errorf("illegal file destination %q for entry %s", destPath, match.Name)
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
		destPath := filepath.Clean(filepath.Join(cleanWadsDir, cleanName))
		prefix := cleanWadsDir + string(filepath.Separator)
		if !strings.HasPrefix(destPath, prefix) {
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
	cleanTarget := filepath.Clean(targetDir)
	cleanDest := filepath.Clean(destPath)
	prefix := cleanTarget + string(filepath.Separator)
	if !strings.HasPrefix(cleanDest, prefix) {
		return fmt.Errorf("illegal file path in archive: %s", zf.Name)
	}

	if err := os.MkdirAll(filepath.Dir(cleanDest), 0755); err != nil {
		return err
	}

	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(cleanDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
