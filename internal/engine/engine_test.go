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
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/preset"
)

func TestPrepareLaunch_DSDADoom(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(wadsDir, 0o755)

	fakeBin := filepath.Join(binDir, "dsda-doom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)

	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.wad"), []byte("wad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.deh"), []byte("deh"), 0o644)

	p := preset.Preset{
		Name:   "Alien Vendetta",
		Engine: "dsda-doom",
		IWAD:   "doom2.wad",
		Files:  []preset.FileSpec{{Name: "av.wad"}, {Name: "av.deh"}},
		Args:   []string{"-complevel", "2"},
	}

	opts := LaunchOptions{
		BinDir:    binDir,
		WadsDir:   wadsDir,
		DryRun:    true,
		ExtraArgs: []string{"-skill", "4"},
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}

	if plan.Engine != "dsda-doom" {
		t.Errorf("expected dsda-doom, got %s", plan.Engine)
	}
	if plan.EngineBin != fakeBin {
		t.Errorf("expected binary %s, got %s", fakeBin, plan.EngineBin)
	}

	joinedArgs := strings.Join(plan.Args, " ")
	expectedParts := []string{
		"-iwad " + filepath.Join(wadsDir, "DOOM2.WAD"),
		"-file " + filepath.Join(wadsDir, "av.wad"),
		"-deh " + filepath.Join(wadsDir, "av.deh"),
		"-complevel 2",
		"-skill 4",
	}
	lowerArgs := strings.ToLower(joinedArgs)
	for _, part := range expectedParts {
		if !strings.Contains(lowerArgs, strings.ToLower(part)) {
			t.Errorf("expected args to contain %q, got: %s", part, joinedArgs)
		}
	}

	var out bytes.Buffer
	if err := Execute(plan, &out, &out); err != nil {
		t.Fatalf("dry run Execute failed: %v", err)
	}
	if !strings.Contains(out.String(), "Launching Preset: Alien Vendetta") {
		t.Errorf("expected banner in output, got:\n%s", out.String())
	}
}

func TestPrepareLaunch_UZDoom_OptionalAudioAndLoadMapsAfterMods(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(wadsDir, 0o755)

	fakeBin := filepath.Join(binDir, "uzdoom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)

	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "eviternity.wad"), []byte("wad"), 0o644)

	p := preset.Preset{
		Name:   "Eviternity",
		Engine: "uzdoom",
		IWAD:   "doom2.wad",
		Files: []preset.FileSpec{
			{Name: "idkfa 2024.wad", Optional: true},
			{Name: "eviternity.wad"},
		},
	}

	var out bytes.Buffer
	opts := LaunchOptions{
		BinDir:  binDir,
		WadsDir: wadsDir,
		DryRun:  true,
		Out:     &out,
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}
	if strings.Contains(strings.Join(plan.Args, " "), "idkfa 2024.wad") {
		t.Errorf("did not expect missing idkfa 2024.wad in args: %v", plan.Args)
	}

	if !strings.Contains(out.String(), "Optional soundtrack 'idkfa 2024.wad' not found") {
		t.Errorf("expected optional soundtrack warning, got: %s", out.String())
	}

	// Now create idkfa 2024.wad and verify LoadMapsAfterMods reorders optional mods before required maps
	_ = os.WriteFile(filepath.Join(wadsDir, "idkfa 2024.wad"), []byte("music"), 0o644)
	pReorder := preset.Preset{
		Name:              "Sigil II",
		Engine:            "uzdoom",
		IWAD:              "DOOM2.WAD",
		LoadMapsAfterMods: true,
		Files: []preset.FileSpec{
			{Name: "eviternity.wad", Optional: false},
			{Name: "idkfa 2024.wad", Optional: true},
		},
	}
	planReorder, err := PrepareLaunch(pReorder, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch reorder failed: %v", err)
	}
	if len(planReorder.OrderedFiles) != 2 ||
		filepath.Base(planReorder.OrderedFiles[0]) != "idkfa 2024.wad" ||
		filepath.Base(planReorder.OrderedFiles[1]) != "eviternity.wad" {
		t.Fatalf("expected optional mod before map when LoadMapsAfterMods=true, got: %v", planReorder.OrderedFiles)
	}
}

