package steam

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/preset"
)

type testWADLump struct {
	name string
	data []byte
}

func buildTestPWAD(lumps []testWADLump) []byte {
	var dataBuf bytes.Buffer
	dir := make([]byte, len(lumps)*16)
	curPos := uint32(12)
	for i, l := range lumps {
		_, _ = dataBuf.Write(l.data)
		entry := dir[i*16 : (i+1)*16]
		binary.LittleEndian.PutUint32(entry[0:4], curPos)
		binary.LittleEndian.PutUint32(entry[4:8], uint32(len(l.data)))
		var nameBytes [8]byte
		copy(nameBytes[:], l.name)
		copy(entry[8:16], nameBytes[:])
		curPos += uint32(len(l.data))
	}

	var out bytes.Buffer
	_, _ = out.WriteString("PWAD")
	var hdrNums [8]byte
	binary.LittleEndian.PutUint32(hdrNums[0:4], uint32(len(lumps)))
	binary.LittleEndian.PutUint32(hdrNums[4:8], curPos)
	_, _ = out.Write(hdrNums[:])
	_, _ = out.Write(dataBuf.Bytes())
	_, _ = out.Write(dir)
	return out.Bytes()
}

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
	_ = os.WriteFile(filepath.Join(doomDir, "sunder.wad"), []byte("sunder-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "DOOMZERO.DEH"), []byte("doomzero-deh-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "sigil2.wad"), []byte("sigil2-data"), 0644)
	_ = os.WriteFile(filepath.Join(doomDir, "idkfa_2024.wad"), []byte("idkfa-data"), 0644)

	destWads := filepath.Join(tmpDir, "target_wads")
	var out bytes.Buffer
	count, err := DiscoverAndExtract(nil, []string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtract failed: %v", err)
	}

	if count != 5 {
		t.Fatalf("expected 5 files extracted, got %d\nOutput: %s", count, out.String())
	}

	// Verify target names:
	// doom2.wad -> DOOM2.WAD
	// sunder.wad -> Sunder 2512.wad
	// DOOMZERO.DEH -> DOOMZERO.DEH
	// sigil2.wad -> SIGIL_II_V1_0.WAD
	// idkfa_2024.wad -> idkfa 2024.wad (via normalized matching)
	for _, wantFile := range []string{
		"DOOM2.WAD",
		"Sunder 2512.wad",
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
	count2, err := DiscoverAndExtract(nil, []string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("second DiscoverAndExtract failed: %v", err)
	}
	if count2 != 0 {
		t.Errorf("expected 0 files on second run without force, got %d", count2)
	}

	// Verify DiscoverAndExtract extracts custom user-layer preset aliases
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
	count3, err := DiscoverAndExtract(customCat, []string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtract failed: %v", err)
	}
	if count3 != 1 {
		t.Fatalf("expected 1 custom file extracted, got %d\nOutput: %s", count3, out.String())
	}
	if _, err := os.Stat(filepath.Join(destWads, "custom_addon_v2.wad")); err != nil {
		t.Errorf("custom_addon_v2.wad was not created from custom_addon.wad: %v", err)
	}

	// Verify files already present in extraWadRoots are not re-extracted when !force
	primaryEmpty := filepath.Join(tmpDir, "primary_empty")
	out.Reset()
	count4, err := DiscoverAndExtract(customCat, []string{mockSteam}, primaryEmpty, false, &out, destWads)
	if err != nil {
		t.Fatalf("DiscoverAndExtract with extraWadRoots failed: %v", err)
	}
	if count4 != 0 {
		t.Errorf("expected 0 files extracted when already present in extraWadRoots, got %d", count4)
	}
}

