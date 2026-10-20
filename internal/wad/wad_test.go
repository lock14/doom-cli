package wad

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/preset"
)

func TestFilterExpectedFiles(t *testing.T) {
	t.Parallel()
	files := []preset.FileSpec{
		{Name: "map.wad"},
		{Name: "idkfa 2024.wad", Optional: true},
		{Name: "patch.deh"},
	}
	filtered := FilterExpectedFiles(files)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 files, got %d", len(filtered))
	}
	if filtered[0] != "map.wad" || filtered[1] != "patch.deh" {
		t.Errorf("unexpected filtered slice: %v", filtered)
	}
}

func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write content for %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed closing zip writer: %v", err)
	}
	return buf.Bytes()
}

func TestDownloader_ExtractFromZip(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	zipBytes := createTestZip(t, map[string]string{
		"Eviternity II.wad": "content-eviternity",
		"sub/gd.wad":        "content-gdturbo",
		"custom_short.wad":  "content-custom",
		"patch_file.bex":    "content-bex",
	})

	zipPath := filepath.Join(tmpDir, "test.zip")
	if err := os.WriteFile(zipPath, zipBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	wadsDir := filepath.Join(tmpDir, "wads")
	dl := NewDownloader(wadsDir, false, nil)

	customPreset := preset.Preset{
		Files: []preset.FileSpec{
			{Name: "custom_v2.wad", Aliases: []string{"custom_short.wad"}},
		},
	}

	err := dl.extractFromZip(zipPath, int64(len(zipBytes)), []string{
		"eviternityii.wad",
		"sub/gdturbo.wad",
		"custom_v2.wad",
		"patch.bex",
	}, customPreset)
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}

	c1, err := os.ReadFile(filepath.Join(wadsDir, "eviternityii.wad"))
	if err != nil || string(c1) != "content-eviternity" {
		t.Errorf("eviternityii.wad content mismatch: %s (err: %v)", string(c1), err)
	}

	c2, err := os.ReadFile(filepath.Join(wadsDir, "sub", "gdturbo.wad"))
	if err != nil || string(c2) != "content-gdturbo" {
		t.Errorf("sub/gdturbo.wad content mismatch: %s (err: %v)", string(c2), err)
	}

	cCustom, err := os.ReadFile(filepath.Join(wadsDir, "custom_v2.wad"))
	if err != nil || string(cCustom) != "content-custom" {
		t.Errorf("custom_v2.wad content mismatch: %s (err: %v)", string(cCustom), err)
	}

	c3, err := os.ReadFile(filepath.Join(wadsDir, "patch.bex"))
	if err != nil || string(c3) != "content-bex" {
		t.Errorf("patch.bex content mismatch: %s (err: %v)", string(c3), err)
	}
}

