package catalog

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveFileSubpathsMultiRootAndSubdirs(t *testing.T) {
	t.Parallel()
	root0 := t.TempDir()
	root1 := t.TempDir()

	// Create sub/foo.wad in root0
	if err := os.MkdirAll(filepath.Join(root0, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	subWad := filepath.Join(root0, "sub", "foo.wad")
	_ = os.WriteFile(subWad, []byte("PWAD"), 0o644)

	// Create evil.wad in parent of root0 to test traversal rejection
	evilWad := filepath.Join(filepath.Dir(root0), "evil.wad")
	_ = os.WriteFile(evilWad, []byte("PWAD"), 0o644)
	t.Cleanup(func() { _ = os.Remove(evilWad) })

	// Create 1-level subdirectory megawad in root1: root1/av/AV.WAD
	if err := os.MkdirAll(filepath.Join(root1, "av"), 0o755); err != nil {
		t.Fatal(err)
	}
	avSub := filepath.Join(root1, "av", "AV.WAD")
	_ = os.WriteFile(avSub, []byte("PWAD"), 0o644)

	// Place an IWAD inside a subdirectory only — IWADs must NOT resolve from subdirectories
	_ = os.WriteFile(filepath.Join(root1, "av", "DOOM2.WAD"), []byte("IWAD"), 0o644)

	// Create alias files
	sigilWad := filepath.Join(root0, "sigil.wad")
	_ = os.WriteFile(sigilWad, []byte("PWAD"), 0o644)
	sigil2Wad := filepath.Join(root0, "sigil2.wad")
	_ = os.WriteFile(sigil2Wad, []byte("PWAD"), 0o644)
	sunderWad := filepath.Join(root1, "sunder.wad")
	_ = os.WriteFile(sunderWad, []byte("PWAD"), 0o644)

	idx := NewDirIndex(root0, root1)

	tests := []struct {
		name     string
		query    string
		wantOK   bool
		wantPath string
	}{
		{name: "forward slash subpath", query: "sub/foo.wad", wantOK: true, wantPath: subWad},
		{name: "backslash subpath", query: `sub\foo.wad`, wantOK: true, wantPath: subWad},
		{name: "case-insensitive subpath", query: "sub/FOO.WAD", wantOK: true, wantPath: subWad},
		{name: "traversal rejected", query: "../evil.wad", wantOK: false},
		{name: "1-level subdir non-IWAD in root1", query: "av.wad", wantOK: true, wantPath: avSub},
		{name: "IWAD in subdir ignored", query: "DOOM2.WAD", wantOK: false},
		{name: "alias sigil to SIGIL_V1_23", query: "SIGIL_V1_23.wad", wantOK: true, wantPath: sigilWad},
		{name: "alias sigil2 to SIGIL_II_V1_0", query: "SIGIL_II_V1_0.WAD", wantOK: true, wantPath: sigil2Wad},
		{name: "alias sunder to Sunder 2512", query: "Sunder 2512.wad", wantOK: true, wantPath: sunderWad},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := idx.ResolveFile(tc.query)
			if ok != tc.wantOK {
				t.Fatalf("ResolveFile(%q) ok = %v (path=%q), want %v", tc.query, ok, got, tc.wantOK)
			}
			if tc.wantOK && got != tc.wantPath {
				t.Fatalf("ResolveFile(%q) = %q, want %q", tc.query, got, tc.wantPath)
			}
		})
	}
}

