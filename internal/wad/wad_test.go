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

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/wadfmt"
)

func buildTestPWADWithDehacked(t *testing.T, dehPayload string) []byte {
	t.Helper()
	b, err := wadfmt.BuildBytes("PWAD", []wadfmt.Lump{
		{Name: "DEHACKED", Data: []byte(dehPayload)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFilterExpectedFiles(t *testing.T) {
	t.Parallel()
	files := []catalog.FileSpec{
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
		{name: "sub/sunder.wad", content: "content-sunder"},
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
	dl := NewDownloader([]string{wadsDir}, false, nil)

	customPreset := catalog.Preset{
		Files: []catalog.FileSpec{
			{Name: "custom_v2.wad", Aliases: []string{"custom_short.wad"}},
		},
	}

	err := dl.extractFromZip(zipPath, []string{
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
	sunderDL := NewDownloader([]string{sunderWadsDir}, false, nil)
	if err := sunderDL.extractFromZip(
		sunderZipPath,
		[]string{"Sunder 2512.wad"},
		catalog.Preset{Name: "Sunder"},
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
			_, _ = w.Write(buildTestPWADWithDehacked(t, "Patch File for DeHackEd v3.0"))
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
		dl := NewDownloader([]string{wadsDir}, false, nil)
		p := catalog.Preset{
			Name:   "Alien Vendetta",
			Engine: "dsda-doom",
			IWAD:   "doom2.wad",
			Files:  []catalog.FileSpec{{Name: "alienvendetta.wad"}, {Name: "av.deh"}},
			Sources: []catalog.Source{
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
		dl := NewDownloader([]string{wadsDir}, false, nil)
		p := catalog.Preset{
			Name:   "Bad Digest",
			Engine: "dsda-doom",
			IWAD:   "doom2.wad",
			Files:  []catalog.FileSpec{{Name: "alienvendetta.wad"}},
			Sources: []catalog.Source{
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
		dl := NewDownloader([]string{wadsDir}, false, nil)
		p := catalog.Preset{
			Name:    "Direct PK3",
			Engine:  "uzdoom",
			IWAD:    "doom2.wad",
			Files:   []catalog.FileSpec{{Name: "mod.pk3"}},
			Sources: []catalog.Source{{URL: fmt.Sprintf("%s/mod.pk3", server.URL)}},
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
		dl := NewDownloader([]string{wadsDir}, false, nil)
		p := catalog.Preset{
			Name:    "Unsupported 7z",
			Engine:  "dsda-doom",
			IWAD:    "doom2.wad",
			Files:   []catalog.FileSpec{{Name: "pack.wad"}},
			Sources: []catalog.Source{{URL: fmt.Sprintf("%s/archive.7z", server.URL)}},
		}
		if err := dl.DownloadPreset(p); err == nil || !strings.Contains(err.Error(), "unsupported archive format") {
			t.Fatalf("expected unsupported archive format error, got: %v", err)
		}
	})

	t.Run("direct wad download extracts embedded DEHACKED lump for expected deh file", func(t *testing.T) {
		t.Parallel()
		wadsDir := filepath.Join(t.TempDir(), "wads")
		dl := NewDownloader([]string{wadsDir}, false, nil)
		p := catalog.Preset{
			Name:   "Doom Zero",
			Engine: "dsda-doom",
			IWAD:   "DOOM2.WAD",
			Files: []catalog.FileSpec{
				{Name: "DOOMZERO.DEH"},
				{Name: "DoomZero.wad"},
			},
			Sources: []catalog.Source{{URL: fmt.Sprintf("%s/doomzero.wad", server.URL)}},
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
		dl := NewDownloader([]string{wadsDir}, false, nil)
		p := catalog.Preset{
			Name:    "Traversal",
			Engine:  "dsda-doom",
			IWAD:    "doom2.wad",
			Files:   []catalog.FileSpec{{Name: "../evil.wad"}},
			Sources: []catalog.Source{{URL: fmt.Sprintf("%s/direct.wad", server.URL)}},
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	dl := NewDownloader([]string{wadsDir}, false, nil)

	err := dl.extractFromZip(zipPath, []string{"normal.wad"})
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

func TestDownloader_FallbackReadmeRenamedToPrimaryStem(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	zipBytes := createTestZip(t, map[string]string{
		"MyMegawad.wad": "pwad-data",
		"readme.txt":    "generic readme content",
	})
	zipPath := filepath.Join(tmpDir, "megawad.zip")
	if err := os.WriteFile(zipPath, zipBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	wadsDir := filepath.Join(tmpDir, "wads")
	dl := NewDownloader([]string{wadsDir}, false, nil)
	p := catalog.Preset{
		Name:  "My Megawad",
		Files: []catalog.FileSpec{{Name: "MyMegawad.wad"}},
	}
	if err := dl.extractFromZip(zipPath, []string{"MyMegawad.wad"}, p); err != nil {
		t.Fatalf("extractFromZip failed: %v", err)
	}

	wantTxt := filepath.Join(wadsDir, "MyMegawad.txt")
	data, err := os.ReadFile(wantTxt)
	if err != nil || string(data) != "generic readme content" {
		t.Fatalf("expected %s with generic readme content, got %q (err=%v)", wantTxt, string(data), err)
	}
	idx := catalog.NewDirIndex(wadsDir)
	if resolved, ok := idx.ResolveReadme(p); !ok || resolved != wantTxt {
		t.Errorf("ResolveReadme = (%q, %v), want (%q, true)", resolved, ok, wantTxt)
	}
}

func TestDownloader_ExtractPK3AndReadmeFromZip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		entries     map[string]string
		expected    string
		wantContent string
		wantReadme  string
	}{
		{
			name: "nested pk3 and matching txt extracted while ignoring extra archives",
			entries: map[string]string{
				"MyHouse/Journal.docx":      "journal-bytes",
				"MyHouse/files/myhouse.zip": "standalone-wad-zip",
				"MyHouse/files/myhouse.txt": "myhouse-lore-readme",
				"MyHouse/files/myhouse.pk3": "updated-2025-pk3",
				"MyHouse/photos/photo1.jpg": "photo-bytes",
			},
			expected:    "myhouse.pk3",
			wantContent: "updated-2025-pk3",
			wantReadme:  "myhouse-lore-readme",
		},
		{
			name: "single pk3 fallback when archive entry carries date suffix",
			entries: map[string]string{
				"myhouse-20250428.pk3": "single-pk3-payload",
				"readme.txt":           "fallback-readme",
			},
			expected:    "myhouse.pk3",
			wantContent: "single-pk3-payload",
			wantReadme:  "fallback-readme",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			zipBytes := createTestZip(t, tc.entries)
			zipPath := filepath.Join(tmpDir, "bundle.zip")
			if err := os.WriteFile(zipPath, zipBytes, 0o644); err != nil {
				t.Fatal(err)
			}

			wadsDir := filepath.Join(tmpDir, "wads")
			dl := NewDownloader([]string{wadsDir}, false, nil)
			p := catalog.Preset{
				ID:    "myhouse",
				Name:  "MyHouse.pk3",
				Files: []catalog.FileSpec{{Name: tc.expected}},
			}
			if err := dl.extractFromZip(zipPath, []string{tc.expected}, p); err != nil {
				t.Fatalf("extractFromZip failed: %v", err)
			}

			gotPK3, err := os.ReadFile(filepath.Join(wadsDir, tc.expected))
			if err != nil || string(gotPK3) != tc.wantContent {
				t.Fatalf("extracted %s = %q (err=%v), want %q", tc.expected, string(gotPK3), err, tc.wantContent)
			}
			if _, err := os.Stat(filepath.Join(wadsDir, "myhouse.zip")); !os.IsNotExist(err) {
				t.Errorf("expected inner myhouse.zip not to be extracted to wadsDir")
			}

			idx := catalog.NewDirIndex(wadsDir)
			readmePath, ok := idx.ResolveReadme(p)
			if !ok {
				t.Fatalf("expected ResolveReadme to locate extracted myhouse.txt")
			}
			gotReadme, err := os.ReadFile(readmePath)
			if err != nil || string(gotReadme) != tc.wantReadme {
				t.Errorf("readme content = %q (err=%v), want %q", string(gotReadme), err, tc.wantReadme)
			}
		})
	}
}

func TestExtractWADLumpFromReaderAndZipSymlinkSkipped(t *testing.T) {
	t.Parallel()
	wadBytes := buildTestPWADWithDehacked(t, "Patch File for DeHackEd v3.0\nDoom version = 19\n")
	r := bytes.NewReader(wadBytes)
	got, ok := ExtractWADLumpFromReader(r, int64(len(wadBytes)), "DEHACKED")
	if !ok || !strings.Contains(string(got), "Doom version = 19") {
		t.Fatalf("ExtractWADLumpFromReader = (%q, %v), want DEHACKED lump", string(got), ok)
	}
	if _, okMissing := ExtractWADLumpFromReader(r, int64(len(wadBytes)), "MISSING"); okMissing {
		t.Errorf("expected missing lump to return false")
	}

	// Verify zip symlink entries are skipped and never extracted as regular files.
	tmpDir := t.TempDir()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	symHdr := &zip.FileHeader{Name: "evil.wad"}
	symHdr.SetMode(os.ModeSymlink | 0o777)
	sw, err := zw.CreateHeader(symHdr)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = sw.Write([]byte("/etc/passwd"))
	fw, err := zw.Create("good.wad")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("good-pwad"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(tmpDir, "sym.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	wadsDir := filepath.Join(tmpDir, "wads")
	dl := NewDownloader([]string{wadsDir}, false, nil)
	if err := dl.extractFromZip(zipPath, []string{"good.wad", "evil.wad"}); err == nil {
		t.Fatalf("expected extractFromZip to reject missing non-symlink match for evil.wad")
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "evil.wad")); !os.IsNotExist(err) {
		t.Errorf("expected symlink zip entry evil.wad to be skipped")
	}
	if data, err := os.ReadFile(filepath.Join(wadsDir, "good.wad")); err != nil || string(data) != "good-pwad" {
		t.Errorf("good.wad = %q (%v), want good-pwad", string(data), err)
	}
}

func TestDownloadAllIWADGateAndUninstallPresetSafety(t *testing.T) {
	t.Parallel()

	zipBytes := createTestZip(t, map[string]string{
		"sunder.wad": "sunder-pwad",
		"sunder.txt": "sunder-readme",
		"pl2.wad":    "plutonia2-pwad",
	})
	sum := sha256.Sum256(zipBytes)
	zipHex := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	}))
	t.Cleanup(srv.Close)

	wadsDir := filepath.Join(t.TempDir(), "wads")
	extraDir := filepath.Join(t.TempDir(), "readonly_wads")
	for _, d := range []string{wadsDir, extraDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Only DOOM2.WAD and shared idkfa 2024.wad are installed; PLUTONIA.WAD is absent.
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wadsDir, "idkfa 2024.wad"), []byte("music"), 0o644); err != nil {
		t.Fatal(err)
	}

	sunderPreset := catalog.Preset{
		ID:     "sunder",
		Name:   "Sunder",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files: []catalog.FileSpec{
			{Name: "sunder.wad"},
			{Name: "idkfa 2024.wad", Optional: true},
		},
		Sources: []catalog.Source{{URL: srv.URL + "/sunder.zip", SHA256: zipHex}},
	}
	pl2Preset := catalog.Preset{
		ID:      "plutonia-2",
		Name:    "Plutonia 2",
		Engine:  "dsda-doom",
		IWAD:    "PLUTONIA.WAD",
		Files:   []catalog.FileSpec{{Name: "pl2.wad"}},
		Sources: []catalog.Source{{URL: srv.URL + "/pl2.zip", SHA256: zipHex}},
	}
	doom2Preset := catalog.Preset{
		ID:     "doom-ii",
		Name:   "Doom II",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files:  []catalog.FileSpec{{Name: "idkfa 2024.wad", Optional: true}},
	}
	cat := &catalog.Catalog{
		Presets: []catalog.Preset{doom2Preset, sunderPreset, pl2Preset},
	}

	var dlOut bytes.Buffer
	dl := NewDownloader([]string{wadsDir}, false, &dlOut)
	dl.Client = srv.Client()
	if err := dl.DownloadAll(cat); err != nil {
		t.Fatalf("DownloadAll failed: %v", err)
	}
	if !strings.Contains(dlOut.String(), "Skipping Plutonia 2 (base IWAD PLUTONIA.WAD not installed)") {
		t.Errorf("expected DownloadAll to skip Plutonia 2 due to missing PLUTONIA.WAD, got:\n%s", dlOut.String())
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "pl2.wad")); !os.IsNotExist(err) {
		t.Errorf("expected pl2.wad not to be downloaded when PLUTONIA.WAD is missing")
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "sunder.wad")); err != nil {
		t.Fatalf("expected sunder.wad to be downloaded: %v", err)
	}

	// DownloadSelected explicitly downloads Plutonia 2 even without PLUTONIA.WAD
	if err := dl.DownloadSelected([]catalog.Preset{pl2Preset}, cat); err != nil {
		t.Fatalf("DownloadSelected failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wadsDir, "pl2.wad")); err != nil {
		t.Fatalf("expected DownloadSelected to install pl2.wad: %v", err)
	}

	// Create a backup file for sunder.wad to verify backup cleanup on uninstall
	sunderBak := filepath.Join(wadsDir, "sunder.wad.bak.20260101-000000")
	if err := os.WriteFile(sunderBak, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Base IWAD presets are protected from UninstallPreset
	if _, err := UninstallPreset(doom2Preset, cat, wadsDir, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "base IWADs are protected") {
		t.Fatalf("expected base IWAD protection error, got %v", err)
	}

	// 2. UninstallPreset removes sunder.wad, sunder.txt, and .bak.*, while preserving DOOM2.WAD and idkfa 2024.wad
	var unOut bytes.Buffer
	removed, err := UninstallPreset(sunderPreset, cat, wadsDir, nil, &unOut)
	if err != nil {
		t.Fatalf("UninstallPreset(sunder) error: %v", err)
	}
	if removed != 2 {
		t.Errorf("UninstallPreset removed %d files, want 2 (sunder.wad + sunder.txt); out:\n%s",
			removed, unOut.String())
	}
	for _, gone := range []string{"sunder.wad", "sunder.txt", "sunder.wad.bak.20260101-000000"} {
		if _, statErr := os.Stat(filepath.Join(wadsDir, gone)); !os.IsNotExist(statErr) {
			t.Errorf("expected %s to be removed", gone)
		}
	}
	for _, kept := range []string{"DOOM2.WAD", "idkfa 2024.wad"} {
		if _, statErr := os.Stat(filepath.Join(wadsDir, kept)); statErr != nil {
			t.Errorf("expected shared file %s to be preserved: %v", kept, statErr)
		}
	}

	// 3. Re-running UninstallPreset when already uninstalled is a clean no-op
	removedAgain, err := UninstallPreset(sunderPreset, cat, wadsDir, nil, &unOut)
	if err != nil || removedAgain != 0 {
		t.Errorf("second UninstallPreset = (%d, %v), want (0, nil)", removedAgain, err)
	}

	// 4. Files residing in secondary read-only extraWadRoots are never deleted
	roWad := filepath.Join(extraDir, "ro_map.wad")
	if err := os.WriteFile(roWad, []byte("readonly"), 0o644); err != nil {
		t.Fatal(err)
	}
	roPreset := catalog.Preset{
		ID:     "ro-map",
		Name:   "ReadOnly Map",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files:  []catalog.FileSpec{{Name: "ro_map.wad"}},
	}
	if _, err := UninstallPreset(roPreset, cat, wadsDir, []string{extraDir}, nil); err == nil ||
		!strings.Contains(err.Error(), "read-only wads-dirs") {
		t.Errorf("expected read-only wads-dirs error, got %v", err)
	}
	if _, err := os.Stat(roWad); err != nil {
		t.Errorf("read-only WAD %s was unexpectedly deleted", roWad)
	}
}
