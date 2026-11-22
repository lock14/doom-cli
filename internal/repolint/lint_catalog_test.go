package repolint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/catalog"
)

func TestCuratedCatalogInvariantsAndLint(t *testing.T) {
	t.Parallel()
	rootDir := filepath.Join("..", "..")
	catalogDir := filepath.Join(rootDir, "catalog")

	entries, err := os.ReadDir(catalogDir)
	if err != nil {
		t.Fatalf("reading %s: %v", catalogDir, err)
	}
	diskPresets := 0
	diskEngines := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "preset.") && strings.HasSuffix(name, ".json") {
			diskPresets++
		}
		if strings.HasPrefix(name, "engine.") && strings.HasSuffix(name, ".json") {
			diskEngines++
		}
	}
	if diskPresets == 0 || diskEngines == 0 {
		t.Fatalf("expected non-zero curated presets (%d) and engines (%d)", diskPresets, diskEngines)
	}

	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated failed: %v", err)
	}
	if len(cat.Presets) != diskPresets {
		t.Fatalf("embedded preset count = %d, on-disk preset.*.json count = %d", len(cat.Presets), diskPresets)
	}
	if len(cat.Engines) != diskEngines {
		t.Fatalf("embedded engine count = %d, on-disk engine.*.json count = %d", len(cat.Engines), diskEngines)
	}

	prevSort := 0
	for i, p := range cat.Presets {
		if p.Sort <= prevSort {
			t.Errorf("preset[%d] %s sort %d not strictly greater than %d", i, p.ID, p.Sort, prevSort)
		}
		prevSort = p.Sort
	}

	diags, err := LintCatalog(rootDir, "")
	if err != nil {
		t.Fatalf("LintCatalog failed: %v", err)
	}
	if HasErrors(diags) {
		for _, d := range diags {
			t.Errorf("lint finding: %s", d.String())
		}
	}
}

