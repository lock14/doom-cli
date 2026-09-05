package preset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}
	if len(cat.Presets) != 32 {
		t.Errorf("expected 32 presets, got %d", len(cat.Presets))
	}
}

func TestFind(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	// Exact match
	p := cat.Find("Eviternity II")
	if p == nil || p.Name != "Eviternity II" {
		t.Errorf("expected Eviternity II, got %v", p)
	}

	// Case-insensitive
	p = cat.Find("sunlust")
	if p == nil || p.Name != "Sunlust" {
		t.Errorf("expected Sunlust, got %v", p)
	}

	// Prefix
	p = cat.Find("Alien Vend")
	if p == nil || p.Name != "Alien Vendetta" {
		t.Errorf("expected Alien Vendetta, got %v", p)
	}
}

func TestResolveFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wads_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create spaced file
	spacedFile := filepath.Join(tmpDir, "Eviternity II.wad")
	if err := os.WriteFile(spacedFile, []byte("wad"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test normalized matching
	path, found := ResolveFile(tmpDir, "eviternityii.wad")
	if !found || filepath.Base(path) != "Eviternity II.wad" {
		t.Errorf("expected to find Eviternity II.wad, got %s, found=%v", path, found)
	}

	// Create alias file
	gdFile := filepath.Join(tmpDir, "gd.wad")
	if err := os.WriteFile(gdFile, []byte("wad"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	path, found = ResolveFile(tmpDir, "gdturbo.wad")
	if !found || filepath.Base(path) != "gd.wad" {
		t.Errorf("expected to find gd.wad for gdturbo.wad, got %s, found=%v", path, found)
	}
}
