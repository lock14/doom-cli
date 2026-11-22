package repolint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateReadmeTableAndSyncReadme(t *testing.T) {
	t.Parallel()

	headerOnly := GenerateReadmeTable(nil)
	if !strings.Contains(headerOnly, "| Megawad or expansion |") {
		t.Fatalf("expected header row for nil catalog, got %q", headerOnly)
	}

	tmpRoot := t.TempDir()
	catDir := filepath.Join(tmpRoot, "catalog")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatal(err)
	}
	engineJSON := `{
  "schema-version": 1,
  "kind": "engine",
  "id": "dsda-doom",
  "display-name": "DSDA-Doom",
  "args-style": "boom"
}`
	presetJSON := `{
  "schema-version": 1,
  "kind": "preset",
  "id": "sunlust",
  "name": "Sunlust",
  "engine": "dsda-doom",
  "iwad": "DOOM2.WAD",
  "files": [{"name": "sunlust.wad"}],
  "complevel": 9,
  "genre": "Slaughter",
  "difficulty": "Extreme",
  "quality": "S-Tier (9.5/10)",
  "description": "Precision combat megawad",
  "links": [{"url": "https://example.com/sunlust"}],
  "sort": 10
}`
	if err := os.WriteFile(filepath.Join(catDir, "engine.dsda-doom.json"), []byte(engineJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catDir, "preset.sunlust.json"), []byte(presetJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	readmePath := filepath.Join(tmpRoot, "README.md")
	initialReadme := strings.Join([]string{
		"# doom-cli",
		"",
		"## Preconfigured presets",
		"",
		"Curated table below:",
		"",
		"| stale |",
		"| :--- |",
		"",
		"## Engine profiles",
	}, "\n")
	if err := os.WriteFile(readmePath, []byte(initialReadme), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncReadme(tmpRoot); err != nil {
		t.Fatalf("SyncReadme failed: %v", err)
	}

	updatedRaw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	updated := string(updatedRaw)
	wantRow := "| [Sunlust](https://example.com/sunlust) | DSDA-Doom | Slaughter | Extreme | " +
		"S-Tier (9.5/10) | Precision combat megawad |"
	if !strings.Contains(updated, wantRow) {
		t.Errorf("expected updated README.md to contain %q, got:\n%s", wantRow, updated)
	}

	// Verify no .bak.* files remain in tmpRoot after SyncReadme
	entries, _ := os.ReadDir(tmpRoot)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".bak.") {
			t.Errorf("unexpected backup file left behind by SyncReadme: %s", entry.Name())
		}
	}
}
