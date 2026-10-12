package fsutil

import (
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
