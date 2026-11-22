package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeEntityStrictValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid preset with UTF-8 BOM",
			raw: "\xef\xbb\xbf{\"schema-version\":1,\"kind\":\"preset\",\"id\":\"my-wad\"," +
				"\"name\":\"My WAD\",\"engine\":\"dsda-doom\",\"iwad\":\"DOOM2.WAD\",\"complevel\":9," +
				"\"engine-args\":{\"dsda-doom\":[\"-solonet\"]}}",
			wantErr: "",
		},
		{
			name: "unknown field rejected",
			raw: `{"kind":"preset","id":"my-wad","name":"My WAD",` +
				`"engine":"dsda-doom","iwad":"DOOM2.WAD","mappacks":["a.wad"]}`,
			wantErr: "unknown field",
		},
		{
			name: "legacy snake_case key rejected",
			raw: `{"schema_version":1,"kind":"preset","id":"my-wad",` +
				`"name":"My WAD","engine":"dsda-doom","iwad":"DOOM2.WAD"}`,
			wantErr: "unknown field",
		},
		{
			name: "schema key rejected",
			raw: `{"$schema":"foo","kind":"preset","id":"my-wad",` +
				`"name":"My WAD","engine":"dsda-doom","iwad":"DOOM2.WAD"}`,
			wantErr: "unknown field",
		},
		{
			name: "unsupported schema-version",
			raw: `{"schema-version":2,"kind":"preset","id":"my-wad",` +
				`"name":"My WAD","engine":"dsda-doom","iwad":"DOOM2.WAD"}`,
			wantErr: "unsupported schema-version 2",
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
			name:    "invalid engine args-style",
			raw:     `{"kind":"engine","id":"woof","args-style":"invalid"}`,
			wantErr: "args-style",
		},
		{
			name: "invalid preset complevel",
			raw: `{"kind":"preset","id":"my-wad","name":"My WAD",` +
				`"engine":"dsda-doom","iwad":"DOOM2.WAD","complevel":99}`,
			wantErr: "invalid complevel",
		},
		{
			name: "invalid engine-args key",
			raw: `{"kind":"preset","id":"my-wad","name":"My WAD","engine":"dsda-doom","iwad":"DOOM2.WAD",` +
				`"engine-args":{"bad_key":["-fast"]}}`,
			wantErr: "engine-args key",
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

func TestMergeUserDir_IdempotentOnAlreadyMergedCatalog(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	cat, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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

	// Simulate starting TUI with an already-merged catalog from LoadLayered
	alreadyMerged, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered with overrides: %v", err)
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

func TestCuratedNewPresetsTaxonomyAndPrimaryURL(t *testing.T) {
	t.Parallel()
	cat, err := LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
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
			name:       "MyHouse.pk3 by spaced query",
			query:      "my house",
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

	// Verify MyHouse.pk3 points to Veddge's April 2025 UZDoom-compatible sources rather than the 2023 build.
	mh, err := cat.FindExact("my house")
	if err != nil {
		t.Fatalf("cat.FindExact(\"my house\") failed: %v", err)
	}
	if len(mh.Sources) < 2 {
		t.Fatalf("expected at least 2 mirrors for myhouse, got %d", len(mh.Sources))
	}
	for _, src := range mh.Sources {
		if src.SHA256 == "25ee4c7464b2c01c761b91236e50d7c3ec5fbc8de79118a5fa4acb9c4f34ae81" {
			t.Errorf("myhouse source %s still uses incompatible 2023 sha256", src.URL)
		}
	}
}

func TestMergeUserDir_DropsDeletedUserEntities(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	curated, err := LoadCurated()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddEngine(userDir, curated, EngineConfig{
		ID:     "nugget",
		Binary: "nugget-doom",
	}); err != nil {
		t.Fatalf("AddEngine(nugget): %v", err)
	}
	layered, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddPreset(userDir, curated, layered, Preset{
		ID:     "temp-pack",
		Name:   "Temp Pack",
		Engine: "nugget",
		IWAD:   "DOOM2.WAD",
		Files:  []FileSpec{{Name: "temppack.wad"}},
	}); err != nil {
		t.Fatalf("AddPreset(temp-pack): %v", err)
	}

	initialMerged, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if initialMerged.Find("temp-pack") == nil {
		t.Fatal("expected temp-pack in initialMerged")
	}

	if _, err := RemovePreset(userDir, curated, initialMerged, "temp-pack"); err != nil {
		t.Fatalf("RemovePreset(temp-pack): %v", err)
	}
	if _, _, err := RemoveEngine(userDir, curated, initialMerged, "nugget"); err != nil {
		t.Fatalf("RemoveEngine(nugget): %v", err)
	}

	reMerged, err := MergeUserDir(initialMerged, userDir)
	if err != nil {
		t.Fatalf("MergeUserDir after deletion: %v", err)
	}
	if reMerged.Find("temp-pack") != nil {
		t.Errorf("expected deleted user preset temp-pack not to be resurrected by MergeUserDir")
	}
	if _, exists := reMerged.Engines["nugget"]; exists {
		t.Errorf("expected deleted user engine nugget not to be resurrected by MergeUserDir")
	}
}