func TestLintCatalog_FileAliasesValidation(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()

	// Redundant alias, path-separator alias, cross-IWAD alias (HERETIC.WAD on DOOM2 preset),
	// and canonical target collision (scythe.wad)
	badPreset := `{
  "schema_version": 1,
  "kind": "preset",
  "id": "custom-alias-test",
  "name": "Custom Alias Test",
  "engine": "dsda-doom",
  "iwad": "DOOM2.WAD",
  "files": [
    {
      "name": "my_map.wad",
      "aliases": ["MY-MAP.WAD", "sub/other.wad", "HERETIC.WAD", "SCYTHE.WAD"]
    }
  ]
}`
	targetFile := filepath.Join(userDir, "preset.custom-alias-test.json")
	if err := os.WriteFile(targetFile, []byte(badPreset), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override with invalid alias
	badOverride := `{
  "schema_version": 1,
  "kind": "override",
  "target": { "kind": "preset", "id": "scythe" },
  "append_files": [
    {
      "name": "extra_mod.wad",
      "aliases": ["EXTRA-MOD.WAD"]
    }
  ]
}`
	ovFile := filepath.Join(userDir, "override.preset.scythe.json")
	if err := os.WriteFile(ovFile, []byte(badOverride), 0o644); err != nil {
		t.Fatal(err)
	}

	diags, err := LintCatalog("", userDir)
	if err != nil {
		t.Fatalf("LintCatalog error: %v", err)
	}
	if !HasErrors(diags) {
		t.Fatal("expected lint errors for invalid file aliases, got none")
	}

	var joined strings.Builder
	for _, d := range diags {
		joined.WriteString(d.Message + "\n")
	}
	out := joined.String()
	for _, wantSub := range []string{
		"redundant with normalized filename",
		"must not contain path separators",
		"must not match iwad \"HERETIC.WAD\"",
		"conflicts with canonical target \"scythe.wad\"",
		"file extra_mod.wad alias \"EXTRA-MOD.WAD\" is redundant",
	} {
		if !strings.Contains(out, wantSub) {
			t.Errorf("expected lint diagnostics to contain %q, got:\n%s", wantSub, out)
		}
	}
}

func TestLintCatalog_CuratedSHA256_IWADAlias_And_NonRepoSkip(t *testing.T) {
	t.Parallel()

	// 1. LintCatalog skips repo-level checks (README table parity, lintDocumentation) when <rootDir>/catalog is absent
	nonRepoDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(nonRepoDir, "README.md"),
		[]byte("# External project\n\n---\n\nEmoji: \U0001f41b and step A -> step B\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	nonRepoDiags, err := LintCatalog(nonRepoDir, "")
	if err != nil {
		t.Fatalf("LintCatalog on nonRepoDir: %v", err)
	}
	if len(nonRepoDiags) != 0 {
		t.Errorf("expected 0 diagnostics when <rootDir>/catalog does not exist, got %v", nonRepoDiags)
	}

	// 2. LintCatalog enforces curated SHA-256, normalized IWAD alias check, and JSON-escaped Windows path audit
	lintRoot := t.TempDir()
	lintCatDir := filepath.Join(lintRoot, "catalog")
	_ = os.MkdirAll(lintCatDir, 0o755)
	_ = os.WriteFile(
		filepath.Join(lintCatDir, "engine.dsda-doom.json"),
		[]byte(`{"schema_version":1,"kind":"engine","id":"dsda-doom","args_style":"boom"}`),
		0o644,
	)
	winEscaped := "C:" + `\\Users\\` + "alice"
	unpinnedCurated := `{
  "schema_version": 1,
  "kind": "preset",
  "id": "unpinned-test",
  "name": "Unpinned Test",
  "engine": "dsda-doom",
  "iwad": "DOOM2.WAD",
  "files": [{"name": "map.wad", "aliases": ["FREE_DOOM_2.WAD"]}],
  "category": "Community Megawad",
  "genre": "Action",
  "difficulty": "Moderate",
  "quality": "A-Tier",
  "author": "Tester",
  "release_date": "2024",
  "description": "` + winEscaped + `",
  "sources": [{"url": "https://example.com/map.zip"}],
  "links": [{"url": "https://example.com/info"}],
  "sort": 10
}`
	_ = os.WriteFile(filepath.Join(lintCatDir, "preset.unpinned-test.json"), []byte(unpinnedCurated), 0o644)
	missingGameHeretic := `{
  "schema_version": 1,
  "kind": "preset",
  "id": "heretic-no-game",
  "name": "Heretic No Game",
  "engine": "dsda-doom",
  "iwad": "HERETIC.WAD",
  "category": "Official IWAD",
  "genre": "Fantasy",
  "difficulty": "Moderate",
  "quality": "A-Tier",
  "author": "Raven",
  "release_date": "1994",
  "links": [{"url": "https://example.com/heretic"}],
  "sort": 20
}`
	_ = os.WriteFile(filepath.Join(lintCatDir, "preset.heretic-no-game.json"), []byte(missingGameHeretic), 0o644)
	conflictingGame := `{
  "schema_version": 1,
  "kind": "preset",
  "id": "conflict-game",
  "name": "Conflict Game",
  "game": "heretic",
  "engine": "dsda-doom",
  "iwad": "DOOM2.WAD",
  "category": "Official IWAD",
  "genre": "Action",
  "difficulty": "Moderate",
  "quality": "A-Tier",
  "author": "id",
  "release_date": "1994",
  "links": [{"url": "https://example.com/doom2"}],
  "sort": 30
}`
	_ = os.WriteFile(filepath.Join(lintCatDir, "preset.conflict-game.json"), []byte(conflictingGame), 0o644)
	diags, err := LintCatalog(lintRoot, "")
	if err != nil {
		t.Fatalf("LintCatalog: %v", err)
	}
	var diagText strings.Builder
	for _, d := range diags {
		diagText.WriteString(d.String() + "\n")
	}
	for _, want := range []string{
		"requires a non-empty sha256 digest",
		"must not match iwad \"FREE_DOOM_2.WAD\"",
		"contains hardcoded personal path",
		"must explicitly declare \"game\": \"heretic\"",
		"declares game \"heretic\" which conflicts with iwad \"DOOM2.WAD\"",
	} {
		if !strings.Contains(diagText.String(), want) {
			t.Errorf("expected lint output to contain %q, got:\n%s", want, diagText.String())
		}
	}
}
