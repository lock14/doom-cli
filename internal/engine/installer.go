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
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lock14/doom-cli/internal/fsutil"
	"github.com/lock14/doom-cli/internal/preset"
)

var rePlistBundleExec = regexp.MustCompile(
	`(?s)<key>\s*CFBundleExecutable\s*</key>\s*<string>\s*([^<\r\n]+?)\s*</string>`,
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

func (ins *Installer) appsDir() (string, error) {
	if strings.TrimSpace(ins.AppsDir) != "" {
		return ins.AppsDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home directory: %w", err)
	}
	return filepath.Join(home, "Applications"), nil
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

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func writeMacAppWrapper(dest, appBin string) error {
	wrapper := fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", shellSingleQuote(appBin))
	_, err := fsutil.WriteFileAtomic(dest, []byte(wrapper), 0o755)
	return err
}

func swapDirectoryAtomic(stagedDir, appDest string) error {
	backupDir := appDest + ".doom-bak"
	_ = os.RemoveAll(backupDir)

	hadExisting := false
	if _, err := os.Lstat(appDest); err == nil {
		if err := os.Rename(appDest, backupDir); err != nil {
			return fmt.Errorf("backing up existing bundle %s: %w", appDest, err)
		}
		hadExisting = true
	}

	if err := os.Rename(stagedDir, appDest); err != nil {
		if hadExisting {
			_ = os.Rename(backupDir, appDest)
		}
		return fmt.Errorf("installing bundle %s: %w", appDest, err)
	}

	if hadExisting {
		_ = os.RemoveAll(backupDir)
	}
	return nil
}

func resolveMacBundleExecutable(appDir, targetName string) (string, error) {
	macOSDir := filepath.Join(appDir, "Contents", "MacOS")
	entries, err := os.ReadDir(macOSDir)
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("executable not found in %s", macOSDir)
	}

	cleanTarget := strings.TrimSuffix(strings.ToLower(targetName), ".exe")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.ToLower(e.Name()) == cleanTarget {
			return filepath.Join(macOSDir, e.Name()), nil
		}
	}

	plistPath := filepath.Join(appDir, "Contents", "Info.plist")
	if plistData, readErr := os.ReadFile(plistPath); readErr == nil {
		if m := rePlistBundleExec.FindSubmatch(plistData); len(m) >= 2 {
			execName := filepath.Base(strings.TrimSpace(string(m[1])))
			if execName != "" && execName != "." && execName != ".." {
				cand := filepath.Join(macOSDir, execName)
				if fi, statErr := os.Stat(cand); statErr == nil && !fi.IsDir() {
					return cand, nil
				}
			}
		}
	}

	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			return filepath.Join(macOSDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no valid executable found in %s", macOSDir)
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

func findAppBundlePrefix(slashPath string) (string, string, bool) {
	parts := strings.Split(slashPath, "/")
	for i, part := range parts {
		if strings.HasSuffix(strings.ToLower(part), ".app") && part != ".app" {
			prefix := strings.Join(parts[:i+1], "/") + "/"
			return prefix, part, true
		}
	}
	return "", "", false
}

func (ins *Installer) extractZipMacAppBundle(
	zr *zip.ReadCloser,
	appPrefix, appBundleName, targetName, dest string,
) error {
	appsDir, err := ins.appsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return err
	}

	appDest, err := fsutil.SafeJoin(appsDir, appBundleName)
	if err != nil {
		return fmt.Errorf("illegal .app bundle name %q: %w", appBundleName, err)
	}

	stageParent, err := os.MkdirTemp(appsDir, ".doom-app-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageParent)

	stagedApp := filepath.Join(stageParent, appBundleName)
	if err := os.MkdirAll(stagedApp, 0o755); err != nil {
		return err
	}

	for _, zf := range zr.File {
		slashName := zipEntrySlashName(zf)
		if !strings.HasPrefix(slashName, appPrefix) {
			continue
		}
		relInApp := strings.TrimPrefix(slashName, appPrefix)
		if relInApp == "" || strings.HasSuffix(relInApp, "/") || zf.FileInfo().IsDir() {
			continue
		}
		entryDest, joinErr := fsutil.SafeJoin(stagedApp, relInApp)
		if joinErr != nil {
			return fmt.Errorf("unsafe path in .app archive %q: %w", zf.Name, joinErr)
		}
		if isZipSymlink(zf) {
			if err := extractSafeBundleSymlink(zf, stagedApp, entryDest); err != nil {
				return err
			}
			continue
		}
		perm := os.FileMode(0o644)
		if zf.Mode()&0o111 != 0 || strings.HasPrefix(relInApp, "Contents/MacOS/") ||
			strings.HasSuffix(strings.ToLower(relInApp), ".dylib") {
			perm = 0o755
		}
		if err := extractZipEntry(zf, stagedApp, entryDest, perm); err != nil {
			return err
		}
	}

	if _, err := resolveMacBundleExecutable(stagedApp, targetName); err != nil {
		return err
	}
	if err := swapDirectoryAtomic(stagedApp, appDest); err != nil {
		return err
	}

	appBin, err := resolveMacBundleExecutable(appDest, targetName)
	if err != nil {
		return err
	}
	if err := writeMacAppWrapper(dest, appBin); err != nil {
		return err
	}

	fmt.Fprintf(ins.Out, "✓ Installed %s (.app bundle wrapper) to %s\n\n", targetName, dest)
	return nil
}

func extractSafeBundleSymlink(zf *zip.File, bundleRoot, linkPath string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	linkBytes, err := io.ReadAll(io.LimitReader(rc, 4096))
	if err != nil {
		return err
	}
	target := strings.TrimSpace(string(linkBytes))
	if target == "" || filepath.IsAbs(target) || strings.HasPrefix(target, "/") || strings.HasPrefix(target, `\`) {
		return fmt.Errorf("unsafe symlink target %q in archive entry %q", target, zf.Name)
	}

	linkDir := filepath.Dir(linkPath)
	resolved := filepath.Clean(filepath.Join(linkDir, filepath.FromSlash(strings.ReplaceAll(target, `\`, `/`))))
	cleanRoot := filepath.Clean(bundleRoot)
	prefix := cleanRoot
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	if resolved != cleanRoot && !strings.HasPrefix(resolved, prefix) {
		return fmt.Errorf("symlink %q target %q escapes .app bundle", zf.Name, target)
	}

	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		return err
	}
	_ = os.Remove(linkPath)
	return os.Symlink(target, linkPath)
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
		if zf.FileInfo().IsDir() || isZipSymlink(zf) {
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
			if zf.FileInfo().IsDir() || isZipSymlink(zf) {
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

func (ins *Installer) extractDMGBinary(dmgPath, targetName, dest string) error {
	mountPoint, err := os.MkdirTemp("", "doom_mount_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mountPoint)

	attachCmd := exec.CommandContext(
		ins.ctx(),
		"hdiutil", "attach", dmgPath,
		"-mountpoint", mountPoint,
		"-nobrowse", "-readonly", "-noautoopen", "-quiet",
	)
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
	var appBundleName string
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".app") && e.Name() != ".app" {
			appBundleName = e.Name()
			appEntry = filepath.Join(mountPoint, e.Name())
			break
		}
	}

	if appEntry == "" {
		return fmt.Errorf("no .app bundle found in DMG")
	}

	appsDir, err := ins.appsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return err
	}
	appDest, err := fsutil.SafeJoin(appsDir, appBundleName)
	if err != nil {
		return fmt.Errorf("illegal .app bundle name %q: %w", appBundleName, err)
	}

	stageParent, err := os.MkdirTemp(appsDir, ".doom-dmg-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageParent)

	stagedApp := filepath.Join(stageParent, appBundleName)
	if err := exec.CommandContext(ins.ctx(), "cp", "-RP", appEntry, stagedApp).Run(); err != nil {
		return fmt.Errorf("failed to copy .app to %s: %w", appsDir, err)
	}

	if _, err := resolveMacBundleExecutable(stagedApp, targetName); err != nil {
		return err
	}
	if err := swapDirectoryAtomic(stagedApp, appDest); err != nil {
		return err
	}

	appBin, err := resolveMacBundleExecutable(appDest, targetName)
	if err != nil {
		return err
	}
	if err := writeMacAppWrapper(dest, appBin); err != nil {
		return err
	}

	fmt.Fprintf(ins.Out, "✓ Installed %s (.app wrapper) to %s\n\n", targetName, dest)
	return nil
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
