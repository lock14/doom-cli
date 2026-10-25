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

	"github.com/lock14/doom-cli/internal/fsutil"
	"github.com/lock14/doom-cli/internal/preset"
)

// Downloader manages downloading and extracting WAD presets.
type Downloader struct {
	Client   *http.Client
	WadsDir  string
	WadRoots []string
	Force    bool
	Out      io.Writer
}

// NewDownloader creates a new Downloader using roots[0] as the write directory
// and checking all roots for existing files.
func NewDownloader(roots []string, force bool, out io.Writer) *Downloader {
	if out == nil {
		out = io.Discard
	}
	var primary string
	if len(roots) > 0 {
		primary = roots[0]
	}
	return &Downloader{
		Client:   &http.Client{Timeout: 10 * time.Minute},
		WadsDir:  primary,
		WadRoots: append([]string(nil), roots...),
		Force:    force,
		Out:      out,
	}
}

func (d *Downloader) roots() []string {
	if len(d.WadRoots) > 0 {
		return d.WadRoots
	}
	if d.WadsDir != "" {
		return []string{d.WadsDir}
	}
	return nil
}

// FilterExpectedFiles returns the required file names from files, omitting optional entries.
func FilterExpectedFiles(files []preset.FileSpec) []string {
	var filtered []string
	for _, f := range files {
		if f.Optional {
			continue
		}
		filtered = append(filtered, f.Name)
	}
	return filtered
}

// IsPresetInstalled checks whether all required files for the preset are present across configured WAD roots.
func (d *Downloader) IsPresetInstalled(p preset.Preset) bool {
	return preset.NewDirIndex(d.roots()...).WithPresets(p).ArePresetFilesInstalled(p)
}

// DownloadPreset downloads and extracts the required files for a specific preset.
func (d *Downloader) DownloadPreset(p preset.Preset) error {
	_, err := d.downloadPresetWithIndex(p, nil)
	return err
}

func (d *Downloader) downloadPresetWithIndex(p preset.Preset, idx *preset.DirIndex) (bool, error) {
	expected := FilterExpectedFiles(p.Files)
	if len(p.Sources) == 0 {
		if len(expected) == 0 {
			return false, nil
		}
		if p.Category == "Community Megawad" || p.Category == "Community Episode" {
			fmt.Fprintf(
				d.Out,
				"Note: '%s' is a community release without automated mirrors; "+
					"place files (%s) in %s or discover via Steam add-ons.\n",
				p.Name,
				strings.Join(expected, ", "),
				d.WadsDir,
			)
		} else {
			fmt.Fprintf(
				d.Out,
				"Note: '%s' is an official commercial release; "+
					"files must be provided by user or Steam or GOG extractor.\n",
				p.Name,
			)
		}
		return false, nil
	}

	if len(expected) == 0 {
		return false, nil
	}

	// Validate all destination paths before performing network I/O
	for _, exp := range expected {
		if _, err := fsutil.SafeJoin(d.WadsDir, exp); err != nil {
			return false, fmt.Errorf("invalid destination file %q: %w", exp, err)
		}
	}

	if !d.Force {
		installed := false
		if idx != nil {
			installed = idx.ArePresetFilesInstalled(p)
		} else {
			installed = d.IsPresetInstalled(p)
		}
		if installed {
			fmt.Fprintf(
				d.Out,
				"✓ [%s] All required files already exist in %s. (Use --force to re-download)\n",
				p.Name,
				d.WadsDir,
			)
			return false, nil
		}
	}

	if err := os.MkdirAll(d.WadsDir, 0o755); err != nil {
		return false, fmt.Errorf("failed to create directory %s: %w", d.WadsDir, err)
	}

	fmt.Fprintf(d.Out, ">>> Downloading: %s\n", p.Name)

	var lastErr error
	for _, src := range p.Sources {
		cleanURL := strings.TrimSpace(src.URL)
		if cleanURL == "" {
			continue
		}

		if !preset.IsSupportedSourceURL(cleanURL) {
			lastErr = fmt.Errorf(
				"unsupported archive format in source URL %q (only .zip, .wad, and .pk3 are supported)",
				cleanURL,
			)
			fmt.Fprintf(d.Out, "    Failed mirror: %v\n", lastErr)
			continue
		}

		fmt.Fprintf(d.Out, "    Trying: %s\n", cleanURL)
		err := d.downloadAndExtract(src, expected, p)
		if err == nil {
			fmt.Fprintln(d.Out)
			return true, nil
		}
		lastErr = err
		fmt.Fprintf(d.Out, "    Failed mirror: %v\n", err)
	}

	if lastErr != nil {
		return false, fmt.Errorf("failed to download %s from available mirrors: %w", p.Name, lastErr)
	}
	return false, fmt.Errorf("no valid archive download mirrors available for %s", p.Name)
}

