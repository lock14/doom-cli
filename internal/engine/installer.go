package engine

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lock14/doom-cli/internal/fsutil"
	"github.com/lock14/doom-cli/internal/preset"
)

// Installer manages downloading and deploying Doom source ports from declarative recipes.
type Installer struct {
	BinDir  string
	Client  *http.Client
	Out     io.Writer
	APIBase string
	GOOS    string
	GOARCH  string
	Force   bool
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
func SelectAsset(recipe preset.PlatformRecipe, assets []GitHubAsset) (string, string, bool) {
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

// ResolveLatestGitHubURL queries the GitHub Releases API for an asset matching patternRegex,
// falling back to fallbackURL if the API request fails or no asset matches.
func (ins *Installer) ResolveLatestGitHubURL(repo, patternRegex, fallbackURL string) string {
	recipe := preset.PlatformRecipe{
		Assets: []string{patternRegex},
	}
	if fallbackURL != "" {
		recipe.Pinned = &preset.PinnedRelease{URL: fallbackURL}
	}
	url, _, err := ins.resolveRecipeAsset(repo, recipe)
	if err != nil {
		return fallbackURL
	}
	return url
}

func (ins *Installer) resolveRecipeAsset(repo string, recipe preset.PlatformRecipe) (string, string, error) {
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
		req, err := http.NewRequest(http.MethodGet, apiURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "doom-cli/2.0")
			resp, doErr := ins.Client.Do(req)
			if doErr == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var release GitHubRelease
					if decErr := json.NewDecoder(resp.Body).Decode(&release); decErr == nil {
						if url, digest, ok := SelectAsset(recipe, release.Assets); ok {
							return url, digest, nil
						}
					}
				}
			}
		}
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

// DownloadFile downloads a file from url to targetPath and verifies expectedSHA256 when non-empty.
func (ins *Installer) DownloadFile(url, targetPath string) error {
	return ins.DownloadFileVerified(url, targetPath, "")
}

// DownloadFileVerified downloads a file from url to targetPath and checks expectedSHA256 when provided.
func (ins *Installer) DownloadFileVerified(url, targetPath, expectedSHA256 string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "doom-cli/2.0")

	resp, err := ins.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, hasher), resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(targetPath)
		return closeErr
	}

	if cleanSum := strings.TrimSpace(expectedSHA256); cleanSum != "" {
		actualSum := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualSum, cleanSum) {
			_ = os.Remove(targetPath)
			return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", url, actualSum, cleanSum)
		}
	}
	return nil
}

