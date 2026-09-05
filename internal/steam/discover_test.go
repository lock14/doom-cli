package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLibraryFolders(t *testing.T) {
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

func TestDiscoverAndExtract(t *testing.T) {
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

	// Create mock game files
	_ = os.WriteFile(filepath.Join(doomDir, "doom2.wad"), []byte("doom2-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "gd.wad"), []byte("gdturbo-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "DOOMZERO.DEH"), []byte("doomzero-deh-data"), 0644)

	destWads := filepath.Join(tmpDir, "target_wads")
	var out bytes.Buffer
	count, err := DiscoverAndExtract([]string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtract failed: %v", err)
	}

	if count != 3 {
		t.Fatalf("expected 3 files extracted, got %d\nOutput: %s", count, out.String())
	}

	// Verify target names:
	// doom2.wad -> DOOM2.WAD
	// gd.wad -> gdturbo.wad
	// DOOMZERO.DEH -> DOOMZERO.DEH
	if _, err := os.Stat(filepath.Join(destWads, "DOOM2.WAD")); err != nil {
		t.Errorf("DOOM2.WAD was not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destWads, "gdturbo.wad")); err != nil {
		t.Errorf("gdturbo.wad was not created from gd.wad: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destWads, "DOOMZERO.DEH")); err != nil {
		t.Errorf("DOOMZERO.DEH was not created: %v", err)
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
}
