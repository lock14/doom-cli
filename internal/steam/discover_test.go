package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/catalog"
)

func TestParseLibraryFolders(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	lib1 := filepath.Join(tmpDir, "library1")
	lib2 := filepath.Join(tmpDir, "library2")
	if err := os.MkdirAll(lib1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lib2, 0o755); err != nil {
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
	if err := os.WriteFile(vdfPath, []byte(vdfContent), 0o644); err != nil {
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
	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	patterns := DefaultTargetPatterns()
	if len(patterns) == 0 {
		t.Fatal("expected non-empty DefaultTargetPatterns()")
	}

	destByNormSource := make(map[string]string)
	for _, tp := range patterns {
		key := catalog.NormalizeFilename(tp.SourcePattern)
		destLower := strings.ToLower(tp.DestName)
		if prev, exists := destByNormSource[key]; exists && prev == destLower {
			t.Errorf("duplicate normalized target pattern %q -> %q", tp.SourcePattern, tp.DestName)
		}
		destByNormSource[key] = destLower
	}

	for _, p := range cat.Presets {
		iwadNorm := catalog.NormalizeFilename(filepath.Base(p.IWAD))
		if _, ok := destByNormSource[iwadNorm]; !ok {
			t.Errorf("preset %s IWAD %q missing from DefaultTargetPatterns", p.ID, p.IWAD)
		}
		for _, f := range p.Files {
			fNorm := catalog.NormalizeFilename(filepath.Base(f.Name))
			if _, ok := destByNormSource[fNorm]; !ok {
				t.Errorf("preset %s file %q missing from DefaultTargetPatterns", p.ID, f.Name)
			}
			for _, al := range f.Aliases {
				alNorm := catalog.NormalizeFilename(al)
				if gotDest, ok := destByNormSource[alNorm]; !ok || gotDest != strings.ToLower(filepath.Base(f.Name)) {
					t.Errorf("preset %s alias %q -> %q missing from DefaultTargetPatterns", p.ID, al, f.Name)
				}
			}
		}
	}
}

func TestDiscoverAndExtract(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	mockSteam := filepath.Join(tmpDir, "steam_root")
	doomDir := filepath.Join(mockSteam, "steamapps", "common", "Doom 2", "base")
	if err := os.MkdirAll(doomDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create mock game files (including normalized underscore variant idkfa_2024.wad and aliases)
	_ = os.WriteFile(filepath.Join(doomDir, "doom2.wad"), []byte("doom2-data"), 0o644)
	_ = os.WriteFile(filepath.Join(doomDir, "sunder.wad"), []byte("sunder-data"), 0o644)
	_ = os.WriteFile(filepath.Join(doomDir, "DOOMZERO.DEH"), []byte("doomzero-deh-data"), 0o644)
	_ = os.WriteFile(filepath.Join(doomDir, "sigil2.wad"), []byte("sigil2-data"), 0o644)
	_ = os.WriteFile(filepath.Join(doomDir, "idkfa_2024.wad"), []byte("idkfa-data"), 0o644)

	destWads := filepath.Join(tmpDir, "target_wads")
	var out bytes.Buffer
	count, err := DiscoverAndExtract(nil, []string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtract failed: %v", err)
	}

	if count != 5 {
		t.Fatalf("expected 5 files extracted, got %d\nOutput: %s", count, out.String())
	}

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
	_ = os.WriteFile(filepath.Join(doomDir, "custom_addon.wad"), []byte("custom-data"), 0o644)
	customCat := &catalog.Catalog{
		Presets: []catalog.Preset{
			{
				ID:     "custom-addon",
				Name:   "Custom Addon",
				Engine: "dsda-doom",
				IWAD:   "DOOM2.WAD",
				Files: []catalog.FileSpec{
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

func TestDiscoverAndExtract_SkipsWadsDirSelfCopyAndExtractsBex(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Nest destWads inside scanRoot to verify WalkDir skips destWads even under force=true.
	scanRoot := filepath.Join(tmpDir, "Games")
	gameDir := filepath.Join(scanRoot, "DoomPack")
	destWads := filepath.Join(scanRoot, "wads")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destWads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destWads, "DOOM2.WAD"), []byte("existing-dest-doom2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "sunlust.bex"), []byte("bex-patch"), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := &catalog.Catalog{
		Presets: []catalog.Preset{
			{
				ID:     "sunlust-bex",
				Name:   "Sunlust BEX",
				Engine: "dsda-doom",
				IWAD:   "DOOM2.WAD",
				Files:  []catalog.FileSpec{{Name: "sunlust.bex"}},
			},
		},
	}

	count, err := DiscoverAndExtract(cat, []string{scanRoot}, destWads, true, nil)
	if err != nil {
		t.Fatalf("DiscoverAndExtract(force=true) failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only sunlust.bex extracted (skipping destWads self-copy), got %d", count)
	}
	if got, err := os.ReadFile(filepath.Join(destWads, "sunlust.bex")); err != nil || string(got) != "bex-patch" {
		t.Errorf("sunlust.bex = %q (%v), want bex-patch", string(got), err)
	}
	if got, err := os.ReadFile(filepath.Join(destWads, "DOOM2.WAD")); err != nil ||
		string(got) != "existing-dest-doom2" {
		t.Errorf("DOOM2.WAD modified unexpectedly: %q (%v)", string(got), err)
	}
}

func TestParseWindowsRegPathsAndCollectPlatformRoots(t *testing.T) {
	t.Parallel()

	regSample := `
HKEY_CURRENT_USER\Software\Valve\Steam
    SteamPath    REG_SZ    d:/games/steam

HKEY_LOCAL_MACHINE\SOFTWARE\WOW6432Node\GOG.com\Games\1435848814
    path    REG_EXPAND_SZ    "E:\GOG Galaxy\Games\DOOM"
    exe     REG_DWORD        0x1
`
	gotReg := parseWindowsRegPaths(regSample)
	if len(gotReg) != 2 {
		t.Fatalf("expected 2 parsed registry paths, got %d: %v", len(gotReg), gotReg)
	}
	if gotReg[0] != filepath.FromSlash("d:/games/steam") ||
		gotReg[1] != filepath.FromSlash(`E:\GOG Galaxy\Games\DOOM`) {
		t.Errorf("unexpected parsed registry paths: %v", gotReg)
	}

	mockReg := func(args ...string) []string {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, `Valve\Steam`) && strings.Contains(joined, "SteamPath"):
			return []string{`H:\CustomSteam`}
		case strings.Contains(joined, `GOG.com\Games`):
			return []string{`I:\GOGCustom\DOOM`}
		default:
			return nil
		}
	}

	winSteam, winDirect := collectPlatformRoots(
		"windows", `D:\MockProfile`, "", `C:\Program Files`, `C:\Program Files (x86)`, mockReg,
	)
	winAll := strings.Join(append(winSteam, winDirect...), "\n")
	for _, want := range []string{
		`H:\CustomSteam`,
		filepath.Join("D:\\MockProfile", "Saved Games", "Nightdive Studios", "Heretic + Hexen"),
		filepath.Join(`C:\Program Files (x86)`, "GOG Galaxy", "Games"),
		`E:\SteamLibrary`,
		`E:\XboxGames`,
		`E:\Epic Games`,
		`I:\GOGCustom\DOOM`,
	} {
		if !strings.Contains(winAll, want) {
			t.Errorf("windows roots missing %q in:\n%s", want, winAll)
		}
	}

	linuxSteam, linuxDirect := collectPlatformRoots(
		"linux", "/opt/mockhome", "/opt/xdgdata", "", "", nil,
	)
	linuxAll := strings.Join(append(linuxSteam, linuxDirect...), "\n")
	for _, want := range []string{
		filepath.Join("/opt/xdgdata", "Steam"),
		filepath.Join("/opt/mockhome", "snap", "steam", "common", ".local", "share", "Steam"),
		filepath.Join("/opt/mockhome", "Games", "Heroic"),
		filepath.Join("/opt/mockhome", ".local", "share", "lutris"),
		filepath.Join("/opt/mockhome", ".var", "app", "com.heroicgameslauncher.hgl", "Games"),
		filepath.Join("/opt/mockhome", ".var", "app", "net.lutris.Lutris", "Games"),
		filepath.Join("/opt/mockhome", ".var", "app", "com.usebottles.bottles", "data", "bottles"),
		"/usr/share/games/doom",
		"/usr/local/share/games/doom",
		"/usr/share/doom",
	} {
		if !strings.Contains(linuxAll, want) {
			t.Errorf("linux roots missing %q in:\n%s", want, linuxAll)
		}
	}

	macSteam, macDirect := collectPlatformRoots("darwin", "/opt/machome", "", "", "", nil)
	macAll := strings.Join(append(macSteam, macDirect...), "\n")
	for _, want := range []string{
		filepath.Join("/opt/machome", "Library", "Application Support", "Steam"),
		filepath.Join("/opt/machome", "Library", "Application Support", "CrossOver", "Bottles"),
		filepath.Join("/opt/machome", "Library", "Containers", "com.isaacmarovitz.Whisky", "Bottles"),
		filepath.Join("/", "Users", "Shared", "Epic Games"),
		filepath.Join("/opt/machome", "Games", "Heroic"),
	} {
		if !strings.Contains(macAll, want) {
			t.Errorf("darwin roots missing %q in:\n%s", want, macAll)
		}
	}
}

func TestResolveSteamCommonDirsAndExpandSearchRoots(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Primary Steam install has steamapps + libraryfolders.vdf, but no common/ directory.
	primarySteam := filepath.Join(tmpDir, "PrimarySteam")
	primarySteamApps := filepath.Join(primarySteam, "steamapps")
	secondaryLib := filepath.Join(tmpDir, "SecondaryLibrary")
	secondaryCommon := filepath.Join(secondaryLib, "steamapps", "common")
	looseGOG := filepath.Join(tmpDir, "GOGPack")

	for _, d := range []string{primarySteamApps, secondaryCommon, looseGOG} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	escapedLib := strings.ReplaceAll(secondaryLib, `\`, `\\`)
	vdf := "\"libraryfolders\"\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\"" + escapedLib + "\"\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(primarySteamApps, "libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}

	gotCommons := normalizeExistingDirs(resolveSteamCommonDirs([]string{primarySteam}))
	if len(gotCommons) != 1 || gotCommons[0] != filepath.Clean(secondaryCommon) {
		t.Fatalf("resolveSteamCommonDirs = %v, want [%s]", gotCommons, filepath.Clean(secondaryCommon))
	}

	gotExpanded := ExpandSearchRoots([]string{primarySteam, looseGOG})
	wantExpanded := []string{
		filepath.Clean(primarySteam),
		filepath.Clean(secondaryCommon),
		filepath.Clean(looseGOG),
	}
	if len(gotExpanded) != len(wantExpanded) {
		t.Fatalf("ExpandSearchRoots = %v, want %v", gotExpanded, wantExpanded)
	}
	for i := range wantExpanded {
		if gotExpanded[i] != wantExpanded[i] {
			t.Errorf("ExpandSearchRoots[%d] = %q, want %q", i, gotExpanded[i], wantExpanded[i])
		}
	}

	// Explicitly passing secondaryLib directly narrows to secondaryCommon.
	gotDirectLib := ExpandSearchRoots([]string{secondaryLib})
	if len(gotDirectLib) != 1 || gotDirectLib[0] != filepath.Clean(secondaryCommon) {
		t.Errorf("ExpandSearchRoots([secondaryLib]) = %v, want [%s]", gotDirectLib, filepath.Clean(secondaryCommon))
	}
}

func TestDiscoverAndExtract_EarlyExitAndSummaryMessages(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	scanDir := filepath.Join(tmpDir, "scan")
	destWads := filepath.Join(tmpDir, "wads")
	for _, d := range []string{scanDir, destWads} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cat := &catalog.Catalog{
		Presets: []catalog.Preset{
			{
				ID:     "mini-doom",
				Name:   "Mini Doom",
				Engine: "dsda-doom",
				IWAD:   "DOOM2.WAD",
				Files:  []catalog.FileSpec{{Name: "NERVE.WAD"}},
			},
		},
	}

	// 1. Zero installed and zero found -> clear "No matching game files found" message
	var out bytes.Buffer
	count, err := DiscoverAndExtract(cat, []string{scanDir}, destWads, false, &out)
	if err != nil || count != 0 {
		t.Fatalf("empty scan: count=%d err=%v", count, err)
	}
	if !strings.Contains(out.String(), "No matching game files found across searched directories") {
		t.Errorf("expected no-matching message, got:\n%s", out.String())
	}

	// 2. Partial installed (1 of 2) and zero newly found -> reports 1 existing file installed
	if err := os.WriteFile(filepath.Join(destWads, "DOOM2.WAD"), []byte("iwad"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	count, err = DiscoverAndExtract(cat, []string{scanDir}, destWads, false, &out)
	if err != nil || count != 0 {
		t.Fatalf("partial scan: count=%d err=%v", count, err)
	}
	if !strings.Contains(out.String(), "1 existing file(s) already installed") {
		t.Errorf("expected partial existing count in message, got:\n%s", out.String())
	}

	// 3. All target files installed (!force) -> skips walking searchRoots entirely
	if err := os.WriteFile(filepath.Join(destWads, "NERVE.WAD"), []byte("pwad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scanDir, "NERVE.WAD"), []byte("updated-pwad"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	count, err = DiscoverAndExtract(cat, []string{scanDir}, destWads, false, &out)
	if err != nil || count != 0 {
		t.Fatalf("all-installed scan: count=%d err=%v", count, err)
	}
	if !strings.Contains(out.String(), "All 2 target game file(s) are already installed") {
		t.Errorf("expected early-exit message, got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Searching for official game files across:") {
		t.Errorf("expected directory walk to be skipped when all targets are installed, got:\n%s", out.String())
	}

	// 4. force=true bypasses early-exit and overwrites with updated source
	out.Reset()
	count, err = DiscoverAndExtract(cat, []string{scanDir}, destWads, true, &out)
	if err != nil || count != 1 {
		t.Fatalf("force scan: count=%d err=%v\n%s", count, err, out.String())
	}
	if got, err := os.ReadFile(filepath.Join(destWads, "NERVE.WAD")); err != nil || string(got) != "updated-pwad" {
		t.Errorf("NERVE.WAD after force = %q (%v), want updated-pwad", string(got), err)
	}
}

func TestDiscoverAndExtract_FollowsSymlinkDirAndGuardsCycles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	externalGame := filepath.Join(tmpDir, "external_drive", "Ultimate Doom", "rerelease")
	steamCommon := filepath.Join(tmpDir, "Steam", "steamapps", "common")
	destWads := filepath.Join(tmpDir, "wads")
	for _, d := range []string{externalGame, steamCommon, destWads} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(externalGame, "DOOM.WAD"), []byte("symlink-doom"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(steamCommon, "Ultimate Doom")
	if err := os.Symlink(filepath.Dir(externalGame), linkPath); err != nil {
		t.Skipf("skipping symlink directory test (os.Symlink unavailable): %v", err)
	}
	// Create a symlink loop inside externalGame pointing back to steamCommon to test cycle guard.
	_ = os.Symlink(steamCommon, filepath.Join(externalGame, "cycle_back"))

	cat := &catalog.Catalog{
		Presets: []catalog.Preset{
			{ID: "doom", Name: "Doom", Engine: "dsda-doom", IWAD: "DOOM.WAD"},
		},
	}
	count, err := DiscoverAndExtract(cat, []string{steamCommon}, destWads, false, nil)
	if err != nil {
		t.Fatalf("DiscoverAndExtract through symlink dir failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 file extracted via symlinked game dir, got %d", count)
	}
	if got, err := os.ReadFile(filepath.Join(destWads, "DOOM.WAD")); err != nil || string(got) != "symlink-doom" {
		t.Errorf("DOOM.WAD = %q (%v), want symlink-doom", string(got), err)
	}
}

func TestMacBundlesProtonCompatAndPruneNestedDirs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// 1. Case-insensitive macOS app bundle discovery across system and user Applications dirs
	sysApps := filepath.Join(tmpDir, "Applications")
	userApps := filepath.Join(tmpDir, "UserApplications")
	for _, d := range []string{
		filepath.Join(sysApps, "HERETIC + HEXEN.app"),
		filepath.Join(sysApps, "Safari.app"),
		filepath.Join(userApps, "strife-veteran.app"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bundles := findMacAppBundles(sysApps, userApps)
	if len(bundles) != 2 {
		t.Fatalf("expected 2 matching macOS app bundles, got %d: %v", len(bundles), bundles)
	}

	// 2. Proton compatdata Nightdive Saved Games resolution alongside steamapps/common
	steamLib := filepath.Join(tmpDir, "SteamDeckLib")
	steamCommon := filepath.Join(steamLib, "steamapps", "common")
	protonNightdive := filepath.Join(
		steamLib, "steamapps", "compatdata", "2280",
		"pfx", "drive_c", "users", "steamuser", "Saved Games", "Nightdive Studios", "DOOM",
	)
	for _, d := range []string{steamCommon, protonNightdive} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gotSteamDirs := resolveSteamCommonDirs([]string{steamLib})
	if len(gotSteamDirs) != 2 ||
		gotSteamDirs[0] != steamCommon ||
		gotSteamDirs[1] != protonNightdive {
		t.Errorf("resolveSteamCommonDirs = %v, want [%s, %s]", gotSteamDirs, steamCommon, protonNightdive)
	}

	// 3. Nested-root pruning drops child directories while keeping prefix-similar siblings
	gamesRoot := filepath.Join(tmpDir, "Games")
	heroicChild := filepath.Join(gamesRoot, "Heroic")
	gamesExtraSibling := filepath.Join(tmpDir, "GamesExtra")
	for _, d := range []string{heroicChild, gamesExtraSibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	pruned := ExpandSearchRoots([]string{heroicChild, gamesRoot, gamesExtraSibling})
	wantPruned := []string{filepath.Clean(gamesRoot), filepath.Clean(gamesExtraSibling)}
	if len(pruned) != len(wantPruned) || pruned[0] != wantPruned[0] || pruned[1] != wantPruned[1] {
		t.Errorf("ExpandSearchRoots nested pruning = %v, want %v", pruned, wantPruned)
	}

	// 4. Wine/CrossOver dosdevices directory is skipped so z: -> / symlink never escapes prefix
	winePrefix := filepath.Join(tmpDir, "wine_prefix")
	dosDevices := filepath.Join(winePrefix, "dosdevices", "z_drive")
	driveCGOG := filepath.Join(winePrefix, "drive_c", "GOG Games", "Doom")
	destWads := filepath.Join(tmpDir, "out_wads")
	for _, d := range []string{dosDevices, driveCGOG, destWads} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dosDevices, "DOOM.WAD"), []byte("wrong-dosdevices"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(driveCGOG, "DOOM2.WAD"), []byte("good-drivec"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{
		Presets: []catalog.Preset{
			{ID: "doom", Name: "Doom", Engine: "dsda-doom", IWAD: "DOOM.WAD"},
			{ID: "doom-ii", Name: "Doom II", Engine: "dsda-doom", IWAD: "DOOM2.WAD"},
		},
	}
	extracted, err := DiscoverAndExtract(cat, []string{winePrefix}, destWads, false, nil)
	if err != nil || extracted != 1 {
		t.Fatalf("DiscoverAndExtract(winePrefix) extracted=%d err=%v, want 1", extracted, err)
	}
	if _, err := os.Stat(filepath.Join(destWads, "DOOM.WAD")); !os.IsNotExist(err) {
		t.Errorf("expected dosdevices/DOOM.WAD to be skipped")
	}
}
