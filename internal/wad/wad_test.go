package wad

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lock14/doom-cli/internal/preset"
)

func TestFilterExpectedFiles(t *testing.T) {
	files := []string{"map.wad", "idkfa 2024.wad", "patch.deh"}
	filtered := FilterExpectedFiles(files)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 files, got %d", len(filtered))
	}
	if filtered[0] != "map.wad" || filtered[1] != "patch.deh" {
		t.Errorf("unexpected filtered slice: %v", filtered)
	}
}

func createTestZip(t *testing.T, files map[string]string) []byte {
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
	tmpDir, err := os.MkdirTemp("", "wad_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	zipBytes := createTestZip(t, map[string]string{
		"Eviternity II.wad": "content-eviternity",
		"sub/gd.wad":        "content-gdturbo",
		"patch_file.deh":    "content-deh",
	})

	zipPath := filepath.Join(tmpDir, "test.zip")
	if err := os.WriteFile(zipPath, zipBytes, 0644); err != nil {
		t.Fatal(err)
	}

	wadsDir := filepath.Join(tmpDir, "wads")
	dl := NewDownloader(wadsDir, false, nil)

	// 1. Normalized match: "eviternityii.wad" should match "Eviternity II.wad"
	// 2. Alias match: "gdturbo.wad" should match "gd.wad"
	// 3. Fallback match: "patch.deh" should match single .deh file "patch_file.deh"
	err = dl.extractFromZip(zipPath, int64(len(zipBytes)), []string{
		"eviternityii.wad",
		"gdturbo.wad",
		"patch.deh",
	})
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}

	c1, err := os.ReadFile(filepath.Join(wadsDir, "eviternityii.wad"))
	if err != nil || string(c1) != "content-eviternity" {
		t.Errorf("eviternityii.wad content mismatch: %s (err: %v)", string(c1), err)
	}

	c2, err := os.ReadFile(filepath.Join(wadsDir, "gdturbo.wad"))
	if err != nil || string(c2) != "content-gdturbo" {
		t.Errorf("gdturbo.wad content mismatch: %s (err: %v)", string(c2), err)
	}

	c3, err := os.ReadFile(filepath.Join(wadsDir, "patch.deh"))
	if err != nil || string(c3) != "content-deh" {
		t.Errorf("patch.deh content mismatch: %s (err: %v)", string(c3), err)
	}
}

func TestDownloader_DownloadPreset(t *testing.T) {
	zipBytes := createTestZip(t, map[string]string{
		"alienvendetta.wad": "av-content",
		"av.deh":            "av-deh",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/alienv.zip" {
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBytes)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "wad_dl_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	wadsDir := filepath.Join(tmpDir, "wads")
	dl := NewDownloader(wadsDir, false, nil)

	p := preset.Preset{
		Name:         "Alien Vendetta",
		Engine:       "dsda-doom",
		IWAD:         "doom2.wad",
		Mappacks:     []string{"alienvendetta.wad", "av.deh"},
		DownloadURLs: []string{fmt.Sprintf("%s/alienv.zip", server.URL)},
	}

	if err := dl.DownloadPreset(p); err != nil {
		t.Fatalf("DownloadPreset failed: %v", err)
	}

	if !dl.IsPresetInstalled(p) {
		t.Errorf("preset should be reported as installed")
	}

	// Verify idempotence
	if err := dl.DownloadPreset(p); err != nil {
		t.Fatalf("second DownloadPreset run failed: %v", err)
	}
}

func TestInstallSoundFont(t *testing.T) {
	fakeSF := []byte("RIFF-SF2-MOCK-CONTENT")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fakeSF)
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "sf_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	installed, err := InstallSoundFontURL(server.URL, tmpDir, false, nil)
	if err != nil {
		t.Fatalf("InstallSoundFontURL failed: %v", err)
	}

	data, err := os.ReadFile(installed)
	if err != nil || !bytes.Equal(data, fakeSF) {
		t.Fatalf("installed soundfont content mismatch: %s (err: %v)", string(data), err)
	}

	// Test idempotence when not forced
	installed2, err := InstallSoundFontURL(server.URL, tmpDir, false, nil)
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}
	if installed2 != installed {
		t.Errorf("expected path %s, got %s", installed, installed2)
	}
}

func TestDownloader_ExtractFromZip_ZipSlipProtection(t *testing.T) {
	tmpDir := t.TempDir()
	zipBytes := createTestZip(t, map[string]string{
		"../escape.wad": "malicious-content",
		"../readme.txt": "malicious-readme",
		"normal.wad":    "normal-content",
	})

	zipPath := filepath.Join(tmpDir, "traversal.zip")
	if err := os.WriteFile(zipPath, zipBytes, 0644); err != nil {
		t.Fatal(err)
	}

	wadsDir := filepath.Join(tmpDir, "wads")
	dl := NewDownloader(wadsDir, false, nil)

	// Attempt to extract entry that tries to escape directory
	err := dl.extractFromZip(zipPath, int64(len(zipBytes)), []string{"normal.wad"})
	if err != nil {
		t.Fatalf("unexpected error extracting normal file: %v", err)
	}

	// Verify that escaped file was not created outside wadsDir
	escapedPath := filepath.Join(tmpDir, "escape.wad")
	if _, err := os.Stat(escapedPath); !os.IsNotExist(err) {
		t.Errorf("security violation: escaped file was extracted to %s", escapedPath)
	}
	escapedTxt := filepath.Join(tmpDir, "readme.txt")
	if _, err := os.Stat(escapedTxt); !os.IsNotExist(err) {
		t.Errorf("security violation: escaped readme was extracted to %s", escapedTxt)
	}
}
