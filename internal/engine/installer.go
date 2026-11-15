package engine

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/fsutil"
)

// Installer manages downloading and deploying Doom source ports from declarative recipes.
type Installer struct {
	BinDir  string
	AppsDir string
	Client  *http.Client
	Out     io.Writer
	APIBase string
	GOOS    string
	GOARCH  string
	Force   bool
	Ctx     context.Context
}

// NewInstaller creates a new engine installer targeting binDir.
func NewInstaller(binDir string, out io.Writer) *Installer {
	if out == nil {
		out = io.Discard
	}
	return &Installer{
		BinDir:  binDir,
		Client:  &http.Client{Timeout: 5 * time.Minute},
		Out:     out,
		APIBase: "https://api.github.com",
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
	}
}

func (ins *Installer) ctx() context.Context {
	if ins.Ctx != nil {
		return ins.Ctx
	}
	return context.Background()
}

func (ins *Installer) goos() string {
	if ins.GOOS != "" {
		return ins.GOOS
	}
	return runtime.GOOS
}

func (ins *Installer) goarch() string {
	if ins.GOARCH != "" {
		return ins.GOARCH
	}
	return runtime.GOARCH
}

// PlatformKey formats an OS/arch pair as "<goos>/<goarch>".
func PlatformKey(goos, goarch string) string {
	return fmt.Sprintf("%s/%s", goos, goarch)
}

func (ins *Installer) platformKey() string {
	return PlatformKey(ins.goos(), ins.goarch())
}

// GitHubRelease represents a release object from the GitHub Releases API.
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a downloadable release asset from the GitHub Releases API.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest,omitempty"`
}

// SelectAsset evaluates recipe.Assets regex patterns in priority order against assets,
// returning the first matching download URL and optional SHA-256 hex digest.
func SelectAsset(recipe catalog.PlatformRecipe, assets []GitHubAsset) (string, string, bool) {
	for _, pattern := range recipe.Assets {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			continue
		}
		for _, asset := range assets {
			name := asset.Name
			if re.MatchString(name) &&
				!strings.HasSuffix(name, ".zsync") &&
				!strings.HasSuffix(name, ".asc") &&
				!strings.HasSuffix(name, ".sig") {
				digest := strings.TrimSpace(asset.Digest)
				digest = strings.TrimPrefix(strings.ToLower(digest), "sha256:")
				return asset.BrowserDownloadURL, digest, true
			}
		}
	}
	return "", "", false
}

func (ins *Installer) resolveRecipeAsset(repo string, recipe catalog.PlatformRecipe) (string, string, error) {
	var apiURL string
	if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") {
		apiURL = repo
	} else if repo != "" {
		base := strings.TrimRight(ins.APIBase, "/")
		if base == "" {
			base = "https://api.github.com"
		}
		apiURL = fmt.Sprintf("%s/repos/%s/releases/latest", base, repo)
	}

	if apiURL != "" && len(recipe.Assets) > 0 {
		req, err := http.NewRequestWithContext(ins.ctx(), http.MethodGet, apiURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", fsutil.UserAgent)
			client := ins.Client
			if client == nil {
				client = &http.Client{Timeout: 30 * time.Second}
			}
			resp, doErr := client.Do(req)
			if doErr == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var release GitHubRelease
					if decErr := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&release); decErr == nil {
						if url, digest, ok := SelectAsset(recipe, release.Assets); ok {
							return url, digest, nil
						}
					}
				}
			}
		}
	}

	if err := ins.ctx().Err(); err != nil {
		return "", "", err
	}

	if recipe.Pinned != nil && strings.TrimSpace(recipe.Pinned.URL) != "" {
		ver := recipe.Pinned.Version
		if ver == "" {
			ver = recipe.Pinned.URL
		}
		fmt.Fprintf(
			ins.Out,
			"  Warning: live release lookup unavailable for %s; using pinned release %s\n",
			repo,
			ver,
		)
		return recipe.Pinned.URL, strings.TrimSpace(recipe.Pinned.SHA256), nil
	}

	return "", "", fmt.Errorf("no matching release asset found for %s", repo)
}

