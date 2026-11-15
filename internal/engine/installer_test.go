package engine

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/catalog"
)

func TestInstaller_ExtractZipBinary(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	_ = os.MkdirAll(binDir, 0o755)

	zipPath := filepath.Join(tmpDir, "test.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create test zip: %v", err)
	}

	zw := zip.NewWriter(zf)
	files := map[string]string{
		"soundfonts/uzdoom.sf2":    "soundfont data",
		"fm_banks/GENMIDI.GS.wopl": "opl data",
		"uzdoom.exe":               "executable binary",
		"uzdoom.pk3":               "pk3 archive",
		"libcurl-x64.dll":          "dll binary",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	_ = zf.Close()

	dest := filepath.Join(binDir, "uzdoom.exe")
	ins := NewInstaller(binDir, nil)
	if err := ins.extractZipBinary(zipPath, "uzdoom.exe", dest); err != nil {
		t.Fatalf("extractZipBinary failed: %v", err)
	}

	binData, err := os.ReadFile(dest)
	if err != nil || string(binData) != "executable binary" {
		t.Fatalf("expected binary content 'executable binary', got %q (%v)", string(binData), err)
	}

	// Verify missing binary returns an error rather than falling back to zr.File[0]
	if err := ins.extractZipBinary(zipPath, "nonexistent-engine", filepath.Join(binDir, "nonexistent")); err == nil {
		t.Fatal("expected error when target binary is not in zip")
	}
}

func TestSelectAsset_MacUniAndPatterns(t *testing.T) {
	t.Parallel()
	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	dsda, ok := cat.FindEngine("dsda-doom")
	if !ok || dsda.Install == nil {
		t.Fatal("dsda-doom install recipe missing")
	}

	liveAssets := []GitHubAsset{
		{
			Name:               "dsda-doom-0.30.1-linux-x86_64.appimage",
			BrowserDownloadURL: "https://example.com/linux.appimage",
			Digest:             "sha256:b06adf3f0c7d4e82dc1870baf97e67e4dc4e663e37f69070ddc02fa3e97608b0",
		},
		{
			Name:               "dsda-doom-0.30.1-mac-uni.zip",
			BrowserDownloadURL: "https://example.com/mac-uni.zip",
			Digest:             "sha256:9dced51464da3c5d915b4970b49128a798d0b59bfd3010a9a87edd8ebac01503",
		},
	}

	for _, plat := range []string{"darwin/arm64", "darwin/amd64"} {
		recipe := dsda.Install.Platforms[plat]
		url, digest, matched := SelectAsset(recipe, liveAssets)
		if !matched || url != "https://example.com/mac-uni.zip" {
			t.Fatalf("SelectAsset(%s) matched=%v url=%q, want mac-uni.zip", plat, matched, url)
		}
		if digest != "9dced51464da3c5d915b4970b49128a798d0b59bfd3010a9a87edd8ebac01503" {
			t.Fatalf("SelectAsset(%s) digest=%q", plat, digest)
		}
	}
}

func TestInstallCatalog_AggregatesErrorsAndVerifiesDigest(t *testing.T) {
	t.Parallel()
	payload := []byte("#!/bin/sh\necho ok\n")
	sum := sha256.Sum256(payload)
	sumHex := hex.EncodeToString(sum[:])

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/org/good/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1","assets":[{"name":"good-linux.appimage","browser_download_url":"` +
				serverURL + `/dl/good.appimage","digest":"sha256:` + sumHex + `"}]}`))
		case "/repos/org/bad/releases/latest":
			w.WriteHeader(http.StatusInternalServerError)
		case "/dl/good.appimage":
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	serverURL = server.URL

	binDir := t.TempDir()
	var out bytes.Buffer
	ins := NewInstaller(binDir, &out)
	ins.APIBase = server.URL
	ins.GOOS = "linux"
	ins.GOARCH = "amd64"

	cat := &catalog.Catalog{
		Engines: map[string]catalog.EngineConfig{
			"a-bad": {
				ID:     "a-bad",
				Binary: "a-bad",
				Install: &catalog.InstallRecipe{
					Repo: "org/bad",
					Platforms: map[string]catalog.PlatformRecipe{
						"linux/amd64": {Assets: []string{`^bad\.appimage$`}},
					},
				},
			},
			"b-good": {
				ID:     "b-good",
				Binary: "b-good",
				Install: &catalog.InstallRecipe{
					Repo: "org/good",
					Platforms: map[string]catalog.PlatformRecipe{
						"linux/amd64": {Assets: []string{`^good-linux\.appimage$`}},
					},
				},
			},
		},
	}

	err := ins.InstallCatalog(cat)
	if err == nil || !strings.Contains(err.Error(), "a-bad") {
		t.Fatalf("expected error from a-bad, got: %v", err)
	}

	// Verify b-good was STILL installed even though a-bad failed first
	if _, statErr := os.Stat(filepath.Join(binDir, "b-good")); statErr != nil {
		t.Fatalf("expected b-good to be installed despite a-bad failure: %v", statErr)
	}
}