func urlExtension(rawURL string) string {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	if idx := strings.IndexAny(lower, "?#"); idx >= 0 {
		lower = lower[:idx]
	}
	return filepath.Ext(lower)
}

func (d *Downloader) downloadAndExtract(src preset.Source, expectedFiles []string, p preset.Preset) error {
	tmpDir, err := os.MkdirTemp("", "doom_dl_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpArchive := filepath.Join(tmpDir, "archive.bin")
	size, err := fsutil.DownloadVerified(d.Client, src.URL, tmpArchive, src.SHA256, 0o644)
	if err != nil {
		return err
	}

	ext := urlExtension(src.URL)
	if ext == ".wad" || ext == ".pk3" {
		return d.installDirectAsset(ext, tmpArchive, expectedFiles)
	}

	if isZip(tmpArchive) {
		return d.extractFromZip(tmpArchive, size, expectedFiles, p)
	}

	return fmt.Errorf("downloaded payload from %s is not a valid zip archive", src.URL)
}

func (d *Downloader) installDirectAsset(ext, tmpArchive string, expectedFiles []string) error {
	if len(expectedFiles) == 1 {
		dest, err := fsutil.SafeJoin(d.WadsDir, expectedFiles[0])
		if err != nil {
			return err
		}
		if _, err := fsutil.CopyFileAtomic(tmpArchive, dest, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(d.Out, "    Installed: %s -> %s\n", expectedFiles[0], dest)
		return nil
	}

	data, err := os.ReadFile(tmpArchive)
	if err != nil {
		return err
	}

	if ext == ".wad" {
		var wadTargets []string
		var dehTargets []string
		for _, exp := range expectedFiles {
			lower := strings.ToLower(exp)
			switch {
			case strings.HasSuffix(lower, ".wad"):
				wadTargets = append(wadTargets, exp)
			case strings.HasSuffix(lower, ".deh") || strings.HasSuffix(lower, ".bex"):
				dehTargets = append(dehTargets, exp)
			default:
				return fmt.Errorf("direct %s download requires 1 expected file, got %d", ext, len(expectedFiles))
			}
		}
		if len(wadTargets) == 1 && len(dehTargets) == 1 {
			dehData, ok := ExtractWADLump(data, "DEHACKED")
			if !ok {
				return fmt.Errorf("direct .wad download missing embedded DEHACKED lump for %s", dehTargets[0])
			}
			wadDest, err := fsutil.SafeJoin(d.WadsDir, wadTargets[0])
			if err != nil {
				return err
			}
			dehDest, err := fsutil.SafeJoin(d.WadsDir, dehTargets[0])
			if err != nil {
				return err
			}
			if _, err := fsutil.WriteFileAtomic(wadDest, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(d.Out, "    Installed: %s -> %s\n", wadTargets[0], wadDest)
			if _, err := fsutil.WriteFileAtomic(dehDest, dehData, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(d.Out, "    Extracted: DEHACKED -> %s\n", dehDest)
			return nil
		}
	}

	return fmt.Errorf("direct %s download requires 1 expected file, got %d", ext, len(expectedFiles))
}

// DownloadAll downloads all downloadable community megawads in the catalog.
func (d *Downloader) DownloadAll(catalog *preset.Catalog) error {
	fmt.Fprintf(d.Out, "=== Doom Community Megawad Downloader ===\n")
	fmt.Fprintf(d.Out, "Target directory: %s\n\n", d.WadsDir)

	idx := preset.NewDirIndex(d.roots()...).WithCatalog(catalog)
	var failures []string
	for _, p := range catalog.Presets {
		if len(p.Sources) == 0 {
			continue
		}
		downloaded, err := d.downloadPresetWithIndex(p, idx)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s (%v)", p.Name, err))
			continue
		}
		if downloaded {
			idx = preset.NewDirIndex(d.roots()...).WithCatalog(catalog)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("failed downloading %d presets:\n - %s", len(failures), strings.Join(failures, "\n - "))
	}

	fmt.Fprintln(d.Out, "All community megawads processed!")
	return nil
}
