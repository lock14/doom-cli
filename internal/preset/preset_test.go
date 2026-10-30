package preset

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

	cat, err := LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog failed: %v", err)
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

func TestDecodeEntityStrictValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid preset with UTF-8 BOM",
			raw: "\xef\xbb\xbf{\"schema_version\":1,\"kind\":\"preset\",\"id\":\"my-wad\"," +
				"\"name\":\"My WAD\",\"engine\":\"dsda-doom\",\"iwad\":\"DOOM2.WAD\"}",
			wantErr: "",
		},
		{
			name: "unknown field rejected",
			raw: `{"kind":"preset","id":"my-wad","name":"My WAD",` +
				`"engine":"dsda-doom","iwad":"DOOM2.WAD","mappacks":["a.wad"]}`,
			wantErr: "unknown field",
		},
		{
			name: "schema key rejected",
			raw: `{"$schema":"foo","kind":"preset","id":"my-wad",` +
				`"name":"My WAD","engine":"dsda-doom","iwad":"DOOM2.WAD"}`,
			wantErr: "unknown field",
		},
		{
			name: "unsupported schema_version",
			raw: `{"schema_version":2,"kind":"preset","id":"my-wad",` +
				`"name":"My WAD","engine":"dsda-doom","iwad":"DOOM2.WAD"}`,
			wantErr: "unsupported schema_version 2",
		},
		{
			name: "trailing token rejected",
			raw: `{"kind":"preset","id":"my-wad","name":"My WAD",` +
				`"engine":"dsda-doom","iwad":"DOOM2.WAD"} {"extra":1}`,
			wantErr: "after top-level value",
		},
		{
			name:    "invalid id uppercase",
			raw:     `{"kind":"preset","id":"My-Wad","name":"My WAD","engine":"dsda-doom","iwad":"DOOM2.WAD"}`,
			wantErr: "invalid id",
		},
		{
			name:    "invalid engine args_style",
			raw:     `{"kind":"engine","id":"woof","args_style":"invalid"}`,
			wantErr: "args_style",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeEntity([]byte(tc.raw))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got error %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestStoreLifecycleAndQuarantine(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	curated, err := LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog: %v", err)
	}

	// 1. Refuse curated engine collision
	if _, err := AddEngine(userDir, curated, EngineConfig{ID: "dsda-doom", Binary: "dsda-doom"}); err == nil {
		t.Fatal("expected AddEngine to reject curated engine collision")
	}

	// 2. Add custom engine Woof
	if _, err := AddEngine(userDir, curated, EngineConfig{
		ID:          "woof",
		DisplayName: "Woof!",
		Binary:      "/usr/local/bin/woof",
		ArgsStyle:   "boom",
		Badge:       "WOOF",
		Description: "Woof MBF21 port",
	}); err != nil {
		t.Fatalf("AddEngine(woof): %v", err)
	}

	layered, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}

	// 3. Refuse curated preset collision
	if _, err := AddPreset(userDir, curated, layered, Preset{
		Name:   "Doom-II",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
	}); err == nil {
		t.Fatal("expected AddPreset(Doom-II) to reject curated slug collision with doom-ii")
	}

	// 4. Add user preset and reject distinct-name slug collision
	if _, err := AddPreset(userDir, curated, layered, Preset{
		Name:   "Foo Bar",
		Engine: "woof",
		IWAD:   "DOOM2.WAD",
		Files:  []FileSpec{{Name: "foobar.wad"}},
	}); err != nil {
		t.Fatalf("AddPreset(Foo Bar): %v", err)
	}
	if _, err := AddPreset(userDir, curated, layered, Preset{
		Name:   "Foo-Bar",
		Engine: "woof",
		IWAD:   "DOOM2.WAD",
		Files:  []FileSpec{{Name: "foobar2.wad"}},
	}); err == nil {
		t.Fatal("expected AddPreset(Foo-Bar) to reject slug collision with Foo Bar")
	}

	// 5. Override preservation across multiple ConfigPreset calls (Defect #5 fix)
	engChoice := "woof"
	if _, _, err := ConfigPreset(userDir, layered, "Sunlust", OverridePatchRequest{
		Engine: &engChoice,
	}); err != nil {
		t.Fatalf("first ConfigPreset: %v", err)
	}
	argsChoice := []string{"-skill", "4"}
	if _, _, err := ConfigPreset(userDir, layered, "sunlust", OverridePatchRequest{
		AppendArgs: &argsChoice,
	}); err != nil {
		t.Fatalf("second ConfigPreset (lowercase): %v", err)
	}
	filesChoice := []FileSpec{{Name: "hud.wad", Optional: true}}
	if _, _, err := ConfigPreset(userDir, layered, "SUNLUST", OverridePatchRequest{
		AppendFiles: &filesChoice,
	}); err != nil {
		t.Fatalf("third ConfigPreset (uppercase): %v", err)
	}

	// 6. Write a malformed entity file in userDir and verify quarantine fault isolation
	badFile := filepath.Join(userDir, "preset.broken.json")
	if err := os.WriteFile(badFile, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("WriteFile broken: %v", err)
	}

	layeredAfter, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog with quarantined file should succeed, got: %v", err)
	}
	if len(layeredAfter.Diagnostics) == 0 {
		t.Fatal("expected quarantine diagnostic for preset.broken.json")
	}

	sunlust := layeredAfter.Find("sunlust")
	if sunlust == nil {
		t.Fatal("sunlust not found after reload")
	}
	if sunlust.Engine != "woof" {
		t.Fatalf("expected sunlust engine override 'woof' to be preserved, got %q", sunlust.Engine)
	}
	if len(sunlust.Args) != 2 || sunlust.Args[0] != "-skill" || sunlust.Args[1] != "4" {
		t.Fatalf("expected sunlust args [-skill 4], got %v", sunlust.Args)
	}
	if len(sunlust.Files) != 2 || sunlust.Files[1].Name != "hud.wad" {
		t.Fatalf("expected sunlust appended file hud.wad, got %v", sunlust.Files)
	}
}