func TestDownloader_DownloadPresetAndSecurity(t *testing.T) {
	t.Parallel()
	zipBytes := createTestZip(t, map[string]string{
		"alienvendetta.wad": "av-content",
		"av.deh":            "av-deh",
	})
	zipSum := sha256.Sum256(zipBytes)
	zipHex := hex.EncodeToString(zipSum[:])

	pk3Bytes := createTestZip(t, map[string]string{
		"maps/map01.wad": "pk3-map-content",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/alienv.zip":
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBytes)
		case "/mod.pk3":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(pk3Bytes)
		case "/direct.wad":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("PWAD-DIRECT"))
		case "/archive.7z":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("7z-content"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	t.Run("zip download with valid sha256", func(t *testing.T) {
		t.Parallel()
		wadsDir := filepath.Join(t.TempDir(), "wads")
		dl := NewDownloader(wadsDir, false, nil)
		p := preset.Preset{
			Name:   "Alien Vendetta",
			Engine: "dsda-doom",
			IWAD:   "doom2.wad",
			Files:  []preset.FileSpec{{Name: "alienvendetta.wad"}, {Name: "av.deh"}},
			Sources: []preset.Source{
				{URL: fmt.Sprintf("%s/alienv.zip", server.URL), SHA256: zipHex},
			},
		}
		if err := dl.DownloadPreset(p); err != nil {
			t.Fatalf("DownloadPreset failed: %v", err)
		}
		if !dl.IsPresetInstalled(p) {
			t.Errorf("preset should be reported as installed")
		}
	})

	t.Run("sha256 mismatch rejected", func(t *testing.T) {
		t.Parallel()
		wadsDir := filepath.Join(t.TempDir(), "wads")
		dl := NewDownloader(wadsDir, false, nil)
		p := preset.Preset{
			Name:   "Bad Digest",
			Engine: "dsda-doom",
			IWAD:   "doom2.wad",
			Files:  []preset.FileSpec{{Name: "alienvendetta.wad"}},
			Sources: []preset.Source{
				{URL: fmt.Sprintf("%s/alienv.zip", server.URL), SHA256: strings.Repeat("0", 64)},
			},
		}
		if err := dl.DownloadPreset(p); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
			t.Fatalf("expected sha256 mismatch error, got: %v", err)
		}
	})

	t.Run("direct pk3 download saved as pk3 without unzipping", func(t *testing.T) {
		t.Parallel()
		wadsDir := filepath.Join(t.TempDir(), "wads")
		dl := NewDownloader(wadsDir, false, nil)
		p := preset.Preset{
			Name:    "Direct PK3",
			Engine:  "uzdoom",
			IWAD:    "doom2.wad",
			Files:   []preset.FileSpec{{Name: "mod.pk3"}},
			Sources: []preset.Source{{URL: fmt.Sprintf("%s/mod.pk3", server.URL)}},
		}
		if err := dl.DownloadPreset(p); err != nil {
			t.Fatalf("direct .pk3 DownloadPreset failed: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(wadsDir, "mod.pk3"))
		if err != nil || !bytes.Equal(got, pk3Bytes) {
			t.Fatalf("saved .pk3 mismatch (err=%v)", err)
		}
	})

	t.Run("7z extension rejected before saving as wad", func(t *testing.T) {
		t.Parallel()
		wadsDir := filepath.Join(t.TempDir(), "wads")
		dl := NewDownloader(wadsDir, false, nil)
		p := preset.Preset{
			Name:    "Unsupported 7z",
			Engine:  "dsda-doom",
			IWAD:    "doom2.wad",
			Files:   []preset.FileSpec{{Name: "pack.wad"}},
			Sources: []preset.Source{{URL: fmt.Sprintf("%s/archive.7z", server.URL)}},
		}
		if err := dl.DownloadPreset(p); err == nil || !strings.Contains(err.Error(), "unsupported archive format") {
			t.Fatalf("expected unsupported archive format error, got: %v", err)
		}
	})

	t.Run("direct save path traversal rejected", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()
		wadsDir := filepath.Join(baseDir, "wads")
		dl := NewDownloader(wadsDir, false, nil)
		p := preset.Preset{
			Name:    "Traversal",
			Engine:  "dsda-doom",
			IWAD:    "doom2.wad",
			Files:   []preset.FileSpec{{Name: "../evil.wad"}},
			Sources: []preset.Source{{URL: fmt.Sprintf("%s/direct.wad", server.URL)}},
		}
		if err := dl.DownloadPreset(p); err == nil {
			t.Fatal("expected path traversal in Files[0].Name to be rejected")
		}
		if _, err := os.Stat(filepath.Join(baseDir, "evil.wad")); !os.IsNotExist(err) {
			t.Fatalf("security violation: ../evil.wad was written outside wadsDir")
		}
	})
}

func TestInstallSoundFont(t *testing.T) {
	t.Parallel()
	fakeSF := []byte("RIFF-SF2-MOCK-CONTENT")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeSF)
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	installed, err := InstallSoundFontURL(server.URL, tmpDir, false, nil)
	if err != nil {
		t.Fatalf("InstallSoundFontURL failed: %v", err)
	}

	data, err := os.ReadFile(installed)
	if err != nil || !bytes.Equal(data, fakeSF) {
		t.Fatalf("installed soundfont content mismatch: %s (err: %v)", string(data), err)
	}

	installed2, err := InstallSoundFontURL(server.URL, tmpDir, false, nil)
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}
	if installed2 != installed {
		t.Errorf("expected path %s, got %s", installed, installed2)
	}
}

func TestDownloader_ExtractFromZip_ZipSlipProtection(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	zipBytes := createTestZip(t, map[string]string{
		"../escape.wad": "malicious-content",
		"../readme.txt": "malicious-readme",
		"normal.wad":    "normal-content",
	})

	zipPath := filepath.Join(tmpDir, "traversal.zip")
	if err := os.WriteFile(zipPath, zipBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	wadsDir := filepath.Join(tmpDir, "wads")
	dl := NewDownloader(wadsDir, false, nil)

	err := dl.extractFromZip(zipPath, int64(len(zipBytes)), []string{"normal.wad"})
	if err != nil {
		t.Fatalf("unexpected error extracting normal file: %v", err)
	}

	escapedPath := filepath.Join(tmpDir, "escape.wad")
	if _, err := os.Stat(escapedPath); !os.IsNotExist(err) {
		t.Errorf("security violation: escaped file was extracted to %s", escapedPath)
	}
	escapedTxt := filepath.Join(tmpDir, "readme.txt")
	if _, err := os.Stat(escapedTxt); !os.IsNotExist(err) {
		t.Errorf("security violation: escaped readme was extracted to %s", escapedTxt)
	}
}