func TestInstallEngine_AlreadyInstalledSkipsUnlessForce(t *testing.T) {
	t.Parallel()
	binDir := t.TempDir()
	existingBin := filepath.Join(binDir, "my-engine")
	if err := os.WriteFile(existingBin, []byte("installed"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	ins := NewInstaller(binDir, &out)
	ins.GOOS = "linux"
	ins.GOARCH = "amd64"
	ins.Force = false

	eng := catalog.EngineConfig{
		ID:          "my-engine",
		DisplayName: "My Engine",
		Binary:      "my-engine",
		Install: &catalog.InstallRecipe{
			Repo: "org/dummy",
			Platforms: map[string]catalog.PlatformRecipe{
				"linux/amd64": {Assets: []string{`^dummy\.appimage$`}},
			},
		},
	}

	if err := ins.InstallEngine(eng); err != nil {
		t.Fatalf("InstallEngine failed on already installed binary: %v", err)
	}
	if !strings.Contains(out.String(), "already installed") {
		t.Fatalf("expected already installed message, got: %s", out.String())
	}
}

func TestInstaller_DarwinZipAppBundleAndSafeShellWrapper(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	appsDir := filepath.Join(tmpDir, "Apps$With'Special`Chars")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(tmpDir, "dsda-doom-mac-uni.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	entries := map[string]string{
		"dsda-doom.app/Contents/Info.plist": "<plist><dict><key>CFBundleExecutable</key>" +
			"<string>dsda-doom</string></dict></plist>",
		"dsda-doom.app/Contents/MacOS/helper":          "helper-bin",
		"dsda-doom.app/Contents/MacOS/dsda-doom":       "macho-main-bin",
		"dsda-doom.app/Contents/Frameworks/libSDL2.so": "sdl2-dylib",
		"dsda-doom.app/Contents/Resources/dsda.wad":    "dsda-resource-wad",
	}
	for name, body := range entries {
		w, createErr := zw.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = zf.Close()

	ins := NewInstaller(binDir, nil)
	ins.GOOS = "darwin"
	ins.AppsDir = appsDir
	wrapperDest := filepath.Join(binDir, "dsda-doom")
	if err := ins.extractZipBinary(zipPath, "dsda-doom", wrapperDest); err != nil {
		t.Fatalf("extractZipBinary(darwin .app): %v", err)
	}

	// Verify Frameworks and Resources inside .app bundle are preserved intact.
	fwPath := filepath.Join(appsDir, "dsda-doom.app", "Contents", "Frameworks", "libSDL2.so")
	if got, err := os.ReadFile(fwPath); err != nil || string(got) != "sdl2-dylib" {
		t.Fatalf("expected framework preserved at %s, got %q (%v)", fwPath, string(got), err)
	}
	resPath := filepath.Join(appsDir, "dsda-doom.app", "Contents", "Resources", "dsda.wad")
	if got, err := os.ReadFile(resPath); err != nil || string(got) != "dsda-resource-wad" {
		t.Fatalf("expected resource preserved at %s, got %q (%v)", resPath, string(got), err)
	}

	// Verify POSIX shell wrapper uses single quotes so $With and backticks cannot execute commands.
	wrapperBytes, err := os.ReadFile(wrapperDest)
	if err != nil {
		t.Fatal(err)
	}
	wantExec := "exec " + shellSingleQuote(
		filepath.Join(appsDir, "dsda-doom.app", "Contents", "MacOS", "dsda-doom"),
	) + " \"$@\"\n"
	if !strings.Contains(string(wrapperBytes), wantExec) {
		t.Errorf("wrapper content = %q, want %q", string(wrapperBytes), wantExec)
	}
}

func TestInstaller_DarwinZipAppBundleSymlinkConfinement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on windows")
	}
	t.Parallel()
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	appsDir := filepath.Join(tmpDir, "apps")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeZipSymlink := func(zw *zip.Writer, name, target string) {
		t.Helper()
		hdr := &zip.FileHeader{Name: name}
		hdr.SetMode(os.ModeSymlink | 0o777)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(target)); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Valid multi-hop macOS .framework symlink chain inside .app succeeds.
	validZip := filepath.Join(tmpDir, "valid-framework.zip")
	vf, err := os.Create(validZip)
	if err != nil {
		t.Fatal(err)
	}
	vzw := zip.NewWriter(vf)
	for name, body := range map[string]string{
		"dsda-doom.app/Contents/MacOS/dsda-doom":                                 "macho-bin",
		"dsda-doom.app/Contents/Frameworks/SDL2.framework/Versions/A/SDL2":       "sdl2-bin",
		"dsda-doom.app/Contents/Frameworks/SDL2.framework/Versions/A/Info.plist": "<plist/>",
	} {
		w, createErr := vzw.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		_, _ = w.Write([]byte(body))
	}
	// Place top-level framework symlink BEFORE Versions/Current to exercise multi-pass ordering.
	writeZipSymlink(vzw, "dsda-doom.app/Contents/Frameworks/SDL2.framework/SDL2", "Versions/Current/SDL2")
	writeZipSymlink(vzw, "dsda-doom.app/Contents/Frameworks/SDL2.framework/Versions/Current", "A")
	if err := vzw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = vf.Close()

	ins := NewInstaller(binDir, nil)
	ins.GOOS = "darwin"
	ins.AppsDir = appsDir
	if err := ins.extractZipBinary(validZip, "dsda-doom", filepath.Join(binDir, "dsda-doom")); err != nil {
		t.Fatalf("expected valid .framework symlinks in .app to succeed, got: %v", err)
	}
	fwLink := filepath.Join(appsDir, "dsda-doom.app", "Contents", "Frameworks", "SDL2.framework", "SDL2")
	if got, readErr := os.ReadFile(fwLink); readErr != nil || string(got) != "sdl2-bin" {
		t.Fatalf("expected multi-hop framework symlink to resolve to sdl2-bin, got %q (%v)", string(got), readErr)
	}

	// 2. Malicious chained symlink escape (hop -> .., hop/escape -> ../../../outside) is rejected.
	outsideTarget := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outsideTarget, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	evilZip := filepath.Join(tmpDir, "evil-symlink.zip")
	ef, err := os.Create(evilZip)
	if err != nil {
		t.Fatal(err)
	}
	ezw := zip.NewWriter(ef)
	w, err := ezw.Create("evil.app/Contents/MacOS/dsda-doom")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("macho-bin"))
	writeZipSymlink(ezw, "evil.app/Contents/Frameworks/hop", "..")
	writeZipSymlink(ezw, "evil.app/Contents/Frameworks/hop/escape", "../../../outside.txt")
	if err := ezw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = ef.Close()

	if err := ins.extractZipBinary(evilZip, "dsda-doom", filepath.Join(binDir, "dsda-doom")); err == nil {
		t.Fatal("expected chained symlink escape outside .app bundle to be rejected, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(appsDir, "evil.app")); !os.IsNotExist(statErr) {
		t.Errorf("expected rejected evil.app bundle not to be installed, stat err: %v", statErr)
	}
}