func TestResolveFileSubpathsMultiRootAndSubdirs(t *testing.T) {
	t.Parallel()
	root0 := t.TempDir()
	root1 := t.TempDir()

	// Create sub/foo.wad in root0 (Regression 1 fix)
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

	cat, err := LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog: %v", err)
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

func TestDecodeTextCP437(t *testing.T) {
	t.Parallel()
	ascii := []byte("Simple ASCII text\r\nLine 2")
	if got := DecodeText(ascii); got != "Simple ASCII text\nLine 2" {
		t.Fatalf("unexpected ASCII decode: %q", got)
	}
	cp437 := []byte{0xc9, 0xcd, 0xbb} // ╔═╗ in CP437
	if got := DecodeText(cp437); got != "╔═╗" {
		t.Fatalf("unexpected CP437 decode: %q", got)
	}
}

func TestLintDocumentation_ViolationsDetected(t *testing.T) {
	t.Parallel()
	tmpRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpRoot, "catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	sampleUserPath := "/Users/" + "johndoe"

	badContributing := strings.Join([]string{
		"# Contributing",
		"",
		"Path `" + sampleUserPath + "/doom` and **bold `code`**.",
		"Second stacked line: **[link](missing.md)**.",
		"*(Note: parenthetical aside)*",
		"Emoji: \U0001f41b and :white_check_mark:",
		"Prose arrow: step A -> step B",
		"Second arrow: step C => step D",
		"",
		"| Column | Value |",
		"| :--- | :--- |",
		"| **Bold Cell** | Val |",
		"| **Second Bold Cell** | Val2 |",
		"",
		"---",
		"",
		"[Broken anchor](#nonexistent-heading)",
		"[Broken titled link](missing_titled.md \"Title\")",
		"",
		"```json",
		"{invalid json}",
		"```",
	}, "\n")

	if err := os.WriteFile(filepath.Join(tmpRoot, "CONTRIBUTING.md"), []byte(badContributing), 0o644); err != nil {
		t.Fatal(err)
	}

	wfDir := filepath.Join(tmpRoot, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	badBenchmark := "echo \"## \U0001f4ca Benchmark\" && ls " + sampleUserPath + "/doom\n"
	if err := os.WriteFile(filepath.Join(wfDir, "benchmark.yml"), []byte(badBenchmark), 0o644); err != nil {
		t.Fatal(err)
	}

	diags, err := LintCatalog(tmpRoot, "")
	if err != nil {
		t.Fatalf("LintCatalog error: %v", err)
	}

	var joined strings.Builder
	for _, d := range diags {
		joined.WriteString(d.String() + "\n")
	}
	out := joined.String()

	for _, wantSub := range []string{
		"contains personal path",
		"contains decorative emoji or shortcode",
		"contains stacked bold span \"**bold `code`**\"",
		"contains stacked bold span \"**[link](missing.md)**\"",
		"contains italic parenthetical aside",
		"contains bolded first-column table cell \"| **Bold Cell** |\"",
		"contains bolded first-column table cell \"| **Second Bold Cell** |\"",
		"contains ASCII arrow \"->\"",
		"contains ASCII arrow \"=>\"",
		"contains horizontal rule divider '---'",
		"broken relative file link \"missing.md\"",
		"broken relative file link \"missing_titled.md\"",
		"broken anchor link \"#nonexistent-heading\"",
		"invalid JSON code block",
		"benchmark.yml: contains decorative emoji or shortcode \"\U0001f4ca\"",
		"benchmark.yml: contains personal path \"" + sampleUserPath + "\"",
	} {
		if !strings.Contains(out, wantSub) {
			t.Errorf("expected lint diagnostics to contain %q, got:\n%s", wantSub, out)
		}
	}
}

func TestLintDocumentation_EdgeCasesAndStackedFormatting(t *testing.T) {
	t.Parallel()
	tmpRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpRoot, "catalog"), 0o755); err != nil {
		t.Fatal(err)
	}

	parenFile := filepath.Join(tmpRoot, "file(1).md")
	if err := os.WriteFile(parenFile, []byte("# Paren file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	validContributing := strings.Join([]string{
		"# Contributing",
		"",
		"<!-- HTML comment should not trigger arrow lint -->",
		"",
		"## Diátaxis config_file",
		"",
		"### Examples",
		"",
		"- **First label**: use `doom play` and **Second label**: see [Contributing](CONTRIBUTING.md).",
		"- **Inline code**: do not write `*(Note: aside)*`, ``**`code`**``, `->`, or `<->` in prose.",
		"- **Escaped link**: see [Paren](file\\(1\\).md), [Duplicate](#examples-1), " +
			"and [Slug](#diátaxis-config_file).",
		"",
		"| Column | Value |",
		"| :--- | :--- |",
		"| Plain cell | Val |",
		"",
		"### Examples",
		"",
		"```markdown",
		"**`code inside fenced block`**",
		"*(Note: inside fenced block)*",
		"| **Bold inside fenced block** | Val |",
		"a -> b <-> c",
		"---",
		"```",
	}, "\n")

	if err := os.WriteFile(filepath.Join(tmpRoot, "CONTRIBUTING.md"), []byte(validContributing), 0o644); err != nil {
		t.Fatal(err)
	}

	diags, err := LintCatalog(tmpRoot, "")
	if err != nil {
		t.Fatalf("LintCatalog error: %v", err)
	}
	for _, d := range diags {
		if strings.HasSuffix(d.Path, "CONTRIBUTING.md") {
			t.Errorf("unexpected lint diagnostic on valid CONTRIBUTING.md: %s", d.String())
		}
	}

	stackedCases := []struct {
		name      string
		line      string
		wantMatch string
	}{
		{
			name:      "bold wrapping code",
			line:      "Use **`doom play`** here",
			wantMatch: "**`doom play`**",
		},
		{
			name:      "bold wrapping link",
			line:      "See **[DSDA-Doom](https://example.com)**",
			wantMatch: "**[DSDA-Doom](https://example.com)**",
		},
		{
			name:      "link wrapping bold",
			line:      "See [**DSDA-Doom**](https://example.com)",
			wantMatch: "[**DSDA-Doom**](https://example.com)",
		},
		{
			name:      "triple asterisk bold italics",
			line:      "Avoid ***bold italics*** in prose",
			wantMatch: "***bold italics***",
		},
		{
			name:      "bold wrapping underscore italics",
			line:      "Avoid **_bold italics_** in prose",
			wantMatch: "**_bold italics_**",
		},
		{
			name:      "underscore italics wrapping bold",
			line:      "Avoid _**bold italics**_ in prose",
			wantMatch: "_**bold italics**_",
		},
		{
			name:      "two separate bold spans with code between",
			line:      "- **First**: run `doom` and **Second**: done.",
			wantMatch: "",
		},
	}

	for _, tc := range stackedCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := findStackedFormatting(tc.line)
			if got != tc.wantMatch {
				t.Errorf("findStackedFormatting(%q) = %q, want %q", tc.line, got, tc.wantMatch)
			}
		})
	}
}

