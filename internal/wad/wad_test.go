package wad

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
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

func buildTestPWADWithDehacked(dehPayload string) []byte {
	data := []byte(dehPayload)
	var out bytes.Buffer
	_, _ = out.WriteString("PWAD")
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], 1)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(12+len(data)))
	_, _ = out.Write(hdr[:])
	_, _ = out.Write(data)
	var entry [16]byte
	binary.LittleEndian.PutUint32(entry[0:4], 12)
	binary.LittleEndian.PutUint32(entry[4:8], uint32(len(data)))
	copy(entry[8:16], "DEHACKED")
	_, _ = out.Write(entry[:])
	return out.Bytes()
}

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

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for _, entry := range []struct {
		name    string
		content string
	}{
		{name: "Credits.txt", content: "credits-only"},
		{name: "Eviternity II.wad", content: "content-eviternity"},
		{name: "eviternityii.txt", content: "eviternity-ii-readme"},
		{name: "sub/SUNDER_V2512.wad", content: "content-sunder"},
		{name: "custom_short.wad", content: "content-custom"},
		{name: "patch_file.bex", content: "content-bex"},
	} {
		f, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipBytes := buf.Bytes()

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
		"Eviternity II.wad",
		"sub/Sunder 2512.wad",
		"custom_v2.wad",
		"patch.bex",
	}, customPreset)
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}

	c1, err := os.ReadFile(filepath.Join(wadsDir, "Eviternity II.wad"))
	if err != nil || string(c1) != "content-eviternity" {
		t.Errorf("Eviternity II.wad content mismatch: %s (err: %v)", string(c1), err)
	}

	c2, err := os.ReadFile(filepath.Join(wadsDir, "sub", "Sunder 2512.wad"))
	if err != nil || string(c2) != "content-sunder" {
		t.Errorf("sub/Sunder 2512.wad content mismatch: %s (err: %v)", string(c2), err)
	}

	cCustom, err := os.ReadFile(filepath.Join(wadsDir, "custom_v2.wad"))
	if err != nil || string(cCustom) != "content-custom" {
		t.Errorf("custom_v2.wad content mismatch: %s (err: %v)", string(cCustom), err)
	}

	c3, err := os.ReadFile(filepath.Join(wadsDir, "patch.bex"))
	if err != nil || string(c3) != "content-bex" {
		t.Errorf("patch.bex content mismatch: %s (err: %v)", string(c3), err)
	}

	// Verify eviternityii.txt was preferred over Credits.txt
	cReadme, err := os.ReadFile(filepath.Join(wadsDir, "eviternityii.txt"))
	if err != nil || string(cReadme) != "eviternity-ii-readme" {
		t.Errorf("eviternityii.txt content mismatch: %s (err: %v)", string(cReadme), err)
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "Credits.txt")); !os.IsNotExist(err) {
		t.Errorf("expected Credits.txt not to be extracted when eviternityii.txt matches")
	}

	// Verify <stem>ReadMe*.txt is preferred over <stem>_Changelog.txt when both share the stem prefix
	sunderBuf := new(bytes.Buffer)
	sunderZW := zip.NewWriter(sunderBuf)
	for _, entry := range []struct {
		name    string
		content string
	}{
		{name: "Sunder_Changelog.txt", content: "changelog-only"},
		{name: "Sunder 2512.wad", content: "sunder-wad"},
		{name: "SunderReadMeV2.TXT", content: "sunder-readme-v2"},
	} {
		f, err := sunderZW.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := sunderZW.Close(); err != nil {
		t.Fatal(err)
	}
	sunderZipPath := filepath.Join(tmpDir, "sunder.zip")
	if err := os.WriteFile(sunderZipPath, sunderBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sunderWadsDir := filepath.Join(tmpDir, "sunder_wads")
	sunderDL := NewDownloader(sunderWadsDir, false, nil)
	if err := sunderDL.extractFromZip(
		sunderZipPath,
		int64(sunderBuf.Len()),
		[]string{"Sunder 2512.wad"},
		preset.Preset{Name: "Sunder"},
	); err != nil {
		t.Fatalf("unexpected sunder extract error: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(sunderWadsDir, "SunderReadMeV2.TXT")); err != nil ||
		string(got) != "sunder-readme-v2" {
		t.Errorf(
			"expected SunderReadMeV2.TXT to be extracted over Sunder_Changelog.txt, got %q (err=%v)",
			string(got),
			err,
		)
	}
	if _, err := os.Stat(filepath.Join(sunderWadsDir, "Sunder_Changelog.txt")); !os.IsNotExist(err) {
		t.Errorf("expected Sunder_Changelog.txt not to be extracted when SunderReadMeV2.TXT is present")
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
		case "/doomzero.wad":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(buildTestPWADWithDehacked("Patch File for DeHackEd v3.0"))
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

	t.Run("direct wad download extracts embedded DEHACKED lump for expected deh file", func(t *testing.T) {
		t.Parallel()
		wadsDir := filepath.Join(t.TempDir(), "wads")
		dl := NewDownloader(wadsDir, false, nil)
		p := preset.Preset{
			Name:   "Doom Zero",
			Engine: "dsda-doom",
			IWAD:   "DOOM2.WAD",
			Files: []preset.FileSpec{
				{Name: "DOOMZERO.DEH"},
				{Name: "DoomZero.wad"},
			},
			Sources: []preset.Source{{URL: fmt.Sprintf("%s/doomzero.wad", server.URL)}},
		}
		if err := dl.DownloadPreset(p); err != nil {
			t.Fatalf("direct .wad with embedded DEHACKED failed: %v", err)
		}
		if !dl.IsPresetInstalled(p) {
			t.Fatalf("expected Doom Zero preset to be installed")
		}
		gotDeh, err := os.ReadFile(filepath.Join(wadsDir, "DOOMZERO.DEH"))
		if err != nil || string(gotDeh) != "Patch File for DeHackEd v3.0" {
			t.Fatalf("extracted DOOMZERO.DEH mismatch: %q (err=%v)", string(gotDeh), err)
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
