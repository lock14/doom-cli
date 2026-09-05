package engine

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Installer manages downloading and deploying Doom engines and launchers.
type Installer struct {
	BinDir string
	Client *http.Client
	Out    io.Writer
}

// NewInstaller creates a new engine installer.
func NewInstaller(binDir string, out io.Writer) *Installer {
	if out == nil {
		out = io.Discard
	}
	return &Installer{
		BinDir: binDir,
		Client: &http.Client{Timeout: 5 * time.Minute},
		Out:    out,
	}
}

// GitHubRelease represents a release object from GitHub API.
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a downloadable release asset.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ResolveLatestGitHubURL queries GitHub Releases API for an asset matching regex pattern, falling back to fallbackURL.
func (ins *Installer) ResolveLatestGitHubURL(repo, patternRegex, fallbackURL string) string {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "doom-configs/2.0")
		resp, err := ins.Client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var release GitHubRelease
			if err := json.NewDecoder(resp.Body).Decode(&release); err == nil {
				re, err := regexp.Compile("(?i)" + patternRegex)
				if err == nil {
					for _, asset := range release.Assets {
						name := asset.Name
						if re.MatchString(name) &&
							!strings.HasSuffix(name, ".zsync") &&
							!strings.HasSuffix(name, ".asc") &&
							!strings.HasSuffix(name, ".sig") {
							return asset.BrowserDownloadURL
						}
					}
				}
			}
		}
	}
	return fallbackURL
}

// DownloadFile downloads a file from URL to local targetPath.
func (ins *Installer) DownloadFile(url, targetPath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "doom-configs/2.0")

	resp, err := ins.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// InstallUZDoom downloads and installs UZDoom for current platform.
func (ins *Installer) InstallUZDoom() error {
	fmt.Fprintf(ins.Out, "Installing UZDoom...\n")
	var pattern, fallback, targetName string

	switch runtime.GOOS {
	case "darwin":
		targetName = "uzdoom"
		if runtime.GOARCH == "arm64" {
			pattern = `.*(macos|mac|darwin).*uzdoom.*arm64.*(\.dmg|\.zip)`
			fallback = "https://github.com/UZDoom/UZDoom/releases/download/5.0.0/MacOS-UZDoom-Release-ARM64.dmg"
		} else {
			pattern = `.*(macos|mac|darwin).*uzdoom.*(x86_64|x64|intel).*(\.dmg|\.zip)`
			fallback = "https://github.com/UZDoom/UZDoom/releases/download/5.0.0/MacOS-UZDoom-Release-ARM64.dmg"
		}
	case "windows":
		targetName = "uzdoom.exe"
		pattern = `Windows.*UZDoom.*\.zip`
		fallback = "https://github.com/UZDoom/UZDoom/releases/download/5.0.0/Windows-UZDoom-Release-x86_64.zip"
	default:
		targetName = "uzdoom"
		pattern = `Linux.*UZDoom.*\.AppImage`
		fallback = "https://github.com/UZDoom/UZDoom/releases/download/5.0.0/Linux-UZDoom-Release-x86_64.AppImage"
	}

	url := ins.ResolveLatestGitHubURL("UZDoom/UZDoom", pattern, fallback)
	return ins.deployAsset(targetName, url)
}

// InstallDSDA downloads and installs DSDA-Doom for current platform.
func (ins *Installer) InstallDSDA() error {
	fmt.Fprintf(ins.Out, "Installing DSDA-Doom...\n")
	var pattern, fallback, targetName string
	const fallbackBase = "https://github.com/kraflab/dsda-doom/releases/download/v0.29.4/"

	switch runtime.GOOS {
	case "darwin":
		targetName = "dsda-doom"
		if runtime.GOARCH == "arm64" {
			pattern = `dsda-doom-.*-mac-arm64\.zip`
			fallback = fallbackBase + "dsda-doom-0.29.4-mac-arm64.zip"
		} else {
			pattern = `dsda-doom-.*-mac-x86_64\.zip`
			fallback = fallbackBase + "dsda-doom-0.29.4-mac-x86_64.zip"
		}
	case "windows":
		targetName = "dsda-doom.exe"
		pattern = `dsda-doom-.*-win-x64\.zip`
		fallback = fallbackBase + "dsda-doom-0.29.4-win-x64.zip"
	default:
		targetName = "dsda-doom"
		pattern = `dsda-doom-.*-linux-x86_64\.appimage`
		fallback = fallbackBase + "dsda-doom-0.29.4-linux-x86_64.appimage"
	}

	url := ins.ResolveLatestGitHubURL("kraflab/dsda-doom", pattern, fallback)
	return ins.deployAsset(targetName, url)
}

// InstallDoomRunner downloads and installs DoomRunner for current platform.
func (ins *Installer) InstallDoomRunner() error {
	fmt.Fprintf(ins.Out, "Installing DoomRunner...\n")
	var pattern, fallback, targetName string
	const fallbackBase = "https://github.com/Youda008/DoomRunner/releases/download/v1.9.2/"

	switch runtime.GOOS {
	case "darwin":
		targetName = "doomrunner"
		if runtime.GOARCH == "arm64" {
			pattern = `DoomRunner-.*-MacOS-arm64\.dmg`
			fallback = fallbackBase + "DoomRunner-1.9.2-MacOS-arm64.dmg"
		} else {
			pattern = `DoomRunner-.*-MacOS-x86_64\.dmg`
			fallback = fallbackBase + "DoomRunner-1.9.2-MacOS-x86_64.dmg"
		}
	case "windows":
		targetName = "DoomRunner.exe"
		pattern = `DoomRunner-.*-Windows-x64\.zip`
		fallback = fallbackBase + "DoomRunner-1.9.2-Windows-x64.zip"
	default:
		targetName = "doomrunner"
		pattern = `DoomRunner-.*-Linux-x86_64\.AppImage`
		fallback = fallbackBase + "DoomRunner-1.9.2-Linux-x86_64.AppImage"
	}

	url := ins.ResolveLatestGitHubURL("Youda008/DoomRunner", pattern, fallback)
	return ins.deployAsset(targetName, url)
}