// InstallEngine downloads and installs a single engine using its declarative InstallRecipe.
func (ins *Installer) InstallEngine(eng preset.EngineConfig) error {
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

	targetName := filepath.Base(eng.EffectiveBinary())
	if ins.goos() == "windows" && !strings.HasSuffix(strings.ToLower(targetName), ".exe") {
		targetName += ".exe"
	}
	dest := filepath.Join(ins.BinDir, targetName)

	if !ins.Force {
		if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
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
func (ins *Installer) InstallCatalog(cat *preset.Catalog) error {
	if cat == nil {
		var err error
		cat, err = preset.LoadCuratedCatalog()
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
		if err := ins.InstallEngine(cat.Engines[id]); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// InstallUZDoom downloads and installs UZDoom using its curated catalog recipe.
func (ins *Installer) InstallUZDoom() error {
	cat, err := preset.LoadCuratedCatalog()
	if err != nil {
		return err
	}
	eng, ok := cat.FindEngine("uzdoom")
	if !ok {
		return fmt.Errorf("curated engine uzdoom not found")
	}
	return ins.InstallEngine(eng)
}

// InstallDSDA downloads and installs DSDA-Doom using its curated catalog recipe.
func (ins *Installer) InstallDSDA() error {
	cat, err := preset.LoadCuratedCatalog()
	if err != nil {
		return err
	}
	eng, ok := cat.FindEngine("dsda-doom")
	if !ok {
		return fmt.Errorf("curated engine dsda-doom not found")
	}
	return ins.InstallEngine(eng)
}

// InstallAll installs all curated source ports, attempting each engine even if one fails.
func (ins *Installer) InstallAll() error {
	cat, err := preset.LoadCuratedCatalog()
	if err != nil {
		return err
	}
	return ins.InstallCatalog(cat)
}

func (ins *Installer) deployAsset(targetName, url, expectedSHA256 string) error {
	if url == "" {
		return fmt.Errorf("no download URL found for %s", targetName)
	}

	dest := filepath.Join(ins.BinDir, targetName)
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

	lowerURL := strings.ToLower(url)
	if strings.HasSuffix(lowerURL, ".zip") {
		return ins.extractZipBinary(tmpFile, targetName, dest)
	} else if strings.HasSuffix(lowerURL, ".dmg") && ins.goos() == "darwin" {
		return ins.extractDMGBinary(tmpFile, targetName, dest)
	}

	if err := copyBinaryFile(tmpFile, dest); err != nil {
		return err
	}
	_ = os.Chmod(dest, 0o755)
	fmt.Fprintf(ins.Out, "✓ Installed %s to %s\n\n", targetName, dest)
	return nil
}

func (ins *Installer) extractZipBinary(zipPath, targetName, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	var binMatch *zip.File
	cleanTarget := strings.TrimSuffix(strings.ToLower(targetName), ".exe")

	// 1. Exact match on targetName (e.g. "uzdoom.exe" or "dsda-doom")
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(zf.Name))
		if base == strings.ToLower(targetName) {
			binMatch = zf
			break
		}
	}

	// 2. Fallback: match without .exe extension, ensuring on Windows it has .exe extension
	if binMatch == nil {
		for _, zf := range zr.File {
			if zf.FileInfo().IsDir() {
				continue
			}
			base := strings.ToLower(filepath.Base(zf.Name))
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

	cleanDest := filepath.Clean(dest)
	cleanBinDir := filepath.Clean(ins.BinDir)
	prefix := cleanBinDir + string(filepath.Separator)
	if !strings.HasPrefix(cleanDest, prefix) {
		return fmt.Errorf("illegal binary target: %s", targetName)
	}

	if err := extractZipEntry(binMatch, cleanBinDir, cleanDest); err != nil {
		return err
	}
	_ = os.Chmod(dest, 0o755)

	// Also extract companion files (.wad, .pk3, dlls, soundfonts, etc.)
	binParent := filepath.Dir(binMatch.Name)
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || zf == binMatch {
			continue
		}
		relPath, relErr := filepath.Rel(binParent, zf.Name)
		if relErr != nil || strings.HasPrefix(relPath, "..") {
			continue
		}
		base := filepath.Base(filepath.Clean(zf.Name))
		if base == "." || base == ".." || strings.Contains(base, "..") {
			continue
		}
		if isCompanionFile(base) {
			companionDest := filepath.Clean(filepath.Join(cleanBinDir, filepath.FromSlash(relPath)))
			if !strings.HasPrefix(companionDest, prefix) {
				continue
			}
			_ = extractZipEntry(zf, cleanBinDir, companionDest)
		}
	}

	fmt.Fprintf(ins.Out, "✓ Installed %s to %s\n\n", targetName, dest)
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

func (ins *Installer) extractDMGBinary(dmgPath, targetName, dest string) error {
	mountPoint, err := os.MkdirTemp("", "doom_mount_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mountPoint)

	attachCmd := exec.Command("hdiutil", "attach", dmgPath, "-mountpoint", mountPoint, "-nobrowse", "-quiet")
	if err := attachCmd.Run(); err != nil {
		return fmt.Errorf("failed to attach DMG: %w", err)
	}
	defer func() {
		_ = exec.Command("hdiutil", "detach", mountPoint, "-quiet").Run()
	}()

	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		return err
	}

	var appEntry string
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".app") {
			appEntry = filepath.Join(mountPoint, e.Name())
			break
		}
	}

	if appEntry == "" {
		return fmt.Errorf("no .app bundle found in DMG")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving user home directory: %w", err)
	}
	appsDir := filepath.Join(home, "Applications")
	_ = os.MkdirAll(appsDir, 0o755)
	appDest := filepath.Join(appsDir, filepath.Base(appEntry))

	_ = os.RemoveAll(appDest)
	if err := exec.Command("cp", "-R", appEntry, appsDir+"/").Run(); err != nil {
		return fmt.Errorf("failed to copy .app to %s: %w", appsDir, err)
	}

	macOSDir := filepath.Join(appDest, "Contents", "MacOS")
	macEntries, err := os.ReadDir(macOSDir)
	if err != nil || len(macEntries) == 0 {
		return fmt.Errorf("executable not found in %s", macOSDir)
	}

	appBin := filepath.Join(macOSDir, macEntries[0].Name())
	wrapper := fmt.Sprintf("#!/bin/sh\nexec %q \"$@\"\n", appBin)
	if _, err := fsutil.WriteFileAtomic(dest, []byte(wrapper), 0o755); err != nil {
		return err
	}

	fmt.Fprintf(ins.Out, "✓ Installed %s (.app wrapper) to %s\n\n", targetName, dest)
	return nil
}

func extractZipEntry(zf *zip.File, destDir, destPath string) error {
	cleanDir := filepath.Clean(destDir)
	cleanDest := filepath.Clean(destPath)
	prefix := cleanDir + string(filepath.Separator)
	if !strings.HasPrefix(cleanDest, prefix) {
		return fmt.Errorf("illegal file path in archive: %s", zf.Name)
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

func copyBinaryFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_, err = fsutil.WriteFileAtomic(dst, data, 0o755)
	return err
}
