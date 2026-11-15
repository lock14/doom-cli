package engine

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lock14/doom-cli/internal/fsutil"
)

var rePlistBundleExec = regexp.MustCompile(
	`(?s)<key>\s*CFBundleExecutable\s*</key>\s*<string>\s*([^<\r\n]+?)\s*</string>`,
)

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

func findAppBundlePrefix(slashPath string) (string, string, bool) {
	if strings.Contains(slashPath, "..") {
		return "", "", false
	}
	parts := strings.Split(slashPath, "/")
	for i, part := range parts {
		if strings.Contains(part, "..") || strings.ContainsAny(part, `/\`) {
			return "", "", false
		}
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
	if strings.Contains(appBundleName, "..") || strings.ContainsAny(appBundleName, `/\`) {
		return fmt.Errorf("illegal .app bundle name %q", appBundleName)
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

	stageParent, err := os.MkdirTemp(appsDir, ".doom-app-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageParent)

	stagedApp, err := fsutil.SafeJoin(stageParent, appBundleName)
	if err != nil {
		return fmt.Errorf("illegal staged .app bundle path %q: %w", appBundleName, err)
	}
	if err := os.MkdirAll(stagedApp, 0o755); err != nil {
		return err
	}

	appRoot, err := os.OpenRoot(stagedApp)
	if err != nil {
		return err
	}
	defer appRoot.Close()

	var symlinks []*zip.File
	var symlinkRels []string

	// Pass 1: extract regular files inside os.Root before creating any symlinks.
	for _, zf := range zr.File {
		slashName := zipEntrySlashName(zf)
		if !strings.HasPrefix(slashName, appPrefix) {
			continue
		}
		relInApp := strings.TrimPrefix(slashName, appPrefix)
		if relInApp == "" || strings.HasSuffix(relInApp, "/") || zf.FileInfo().IsDir() {
			continue
		}
		cleanRel, relErr := fsutil.CleanRel(relInApp)
		if relErr != nil {
			return fmt.Errorf("unsafe path in .app archive %q: %w", zf.Name, relErr)
		}
		if isZipSymlink(zf) {
			symlinks = append(symlinks, zf)
			symlinkRels = append(symlinkRels, cleanRel)
			continue
		}
		perm := os.FileMode(0o644)
		if zf.Mode()&0o111 != 0 || strings.HasPrefix(relInApp, "Contents/MacOS/") ||
			strings.HasSuffix(strings.ToLower(relInApp), ".dylib") {
			perm = 0o755
		}
		if err := extractRootZipEntry(appRoot, zf, cleanRel, perm); err != nil {
			return err
		}
	}

	// Pass 2: create validated bundle symlinks inside os.Root.
	if len(symlinks) > 0 {
		realRoot, evalErr := filepath.EvalSymlinks(stagedApp)
		if evalErr != nil {
			return evalErr
		}
		for i, zf := range symlinks {
			if err := extractSafeBundleSymlink(appRoot, realRoot, stagedApp, zf, symlinkRels[i]); err != nil {
				return err
			}
		}
		// Pass 3: verify every symlink chain resolves strictly inside os.Root.
		for i, cleanRel := range symlinkRels {
			if _, statErr := appRoot.Stat(cleanRel); statErr != nil {
				return fmt.Errorf(
					"symlink %q resolves outside or missing in .app bundle: %w",
					symlinks[i].Name, statErr,
				)
			}
		}
	}

	if err := appRoot.Close(); err != nil {
		return err
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

func extractRootZipEntry(appRoot *os.Root, zf *zip.File, cleanRel string, perm os.FileMode) error {
	if strings.Contains(cleanRel, "..") {
		return fmt.Errorf("unsafe path in .app archive %q", zf.Name)
	}
	parent := filepath.Dir(cleanRel)
	if parent != "." && parent != "" {
		if err := appRoot.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}

	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := appRoot.OpenFile(cleanRel, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	limited := &io.LimitedReader{R: rc, N: fsutil.MaxExtractFileBytes + 1}
	written, copyErr := io.Copy(outFile, limited)
	closeErr := outFile.Close()
	if copyErr != nil {
		_ = appRoot.Remove(cleanRel)
		return copyErr
	}
	if written > fsutil.MaxExtractFileBytes {
		_ = appRoot.Remove(cleanRel)
		return fmt.Errorf("archive entry %q exceeds maximum size (%d bytes)", zf.Name, fsutil.MaxExtractFileBytes)
	}
	return closeErr
}

func extractSafeBundleSymlink(
	appRoot *os.Root,
	realRoot, bundleRoot string,
	zf *zip.File,
	cleanRel string,
) error {
	if strings.Contains(cleanRel, "..") {
		return fmt.Errorf("unsafe symlink path in .app archive %q", zf.Name)
	}
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
	slashTarget := strings.ReplaceAll(target, `\`, `/`)
	if target == "" || strings.ContainsRune(target, 0) || filepath.IsAbs(target) ||
		strings.HasPrefix(slashTarget, "/") || (len(slashTarget) >= 2 && slashTarget[1] == ':') {
		return fmt.Errorf("unsafe symlink target %q in archive entry %q", target, zf.Name)
	}

	parent := filepath.Dir(cleanRel)
	if parent != "." && parent != "" {
		if err := appRoot.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}

	realLinkDir, err := filepath.EvalSymlinks(filepath.Join(bundleRoot, parent))
	if err != nil {
		return err
	}
	relDir, err := filepath.Rel(realRoot, realLinkDir)
	if err != nil || relDir == ".." || strings.HasPrefix(filepath.Clean(relDir), "..") {
		return fmt.Errorf("symlink parent %q escapes .app bundle", zf.Name)
	}

	resolvedTarget := filepath.Clean(filepath.Join(realLinkDir, filepath.FromSlash(slashTarget)))
	relTarget, err := filepath.Rel(realRoot, resolvedTarget)
	if err != nil || relTarget == ".." || strings.HasPrefix(filepath.Clean(relTarget), "..") {
		return fmt.Errorf("symlink %q target %q escapes .app bundle", zf.Name, target)
	}

	_ = appRoot.Remove(cleanRel)
	return appRoot.Symlink(filepath.FromSlash(slashTarget), cleanRel)
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

	stagedApp, err := fsutil.SafeJoin(stageParent, appBundleName)
	if err != nil {
		return fmt.Errorf("illegal staged .app bundle path %q: %w", appBundleName, err)
	}
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
