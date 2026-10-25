package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanRelAndSafeJoin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	tests := []struct {
		name    string
		rel     string
		wantErr bool
	}{
		{name: "plain filename", rel: "AV.WAD", wantErr: false},
		{name: "single subdir forward slash", rel: "sub/foo.wad", wantErr: false},
		{name: "single subdir backslash", rel: `sub\foo.wad`, wantErr: false},
		{name: "nested subdir", rel: "packs/btsx/btsx_e1a.wad", wantErr: false},
		{name: "empty path", rel: "", wantErr: true},
		{name: "dot", rel: ".", wantErr: true},
		{name: "dotdot", rel: "..", wantErr: true},
		{name: "parent traversal", rel: "../evil.wad", wantErr: true},
		{name: "parent traversal backslash", rel: `..\evil.wad`, wantErr: true},
		{name: "nested parent traversal", rel: "sub/../../evil.wad", wantErr: true},
		{name: "leading slash", rel: "/etc/passwd", wantErr: true},
		{name: "leading backslash", rel: `\Windows\System32`, wantErr: true},
		{name: "UNC path", rel: `\\server\share\foo.wad`, wantErr: true},
		{name: "drive letter", rel: `C:\doom\foo.wad`, wantErr: true},
		{name: "drive relative", rel: `C:foo.wad`, wantErr: true},
		{name: "double slash", rel: "sub//foo.wad", wantErr: true},
		{name: "NUL byte", rel: "foo\x00.wad", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			joined, err := SafeJoin(root, tc.rel)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("SafeJoin(%q) succeeded with %q, want error", tc.rel, joined)
				}
				return
			}
			if err != nil {
				t.Fatalf("SafeJoin(%q) unexpected error: %v", tc.rel, err)
			}
			if !strings.HasPrefix(joined, filepath.Clean(root)+string(filepath.Separator)) {
				t.Fatalf("SafeJoin(%q) = %q, outside root %q", tc.rel, joined, root)
			}
		})
	}
}

func TestWriteFileAtomicAndBackupsNeverClobber(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "config.json")

	bak1, err := WriteFileAtomic(target, []byte("v1\n"), 0o644)
	if err != nil {
		t.Fatalf("first WriteFileAtomic: %v", err)
	}
	if bak1 != "" {
		t.Fatalf("expected no backup on initial create, got %q", bak1)
	}

	bak2, err := WriteFileAtomic(target, []byte("v2\n"), 0o644)
	if err != nil {
		t.Fatalf("second WriteFileAtomic: %v", err)
	}
	if bak2 == "" {
		t.Fatal("expected backup on second write")
	}

	bak3, err := WriteFileAtomic(target, []byte("v3\n"), 0o644)
	if err != nil {
		t.Fatalf("third WriteFileAtomic: %v", err)
	}
	if bak3 == "" || bak3 == bak2 {
		t.Fatalf("expected distinct backup paths, got %q and %q", bak2, bak3)
	}

	got1, err := os.ReadFile(bak2)
	if err != nil || string(got1) != "v1\n" {
		t.Fatalf("bak2 content = %q (%v), want v1", string(got1), err)
	}
	got2, err := os.ReadFile(bak3)
	if err != nil || string(got2) != "v2\n" {
		t.Fatalf("bak3 content = %q (%v), want v2", string(got2), err)
	}
	gotCur, err := os.ReadFile(target)
	if err != nil || string(gotCur) != "v3\n" {
		t.Fatalf("target content = %q (%v), want v3", string(gotCur), err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".doom-tmp-") {
			t.Fatalf("leftover temp file %q", entry.Name())
		}
	}
}

