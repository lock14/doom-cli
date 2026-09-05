package wad

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lock14/doom-configs/internal/preset"
)

// Downloader manages downloading and extracting WAD presets.
type Downloader struct {
	Client  *http.Client
	WadsDir string
	Force   bool
	Out     io.Writer
}

// NewDownloader creates a new Downloader.
func NewDownloader(wadsDir string, force bool, out io.Writer) *Downloader {
	if out == nil {
		out = io.Discard
	}
	return &Downloader{
		Client:  &http.Client{Timeout: 10 * time.Minute},
		WadsDir: wadsDir,
		Force:   force,
		Out:     out,
	}
}

// FilterExpectedFiles returns the files required from a preset, omitting optional enhancements like idkfa 2024.wad.
func FilterExpectedFiles(mappacks []string) []string {
	var filtered []string
	for _, m := range mappacks {
		if strings.EqualFold(m, "idkfa 2024.wad") {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered
}

// IsPresetInstalled checks whether all required mappack files for the preset are already present in wadsDir.
func (d *Downloader) IsPresetInstalled(p preset.Preset) bool {
	expected := FilterExpectedFiles(p.Mappacks)
	if len(expected) == 0 {
		return true
	}
	for _, exp := range expected {
		if _, ok := preset.ResolveFile(d.WadsDir, exp); !ok {
			return false
		}
	}
	return true
}

// DownloadPreset downloads and extracts the files for a specific preset.
func (d *Downloader) DownloadPreset(p preset.Preset) error {
	expected := FilterExpectedFiles(p.Mappacks)
	if len(p.DownloadURLs) == 0 {
		if len(expected) == 0 {
			return nil
		}
		fmt.Fprintf(d.Out, "Note: '%s' is an official commercial release; files must be provided by user or Steam/GOG extractor.\n", p.Name)
		return nil
	}

	if len(expected) == 0 {
		return nil
	}

	if !d.Force && d.IsPresetInstalled(p) {
		fmt.Fprintf(d.Out, "✓ [%s] All required files already exist in %s. (Use --force to re-download)\n", p.Name, d.WadsDir)
		return nil
	}

	if err := os.MkdirAll(d.WadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", d.WadsDir, err)
	}

	fmt.Fprintf(d.Out, ">>> Downloading: %s\n", p.Name)

	var lastErr error
	for _, rawURL := range p.DownloadURLs {
		cleanURL := strings.TrimSpace(rawURL)
		if cleanURL == "" {
			continue
		}

		// Skip informational HTML pages that are not downloadable files
		lower := strings.ToLower(cleanURL)
		if !strings.HasSuffix(lower, ".zip") &&
			!strings.HasSuffix(lower, ".wad") &&
			!strings.HasSuffix(lower, ".pk3") &&
			!strings.HasSuffix(lower, ".7z") {
			continue
		}

		fmt.Fprintf(d.Out, "    Trying: %s\n", cleanURL)
		err := d.downloadAndExtract(cleanURL, expected)
		if err == nil {
			fmt.Fprintln(d.Out)
			return nil
		}
		lastErr = err
		fmt.Fprintf(d.Out, "    Failed mirror: %v\n", err)
	}

	if lastErr != nil {
		return fmt.Errorf("failed to download %s from available mirrors: %w", p.Name, lastErr)
	}
	return fmt.Errorf("no valid archive download mirrors available for %s", p.Name)
}

func (d *Downloader) downloadAndExtract(url string, expectedFiles []string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "doom-configs/2.0")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	// Read downloaded content into temporary file or buffer
	tmpDir, err := os.MkdirTemp("", "doom_dl_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpArchive := filepath.Join(tmpDir, "archive.bin")
	f, err := os.Create(tmpArchive)
	if err != nil {
		return err
	}

	size, err := io.Copy(f, resp.Body)
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("error saving download: %w", err)
	}

	// Determine if zip archive
	if isZip(tmpArchive) {
		return d.extractFromZip(tmpArchive, size, expectedFiles)
	}

	// If direct .wad / .pk3
	if len(expectedFiles) == 1 {
		dest := filepath.Join(d.WadsDir, expectedFiles[0])
		data, err := os.ReadFile(tmpArchive)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return err
		}
		fmt.Fprintf(d.Out, "    Installed: %s -> %s\n", expectedFiles[0], dest)
		return nil
	}

	return fmt.Errorf("unrecognized file format and multiple files expected")
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

func (d *Downloader) extractFromZip(zipPath string, size int64, expectedFiles []string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed opening zip: %w", err)
	}
	defer zr.Close()

	var wadFiles []*zip.File
	var dehFiles []*zip.File
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		baseLower := strings.ToLower(filepath.Base(zf.Name))
		if strings.HasSuffix(baseLower, ".wad") {
			wadFiles = append(wadFiles, zf)
		} else if strings.HasSuffix(baseLower, ".deh") {
			dehFiles = append(dehFiles, zf)
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

		destPath := filepath.Join(d.WadsDir, exp)
		if err := extractZipFile(match, destPath); err != nil {
			return fmt.Errorf("error extracting %s: %w", exp, err)
		}
		fmt.Fprintf(d.Out, "    Installed: %s -> %s\n", filepath.Base(match.Name), destPath)
	}

	return nil
}

func extractZipFile(zf *zip.File, destPath string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// DownloadAll downloads all downloadable community megawads in the catalog.
func (d *Downloader) DownloadAll(catalog *preset.PresetCatalog) error {
	fmt.Fprintf(d.Out, "=== Doom Community Megawad Downloader ===\n")
	fmt.Fprintf(d.Out, "Target directory: %s\n\n", d.WadsDir)

	var failures []string
	for _, p := range catalog.Presets {
		if len(p.DownloadURLs) == 0 {
			continue
		}
		if err := d.DownloadPreset(p); err != nil {
			failures = append(failures, fmt.Sprintf("%s (%v)", p.Name, err))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("failed downloading %d presets:\n - %s", len(failures), strings.Join(failures, "\n - "))
	}

	fmt.Fprintln(d.Out, "All community megawads processed!")
	return nil
}
