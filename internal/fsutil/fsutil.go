// Package fsutil provides hardened filesystem helpers for atomic writes,
// non-destructive timestamped backups, and traversal-safe relative path joins.
package fsutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MaxBackupsPerFile is the maximum number of timestamped backup files retained per target file.
const MaxBackupsPerFile = 3

// UserAgent is the HTTP User-Agent header sent on all outbound downloads.
const UserAgent = "doom-cli/2.0"

// BackupFile creates a non-destructive timestamped backup (.bak.<YYYYMMDDHHMMSS>)
// if path already exists, pruning older backup siblings so at most MaxBackupsPerFile
// backups are retained. If a backup with the same second-resolution timestamp
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
	backupPath := nextBackupPath(path, ts)

	if _, _, err := writeReaderTempAndRename(backupPath, in, info.Mode().Perm(), false, nil); err != nil {
		return "", fmt.Errorf("writing backup %s: %w", backupPath, err)
	}
	pruneBackups(path, MaxBackupsPerFile)
	return backupPath, nil
}

func nextBackupPath(path, ts string) string {
	dir := filepath.Dir(path)
	baseName := filepath.Base(path) + ".bak." + ts
	entries, err := os.ReadDir(dir)
	if err != nil {
		return filepath.Join(dir, baseName)
	}
	maxSeq := -1
	for _, entry := range entries {
		name := entry.Name()
		if name == baseName {
			if maxSeq < 0 {
				maxSeq = 0
			}
			continue
		}
		if strings.HasPrefix(name, baseName+"-") {
			suffix := strings.TrimPrefix(name, baseName+"-")
			if seq, convErr := strconv.Atoi(suffix); convErr == nil && seq > maxSeq {
				maxSeq = seq
			}
		}
	}
	if maxSeq < 0 {
		return filepath.Join(dir, baseName)
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d", baseName, maxSeq+1))
}

type backupEntry struct {
	name string
	ts   string
	seq  int
}

func pruneBackups(path string, keep int) {
	if keep <= 0 || strings.TrimSpace(path) == "" {
		return
	}
	dir := filepath.Dir(path)
	prefix := filepath.Base(path) + ".bak."
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var backups []backupEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		suffix := strings.TrimPrefix(entry.Name(), prefix)
		tsPart := suffix
		seq := 0
		if before, after, ok := strings.Cut(suffix, "-"); ok {
			if parsed, convErr := strconv.Atoi(after); convErr == nil {
				tsPart = before
				seq = parsed
			}
		}
		backups = append(backups, backupEntry{
			name: entry.Name(),
			ts:   tsPart,
			seq:  seq,
		})
	}
	if len(backups) <= keep {
		return
	}
	sort.Slice(backups, func(i, j int) bool {
		if backups[i].ts != backups[j].ts {
			return backups[i].ts < backups[j].ts
		}
		if backups[i].seq != backups[j].seq {
			return backups[i].seq < backups[j].seq
		}
		return backups[i].name < backups[j].name
	})
	for _, old := range backups[:len(backups)-keep] {
		_ = os.Remove(filepath.Join(dir, old.name))
	}
}

// RemoveBackups deletes all timestamped backup files (<path>.bak.*) for path.
func RemoveBackups(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	dir := filepath.Dir(path)
	prefix := filepath.Base(path) + ".bak."
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
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
	return writeReaderTempAndRename(path, r, perm, true, nil)
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

func writeReaderTempAndRename(
	path string,
	r io.Reader,
	perm os.FileMode,
	createBackup bool,
	validate func() error,
) (int64, string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, "", fmt.Errorf("creating directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".doom-tmp-*")
	if err != nil {
		return 0, "", err
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
		return written, "", err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return written, "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return written, "", err
	}
	if err := tmp.Close(); err != nil {
		return written, "", err
	}

	if validate != nil {
		if err := validate(); err != nil {
			return written, "", err
		}
	}

	var backupPath string
	if createBackup {
		backupPath, err = BackupFile(path)
		if err != nil {
			return written, "", err
		}
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return written, backupPath, err
	}
	cleanup = false
	return written, backupPath, nil
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

// DownloadVerified fetches url using client (or http.DefaultClient if nil) with
// UserAgent, verifies an HTTP 200 OK status and optional expectedSHA256 hex
// digest, and atomically writes the verified payload to targetPath with perm.
func DownloadVerified(
	client *http.Client,
	url, targetPath, expectedSHA256 string,
	perm os.FileMode,
) (int64, error) {
	if targetPath == "" {
		return 0, errors.New("empty target path")
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	hasher := sha256.New()
	cleanSum := strings.TrimSpace(expectedSHA256)
	written, _, err := writeReaderTempAndRename(
		targetPath,
		io.TeeReader(resp.Body, hasher),
		perm,
		true,
		func() error {
			if cleanSum != "" {
				actualSum := hex.EncodeToString(hasher.Sum(nil))
				if !strings.EqualFold(actualSum, cleanSum) {
					return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", url, actualSum, cleanSum)
				}
			}
			return nil
		},
	)
	return written, err
}