func TestCopyFileAtomic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.wad")
	dst := filepath.Join(dir, "dst.wad")

	if err := os.WriteFile(src, []byte("DOOM1"), 0o644); err != nil {
		t.Fatal(err)
	}
	bak1, err := CopyFileAtomic(src, dst, 0o644)
	if err != nil || bak1 != "" {
		t.Fatalf("first CopyFileAtomic: bak=%q err=%v", bak1, err)
	}

	if err := os.WriteFile(src, []byte("DOOM2"), 0o644); err != nil {
		t.Fatal(err)
	}
	bak2, err := CopyFileAtomic(src, dst, 0o644)
	if err != nil || bak2 == "" {
		t.Fatalf("second CopyFileAtomic: bak=%q err=%v", bak2, err)
	}
	gotBak, err := os.ReadFile(bak2)
	if err != nil || string(gotBak) != "DOOM1" {
		t.Fatalf("backup content = %q (%v), want DOOM1", string(gotBak), err)
	}
	gotDst, err := os.ReadFile(dst)
	if err != nil || string(gotDst) != "DOOM2" {
		t.Fatalf("dst content = %q (%v), want DOOM2", string(gotDst), err)
	}
}

func TestBackupRetentionAndRemoveBackups(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "entity.json")

	// 1st write creates target, writes 2..7 create 6 backups; only the 3 newest should remain.
	var bakPaths []string
	for i := 1; i <= 7; i++ {
		bak, err := WriteFileAtomic(target, []byte{byte('0' + i)}, 0o644)
		if err != nil {
			t.Fatalf("WriteFileAtomic #%d: %v", i, err)
		}
		if bak != "" {
			bakPaths = append(bakPaths, bak)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var remaining []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "entity.json.bak.") {
			remaining = append(remaining, filepath.Join(dir, entry.Name()))
		}
	}
	if len(remaining) != MaxBackupsPerFile {
		t.Fatalf("retained backups = %d (%v), want %d", len(remaining), remaining, MaxBackupsPerFile)
	}
	// Verify the 3 newest backup paths (representing versions 4, 5, 6) are retained.
	for idx, wantBak := range bakPaths[len(bakPaths)-MaxBackupsPerFile:] {
		data, err := os.ReadFile(wantBak)
		if err != nil {
			t.Fatalf("expected newest backup %q to exist: %v", wantBak, err)
		}
		wantByte := byte('0' + 4 + idx)
		if len(data) != 1 || data[0] != wantByte {
			t.Errorf("backup %s content = %q, want %c", wantBak, string(data), wantByte)
		}
	}

	RemoveBackups(target)
	entriesAfter, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir after RemoveBackups: %v", err)
	}
	for _, entry := range entriesAfter {
		if strings.HasPrefix(entry.Name(), "entity.json.bak.") {
			t.Errorf("leftover backup after RemoveBackups: %s", entry.Name())
		}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated read error")
}

func TestDownloadVerifiedAndFailedReaderNoSpuriousBackup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "payload.bin")

	// 1. Seed initial target file
	if _, err := WriteFileAtomic(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. WriteReaderAtomic with failing reader must not overwrite target or create a spurious backup
	if _, bak, err := WriteReaderAtomic(target, failingReader{}, 0o644); err == nil || bak != "" {
		t.Fatalf("expected WriteReaderAtomic error and empty backup on failingReader, got bak=%q err=%v", bak, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "payload.bin" {
		t.Fatalf("unexpected files after failed WriteReaderAtomic: %v", entries)
	}

	// 3. DownloadVerified verifies User-Agent, SHA-256 mismatch cleanup, and valid overwrite
	payload := []byte("verified-doom-payload")
	sum := sha256.Sum256(payload)
	validHex := hex.EncodeToString(sum[:])

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	// SHA-256 mismatch must leave original file untouched and create no backup or leftover temp file
	if _, err := DownloadVerified(srv.Client(), srv.URL, target, "000000", 0o644); err == nil ||
		!strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch error, got %v", err)
	}
	if gotUA != UserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, UserAgent)
	}
	entriesAfterMismatch, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesAfterMismatch) != 1 || entriesAfterMismatch[0].Name() != "payload.bin" {
		t.Fatalf("expected only payload.bin after sha256 mismatch, got %v", entriesAfterMismatch)
	}

	// Valid SHA-256 overwrites target and creates a backup of "original"
	n, err := DownloadVerified(srv.Client(), srv.URL, target, validHex, 0o644)
	if err != nil || n != int64(len(payload)) {
		t.Fatalf("DownloadVerified valid: n=%d err=%v", n, err)
	}
	gotData, err := os.ReadFile(target)
	if err != nil || string(gotData) != string(payload) {
		t.Fatalf("target content = %q (%v), want %q", string(gotData), err, string(payload))
	}
}