func TestDocumentationEntityJSONBlocks(t *testing.T) {
	t.Parallel()
	rootDir := filepath.Join("..", "..")
	for _, relDoc := range []string{"README.md", "CONTRIBUTING.md"} {
		relDoc := relDoc
		t.Run(relDoc, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join(rootDir, relDoc))
			if err != nil {
				t.Fatalf("reading %s: %v", relDoc, err)
			}
			lines := strings.Split(string(raw), "\n")
			inJSON := false
			var buf strings.Builder
			entityCount := 0
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "```") {
					if !inJSON {
						lang := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
						if lang == "json" {
							inJSON = true
							buf.Reset()
						}
					} else {
						inJSON = false
						block := buf.String()
						if strings.Contains(block, `"kind"`) {
							entityCount++
							if _, decErr := DecodeEntity([]byte(block)); decErr != nil {
								t.Errorf("%s entity JSON block failed DecodeEntity: %v\n%s", relDoc, decErr, block)
							}
						}
					}
					continue
				}
				if inJSON {
					buf.WriteString(line)
					buf.WriteByte('\n')
				}
			}
			if entityCount == 0 {
				t.Errorf("expected at least one entity JSON block in %s", relDoc)
			}
		})
	}
}

func TestMonospaceTUIOverridesNotStale(t *testing.T) {
	t.Parallel()
	rootDir := filepath.Join("..", "..")
	overridesPath := filepath.Join(rootDir, ".agents", "rules", "monospace-tui.md")
	overridesRaw, err := os.ReadFile(overridesPath)
	if err != nil {
		t.Fatalf("reading %s: %v", overridesPath, err)
	}
	stdRaw, err := os.ReadFile(
		filepath.Join(rootDir, ".agents", "skills", "mono-tui-design", "references", "standard.md"),
	)
	if err != nil {
		t.Fatalf("reading standard.md: %v", err)
	}
	rendRaw, err := os.ReadFile(
		filepath.Join(rootDir, ".agents", "skills", "mono-tui-design", "references", "rendering.md"),
	)
	if err != nil {
		t.Fatalf("reading rendering.md: %v", err)
	}
	patRaw, err := os.ReadFile(
		filepath.Join(rootDir, ".agents", "skills", "mono-tui-design", "references", "patterns.md"),
	)
	if err != nil {
		t.Fatalf("reading patterns.md: %v", err)
	}
	corpus := string(stdRaw) + "\n" + string(rendRaw) + "\n" + string(patRaw)

	var currentHeading string
	var currentHeadingLine int
	currentHasOriginal := false
	checked := 0
	overrideHeadings := 0

	checkPreviousHeading := func() {
		if currentHeading == "" {
			return
		}
		if (strings.HasPrefix(currentHeading, "[OVERRIDE]") || strings.HasPrefix(currentHeading, "[WAIVE]")) &&
			!currentHasOriginal {
			t.Errorf(
				"%s:%d (%s): missing **Original**: quote for override or waiver heading",
				overridesPath, currentHeadingLine, currentHeading,
			)
		}
	}

	for idx, line := range strings.Split(string(overridesRaw), "\n") {
		lineNum := idx + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			checkPreviousHeading()
			currentHeading = strings.TrimPrefix(trimmed, "### ")
			currentHeadingLine = lineNum
			currentHasOriginal = false
			if strings.HasPrefix(currentHeading, "[OVERRIDE]") || strings.HasPrefix(currentHeading, "[WAIVE]") {
				overrideHeadings++
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			checkPreviousHeading()
			currentHeading = ""
			currentHeadingLine = 0
			currentHasOriginal = false
			continue
		}
		bulletBody := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "*"), "-"))
		var quoted string
		switch {
		case strings.HasPrefix(bulletBody, "**Original**: "):
			quoted = strings.TrimSpace(strings.TrimPrefix(bulletBody, "**Original**: "))
		case strings.HasPrefix(bulletBody, "**Original:** "):
			quoted = strings.TrimSpace(strings.TrimPrefix(bulletBody, "**Original:** "))
		default:
			continue
		}
		checked++
		currentHasOriginal = true
		if quoted == "" {
			t.Errorf("%s:%d (%s): empty Original field", overridesPath, lineNum, currentHeading)
			continue
		}
		if !strings.Contains(corpus, quoted) {
			t.Errorf(
				"%s:%d (%s): Original text not found verbatim in standard.md, rendering.md, or patterns.md:\n  %s",
				overridesPath, lineNum, currentHeading, quoted,
			)
		}
	}
	checkPreviousHeading()

	if overrideHeadings == 0 {
		t.Fatalf("expected at least one [OVERRIDE] or [WAIVE] heading in %s, found 0", overridesPath)
	}
	if checked == 0 {
		t.Fatalf("expected at least one Original entry in %s, found 0", overridesPath)
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

func TestConfigEngineAndResetEngineOverride(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	cat, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}

	// 1. Configure binary and badge on dsda-doom
	bin := "/opt/doom/dsda-doom"
	badge := "DSDA+"
	ovPath, err := ConfigEngine(userDir, cat, "dsda-doom", EngineOverrideOptions{
		Binary: &bin,
		Badge:  &badge,
	})
	if err != nil {
		t.Fatalf("ConfigEngine first call: %v", err)
	}
	if !strings.HasSuffix(ovPath, "override.engine.dsda-doom.json") {
		t.Errorf("unexpected override path: %s", ovPath)
	}

	// 2. Configure append_args and prefix_args in a second call; prior binary and badge must be preserved
	appendArgs := []string{"-complevel", "21"}
	prefixArgs := []string{"flatpak", "run"}
	if _, err := ConfigEngine(userDir, cat, "dsda", EngineOverrideOptions{
		AppendArgs: &appendArgs,
		PrefixArgs: &prefixArgs,
	}); err != nil {
		t.Fatalf("ConfigEngine second call: %v", err)
	}

	merged, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog after override: %v", err)
	}
	eng, ok := merged.FindEngine("dsda-doom")
	if !ok {
		t.Fatal("dsda-doom not found in merged catalog")
	}
	if eng.Binary != bin || eng.Badge != badge {
		t.Errorf("expected preserved Binary=%q Badge=%q, got Binary=%q Badge=%q", bin, badge, eng.Binary, eng.Badge)
	}
	if !reflect.DeepEqual(eng.PrefixArgs, prefixArgs) {
		t.Errorf("expected PrefixArgs=%v, got %v", prefixArgs, eng.PrefixArgs)
	}
	if len(eng.DefaultArgs) < 2 || eng.DefaultArgs[len(eng.DefaultArgs)-1] != "21" {
		t.Errorf("expected AppendArgs appended to DefaultArgs, got %v", eng.DefaultArgs)
	}

	// 2b. Clear default_args, prefix_args, and badge back to empty and verify round-trip
	emptySlice := []string{}
	emptyStr := ""
	if _, err := ConfigEngine(userDir, cat, "dsda-doom", EngineOverrideOptions{
		DefaultArgs: &emptySlice,
		PrefixArgs:  &emptySlice,
		AppendArgs:  &emptySlice,
		Badge:       &emptyStr,
	}); err != nil {
		t.Fatalf("ConfigEngine clear fields: %v", err)
	}
	mergedCleared, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog after clearing fields: %v", err)
	}
	engCleared, ok := mergedCleared.FindEngine("dsda-doom")
	if !ok {
		t.Fatal("dsda-doom not found after clearing fields")
	}
	if len(engCleared.DefaultArgs) != 0 || len(engCleared.PrefixArgs) != 0 || engCleared.Badge != "" {
		t.Errorf(
			"expected cleared DefaultArgs=[], PrefixArgs=[], Badge=\"\", got DefaultArgs=%v PrefixArgs=%v Badge=%q",
			engCleared.DefaultArgs, engCleared.PrefixArgs, engCleared.Badge,
		)
	}

	// 3. Invalid style rejected in ConfigEngine and DecodeEntity
	badStyle := "quake"
	if _, err := ConfigEngine(userDir, cat, "dsda-doom", EngineOverrideOptions{ArgsStyle: &badStyle}); err == nil {
		t.Error("expected error for invalid args_style, got nil")
	}
	if _, err := DecodeEntity([]byte(
		`{"kind":"override","target":{"kind":"engine","id":"dsda-doom"},"set":{"args_style":"quake"}}`,
	)); err == nil {
		t.Error("expected DecodeEntity to reject invalid override set.args_style")
	}

	// 4. ResetEngineOverride removes override file and backups
	if err := ResetEngineOverride(userDir, merged, "dsda-doom"); err != nil {
		t.Fatalf("ResetEngineOverride: %v", err)
	}
	if _, statErr := os.Stat(ovPath); !os.IsNotExist(statErr) {
		t.Errorf("expected override file removed, statErr=%v", statErr)
	}
	entries, _ := os.ReadDir(userDir)
	for _, e := range entries {
		if strings.Contains(e.Name(), "override.engine.dsda-doom.json") {
			t.Errorf("unexpected leftover override or backup file: %s", e.Name())
		}
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

func TestConfigPreset_ExcludeFilesAndBaseAndOverride(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	cat, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}

	// Exclude idkfa 2024.wad on doom-ii via ConfigPreset
	excluded := []string{"IDKFA_2024.WAD"}
	extra := []string{"-skill", "4"}
	if _, _, err := ConfigPreset(userDir, cat, "doom-ii", OverridePatchRequest{
		ExcludeFiles: &excluded,
		AppendArgs:   &extra,
	}); err != nil {
		t.Fatalf("ConfigPreset: %v", err)
	}

	// Verify LoadPresetBaseAndOverride returns unfiltered base files and stored override
	basePreset, ov, err := LoadPresetBaseAndOverride(userDir, "doom-ii")
	if err != nil {
		t.Fatalf("LoadPresetBaseAndOverride: %v", err)
	}
	if basePreset == nil || len(basePreset.Files) == 0 {
		t.Fatalf("expected basePreset.Files to contain idkfa 2024.wad, got %+v", basePreset)
	}
	if ov == nil || len(ov.ExcludeFiles) != 1 || ov.ExcludeFiles[0] != "IDKFA_2024.WAD" {
		t.Fatalf("expected ov.ExcludeFiles=[IDKFA_2024.WAD], got %+v", ov)
	}

	// Verify MergeUserDir and LoadLayeredCatalog filter out idkfa 2024.wad
	merged, err := MergeUserDir(cat, userDir)
	if err != nil {
		t.Fatalf("MergeUserDir: %v", err)
	}
	p, err := merged.FindExact("doom-ii")
	if err != nil {
		t.Fatalf("doom-ii not found in merged catalog: %v", err)
	}
	for _, f := range p.Files {
		if NormalizeFilename(f.Name) == NormalizeFilename("idkfa 2024.wad") {
			t.Errorf("expected idkfa 2024.wad to be excluded from merged preset files, got %+v", p.Files)
		}
	}
	if !reflect.DeepEqual(p.Args, []string{"-skill", "4"}) {
		t.Errorf("expected p.Args=[-skill 4], got %v", p.Args)
	}

	// Verify empty exclude_files clears the exclusion while preserving AppendArgs
	emptyExcluded := []string{}
	if _, _, err := ConfigPreset(userDir, merged, "doom-ii", OverridePatchRequest{
		ExcludeFiles: &emptyExcluded,
	}); err != nil {
		t.Fatalf("ConfigPreset clear exclude_files: %v", err)
	}
	reloaded, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog after clearing exclude_files: %v", err)
	}
	pReloaded, err := reloaded.FindExact("doom-ii")
	if err != nil {
		t.Fatalf("doom-ii not found in reloaded catalog: %v", err)
	}
	if len(pReloaded.Files) == 0 {
		t.Errorf("expected idkfa 2024.wad restored after clearing exclude_files, got %+v", pReloaded.Files)
	}
	if !reflect.DeepEqual(pReloaded.Args, []string{"-skill", "4"}) {
		t.Errorf("expected AppendArgs preserved when clearing exclude_files, got %v", pReloaded.Args)
	}
}

