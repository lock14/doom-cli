// Package wad handles multi-mirror downloading, zip extraction, and SoundFont deployment.
package wad

import (
	"crypto/sha256"
	"encoding/hex"
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

// NewDownloader creates a new Downloader targeting wadsDir.
func NewDownloader(wadsDir string, force bool, out io.Writer) *Downloader {
	return NewDownloaderWithRoots([]string{wadsDir}, force, out)
}

// NewDownloaderWithRoots creates a new Downloader using roots[0] as the write directory
// and checking all roots for existing files.
func NewDownloaderWithRoots(roots []string, force bool, out io.Writer) *Downloader {
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
	expected := FilterExpectedFiles(p.Files)
	if len(expected) == 0 {
		return true
	}
	idx := preset.NewDirIndex(d.roots()...).WithPresets(p)
	for _, exp := range expected {
		if _, ok := idx.ResolveFile(exp); !ok {
			return false
		}
	}
	return true
}

// DownloadPreset downloads and extracts the required files for a specific preset.
func (d *Downloader) DownloadPreset(p preset.Preset) error {
	expected := FilterExpectedFiles(p.Files)
	if len(p.Sources) == 0 {
		if len(expected) == 0 {
			return nil
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
		return nil
	}

	if len(expected) == 0 {
		return nil
	}

	// Validate all destination paths before performing network I/O
	for _, exp := range expected {
		if _, err := fsutil.SafeJoin(d.WadsDir, exp); err != nil {
			return fmt.Errorf("invalid destination file %q: %w", exp, err)
		}
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

	if err := os.MkdirAll(d.WadsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", d.WadsDir, err)
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

func urlExtension(rawURL string) string {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	if idx := strings.IndexAny(lower, "?#"); idx >= 0 {
		lower = lower[:idx]
	}
	return filepath.Ext(lower)
}

func (d *Downloader) downloadAndExtract(src preset.Source, expectedFiles []string, p preset.Preset) error {
	req, err := http.NewRequest(http.MethodGet, src.URL, nil)
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

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, hasher), resp.Body)
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("error saving download: %w", err)
	}

	if expectedSum := strings.TrimSpace(src.SHA256); expectedSum != "" {
		actualSum := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualSum, expectedSum) {
			return fmt.Errorf("sha256 mismatch: got %s, want %s", actualSum, expectedSum)
		}
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
	data, err := os.ReadFile(tmpArchive)
	if err != nil {
		return err
	}

	if len(expectedFiles) == 1 {
		dest, err := fsutil.SafeJoin(d.WadsDir, expectedFiles[0])
		if err != nil {
			return err
		}
		if _, err := fsutil.WriteFileAtomic(dest, data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(d.Out, "    Installed: %s -> %s\n", expectedFiles[0], dest)
		return nil
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

	var failures []string
	for _, p := range catalog.Presets {
		if len(p.Sources) == 0 {
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