// InstallAll installs UZDoom, DSDA-Doom, and DoomRunner.
func (ins *Installer) InstallAll() error {
	if err := ins.InstallUZDoom(); err != nil {
		return err
	}
	if err := ins.InstallDSDA(); err != nil {
		return err
	}
	return ins.InstallDoomRunner()
}

func (ins *Installer) deployAsset(targetName, url string) error {
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
	if err := ins.DownloadFile(url, tmpFile); err != nil {
		return fmt.Errorf("download error: %w", err)
	}

	lowerURL := strings.ToLower(url)
	if strings.HasSuffix(lowerURL, ".zip") {
		return ins.extractZipBinary(tmpFile, targetName, dest)
	} else if strings.HasSuffix(lowerURL, ".dmg") && runtime.GOOS == "darwin" {
		return ins.extractDMGBinary(tmpFile, targetName, dest)
	}

	// Raw binary or AppImage
	if err := copyBinaryFile(tmpFile, dest); err != nil {
		return err
	}
	_ = os.Chmod(dest, 0755)
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

	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(zf.Name))
		baseClean := strings.TrimSuffix(base, ".exe")
		if baseClean == cleanTarget || strings.HasPrefix(baseClean, cleanTarget) {
			binMatch = zf
			break
		}
	}

	if binMatch == nil && len(zr.File) > 0 {
		binMatch = zr.File[0]
	}

	if binMatch == nil {
		return fmt.Errorf("binary %s not found in zip", targetName)
	}

	if err := extractZipEntry(binMatch, dest); err != nil {
		return err
	}
	_ = os.Chmod(dest, 0755)

	// Also extract companion files (.wad, .pk3, dlls) in the same directory into BinDir
	binParent := filepath.Dir(binMatch.Name)
	cleanBinDir := filepath.Clean(ins.BinDir)
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || zf == binMatch {
			continue
		}
		if filepath.Dir(zf.Name) == binParent {
			base := filepath.Base(filepath.Clean(zf.Name))
			if base == "." || base == ".." || strings.Contains(base, "..") {
				continue
			}
			lower := strings.ToLower(base)
			if strings.HasSuffix(lower, ".wad") ||
				strings.HasSuffix(lower, ".pk3") ||
				strings.HasSuffix(lower, ".dll") ||
				strings.HasSuffix(lower, ".so") ||
				strings.HasSuffix(lower, ".dylib") {
				companionDest := filepath.Join(cleanBinDir, base)
				if !strings.HasPrefix(companionDest, cleanBinDir+string(filepath.Separator)) {
					continue
				}
				_ = extractZipEntry(zf, companionDest)
			}
		}
	}

	fmt.Fprintf(ins.Out, "✓ Installed %s to %s\n\n", targetName, dest)
	return nil
}

func (ins *Installer) extractDMGBinary(dmgPath, targetName, dest string) error {
	mountPoint, err := os.MkdirTemp("", "doom_mount_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mountPoint)

	// Attach DMG
	attachCmd := exec.Command("hdiutil", "attach", dmgPath, "-mountpoint", mountPoint, "-nobrowse", "-quiet")
	if err := attachCmd.Run(); err != nil {
		return fmt.Errorf("failed to attach DMG: %w", err)
	}
	defer func() {
		_ = exec.Command("hdiutil", "detach", mountPoint, "-quiet").Run()
	}()

	// Find .app bundle
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

	home, _ := os.UserHomeDir()
	appsDir := filepath.Join(home, "Applications")
	_ = os.MkdirAll(appsDir, 0755)
	appDest := filepath.Join(appsDir, filepath.Base(appEntry))

	_ = exec.Command("rm", "-rf", appDest).Run()
	if err := exec.Command("cp", "-R", appEntry, appsDir+"/").Run(); err != nil {
		return fmt.Errorf("failed to copy .app to %s: %w", appsDir, err)
	}

	// Locate binary inside Contents/MacOS
	macOSDir := filepath.Join(appDest, "Contents", "MacOS")
	macEntries, err := os.ReadDir(macOSDir)
	if err != nil || len(macEntries) == 0 {
		return fmt.Errorf("executable not found in %s", macOSDir)
	}

	appBin := filepath.Join(macOSDir, macEntries[0].Name())

	// Create launcher wrapper script in dest
	wrapper := fmt.Sprintf("#!/bin/sh\nexec %q \"$@\"\n", appBin)
	if err := os.WriteFile(dest, []byte(wrapper), 0755); err != nil {
		return err
	}

	fmt.Fprintf(ins.Out, "✓ Installed %s (.app wrapper) to %s\n\n", targetName, dest)
	return nil
}

func extractZipEntry(zf *zip.File, destPath string) error {
	cleanDest := filepath.Clean(destPath)
	if err := os.MkdirAll(filepath.Dir(cleanDest), 0755); err != nil {
		return err
	}

	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(cleanDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func copyBinaryFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