func TestMergeUserDir_IdempotentOnAlreadyMergedCatalog(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	cat, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}

	excluded := []string{"idkfa 2024.wad"}
	appendArgs := []string{"-skill", "4"}
	if _, _, err := ConfigPreset(userDir, cat, "doom-ii", OverridePatchRequest{
		ExcludeFiles: &excluded,
		AppendArgs:   &appendArgs,
	}); err != nil {
		t.Fatalf("ConfigPreset: %v", err)
	}

	engAppend := []string{"-complevel", "21"}
	if _, err := ConfigEngine(userDir, cat, "dsda-doom", EngineOverrideOptions{
		AppendArgs: &engAppend,
	}); err != nil {
		t.Fatalf("ConfigEngine: %v", err)
	}

	// Simulate starting TUI with an already-merged catalog from LoadLayeredCatalog
	alreadyMerged, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog with overrides: %v", err)
	}

	// Merging userDir again onto alreadyMerged must not duplicate append_args
	reMerged, err := MergeUserDir(alreadyMerged, userDir)
	if err != nil {
		t.Fatalf("MergeUserDir on alreadyMerged: %v", err)
	}
	p, err := reMerged.FindExact("doom-ii")
	if err != nil {
		t.Fatalf("FindExact doom-ii: %v", err)
	}
	if !reflect.DeepEqual(p.Args, []string{"-skill", "4"}) {
		t.Errorf("expected idempotent p.Args=[-skill 4], got %v", p.Args)
	}
	eng, ok := reMerged.FindEngine("dsda-doom")
	if !ok {
		t.Fatal("dsda-doom not found")
	}
	if !reflect.DeepEqual(eng.DefaultArgs, []string{"-complevel", "21"}) {
		t.Errorf("expected idempotent eng.DefaultArgs=[-complevel 21], got %v", eng.DefaultArgs)
	}

	// Resetting overrides and calling MergeUserDir(alreadyMerged, userDir) must restore curated defaults
	if _, _, err := ResetPresetOverride(userDir, reMerged, "doom-ii"); err != nil {
		t.Fatalf("ResetPresetOverride: %v", err)
	}
	if err := ResetEngineOverride(userDir, reMerged, "dsda-doom"); err != nil {
		t.Fatalf("ResetEngineOverride: %v", err)
	}
	afterReset, err := MergeUserDir(alreadyMerged, userDir)
	if err != nil {
		t.Fatalf("MergeUserDir after reset: %v", err)
	}
	pReset, err := afterReset.FindExact("doom-ii")
	if err != nil {
		t.Fatalf("FindExact doom-ii after reset: %v", err)
	}
	if len(pReset.Args) != 0 || len(pReset.Files) == 0 || pReset.Overridden {
		t.Errorf("expected curated doom-ii restored after reset, got Args=%v Files=%v Overridden=%v",
			pReset.Args, pReset.Files, pReset.Overridden)
	}
	engReset, _ := afterReset.FindEngine("dsda-doom")
	if len(engReset.DefaultArgs) != 0 || engReset.Overridden {
		t.Errorf("expected curated dsda-doom restored after reset, got DefaultArgs=%v Overridden=%v",
			engReset.DefaultArgs, engReset.Overridden)
	}

	// Single-file catalog path must be rejected on store writes with a clear error
	singleFile := filepath.Join(userDir, "custom.json")
	if err := os.WriteFile(singleFile, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ConfigPreset(singleFile, cat, "doom-ii", OverridePatchRequest{
		AppendArgs: &appendArgs,
	}); err == nil || !strings.Contains(err.Error(), "single file") {
		t.Errorf("expected single file error for single-file ConfigPreset, got %v", err)
	}
}