func TestPrepareLaunch_CustomEnginePrefixArgsAndBex(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	customBinDir := filepath.Join(tmpDir, "custom_bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(customBinDir, 0o755)
	_ = os.MkdirAll(wadsDir, 0o755)

	customBinPath := filepath.Join(customBinDir, "woof")
	_ = os.WriteFile(customBinPath, []byte("#!/bin/sh\n"), 0o755)

	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "sunlust.wad"), []byte("wad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "sunlust.bex"), []byte("bex"), 0o644)

	customEngines := map[string]preset.EngineConfig{
		"woof": {
			ID:          "woof",
			Binary:      customBinPath,
			ArgsStyle:   "boom",
			PrefixArgs:  []string{"-config", "/tmp/woof.cfg"},
			DefaultArgs: []string{"-geometry", "1920x1080"},
		},
	}

	p := preset.Preset{
		Name:   "Sunlust",
		Engine: "woof",
		IWAD:   "DOOM2.WAD",
		Files:  []preset.FileSpec{{Name: "sunlust.wad"}, {Name: "sunlust.bex"}},
		Args:   []string{"-skill", "4"},
	}

	opts := LaunchOptions{
		BinDir:    binDir,
		WadsDir:   wadsDir,
		Engines:   customEngines,
		DryRun:    true,
		ExtraArgs: []string{"-warp", "15"},
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}

	if plan.EngineBin != customBinPath {
		t.Errorf("expected engine binary %s, got %s", customBinPath, plan.EngineBin)
	}

	joined := strings.Join(plan.Args, " ")
	expectedTokens := []string{
		"-config /tmp/woof.cfg -iwad",
		"-file " + filepath.Join(wadsDir, "sunlust.wad"),
		"-deh " + filepath.Join(wadsDir, "sunlust.bex"),
		"-geometry 1920x1080",
		"-skill 4",
		"-warp 15",
	}
	for _, tok := range expectedTokens {
		if !strings.Contains(joined, tok) {
			t.Errorf("expected args to contain %q, got: %s", tok, joined)
		}
	}
}

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
	cat, err := preset.LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog: %v", err)
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

	cat := &preset.Catalog{
		Engines: map[string]preset.EngineConfig{
			"a-bad": {
				ID:     "a-bad",
				Binary: "a-bad",
				Install: &preset.InstallRecipe{
					Repo: "org/bad",
					Platforms: map[string]preset.PlatformRecipe{
						"linux/amd64": {Assets: []string{`^bad\.appimage$`}},
					},
				},
			},
			"b-good": {
				ID:     "b-good",
				Binary: "b-good",
				Install: &preset.InstallRecipe{
					Repo: "org/good",
					Platforms: map[string]preset.PlatformRecipe{
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

	// Verify b-good was STILL installed even though a-bad failed first (Defect #15 fix)
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

	eng := preset.EngineConfig{
		ID:          "my-engine",
		DisplayName: "My Engine",
		Binary:      "my-engine",
		Install: &preset.InstallRecipe{
			Repo: "org/dummy",
			Platforms: map[string]preset.PlatformRecipe{
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

func TestPrepareLaunch_CustomFileSpecAliases(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(wadsDir, 0o755)

	fakeBin := filepath.Join(binDir, "dsda-doom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	aliasWad := filepath.Join(wadsDir, "custom_short.wad")
	_ = os.WriteFile(aliasWad, []byte("pwad"), 0o644)

	p := preset.Preset{
		ID:     "custom-alias-pack",
		Name:   "Custom Alias Pack",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files: []preset.FileSpec{
			{Name: "custom_canonical_v2.wad", Aliases: []string{"custom_short.wad"}},
		},
	}

	plan, err := PrepareLaunch(p, LaunchOptions{
		BinDir:  binDir,
		WadsDir: wadsDir,
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("PrepareLaunch failed to resolve custom FileSpec.Aliases: %v", err)
	}
	if len(plan.OrderedFiles) != 1 || plan.OrderedFiles[0] != aliasWad {
		t.Fatalf("OrderedFiles = %v, want [%s]", plan.OrderedFiles, aliasWad)
	}
}
