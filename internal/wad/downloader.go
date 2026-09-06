// Package wad handles multi-mirror downloading, zip extraction, and SoundFont deployment.
package wad

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lock14/doom-cli/internal/preset"
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
		fmt.Fprintf(
			d.Out,
			"Note: '%s' is an official commercial release; "+
				"files must be provided by user or Steam/GOG extractor.\n",
			p.Name,
		)
		return nil
	}

	if len(expected) == 0 {
		return nil
	}

	if !d.Force && d.IsPresetInstalled(p) {
		fmt.Fprintf(
			d.Out,
			"✓ [%s] All required files already exist in %s. (Use --force to re-download)\n",
			p.Name,
			d.WadsDir,
		)
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
	req.Header.Set("User-Agent", "doom-cli/2.0")

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

// DownloadAll downloads all downloadable community megawads in the catalog.
func (d *Downloader) DownloadAll(catalog *preset.Catalog) error {
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