func TestDirIndex_WithCatalogCustomAliases(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	aliasFile := filepath.Join(wadsDir, "custom_short.wad")
	if err := os.WriteFile(aliasFile, []byte("PWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	aliasTxt := filepath.Join(wadsDir, "custom_short.txt")
	if err := os.WriteFile(aliasTxt, []byte("readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	customPreset := Preset{
		ID:     "custom-pack",
		Name:   "Custom Pack",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files: []FileSpec{
			{
				Name:    "custom_pack_v2.wad",
				Aliases: []string{"custom_short.wad"},
			},
		},
	}
	cat := &Catalog{Presets: []Preset{customPreset}}
	idx := NewDirIndex(wadsDir).WithCatalog(cat)

	got, ok := idx.ResolveFile("custom_pack_v2.wad")
	if !ok || got != aliasFile {
		t.Fatalf("ResolveFile(\"custom_pack_v2.wad\") = (%q, %v), want (%q, true)", got, ok, aliasFile)
	}

	gotTxt, okTxt := idx.ResolveReadme(customPreset)
	if !okTxt || gotTxt != aliasTxt {
		t.Fatalf("ResolveReadme(customPreset) = (%q, %v), want (%q, true)", gotTxt, okTxt, aliasTxt)
	}
}

func TestResolveReadmeRegression2(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()

	for _, txt := range []string{
		"doom.txt", "doom2.txt", "plutonia.txt", "tnt.txt", "heretic.txt", "hexen.txt",
		"idkfa 2024.txt", "aaliens.txt", "deathless1.1.txt", "eviternityii.txt", "gdturbo.txt",
		"sigil.txt", "sigil2.txt", "SODFINAL.txt", "sunder.txt",
	} {
		if err := os.WriteFile(filepath.Join(wadsDir, txt), []byte("readme"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cat, err := LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	idx := NewDirIndex(wadsDir).WithCatalog(cat)

	basePresets := []struct {
		name    string
		wantTxt string
	}{
		{name: "Doom", wantTxt: "doom.txt"},
		{name: "Doom II", wantTxt: "doom2.txt"},
		{name: "The Plutonia Experiment", wantTxt: "plutonia.txt"},
		{name: "TNT: Evilution", wantTxt: "tnt.txt"},
		{name: "Heretic", wantTxt: "heretic.txt"},
		{name: "Hexen", wantTxt: "hexen.txt"},
		{name: "Ancient Aliens", wantTxt: "aaliens.txt"},
		{name: "Deathless", wantTxt: "deathless1.1.txt"},
		{name: "Eviternity II", wantTxt: "eviternityii.txt"},
		{name: "Going Down Turbo", wantTxt: "gdturbo.txt"},
		{name: "Sigil", wantTxt: "sigil.txt"},
		{name: "Sigil II", wantTxt: "sigil2.txt"},
		{name: "Speed of Doom", wantTxt: "SODFINAL.txt"},
		{name: "Sunder", wantTxt: "sunder.txt"},
	}

	for _, tc := range basePresets {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := cat.Find(tc.name)
			if p == nil {
				t.Fatalf("preset %q not found", tc.name)
			}
			got, ok := idx.ResolveReadme(*p)
			if !ok || filepath.Base(got) != tc.wantTxt {
				t.Fatalf("ResolveReadme(%q) = (%q, %v), want %s", tc.name, got, ok, tc.wantTxt)
			}
		})
	}

	// Verify a megawad with required PWADs does NOT borrow doom2.txt or sequel .txt
	av := cat.Find("Alien Vendetta")
	if av == nil {
		t.Fatal("Alien Vendetta not found")
	}
	if got, ok := idx.ResolveReadme(*av); ok {
		t.Fatalf("expected Alien Vendetta not to borrow IWAD readme, got %q", got)
	}

	scytheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(scytheDir, "scythe2.txt"), []byte("scythe 2 readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	scythe1 := cat.Find("Scythe")
	if scythe1 == nil {
		t.Fatal("Scythe preset not found")
	}
	scytheIdx := NewDirIndex(scytheDir).WithCatalog(cat)
	if got, ok := scytheIdx.ResolveReadme(*scythe1); ok {
		t.Fatalf("expected Scythe not to falsely resolve scythe2.txt, got %q", got)
	}

	// Verify <stem>ReadMe*.txt fallback (such as SunderReadMeV2.TXT from Sunder_2512.zip)
	sunderDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sunderDir, "SunderReadMeV2.TXT"), []byte("sunder readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	sunderPreset := cat.Find("Sunder")
	if sunderPreset == nil {
		t.Fatal("Sunder preset not found")
	}
	sunderIdx := NewDirIndex(sunderDir).WithCatalog(cat)
	if got, ok := sunderIdx.ResolveReadme(*sunderPreset); !ok || filepath.Base(got) != "SunderReadMeV2.TXT" {
		t.Fatalf("ResolveReadme(Sunder) with SunderReadMeV2.TXT = (%q, %v), want SunderReadMeV2.TXT", got, ok)
	}
}

func TestDirIndex_CachedRelativeSubpath(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	subDir := filepath.Join(wadsDir, "Maps_Sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wadPath := filepath.Join(subDir, "Episode_One.wad")
	if err := os.WriteFile(wadPath, []byte("PWAD"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := NewDirIndex(wadsDir)
	got1, ok1 := idx.ResolveFile("Maps_Sub/episode_one.wad")
	if !ok1 || got1 != wadPath {
		t.Fatalf("first ResolveFile = (%q, %v), want (%q, true)", got1, ok1, wadPath)
	}

	// Remove the file from disk after initial scan to verify subsequent resolution uses cached entries.
	if err := os.Remove(wadPath); err != nil {
		t.Fatal(err)
	}
	got2, ok2 := idx.ResolveFile("Maps_Sub/episode_one.wad")
	if !ok2 || got2 != wadPath {
		t.Fatalf("cached ResolveFile = (%q, %v), want (%q, true)", got2, ok2, wadPath)
	}
}

func TestIsPresetInstalledAndSubpathReadme(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	subDir := filepath.Join(wadsDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "Episode.wad"), []byte("PWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	topTxt := filepath.Join(wadsDir, "Episode.txt")
	if err := os.WriteFile(topTxt, []byte("README"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := Preset{
		Name: "Subpath Episode",
		IWAD: "DOOM2.WAD",
		Files: []FileSpec{
			{Name: "sub/Episode.wad"},
			{Name: "optional_music.wad", Optional: true},
		},
	}

	idx := NewDirIndex(wadsDir)
	if !idx.IsPresetInstalled(p) {
		t.Errorf("expected IsPresetInstalled(p) = true when IWAD and required subpath WAD exist")
	}
	gotReadme, ok := idx.ResolveReadme(p)
	if !ok || gotReadme != topTxt {
		t.Errorf("ResolveReadme(p) = (%q, %v), want (%q, true)", gotReadme, ok, topTxt)
	}

	missingIWAD := p
	missingIWAD.IWAD = "PLUTONIA.WAD"
	if idx.IsPresetInstalled(missingIWAD) {
		t.Errorf("expected IsPresetInstalled = false when required IWAD is missing")
	}
}

func TestNeedsSteamDiscoveryAndNonDirCatalogErrors(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wadsDir, "sigil_shreds_compat.wad"), []byte("music"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := NewDirIndex(wadsDir)

	// 1. Preset with empty IWAD, download sources, and no missing optional files does not need Steam discovery
	pNoIWAD := Preset{
		ID:      "standalone",
		Name:    "Standalone",
		IWAD:    "",
		Sources: []Source{{URL: "https://example.com/standalone.zip"}},
		Files:   []FileSpec{{Name: "standalone.wad"}},
	}
	if idx.NeedsSteamDiscovery(pNoIWAD) {
		t.Errorf("expected NeedsSteamDiscovery=false for preset with empty IWAD and sources")
	}

	// 2. Preset with present IWAD, download sources, and optional file satisfied via FileSpec.Aliases
	pOptAlias := Preset{
		ID:      "sigil-opt",
		Name:    "Sigil Opt",
		IWAD:    "DOOM2.WAD",
		Sources: []Source{{URL: "https://example.com/sigil.zip"}},
		Files: []FileSpec{
			{Name: "SIGIL_V1_23.wad"},
			{Name: "SIGIL_SHREDS.wad", Optional: true, Aliases: []string{"sigil_shreds_compat.wad"}},
		},
	}
	if idx.NeedsSteamDiscovery(pOptAlias) {
		t.Errorf("expected NeedsSteamDiscovery=false when optional file is present via alias")
	}

	// 3. Preset with missing optional file needs Steam discovery
	pMissingOpt := Preset{
		ID:      "doom2-music",
		Name:    "Doom II Music",
		IWAD:    "DOOM2.WAD",
		Sources: []Source{{URL: "https://example.com/map.zip"}},
		Files: []FileSpec{
			{Name: "map.wad"},
			{Name: "idkfa 2024.wad", Optional: true},
		},
	}
	if !idx.NeedsSteamDiscovery(pMissingOpt) {
		t.Errorf("expected NeedsSteamDiscovery=true when optional file is missing")
	}

	// 4. LoadLayered, MergeUserDir, and LoadPresetBaseAndOverride reject regular file userDir
	notADir := filepath.Join(t.TempDir(), "not-a-dir.json")
	if err := os.WriteFile(notADir, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLayered(notADir, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected LoadLayered(file) to fail with 'not a directory', got %v", err)
	}
	curated, err := LoadCurated()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MergeUserDir(curated, notADir); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected MergeUserDir(curated, file) to fail with 'not a directory', got %v", err)
	}
	if _, _, err := LoadPresetBaseAndOverride(notADir, "doom2"); err == nil ||
		!strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected LoadPresetBaseAndOverride(file, doom2) to fail with 'not a directory', got %v", err)
	}
}

func TestIsReadmeSuffix_RootPrecedence_And_Symlinks(t *testing.T) {
	t.Parallel()

	// 1. Verify IsReadmeSuffix rejects sequel+version ("2v1.0") while accepting dotted versions ("1.1")
	if IsReadmeSuffix("2v1.0") {
		t.Errorf("expected IsReadmeSuffix(\"2v1.0\") = false")
	}
	if !IsReadmeSuffix("1.1") || !IsReadmeSuffix("v2.0") || !IsReadmeSuffix("readme_final") {
		t.Errorf("expected IsReadmeSuffix to accept 1.1, v2.0, and readme_final")
	}

	// 2. Verify DirIndex primary root precedence over secondary root and symlink handling
	root0 := t.TempDir()
	root1 := t.TempDir()
	primAlias := filepath.Join(root0, "custom_short.wad")
	secCanon := filepath.Join(root1, "custom_v2.wad")
	_ = os.WriteFile(primAlias, []byte("prim"), 0o644)
	_ = os.WriteFile(secCanon, []byte("sec"), 0o644)

	spec := FileSpec{Name: "custom_v2.wad", Aliases: []string{"custom_short.wad"}}
	idx := NewDirIndex(root0, root1)
	if got, ok := idx.ResolveFileSpec(spec); !ok || got != primAlias {
		t.Errorf("ResolveFileSpec multi-root = (%q, %v), want primary root alias %q", got, ok, primAlias)
	}

	if runtime.GOOS != "windows" {
		symRoot := t.TempDir()
		extSub := t.TempDir()
		subFile := filepath.Join(extSub, "linked_map.wad")
		_ = os.WriteFile(subFile, []byte("pwad"), 0o644)
		if err := os.Symlink(
			filepath.Join(symRoot, "nonexistent.wad"),
			filepath.Join(symRoot, "dangling.wad"),
		); err == nil {
			_ = os.Symlink(extSub, filepath.Join(symRoot, "linked_sub"))
			symIdx := NewDirIndex(symRoot)
			if _, ok := symIdx.ResolveFile("dangling.wad"); ok {
				t.Errorf("expected dangling symlink to be ignored by DirIndex")
			}
			wantLinked := filepath.Join(symRoot, "linked_sub", "linked_map.wad")
			if got, ok := symIdx.ResolveFile("linked_map.wad"); !ok || got != wantLinked {
				t.Errorf("expected symlinked 1-level subdir to resolve %q, got (%q, %v)", wantLinked, got, ok)
			}
		}
	}
}
