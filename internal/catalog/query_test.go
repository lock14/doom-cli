package catalog

import (
	"strings"
	"testing"
)

func TestLookupNormalization_EngineDisplayName_And_RemovePresetCurated(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	curated, err := LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
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
	cat, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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
	catWithOv, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
	}
	if _, err := RemovePreset(userDir, curated, catWithOv, "doom2"); err == nil ||
		!strings.Contains(err.Error(), "cannot remove built-in curated preset") {
		t.Errorf("expected RemovePreset on curated preset to fail with clear error, got %v", err)
	}
	if _, err := RemovePreset(userDir, nil, catWithOv, "doom2"); err == nil ||
		!strings.Contains(err.Error(), "cannot remove built-in curated preset") {
		t.Errorf("expected RemovePreset with nil curated catalog to reject LayerCurated preset, got %v", err)
	}
}