// DownloadFileVerified downloads a file from url to targetPath and checks expectedSHA256 when provided.
func (ins *Installer) DownloadFileVerified(url, targetPath, expectedSHA256 string) error {
	_, err := fsutil.DownloadVerifiedContext(ins.ctx(), ins.Client, url, targetPath, expectedSHA256, 0o755)
	return err
}

// InstallEngine downloads and installs a single engine using its declarative InstallRecipe.
func (ins *Installer) InstallEngine(eng catalog.EngineConfig) error {
	if eng.Install == nil {
		return fmt.Errorf("engine %q does not define an install recipe", eng.ID)
	}
	platKey := ins.platformKey()
	recipe, ok := eng.Install.Platforms[platKey]
	if !ok {
		return fmt.Errorf("engine %q has no install recipe for platform %s", eng.ID, platKey)
	}
	if recipe.Unsupported {
		fmt.Fprintf(ins.Out, "Skipping %s: unsupported on %s\n", eng.EffectiveDisplayName(), platKey)
		return nil
	}

	targetName := path.Base(strings.ReplaceAll(eng.EffectiveBinary(), `\`, `/`))
	if ins.goos() == "windows" && !strings.HasSuffix(strings.ToLower(targetName), ".exe") {
		targetName += ".exe"
	}
	dest, err := fsutil.SafeJoin(ins.BinDir, targetName)
	if err != nil {
		return fmt.Errorf("invalid engine binary target %q: %w", targetName, err)
	}

	if !ins.Force {
		if fi, statErr := os.Stat(dest); statErr == nil && fi.Size() > 0 {
			fmt.Fprintf(
				ins.Out,
				"✓ %s already installed: %s\n  (Use --force to re-download)\n\n",
				eng.EffectiveDisplayName(),
				dest,
			)
			return nil
		}
	}

	fmt.Fprintf(ins.Out, "Installing %s...\n", eng.EffectiveDisplayName())

	url, sha256Hex, err := ins.resolveRecipeAsset(eng.Install.Repo, recipe)
	if err != nil {
		return err
	}
	return ins.deployAsset(targetName, url, sha256Hex)
}

// InstallCatalog attempts to install all engines in cat that define an InstallRecipe,
// collecting and returning any errors after trying every installable engine.
func (ins *Installer) InstallCatalog(cat *catalog.Catalog) error {
	if cat == nil {
		var err error
		cat, err = catalog.LoadCurated()
		if err != nil {
			return err
		}
	}

	ids := make([]string, 0, len(cat.Engines))
	for id, eng := range cat.Engines {
		if eng.Install != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	var errs []error
	for _, id := range ids {
		if err := ins.ctx().Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := ins.InstallEngine(cat.Engines[id]); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func urlPathLower(rawURL string) string {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	if idx := strings.IndexAny(lower, "?#"); idx >= 0 {
		lower = lower[:idx]
	}
	return lower
}

func (ins *Installer) deployAsset(targetName, url, expectedSHA256 string) error {
	if url == "" {
		return fmt.Errorf("no download URL found for %s", targetName)
	}

	dest, err := fsutil.SafeJoin(ins.BinDir, targetName)
	if err != nil {
		return fmt.Errorf("illegal binary target %q: %w", targetName, err)
	}
	fmt.Fprintf(ins.Out, "  Downloading %s from %s\n", targetName, url)

	tmpDir, err := os.MkdirTemp("", "engine_dl_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "downloaded.bin")
	if err := ins.DownloadFileVerified(url, tmpFile, expectedSHA256); err != nil {
		return fmt.Errorf("download error: %w", err)
	}

	lowerPath := urlPathLower(url)
	if strings.HasSuffix(lowerPath, ".zip") {
		return ins.extractZipBinary(tmpFile, targetName, dest)
	} else if strings.HasSuffix(lowerPath, ".dmg") && ins.goos() == "darwin" {
		return ins.extractDMGBinary(tmpFile, targetName, dest)
	}

	if err := copyBinaryFile(tmpFile, dest); err != nil {
		return err
	}
	_ = os.Chmod(dest, 0o755)
	fmt.Fprintf(ins.Out, "✓ Installed %s to %s\n\n", targetName, dest)
	return nil
}

func zipEntrySlashName(zf *zip.File) string {
	return strings.ReplaceAll(zf.Name, `\`, `/`)
}

func isZipSymlink(zf *zip.File) bool {
	return zf.Mode()&os.ModeSymlink != 0
}

func (ins *Installer) extractZipBinary(zipPath, targetName, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	var binMatch *zip.File
	cleanTarget := strings.TrimSuffix(strings.ToLower(targetName), ".exe")

	// 1. Exact match on targetName (such as "uzdoom.exe" or "dsda-doom")
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || isZipSymlink(zf) || strings.Contains(zf.Name, "..") {
			continue
		}
		base := strings.ToLower(path.Base(zipEntrySlashName(zf)))
		if base == strings.ToLower(targetName) {
			binMatch = zf
			break
		}
	}

	// 2. Fallback: match without .exe extension, ensuring on Windows it has .exe extension
	if binMatch == nil {
		for _, zf := range zr.File {
			if zf.FileInfo().IsDir() || isZipSymlink(zf) || strings.Contains(zf.Name, "..") {
				continue
			}
			base := strings.ToLower(path.Base(zipEntrySlashName(zf)))
			if ins.goos() == "windows" && !strings.HasSuffix(base, ".exe") {
				continue
			}
			baseClean := strings.TrimSuffix(base, ".exe")
			if baseClean == cleanTarget {
				binMatch = zf
				break
			}
		}
	}

	if binMatch == nil {
		return fmt.Errorf("binary %s not found in zip", targetName)
	}

	// On macOS, if the binary resides inside a .app bundle (such as dsda-doom-mac-uni.zip),
	// preserve the complete .app bundle (Contents/Frameworks, Contents/Resources) and install a wrapper.
	if ins.goos() == "darwin" {
		if appPrefix, appName, ok := findAppBundlePrefix(zipEntrySlashName(binMatch)); ok {
			return ins.extractZipMacAppBundle(zr, appPrefix, appName, targetName, dest)
		}
	}

	cleanDest, err := fsutil.SafeJoin(ins.BinDir, targetName)
	if err != nil || cleanDest != filepath.Clean(dest) {
		return fmt.Errorf("illegal binary target: %s", targetName)
	}

	if err := extractZipEntry(binMatch, ins.BinDir, cleanDest, 0o755); err != nil {
		return err
	}
	_ = os.Chmod(cleanDest, 0o755)

	// Also extract companion files (.wad, .pk3, dlls, soundfonts, etc.)
	binSlash := zipEntrySlashName(binMatch)
	binParent := path.Dir(binSlash)
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || isZipSymlink(zf) || zf == binMatch {
			continue
		}
		entrySlash := zipEntrySlashName(zf)
		var relSlash string
		if binParent == "." || binParent == "" {
			relSlash = entrySlash
		} else if strings.HasPrefix(entrySlash, binParent+"/") {
			relSlash = strings.TrimPrefix(entrySlash, binParent+"/")
		} else {
			continue
		}

		base := path.Base(relSlash)
		if !isCompanionFile(base) {
			continue
		}
		companionDest, joinErr := fsutil.SafeJoin(ins.BinDir, relSlash)
		if joinErr != nil {
			continue
		}
		if err := extractZipEntry(zf, ins.BinDir, companionDest, 0o644); err != nil {
			return fmt.Errorf("extracting companion file %s: %w", zf.Name, err)
		}
	}

	fmt.Fprintf(ins.Out, "✓ Installed %s to %s\n\n", targetName, cleanDest)
	return nil
}

var companionExtensions = map[string]bool{
	".wad":   true,
	".pk3":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".sf2":   true,
	".wopl":  true,
	".wopn":  true,
}

func isCompanionFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return companionExtensions[ext]
}

func extractZipEntry(zf *zip.File, destDir, destPath string, perm os.FileMode) error {
	cleanDir := filepath.Clean(destDir)
	cleanDest := filepath.Clean(destPath)
	prefix := cleanDir
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	if cleanDest == cleanDir || !strings.HasPrefix(cleanDest, prefix) {
		return fmt.Errorf("illegal file path in archive: %s", zf.Name)
	}

	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, _, err = fsutil.WriteLimitedReaderAtomic(cleanDest, rc, fsutil.MaxExtractFileBytes, perm)
	return err
}

func copyBinaryFile(src, dst string) error {
	_, err := fsutil.CopyFileAtomic(src, dst, 0o755)
	return err
}
