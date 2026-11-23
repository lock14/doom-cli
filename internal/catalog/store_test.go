package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStoreLifecycleAndQuarantine(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	curated, err := LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
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

	layered, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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

	// 5. Override preservation across multiple ConfigPreset calls
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

	layeredAfter, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered with quarantined file should succeed, got: %v", err)
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

func TestConfigEngineAndResetEngineOverride(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	cat, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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

	merged, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered after override: %v", err)
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
	mergedCleared, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered after clearing fields: %v", err)
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
		t.Error("expected error for invalid args-style, got nil")
	}
	if _, err := DecodeEntity([]byte(
		`{"kind":"override","target":{"kind":"engine","id":"dsda-doom"},"set":{"args-style":"quake"}}`,
	)); err == nil {
		t.Error("expected DecodeEntity to reject invalid override set.args-style")
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

func TestConfigPreset_ExcludeFilesAndBaseAndOverride(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	cat, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered: %v", err)
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

	// Verify MergeUserDir and LoadLayered filter out idkfa 2024.wad
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
	reloaded, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered after clearing exclude_files: %v", err)
	}
	pReloaded, err := reloaded.FindExact("doom-ii")
	if err != nil {
		t.Fatalf("doom-ii not found in reloaded catalog: %v", err)
	}
	if len(pReloaded.Files) == 0 {
		t.Errorf("expected idkfa 2024.wad restored after clearing exclude_files, got %+v", pReloaded.Files)
	}
	if !reflect.DeepEqual(pReloaded.Args, []string{"-skill", "4"}) {
		t.Errorf("expected AppendArgs preserved when clearing exclude-files, got %v", pReloaded.Args)
	}

	// Verify Complevel and EngineArgs / AppendEngineArgs surgical round-trips
	clOverride := 21
	setEngMap := map[string][]string{"dsda": {"-solonet"}, "uzdoom": {"+gl_tonemap", "0"}}
	if _, _, err := ConfigPreset(userDir, reloaded, "sunlust", OverridePatchRequest{
		Complevel:     &clOverride,
		SetEngineArgs: &setEngMap,
	}); err != nil {
		t.Fatalf("ConfigPreset complevel/engine-args: %v", err)
	}
	appendEngMap := map[string][]string{"dsda-doom": {"-fast"}}
	if _, _, err := ConfigPreset(userDir, reloaded, "sunlust", OverridePatchRequest{
		AppendEngineArgs: &appendEngMap,
	}); err != nil {
		t.Fatalf("ConfigPreset append-engine-args: %v", err)
	}
	mergedSunlustCat, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered sunlust: %v", err)
	}
	pSunlust := mergedSunlustCat.Find("sunlust")
	if pSunlust == nil || pSunlust.Complevel == nil || *pSunlust.Complevel != 21 {
		t.Fatalf("expected sunlust Complevel=21, got %+v", pSunlust)
	}
	if !reflect.DeepEqual(pSunlust.ArgsForEngine("dsda-doom"), []string{"-solonet", "-fast"}) {
		t.Errorf("expected sunlust dsda-doom engine-args [-solonet -fast], got %v",
			pSunlust.ArgsForEngine("dsda-doom"))
	}
	if !reflect.DeepEqual(pSunlust.ArgsForEngine("uzdoom"), []string{"+gl_tonemap", "0"}) {
		t.Errorf("expected sunlust uzdoom engine-args [+gl_tonemap 0], got %v",
			pSunlust.ArgsForEngine("uzdoom"))
	}

	// Verify default-disabled optional files (nostalgia) vs default-enabled (no-end-in-sight) and IncludeFiles toggle
	pNEIS, err := mergedSunlustCat.FindExact("no-end-in-sight")
	if err != nil || len(pNEIS.Files) != 3 || !pNEIS.Files[2].Optional || pNEIS.Files[2].Disabled {
		t.Fatalf("expected no-end-in-sight to include default-enabled idkfa 2024.wad, got %+v (err=%v)", pNEIS, err)
	}
	pNostalgiaDefault, err := mergedSunlustCat.FindExact("nostalgia")
	if err != nil || len(pNostalgiaDefault.Files) != 2 || !pNostalgiaDefault.Files[1].Disabled {
		t.Fatalf("expected nostalgia idkfa 2024.wad Disabled=true by default, got %+v (err=%v)", pNostalgiaDefault, err)
	}
	included := []string{"idkfa_2024.wad"}
	if _, _, err := ConfigPreset(userDir, mergedSunlustCat, "nostalgia", OverridePatchRequest{
		IncludeFiles: &included,
	}); err != nil {
		t.Fatalf("ConfigPreset IncludeFiles: %v", err)
	}
	baseNos, ovNos, err := LoadPresetBaseAndOverride(userDir, "nostalgia")
	if err != nil || baseNos == nil || !baseNos.Files[1].Disabled {
		t.Fatalf("expected base nostalgia to retain Disabled=true, got %+v (err=%v)", baseNos, err)
	}
	if ovNos == nil || !reflect.DeepEqual(ovNos.IncludeFiles, []string{"idkfa_2024.wad"}) {
		t.Fatalf("expected ovNos.IncludeFiles=[idkfa_2024.wad], got %+v", ovNos)
	}
	enabledCat, err := MergeUserDir(mergedSunlustCat, userDir)
	if err != nil {
		t.Fatalf("MergeUserDir after IncludeFiles: %v", err)
	}
	pNosEnabled, err := enabledCat.FindExact("nostalgia")
	if err != nil || pNosEnabled.Files[1].Disabled {
		t.Fatalf("expected nostalgia idkfa 2024.wad enabled after IncludeFiles, got %+v (err=%v)", pNosEnabled, err)
	}
	// Passing ExcludeFiles without IncludeFiles removes matching entry from IncludeFiles
	reExclude := []string{"idkfa 2024.wad"}
	if _, _, err := ConfigPreset(userDir, enabledCat, "nostalgia", OverridePatchRequest{
		ExcludeFiles: &reExclude,
	}); err != nil {
		t.Fatalf("ConfigPreset ExcludeFiles after IncludeFiles: %v", err)
	}
	_, ovNosAfterEx, err := LoadPresetBaseAndOverride(userDir, "nostalgia")
	if err != nil || ovNosAfterEx == nil || len(ovNosAfterEx.IncludeFiles) != 0 {
		t.Fatalf("expected IncludeFiles cleared when excluded, got %+v (err=%v)", ovNosAfterEx, err)
	}
	// Non-optional file with disabled: true is rejected by DecodeEntity
	invalidDisabledJSON := `{"schema-version":1,"kind":"preset","id":"bad","name":"Bad","engine":"dsda-doom",` +
		`"iwad":"DOOM2.WAD","files":[{"name":"bad.wad","disabled":true}]}`
	if _, decErr := DecodeEntity([]byte(invalidDisabledJSON)); decErr == nil {
		t.Error("expected DecodeEntity to reject disabled:true without optional:true")
	}

	// Clear dsda-doom engine-args and revert Complevel back to curated 9 while keeping uzdoom engine-args
	clearDSDA := map[string][]string{"dsda-doom": {}}
	if _, _, err := ConfigPreset(userDir, mergedSunlustCat, "sunlust", OverridePatchRequest{
		ClearComplevel: true,
		SetEngineArgs:  &clearDSDA,
	}); err != nil {
		t.Fatalf("ConfigPreset clear complevel/dsda-doom: %v", err)
	}
	reloadedSunlustCat, err := LoadLayered(userDir, false)
	if err != nil {
		t.Fatalf("LoadLayered sunlust after clear: %v", err)
	}
	pSunlustCleared := reloadedSunlustCat.Find("sunlust")
	if pSunlustCleared == nil || pSunlustCleared.Complevel == nil || *pSunlustCleared.Complevel != 9 {
		t.Fatalf("expected sunlust Complevel reverted to curated 9, got %+v", pSunlustCleared)
	}
	if len(pSunlustCleared.ArgsForEngine("dsda-doom")) != 0 {
		t.Errorf("expected cleared dsda-doom engine-args, got %v", pSunlustCleared.ArgsForEngine("dsda-doom"))
	}
	if !reflect.DeepEqual(pSunlustCleared.ArgsForEngine("uzdoom"), []string{"+gl_tonemap", "0"}) {
		t.Errorf("expected untouched uzdoom engine-args preserved, got %v",
			pSunlustCleared.ArgsForEngine("uzdoom"))
	}
}
