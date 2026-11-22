package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInferGameFromIWADAndDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		iwad      string
		wantGame  string
		wantTitle string
		wantCore  bool
	}{
		{name: "empty defaults to doom", iwad: "", wantGame: "doom", wantTitle: "Doom", wantCore: true},
		{name: "DOOM.WAD", iwad: "DOOM.WAD", wantGame: "doom", wantTitle: "Doom", wantCore: true},
		{name: "DOOM1.WAD", iwad: "DOOM1.WAD", wantGame: "doom", wantTitle: "Doom", wantCore: true},
		{name: "DOOM2.WAD", iwad: "DOOM2.WAD", wantGame: "doom", wantTitle: "Doom", wantCore: true},
		{name: "TNT.WAD", iwad: "TNT.WAD", wantGame: "doom", wantTitle: "Doom", wantCore: true},
		{name: "PLUTONIA.WAD", iwad: "PLUTONIA.WAD", wantGame: "doom", wantTitle: "Doom", wantCore: true},
		{name: "FREEDOOM2.WAD", iwad: "freedoom2.wad", wantGame: "doom", wantTitle: "Doom", wantCore: true},
		{name: "HERETIC.WAD", iwad: "HERETIC.WAD", wantGame: "heretic", wantTitle: "Heretic", wantCore: false},
		{name: "HEXEN.WAD", iwad: "HEXEN.WAD", wantGame: "hexen", wantTitle: "Hexen", wantCore: false},
		{name: "STRIFE1.WAD", iwad: "STRIFE1.WAD", wantGame: "strife", wantTitle: "Strife", wantCore: false},
		{
			name: "custom chex quest iwad", iwad: "chex-quest.wad",
			wantGame: "chex-quest", wantTitle: "Chex Quest", wantCore: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := InferGameFromIWAD(tc.iwad)
			if got != tc.wantGame {
				t.Errorf("InferGameFromIWAD(%q) = %q, want %q", tc.iwad, got, tc.wantGame)
			}
			p := Preset{IWAD: tc.iwad}
			if p.EffectiveGame() != tc.wantGame {
				t.Errorf("Preset{IWAD:%q}.EffectiveGame() = %q, want %q", tc.iwad, p.EffectiveGame(), tc.wantGame)
			}
			if p.IsCoreGame() != tc.wantCore {
				t.Errorf("Preset{IWAD:%q}.IsCoreGame() = %v, want %v", tc.iwad, p.IsCoreGame(), tc.wantCore)
			}
			if disp := GameDisplayName(got); disp != tc.wantTitle {
				t.Errorf("GameDisplayName(%q) = %q, want %q", got, disp, tc.wantTitle)
			}
		})
	}
}

func TestCatalogOptionalGamesScopingAndLibrary(t *testing.T) {
	t.Parallel()

	cat, err := LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}

	wantOpt := []string{"heretic", "hexen", "strife"}
	if got := cat.OptionalGames(); !reflect.DeepEqual(got, wantOpt) {
		t.Fatalf("OptionalGames() = %v, want %v", got, wantOpt)
	}

	wadsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wadsDir, "HERETIC.WAD"), []byte("iwad"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := NewDirIndex(wadsDir).WithCatalog(cat)
	if gotInst := cat.InstalledOptionalGames(idx); !reflect.DeepEqual(gotInst, []string{"heretic"}) {
		t.Errorf("InstalledOptionalGames = %v, want [heretic]", gotInst)
	}

	// Add a custom uninstalled user preset and verify it remains visible in LibraryPresets and FilterByGames
	cat.Presets = append(cat.Presets, Preset{
		ID:     "my-custom-wad",
		Name:   "My Custom WAD",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files:  []FileSpec{{Name: "missing_custom.wad"}},
		Layer:  LayerUser,
	})

	doomOnly := cat.FilterByGames(map[string]bool{CoreGame: true})
	for _, p := range doomOnly.Presets {
		if !p.IsCustom() && !p.IsCoreGame() {
			t.Errorf("FilterByGames(doom) unexpectedly retained optional preset %s (%s)", p.ID, p.EffectiveGame())
		}
	}

	single := cat.ForPresets(Preset{ID: "hexen", Name: "Hexen", Game: "hexen", IWAD: "HEXEN.WAD"})
	if len(single.Presets) != 1 || single.Presets[0].ID != "hexen" || len(single.Engines) == 0 {
		t.Errorf("ForPresets returned unexpected catalog: presets=%d engines=%d",
			len(single.Presets), len(single.Engines))
	}

	lib, hidden := cat.LibraryPresets(idx)
	if len(lib) != 3 {
		t.Fatalf("expected 3 library presets (Doom II, Heretic, My Custom WAD), got %d: %+v", len(lib), lib)
	}
	if hidden != len(cat.Presets)-3 {
		t.Errorf("hidden = %d, want %d", hidden, len(cat.Presets)-3)
	}

	libCat, hidden2 := cat.FilterLibrary(idx)
	if hidden2 != hidden || len(libCat.Presets) != 3 {
		t.Errorf("FilterLibrary = (%d presets, %d hidden), want (3, %d)", len(libCat.Presets), hidden2, hidden)
	}
}
