package preset

import (
	"os"
	"path/filepath"
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

	layered, err := LoadCatalog(userDir)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
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

	layeredAfter, err := LoadCatalog(userDir)
	if err != nil {
		t.Fatalf("LoadCatalog with quarantined file should succeed, got: %v", err)
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
	aaliensWad := filepath.Join(root1, "aaliens.wad")
	_ = os.WriteFile(aaliensWad, []byte("PWAD"), 0o644)

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
		{name: "alias sunder to SUNDER_V2512", query: "SUNDER_V2512.wad", wantOK: true, wantPath: sunderWad},
		{name: "alias aaliens to aaliens_v1_2", query: "aaliens_v1_2.wad", wantOK: true, wantPath: aaliensWad},
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
	// and canonical target collision (SCYTHE.WAD)
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
		"conflicts with canonical target \"SCYTHE.WAD\"",
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
		"aaliens.txt", "gd.txt", "sigil.txt", "sigil2.txt", "sunder.txt",
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
		{name: "Going Down Turbo", wantTxt: "gd.txt"},
		{name: "Sigil", wantTxt: "sigil.txt"},
		{name: "Sigil II", wantTxt: "sigil2.txt"},
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

	// Verify a megawad with required PWADs does NOT borrow doom2.txt
	av := cat.Find("Alien Vendetta")
	if av == nil {
		t.Fatal("Alien Vendetta not found")
	}
	if got, ok := idx.ResolveReadme(*av); ok {
		t.Fatalf("expected Alien Vendetta not to borrow IWAD readme, got %q", got)
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
