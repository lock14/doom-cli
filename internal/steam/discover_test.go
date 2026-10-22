package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/preset"
)

func TestParseLibraryFolders(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "steam_vdf_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	lib1 := filepath.Join(tmpDir, "library1")
	lib2 := filepath.Join(tmpDir, "library2")
	if err := os.MkdirAll(lib1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lib2, 0755); err != nil {
		t.Fatal(err)
	}

	vdfContent := `
"libraryfolders"
{
	"0"
	{
		"path"		"` + lib1 + `"
		"label"		""
	}
	"1"
	{
		"path"		"` + lib2 + `"
		"label"		"Secondary"
	}
}
`
	vdfPath := filepath.Join(tmpDir, "libraryfolders.vdf")
	if err := os.WriteFile(vdfPath, []byte(vdfContent), 0644); err != nil {
		t.Fatal(err)
	}

	paths := ParseLibraryFolders(vdfPath)
	if len(paths) != 2 {
		t.Fatalf("expected 2 library paths, got %d: %v", len(paths), paths)
	}
	if paths[0] != lib1 || paths[1] != lib2 {
		t.Errorf("unexpected parsed paths: %v", paths)
	}
}

func TestDefaultTargetPatterns_DerivedFromCatalog(t *testing.T) {
	t.Parallel()
	cat, err := preset.LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog: %v", err)
	}
	patterns := DefaultTargetPatterns()
	if len(patterns) == 0 {
		t.Fatal("expected non-empty DefaultTargetPatterns()")
	}

	destByNormSource := make(map[string]string)
	for _, tp := range patterns {
		key := preset.NormalizeFilename(tp.SourcePattern)
		destLower := strings.ToLower(tp.DestName)
		if prev, exists := destByNormSource[key]; exists && prev == destLower {
			t.Errorf("duplicate normalized target pattern %q -> %q", tp.SourcePattern, tp.DestName)
		}
		destByNormSource[key] = destLower
	}

	for _, p := range cat.Presets {
		iwadNorm := preset.NormalizeFilename(filepath.Base(p.IWAD))
		if _, ok := destByNormSource[iwadNorm]; !ok {
			t.Errorf("preset %s IWAD %q missing from DefaultTargetPatterns", p.ID, p.IWAD)
		}
		for _, f := range p.Files {
			fNorm := preset.NormalizeFilename(filepath.Base(f.Name))
			if _, ok := destByNormSource[fNorm]; !ok {
				t.Errorf("preset %s file %q missing from DefaultTargetPatterns", p.ID, f.Name)
			}
			for _, al := range f.Aliases {
				alNorm := preset.NormalizeFilename(al)
				if gotDest, ok := destByNormSource[alNorm]; !ok || gotDest != strings.ToLower(filepath.Base(f.Name)) {
					t.Errorf("preset %s alias %q -> %q missing from DefaultTargetPatterns", p.ID, al, f.Name)
				}
			}
		}
	}
}

func TestDiscoverAndExtract(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "steam_extract_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockSteam := filepath.Join(tmpDir, "steam_root")
	doomDir := filepath.Join(mockSteam, "steamapps", "common", "Doom 2", "base")
	if err := os.MkdirAll(doomDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create mock game files (including normalized underscore variant idkfa_2024.wad and aliases)
	_ = os.WriteFile(filepath.Join(doomDir, "doom2.wad"), []byte("doom2-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "gd.wad"), []byte("gdturbo-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "DOOMZERO.DEH"), []byte("doomzero-deh-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "sigil2.wad"), []byte("sigil2-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "idkfa_2024.wad"), []byte("idkfa-data"), 0644)

	destWads := filepath.Join(tmpDir, "target_wads")
	var out bytes.Buffer
	count, err := DiscoverAndExtract([]string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtract failed: %v", err)
	}

	if count != 5 {
		t.Fatalf("expected 5 files extracted, got %d\nOutput: %s", count, out.String())
	}

	// Verify target names:
	// doom2.wad -> DOOM2.WAD
	// gd.wad -> gdturbo.wad
	// DOOMZERO.DEH -> DOOMZERO.DEH
	// sigil2.wad -> SIGIL_II_V1_0.WAD
	// idkfa_2024.wad -> idkfa 2024.wad (via normalized matching)
	for _, wantFile := range []string{
		"DOOM2.WAD",
		"gdturbo.wad",
		"DOOMZERO.DEH",
		"SIGIL_II_V1_0.WAD",
		"idkfa 2024.wad",
	} {
		if _, err := os.Stat(filepath.Join(destWads, wantFile)); err != nil {
			t.Errorf("%s was not created: %v", wantFile, err)
		}
	}

	// Run again without force: count should be 0
	out.Reset()
	count2, err := DiscoverAndExtract([]string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("second DiscoverAndExtract failed: %v", err)
	}
	if count2 != 0 {
		t.Errorf("expected 0 files on second run without force, got %d", count2)
	}

	// Verify DiscoverAndExtractForCatalog extracts custom user-layer preset aliases
	_ = os.WriteFile(filepath.Join(doomDir, "custom_addon.wad"), []byte("custom-data"), 0644)
	customCat := &preset.Catalog{
		Presets: []preset.Preset{
			{
				ID:     "custom-addon",
				Name:   "Custom Addon",
				Engine: "dsda-doom",
				IWAD:   "DOOM2.WAD",
				Files: []preset.FileSpec{
					{Name: "custom_addon_v2.wad", Aliases: []string{"custom_addon.wad"}},
				},
			},
		},
	}
	out.Reset()
	count3, err := DiscoverAndExtractForCatalog(customCat, []string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtractForCatalog failed: %v", err)
	}
	if count3 != 1 {
		t.Fatalf("expected 1 custom file extracted, got %d\nOutput: %s", count3, out.String())
	}
	if _, err := os.Stat(filepath.Join(destWads, "custom_addon_v2.wad")); err != nil {
		t.Errorf("custom_addon_v2.wad was not created from custom_addon.wad: %v", err)
	}
}