func TestSynthesizeIDKFAWADAndDiscoverFromExtras(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	mockSteam := filepath.Join(tmpDir, "steam_root")
	baseDir := filepath.Join(mockSteam, "steamapps", "common", "Ultimate Doom", "base")
	rereleaseDir := filepath.Join(mockSteam, "steamapps", "common", "Ultimate Doom", "rerelease")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rereleaseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	legacyExtras := buildTestPWAD([]testWADLump{
		{name: "DSSECRET", data: []byte("secret-sfx")},
	})
	if err := os.WriteFile(filepath.Join(baseDir, "extras.wad"), legacyExtras, 0o644); err != nil {
		t.Fatal(err)
	}

	remasterExtras := buildTestPWAD([]testWADLump{
		{name: "TRAKINFO", data: []byte(`{"tracks":[]}`)},
		{name: "O_E1M1", data: []byte("OggS-sc55-e1m1")},
		{name: "H_E1M1", data: []byte("OggS-idkfa-e1m1")},
		{name: "H_E2M3", data: []byte("OggS-idkfa-e2m3")},
		{name: "H_RUNNIN", data: []byte("OggS-idkfa-runnin")},
	})
	if err := os.WriteFile(filepath.Join(rereleaseDir, "extras.wad"), remasterExtras, 0o644); err != nil {
		t.Fatal(err)
	}

	destWads := filepath.Join(tmpDir, "target_wads")
	var out bytes.Buffer
	count, err := DiscoverAndExtract(nil, []string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtract failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 files (extras.wad + synthesized idkfa 2024.wad), got %d\nOutput:\n%s", count, out.String())
	}

	gotExtras, err := os.ReadFile(filepath.Join(destWads, "extras.wad"))
	if err != nil {
		t.Fatalf("reading extracted extras.wad: %v", err)
	}
	if !bytes.Equal(gotExtras, remasterExtras) {
		t.Errorf("expected rerelease/extras.wad to be preferred over base/extras.wad")
	}

	idkfaBytes, err := os.ReadFile(filepath.Join(destWads, "idkfa 2024.wad"))
	if err != nil {
		t.Fatalf("reading synthesized idkfa 2024.wad: %v", err)
	}
	if len(idkfaBytes) < 12 || string(idkfaBytes[0:4]) != "PWAD" {
		t.Fatalf("synthesized idkfa 2024.wad does not have PWAD header")
	}
	numLumps := binary.LittleEndian.Uint32(idkfaBytes[4:8])
	dirOfs := binary.LittleEndian.Uint32(idkfaBytes[8:12])
	// H_E1M1 -> D_E1M1; H_E2M3 -> D_E2M3 + D_INTER; H_RUNNIN -> D_RUNNIN + D_RUNNI2 (5 directory entries)
	wantTracks := []testWADLump{
		{name: "D_E1M1", data: []byte("OggS-idkfa-e1m1")},
		{name: "D_E2M3", data: []byte("OggS-idkfa-e2m3")},
		{name: "D_INTER", data: []byte("OggS-idkfa-e2m3")},
		{name: "D_RUNNIN", data: []byte("OggS-idkfa-runnin")},
		{name: "D_RUNNI2", data: []byte("OggS-idkfa-runnin")},
	}
	if int(numLumps) != len(wantTracks) {
		t.Fatalf("expected %d synthesized directory entries, got %d", len(wantTracks), numLumps)
	}

	for i, want := range wantTracks {
		entry := idkfaBytes[int(dirOfs)+i*16 : int(dirOfs)+(i+1)*16]
		pos := binary.LittleEndian.Uint32(entry[0:4])
		size := binary.LittleEndian.Uint32(entry[4:8])
		nameSlice := entry[8:16]
		if nul := bytes.IndexByte(nameSlice, 0); nul >= 0 {
			nameSlice = nameSlice[:nul]
		}
		if string(nameSlice) != want.name {
			t.Errorf("track %d name = %q, want %q", i, string(nameSlice), want.name)
		}
		payload := idkfaBytes[pos : pos+size]
		if !bytes.Equal(payload, want.data) {
			t.Errorf("track %d payload = %q, want %q", i, string(payload), string(want.data))
		}
	}
}

