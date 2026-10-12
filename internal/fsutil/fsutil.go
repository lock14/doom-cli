// Package fsutil provides hardened filesystem helpers for atomic writes,
// non-destructive timestamped backups, and traversal-safe relative path joins.
package fsutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupFile creates a non-destructive timestamped backup (.bak.<YYYYMMDDHHMMSS>)
// if path already exists. If a backup with the same second-resolution timestamp
// already exists, a numeric suffix (-1, -2, ...) is appended so earlier backups
// are never clobbered. If path does not exist, BackupFile returns ("", nil).
func BackupFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("cannot back up directory %q", path)
	}

	in, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("reading %s for backup: %w", path, err)
	}
	defer in.Close()

	ts := time.Now().Format("20060102150405")
	baseBackup := fmt.Sprintf("%s.bak.%s", path, ts)
	backupPath := baseBackup
	for seq := 1; ; seq++ {
		if _, statErr := os.Stat(backupPath); os.IsNotExist(statErr) {
			break
		}
		backupPath = fmt.Sprintf("%s-%d", baseBackup, seq)
	}

	if _, err := writeReaderTempAndRename(backupPath, in, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("writing backup %s: %w", backupPath, err)
	}
	return backupPath, nil
}

// WriteFileAtomic writes data to path atomically via a temporary file in the
// same directory and os.Rename. If path already exists, a timestamped backup is
// created first via BackupFile and its path is returned.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) (string, error) {
	_, backupPath, err := WriteReaderAtomic(path, bytes.NewReader(data), perm)
	return backupPath, err
}

// WriteReaderAtomic streams r to path atomically via a temporary file in the
// same directory and os.Rename. If path already exists, a timestamped backup is
// created first via BackupFile and its path is returned alongside the byte count.
func WriteReaderAtomic(path string, r io.Reader, perm os.FileMode) (int64, string, error) {
	if path == "" {
		return 0, "", errors.New("empty file path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, "", fmt.Errorf("creating directory %s: %w", dir, err)
	}

	backupPath, err := BackupFile(path)
	if err != nil {
		return 0, "", err
	}

	written, err := writeReaderTempAndRename(path, r, perm)
	if err != nil {
		return written, backupPath, err
	}
	return written, backupPath, nil
}

// CopyFileAtomic copies src to dst atomically via WriteReaderAtomic, creating
// a non-clobbering timestamped backup if dst already exists.
func CopyFileAtomic(src, dst string, perm os.FileMode) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	_, backupPath, err := WriteReaderAtomic(dst, in, perm)
	return backupPath, err
}

func writeReaderTempAndRename(path string, r io.Reader, perm os.FileMode) (int64, error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".doom-tmp-*")
	if err != nil {
		return 0, err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	written, err := io.Copy(tmp, r)
	if err != nil {
		_ = tmp.Close()
		return written, err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return written, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return written, err
	}
	if err := tmp.Close(); err != nil {
		return written, err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return written, err
	}
	cleanup = false
	return written, nil
}

// CleanRel validates and normalizes a relative path (converting backslashes to
// forward slashes), rejecting empty strings, NUL bytes, leading slashes, Windows
// drive-letter prefixes, UNC prefixes, and any "." or ".." path segments.
func CleanRel(rel string) (string, error) {
	if rel == "" {
		return "", errors.New("empty relative path")
	}
	if strings.IndexByte(rel, 0) >= 0 {
		return "", fmt.Errorf("invalid NUL byte in path %q", rel)
	}
	norm := strings.ReplaceAll(rel, "\\", "/")
	if strings.HasPrefix(norm, "/") {
		return "", fmt.Errorf("absolute path %q is not allowed", rel)
	}
	if len(norm) >= 2 && norm[1] == ':' {
		return "", fmt.Errorf("drive-letter path %q is not allowed", rel)
	}
	parts := strings.Split(norm, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid relative path segment in %q", rel)
		}
		if strings.ContainsRune(part, ':') {
			return "", fmt.Errorf("invalid colon in path segment %q", rel)
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/"), nil
}

// SafeJoin joins a relative path under root after validating it with CleanRel
// and verifying that the resulting path remains strictly inside root.
func SafeJoin(root, rel string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("empty root directory")
	}
	cleanRel, err := CleanRel(rel)
	if err != nil {
		return "", err
	}
	cleanRoot := filepath.Clean(root)
	target := filepath.Clean(filepath.Join(cleanRoot, filepath.FromSlash(cleanRel)))
	prefix := cleanRoot + string(filepath.Separator)
	if !strings.HasPrefix(target, prefix) {
		return "", fmt.Errorf("path %q escapes root %q", rel, root)
	}
	return target, nil
}