func TestLookupNormalization_EngineDisplayName_And_RemovePresetCurated(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	curated, err := LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog: %v", err)
	}

	// 1. Find and FindExact resolve Roman numeral, WAD stem, implicit doom prefix, and initialism queries
	for _, tc := range []struct {
		query  string
		wantID string
	}{
		{query: "doom2", wantID: "doom-ii"},
		{query: "doom_ii", wantID: "doom-ii"},
		{query: "doom2.wad", wantID: "doom-ii"},
		{query: "II", wantID: "doom-ii"},
		{query: "ii", wantID: "doom-ii"},
		{query: "2", wantID: "doom-ii"},
		{query: "I", wantID: "doom"},
		{query: "i", wantID: "doom"},
		{query: "1", wantID: "doom"},
		{query: "doom1", wantID: "doom"},
		{query: "doom1.wad", wantID: "doom"},
		{query: "zero", wantID: "doom-zero"},
		{query: "sigil2", wantID: "sigil-ii"},
		{query: "eviternity2", wantID: "eviternity-ii"},
		{query: "sunlust.wad", wantID: "sunlust"},
		{query: "plutonia", wantID: "the-plutonia-experiment"},
		{query: "plutonia.wad", wantID: "the-plutonia-experiment"},
		{query: "pl2", wantID: "plutonia-2"},
		{query: "pl2.wad", wantID: "plutonia-2"},
		{query: "tnt", wantID: "tnt-evilution"},
		{query: "tnt.wad", wantID: "tnt-evilution"},
		{query: "evilution", wantID: "tnt-evilution"},
		{query: "revilution", wantID: "tnt-revilution"},
		{query: "tntr", wantID: "tnt-revilution"},
		{query: "tntr.wad", wantID: "tnt-revilution"},
		{query: "av", wantID: "alien-vendetta"},
		{query: "av.wad", wantID: "alien-vendetta"},
		{query: "aa", wantID: "ancient-aliens"},
		{query: "aaliens", wantID: "ancient-aliens"},
		{query: "aaliens.wad", wantID: "ancient-aliens"},
		{query: "btsx1", wantID: "back-to-saturn-x-episode-1"},
		{query: "btsx_e1a.wad", wantID: "back-to-saturn-x-episode-1"},
		{query: "btsx2", wantID: "back-to-saturn-x-episode-2"},
		{query: "btsx_e2a.wad", wantID: "back-to-saturn-x-episode-2"},
		{query: "dtwid", wantID: "doom-the-way-id-did"},
		{query: "dtwid.wad", wantID: "doom-the-way-id-did"},
		{query: "d2twid", wantID: "doom-ii-the-way-id-did"},
		{query: "d2twid.wad", wantID: "doom-ii-the-way-id-did"},
		{query: "Doom 2: The Way ID Did", wantID: "doom-ii-the-way-id-did"},
		{query: "gdturbo", wantID: "going-down-turbo"},
		{query: "gdt", wantID: "going-down-turbo"},
		{query: "gd", wantID: "going-down-turbo"},
		{query: "going-down", wantID: "going-down-turbo"},
		{query: "id1", wantID: "legacy-of-rust"},
		{query: "id1.wad", wantID: "legacy-of-rust"},
		{query: "lor", wantID: "legacy-of-rust"},
		{query: "rust", wantID: "legacy-of-rust"},
		{query: "neis", wantID: "no-end-in-sight"},
		{query: "neis.wad", wantID: "no-end-in-sight"},
		{query: "nerve", wantID: "no-rest-for-the-living"},
		{query: "nerve.wad", wantID: "no-rest-for-the-living"},
		{query: "nrftl", wantID: "no-rest-for-the-living"},
		{query: "sod", wantID: "speed-of-doom"},
		{query: "sodfinal", wantID: "speed-of-doom"},
		{query: "sodfinal.wad", wantID: "speed-of-doom"},
	} {
		p := curated.Find(tc.query)
		if p == nil || p.ID != tc.wantID {
			t.Errorf("curated.Find(%q) = %+v, want ID %q", tc.query, p, tc.wantID)
		}
		pExact, exactErr := curated.FindExact(tc.query)
		if exactErr != nil || pExact == nil || pExact.ID != tc.wantID {
			t.Errorf("curated.FindExact(%q) = (%+v, %v), want ID %q", tc.query, pExact, exactErr, tc.wantID)
		}
	}

	// 1b. Ambiguous series queries resolve to the first episode in Find, but return an ambiguity error in FindExact
	if p := curated.Find("btsx"); p == nil || p.ID != "back-to-saturn-x-episode-1" {
		t.Errorf("curated.Find(\"btsx\") = %+v, want back-to-saturn-x-episode-1", p)
	}
	if _, err := curated.FindExact("btsx"); err == nil || !strings.Contains(err.Error(), "is ambiguous") {
		t.Errorf("expected curated.FindExact(\"btsx\") to return ambiguity error, got %v", err)
	}

	// 2. Add custom engine "Woof!" and verify FindEngine matches DisplayName "Woof!" and FormatLabel
	if _, err := AddEngine(userDir, curated, EngineConfig{
		ID:          "woof",
		DisplayName: "Woof!",
		Binary:      "woof",
		ArgsStyle:   "boom",
		Description: "MBF21 port",
	}); err != nil {
		t.Fatalf("AddEngine(Woof!): %v", err)
	}
	cat, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}
	woof, ok := cat.FindEngine("Woof!")
	if !ok || woof.ID != "woof" {
		t.Fatalf("cat.FindEngine(\"Woof!\") = (%+v, %v), want woof", woof, ok)
	}
	if got := woof.FormatLabel(); got != "Woof! (MBF21 port)" {
		t.Errorf("woof.FormatLabel() = %q, want %q", got, "Woof! (MBF21 port)")
	}
	dsda, ok := cat.FindEngine("DSDA-Doom")
	if !ok || dsda.ID != "dsda-doom" {
		t.Fatalf("cat.FindEngine(\"DSDA-Doom\") = (%+v, %v), want dsda-doom", dsda, ok)
	}
	if got := dsda.FormatLabel(); got != "DSDA-Doom (MBF21 and speedrunning)" {
		t.Errorf("dsda.FormatLabel() = %q, want %q", got, "DSDA-Doom (MBF21 and speedrunning)")
	}
	for _, tc := range []struct {
		disp string
		want string
	}{
		{disp: "DSDA", want: "DSDA (MBF21 and speedrunning)"},
		{disp: "DSDA-DOOM", want: "DSDA-DOOM (MBF21 and speedrunning)"},
		{disp: "Custom DSDA", want: "Custom DSDA (MBF21 and speedrunning)"},
	} {
		customDSDA := dsda
		customDSDA.DisplayName = tc.disp
		if got := customDSDA.FormatLabel(); got != tc.want {
			t.Errorf("dsda with DisplayName=%q FormatLabel() = %q, want %q", tc.disp, got, tc.want)
		}
	}

	// 3. RemovePreset rejects built-in curated presets even when an override exists or curated is nil
	engChoice := "uzdoom"
	if _, _, err := ConfigPreset(userDir, cat, "doom2", OverridePatchRequest{Engine: &engChoice}); err != nil {
		t.Fatalf("ConfigPreset(doom2): %v", err)
	}
	catWithOv, err := LoadLayeredCatalog(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog: %v", err)
	}
	if _, err := RemovePreset(userDir, curated, catWithOv, "doom2"); err == nil ||
		!strings.Contains(err.Error(), "cannot remove built-in curated preset") {
		t.Errorf("expected RemovePreset on curated preset to fail with clear error, got %v", err)
	}
	if _, err := RemovePreset(userDir, nil, catWithOv, "doom2"); err == nil ||
		!strings.Contains(err.Error(), "cannot remove built-in curated preset") {
		t.Errorf("expected RemovePreset with nil curated catalog to reject LayerCurated preset, got %v", err)
	}

	// 4. LintCatalog skips repo-level checks (README table parity, lintDocumentation) when <rootDir>/catalog is absent
	nonRepoDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(nonRepoDir, "README.md"),
		[]byte("# External project\n\n---\n\nEmoji: \U0001f41b and step A -> step B\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	diags, err := LintCatalog(nonRepoDir, "")
	if err != nil {
		t.Fatalf("LintCatalog on nonRepoDir: %v", err)
	}
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics when <rootDir>/catalog does not exist, got %v", diags)
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

	// 4. LoadLayeredCatalog, MergeUserDir, and LoadPresetBaseAndOverride reject regular file userDir
	notADir := filepath.Join(t.TempDir(), "not-a-dir.json")
	if err := os.WriteFile(notADir, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLayeredCatalog(notADir, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected LoadLayeredCatalog(file) to fail with 'not a directory', got %v", err)
	}
	curated, err := LoadCuratedCatalog()
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

func TestCuratedNewPresetsTaxonomyAndPrimaryURL(t *testing.T) {
	t.Parallel()
	cat, err := LoadCuratedCatalog()
	if err != nil {
		t.Fatalf("LoadCuratedCatalog: %v", err)
	}

	queries := []struct {
		name       string
		query      string
		wantID     string
		wantEngine string
		wantGenre  string
		wantDiff   string
		wantQual   string
	}{
		{
			name:       "AUGER;ZENITH by id",
			query:      "auger-zenith",
			wantID:     "auger-zenith",
			wantEngine: "dsda-doom",
			wantGenre:  "Cyberpunk Adventure",
			wantDiff:   "Moderate",
			wantQual:   "S-Tier (9.5/10)",
		},
		{
			name:       "AUGER;ZENITH by alias filename",
			query:      "augerzenith.wad",
			wantID:     "auger-zenith",
			wantEngine: "dsda-doom",
			wantGenre:  "Cyberpunk Adventure",
			wantDiff:   "Moderate",
			wantQual:   "S-Tier (9.5/10)",
		},
		{
			name:       "MyHouse.pk3 by filename",
			query:      "myhouse.pk3",
			wantID:     "myhouse",
			wantEngine: "uzdoom",
			wantGenre:  "Psychological Horror",
			wantDiff:   "Casual",
			wantQual:   "S-Tier (9.5/10)",
		},
		{
			name:       "Overboard by id",
			query:      "overboard",
			wantID:     "overboard",
			wantEngine: "dsda-doom",
			wantGenre:  "Set-Piece Adventure",
			wantDiff:   "Moderate",
			wantQual:   "S-Tier (9.0/10)",
		},
		{
			name:       "Rowdy Rudy II by filename",
			query:      "rudy2.wad",
			wantID:     "rowdy-rudy-ii",
			wantEngine: "dsda-doom",
			wantGenre:  "DeHackEd Action",
			wantDiff:   "Moderate",
			wantQual:   "A-Tier (8.0/10)",
		},
		{
			name:       "Strife by IWAD",
			query:      "STRIFE1.WAD",
			wantID:     "strife",
			wantEngine: "uzdoom",
			wantGenre:  "Hub-Based Action RPG",
			wantDiff:   "Moderate",
			wantQual:   "A-Tier (8.5/10)",
		},
		{
			name:       "Going Down shorthand resolves to Going Down Turbo",
			query:      "going-down",
			wantID:     "going-down-turbo",
			wantEngine: "dsda-doom",
			wantGenre:  "Set-Piece Chaos",
			wantDiff:   "Hard",
			wantQual:   "S-Tier (9.5/10)",
		},
	}

	for _, tc := range queries {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := cat.Find(tc.query)
			if p == nil {
				t.Fatalf("cat.Find(%q) returned nil", tc.query)
			}
			if p.ID != tc.wantID || p.Engine != tc.wantEngine {
				t.Errorf("got ID=%q Engine=%q, want ID=%q Engine=%q", p.ID, p.Engine, tc.wantID, tc.wantEngine)
			}
			if p.Genre != tc.wantGenre || p.Difficulty != tc.wantDiff || p.Quality != tc.wantQual {
				t.Errorf(
					"got Genre=%q Difficulty=%q Quality=%q, want %q / %q / %q",
					p.Genre, p.Difficulty, p.Quality, tc.wantGenre, tc.wantDiff, tc.wantQual,
				)
			}
			if p.PrimaryURL() == "" {
				t.Errorf("expected non-empty PrimaryURL() for %s", p.ID)
			}
		})
	}

	// Verify applyPresetOverride supports Genre, Difficulty, and Quality
	base := *cat.Find("sunlust")
	customGenre := "Custom Genre"
	customDiff := "Custom Difficulty"
	customQual := "Custom Quality"
	ov := Override{
		Set: &OverrideSet{
			Genre:      &customGenre,
			Difficulty: &customDiff,
			Quality:    &customQual,
		},
	}
	applyPresetOverride(cat, &base, &ov)
	if base.Genre != "Custom Genre" ||
		base.Difficulty != "Custom Difficulty" ||
		base.Quality != "Custom Quality" {
		t.Errorf("override taxonomy fields not applied: %+v", base)
	}
}