func TestDiscoverAndExtract_PrefersRereleaseIWADsOverBase(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	mockSteam := filepath.Join(tmpDir, "steam_root")
	baseDir := filepath.Join(mockSteam, "steamapps", "common", "Ultimate Doom", "base")
	rereleaseDir := filepath.Join(mockSteam, "steamapps", "common", "Ultimate Doom", "rerelease")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rereleaseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	targets := []struct {
		name          string
		baseName      string
		rereleaseName string
		destName      string
		baseData      []byte
		rereleaseData []byte
	}{
		{
			name:          "DOOM.WAD",
			baseName:      "DOOM.WAD",
			rereleaseName: "DOOM.WAD",
			destName:      "DOOM.WAD",
			baseData:      []byte("IWAD-legacy-4x3-doom-1995-padding-longer"),
			rereleaseData: []byte("IWAD-widescreen-doom-2024"),
		},
		{
			name:          "DOOM2.WAD",
			baseName:      "DOOM2.WAD",
			rereleaseName: "DOOM2.WAD",
			destName:      "DOOM2.WAD",
			baseData:      []byte("IWAD-legacy-4x3-doom2-1994"),
			rereleaseData: []byte("IWAD-widescreen-doom2-2024"),
		},
		{
			name:          "TNT.WAD",
			baseName:      "TNT.WAD",
			rereleaseName: "TNT.WAD",
			destName:      "TNT.WAD",
			baseData:      []byte("IWAD-legacy-4x3-tnt-1996"),
			rereleaseData: []byte("IWAD-widescreen-tnt-2024"),
		},
		{
			name:          "PLUTONIA.WAD",
			baseName:      "PLUTONIA.WAD",
			rereleaseName: "PLUTONIA.WAD",
			destName:      "PLUTONIA.WAD",
			baseData:      []byte("IWAD-legacy-4x3-plutonia-1996"),
			rereleaseData: []byte("IWAD-widescreen-plutonia-2024"),
		},
		{
			name:          "rerelease alias sigil.wad beats base canonical SIGIL_V1_23.wad",
			baseName:      "SIGIL_V1_23.wad",
			rereleaseName: "sigil.wad",
			destName:      "SIGIL_V1_23.wad",
			baseData:      []byte("PWAD-base-canonical-sigil-larger-payload"),
			rereleaseData: []byte("PWAD-rerelease-alias-sigil"),
		},
		{
			name:          "rerelease normalized idkfa_2024.wad beats base exact idkfa 2024.wad",
			baseName:      "idkfa 2024.wad",
			rereleaseName: "idkfa_2024.wad",
			destName:      "idkfa 2024.wad",
			baseData:      []byte("PWAD-base-exact-idkfa-larger-payload"),
			rereleaseData: []byte("PWAD-rerelease-norm-idkfa"),
		},
	}

	for _, tc := range targets {
		if err := os.WriteFile(filepath.Join(baseDir, tc.baseName), tc.baseData, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rereleaseDir, tc.rereleaseName), tc.rereleaseData, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	destWads := filepath.Join(tmpDir, "target_wads")
	var out bytes.Buffer
	count, err := DiscoverAndExtract(nil, []string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtract failed: %v", err)
	}
	if count != len(targets) {
		t.Fatalf("expected %d files extracted, got %d\nOutput:\n%s", len(targets), count, out.String())
	}

	for _, tc := range targets {
		got, err := os.ReadFile(filepath.Join(destWads, tc.destName))
		if err != nil {
			t.Fatalf("reading extracted %s: %v", tc.destName, err)
		}
		if !bytes.Equal(got, tc.rereleaseData) {
			t.Errorf(
				"%s (%s) content = %q, want rerelease content %q",
				tc.name, tc.destName, string(got), string(tc.rereleaseData),
			)
		}
	}
}

func TestRankRereleaseCandidates(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	shortBase := filepath.Join(tmpDir, "a", "base", "DOOM.WAD")
	longBase := filepath.Join(tmpDir, "longer_prefix", "base", "DOOM.WAD")
	smallBase := filepath.Join(tmpDir, "small", "base", "DOOM.WAD")
	largeBase := filepath.Join(tmpDir, "large", "base", "DOOM.WAD")
	rereleaseUpper := filepath.Join(tmpDir, "very_long_prefix", "ReRelease", "DOOM.WAD")
	nestedRerelease := filepath.Join(tmpDir, "steam", "rerelease", "id1", "id1.wad")
	nestedBase := filepath.Join(tmpDir, "steam", "base", "id1", "id1.wad")
	outerRereleaseInnerBase := filepath.Join(tmpDir, "rerelease_backup", "doom", "base", "DOOM.WAD")
	neutralGOG := filepath.Join(tmpDir, "gog", "doom", "DOOM.WAD")
	dosboxLegacy := filepath.Join(tmpDir, "gog", "dosbox", "DOOM.WAD")

	for _, p := range []string{
		shortBase,
		longBase,
		smallBase,
		largeBase,
		rereleaseUpper,
		nestedRerelease,
		nestedBase,
		outerRereleaseInnerBase,
		neutralGOG,
		dosboxLegacy,
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(shortBase, []byte("same-size"), 0o644)
	_ = os.WriteFile(longBase, []byte("same-size"), 0o644)
	_ = os.WriteFile(smallBase, []byte("tiny"), 0o644)
	_ = os.WriteFile(largeBase, []byte("much-larger-payload-bytes"), 0o644)
	_ = os.WriteFile(rereleaseUpper, []byte("tiny"), 0o644)
	_ = os.WriteFile(nestedRerelease, []byte("id1-rerelease"), 0o644)
	_ = os.WriteFile(nestedBase, []byte("id1-base-larger-payload"), 0o644)
	_ = os.WriteFile(outerRereleaseInnerBase, []byte("much-larger-payload-bytes"), 0o644)
	_ = os.WriteFile(neutralGOG, []byte("tiny"), 0o644)
	_ = os.WriteFile(dosboxLegacy, []byte("much-larger-payload-bytes"), 0o644)

	tests := []struct {
		name      string
		input     []string
		wantFirst string
	}{
		{
			name:      "case-insensitive ReRelease beats larger base file",
			input:     []string{largeBase, rereleaseUpper},
			wantFirst: rereleaseUpper,
		},
		{
			name:      "nested subdirectory under rerelease beats nested subdirectory under base",
			input:     []string{nestedBase, nestedRerelease},
			wantFirst: nestedRerelease,
		},
		{
			name:      "inner base directory is not misclassified by outer rerelease ancestor",
			input:     []string{outerRereleaseInnerBase, neutralGOG},
			wantFirst: neutralGOG,
		},
		{
			name:      "neutral GOG directory beats dosbox directory",
			input:     []string{dosboxLegacy, neutralGOG},
			wantFirst: neutralGOG,
		},
		{
			name:      "larger file beats smaller file when priority is equal",
			input:     []string{smallBase, largeBase},
			wantFirst: largeBase,
		},
		{
			name:      "shorter directory path beats longer directory path when priority and size are equal",
			input:     []string{longBase, shortBase},
			wantFirst: shortBase,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rankRereleaseCandidates(tc.input)
			if got[0] != tc.wantFirst {
				t.Errorf("rankRereleaseCandidates(%v)[0] = %q, want %q", tc.input, got[0], tc.wantFirst)
			}
		})
	}
}
