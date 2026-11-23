package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/catalog"
)

func TestPrepareLaunch_DSDADoom(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(wadsDir, 0o755)

	fakeBin := filepath.Join(binDir, "dsda-doom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)

	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.wad"), []byte("wad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.deh"), []byte("deh"), 0o644)

	p := catalog.Preset{
		Name:   "Alien Vendetta",
		Engine: "dsda-doom",
		IWAD:   "doom2.wad",
		Files:  []catalog.FileSpec{{Name: "av.wad"}, {Name: "av.deh"}},
		Args:   []string{"-complevel", "2"},
	}

	opts := LaunchOptions{
		BinDir:    binDir,
		WadRoots:  []string{wadsDir},
		DryRun:    true,
		ExtraArgs: []string{"-skill", "4"},
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}

	if plan.Engine != "dsda-doom" {
		t.Errorf("expected dsda-doom, got %s", plan.Engine)
	}
	if plan.EngineBin != fakeBin {
		t.Errorf("expected binary %s, got %s", fakeBin, plan.EngineBin)
	}

	joinedArgs := strings.Join(plan.Args, " ")
	expectedParts := []string{
		"-iwad " + filepath.Join(wadsDir, "DOOM2.WAD"),
		"-file " + filepath.Join(wadsDir, "av.wad"),
		"-deh " + filepath.Join(wadsDir, "av.deh"),
		"-complevel 2",
		"-skill 4",
	}
	lowerArgs := strings.ToLower(joinedArgs)
	for _, part := range expectedParts {
		if !strings.Contains(lowerArgs, strings.ToLower(part)) {
			t.Errorf("expected args to contain %q, got: %s", part, joinedArgs)
		}
	}

	var out bytes.Buffer
	if err := Execute(plan, &out, &out); err != nil {
		t.Fatalf("dry run Execute failed: %v", err)
	}
	if !strings.Contains(out.String(), "Launching Preset: Alien Vendetta") {
		t.Errorf("expected banner in output, got:\n%s", out.String())
	}
}

func TestPrepareLaunch_UZDoom_OptionalAudioAndLoadMapsAfterMods(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(wadsDir, 0o755)

	fakeBin := filepath.Join(binDir, "uzdoom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)

	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "eviternity.wad"), []byte("wad"), 0o644)

	p := catalog.Preset{
		Name:   "Eviternity",
		Engine: "uzdoom",
		IWAD:   "doom2.wad",
		Files: []catalog.FileSpec{
			{Name: "idkfa 2024.wad", Optional: true},
			{Name: "eviternity.wad"},
		},
	}

	var out bytes.Buffer
	opts := LaunchOptions{
		BinDir:   binDir,
		WadRoots: []string{wadsDir},
		DryRun:   true,
		Out:      &out,
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}
	if strings.Contains(strings.Join(plan.Args, " "), "idkfa 2024.wad") {
		t.Errorf("did not expect missing idkfa 2024.wad in args: %v", plan.Args)
	}

	if !strings.Contains(out.String(), "Optional soundtrack 'idkfa 2024.wad' not found") {
		t.Errorf("expected optional soundtrack warning, got: %s", out.String())
	}

	// Now create idkfa 2024.wad and verify LoadMapsAfterMods reorders optional mods before required maps
	_ = os.WriteFile(filepath.Join(wadsDir, "idkfa 2024.wad"), []byte("music"), 0o644)
	pReorder := catalog.Preset{
		Name:              "Sigil II",
		Engine:            "uzdoom",
		IWAD:              "DOOM2.WAD",
		LoadMapsAfterMods: true,
		Files: []catalog.FileSpec{
			{Name: "eviternity.wad", Optional: false},
			{Name: "idkfa 2024.wad", Optional: true},
		},
	}
	planReorder, err := PrepareLaunch(pReorder, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch reorder failed: %v", err)
	}
	if len(planReorder.OrderedFiles) != 2 ||
		filepath.Base(planReorder.OrderedFiles[0]) != "idkfa 2024.wad" ||
		filepath.Base(planReorder.OrderedFiles[1]) != "eviternity.wad" {
		t.Fatalf("expected optional mod before map when LoadMapsAfterMods=true, got: %v", planReorder.OrderedFiles)
	}
}

func TestPrepareLaunch_CustomEnginePrefixArgsAndBex(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	customBinDir := filepath.Join(tmpDir, "custom_bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(customBinDir, 0o755)
	_ = os.MkdirAll(wadsDir, 0o755)

	customBinPath := filepath.Join(customBinDir, "woof")
	_ = os.WriteFile(customBinPath, []byte("#!/bin/sh\n"), 0o755)

	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "sunlust.wad"), []byte("wad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "sunlust.bex"), []byte("bex"), 0o644)

	customEngines := map[string]catalog.EngineConfig{
		"woof": {
			ID:          "woof",
			Binary:      customBinPath,
			ArgsStyle:   "boom",
			PrefixArgs:  []string{"-config", "/tmp/woof.cfg"},
			DefaultArgs: []string{"-geometry", "1920x1080"},
		},
	}

	p := catalog.Preset{
		Name:   "Sunlust",
		Engine: "woof",
		IWAD:   "DOOM2.WAD",
		Files:  []catalog.FileSpec{{Name: "sunlust.wad"}, {Name: "sunlust.bex"}},
		Args:   []string{"-skill", "4"},
	}

	opts := LaunchOptions{
		BinDir:    binDir,
		WadRoots:  []string{wadsDir},
		Catalog:   &catalog.Catalog{Engines: customEngines},
		DryRun:    true,
		ExtraArgs: []string{"-warp", "15"},
	}

	plan, err := PrepareLaunch(p, opts)
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}

	if plan.EngineBin != customBinPath {
		t.Errorf("expected engine binary %s, got %s", customBinPath, plan.EngineBin)
	}

	joined := strings.Join(plan.Args, " ")
	expectedTokens := []string{
		"-config /tmp/woof.cfg -iwad",
		"-file " + filepath.Join(wadsDir, "sunlust.wad"),
		"-deh " + filepath.Join(wadsDir, "sunlust.bex"),
		"-geometry 1920x1080",
		"-skill 4",
		"-warp 15",
	}
	for _, tok := range expectedTokens {
		if !strings.Contains(joined, tok) {
			t.Errorf("expected args to contain %q, got: %s", tok, joined)
		}
	}
}

func TestPrepareLaunch_CustomFileSpecAliases(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.MkdirAll(wadsDir, 0o755)

	fakeBin := filepath.Join(binDir, "dsda-doom")
	_ = os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	aliasWad := filepath.Join(wadsDir, "custom_short.wad")
	_ = os.WriteFile(aliasWad, []byte("pwad"), 0o644)

	p := catalog.Preset{
		ID:     "custom-alias-pack",
		Name:   "Custom Alias Pack",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files: []catalog.FileSpec{
			{Name: "custom_canonical_v2.wad", Aliases: []string{"custom_short.wad"}},
		},
	}

	plan, err := PrepareLaunch(p, LaunchOptions{
		BinDir:   binDir,
		WadRoots: []string{wadsDir},
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("PrepareLaunch failed to resolve custom FileSpec.Aliases: %v", err)
	}
	if len(plan.OrderedFiles) != 1 || plan.OrderedFiles[0] != aliasWad {
		t.Fatalf("OrderedFiles = %v, want [%s]", plan.OrderedFiles, aliasWad)
	}
}

func TestFormatLaunchPreview(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.wad"), []byte("pwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "av.deh"), []byte("deh"), 0o644)

	idx := catalog.NewDirIndex(wadsDir)
	p := catalog.Preset{
		ID:     "alien-vendetta",
		Name:   "Alien Vendetta",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Args:   []string{"-complevel", "2", "-skill", "4"},
		Files: []catalog.FileSpec{
			{Name: "av.wad"},
			{Name: "av.deh"},
			{Name: "idkfa 2024.wad", Optional: true},
		},
	}

	preview := FormatLaunchPreview(p, nil, idx, []string{"-fast"})
	wantParts := []string{
		"dsda-doom -iwad " + filepath.Join(wadsDir, "DOOM2.WAD"),
		"-file " + filepath.Join(wadsDir, "av.wad") + " -deh " + filepath.Join(wadsDir, "av.deh"),
		"-complevel 2 -skill 4 -fast",
	}
	for _, want := range wantParts {
		if !strings.Contains(preview, want) {
			t.Errorf("FormatLaunchPreview = %q, missing %q", preview, want)
		}
	}
	if strings.Contains(preview, "idkfa") {
		t.Errorf("FormatLaunchPreview with missing optional file should omit idkfa 2024.wad, got %q", preview)
	}

	idkfaPath := filepath.Join(wadsDir, "idkfa 2024.wad")
	_ = os.WriteFile(idkfaPath, []byte("music"), 0o644)
	idxWithOptional := catalog.NewDirIndex(wadsDir)
	previewWithOptional := FormatLaunchPreview(p, nil, idxWithOptional, []string{"-fast"})
	wantWithOptional := "-file " + filepath.Join(wadsDir, "av.wad") + " " + idkfaPath
	if !strings.Contains(previewWithOptional, wantWithOptional) {
		t.Errorf(
			"FormatLaunchPreview with present optional file = %q, missing %q",
			previewWithOptional, wantWithOptional,
		)
	}

	previewNoIdx := FormatLaunchPreview(p, nil, nil, []string{"-fast"})
	if !strings.Contains(previewNoIdx, "dsda-doom -iwad DOOM2.WAD -file av.wad idkfa 2024.wad -deh av.deh") {
		t.Errorf("FormatLaunchPreview(nil idx) = %q", previewNoIdx)
	}

	// Multiple .deh and .bex files mixed with present and missing optional .wad files
	_ = os.WriteFile(filepath.Join(wadsDir, "patch2.bex"), []byte("bex"), 0o644)
	idxMulti := catalog.NewDirIndex(wadsDir)
	pMulti := catalog.Preset{
		ID:     "multi-patch",
		Name:   "Multi Patch",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files: []catalog.FileSpec{
			{Name: "av.wad"},
			{Name: "av.deh"},
			{Name: "missing_opt.wad", Optional: true},
			{Name: "patch2.bex"},
			{Name: "idkfa 2024.wad", Optional: true},
		},
	}
	previewMulti := FormatLaunchPreview(pMulti, nil, idxMulti, nil)
	wantMulti := "-file " + filepath.Join(wadsDir, "av.wad") + " " + idkfaPath +
		" -deh " + filepath.Join(wadsDir, "av.deh") + " " + filepath.Join(wadsDir, "patch2.bex")
	if !strings.Contains(previewMulti, wantMulti) {
		t.Errorf("FormatLaunchPreview(multi deh/bex + mixed optional) = %q, want %q", previewMulti, wantMulti)
	}
	if strings.Contains(previewMulti, "missing_opt.wad") {
		t.Errorf("FormatLaunchPreview should omit missing_opt.wad, got %q", previewMulti)
	}

	// Disabled optional files are omitted even when present on disk
	pDisabled := catalog.Preset{
		ID:     "nostalgia",
		Name:   "Nostalgia",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files: []catalog.FileSpec{
			{Name: "av.wad"},
			{Name: "idkfa 2024.wad", Optional: true, Disabled: true},
		},
	}
	if previewDisabled := FormatLaunchPreview(pDisabled, nil, idxWithOptional, nil); strings.Contains(
		previewDisabled, "idkfa",
	) {
		t.Errorf("FormatLaunchPreview should omit disabled optional file, got %q", previewDisabled)
	}
	if previewDisabledNilIdx := FormatLaunchPreview(pDisabled, nil, nil, nil); strings.Contains(
		previewDisabledNilIdx, "idkfa",
	) {
		t.Errorf("FormatLaunchPreview(nil idx) should omit disabled optional file, got %q", previewDisabledNilIdx)
	}
}

func TestExecute_RelativePathsAndDashPrefixedWAD(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell script mock skipped on windows")
	}
	t.Parallel()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	wadsDir := filepath.Join(tmpDir, "wads")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wadsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	outMarker := filepath.Join(tmpDir, "invoked.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellSingleQuote(outMarker) + "\n"
	fakeBin := filepath.Join(binDir, "dsda-doom")
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wadsDir, "-tricky.wad"), []byte("pwad"), 0o644); err != nil {
		t.Fatal(err)
	}

	relBinDir, err := filepath.Rel(wd, binDir)
	if err != nil {
		t.Fatal(err)
	}
	relWadsDir, err := filepath.Rel(wd, wadsDir)
	if err != nil {
		t.Fatal(err)
	}

	p := catalog.Preset{
		Name:   "Relative Launch",
		Engine: "dsda-doom",
		IWAD:   "DOOM2.WAD",
		Files:  []catalog.FileSpec{{Name: "-tricky.wad"}},
	}
	plan, err := PrepareLaunch(p, LaunchOptions{
		BinDir:   relBinDir,
		WadRoots: []string{relWadsDir},
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("PrepareLaunch failed: %v", err)
	}
	if !filepath.IsAbs(plan.EngineBin) {
		t.Fatalf("expected absolute EngineBin, got %q", plan.EngineBin)
	}
	for _, f := range plan.OrderedFiles {
		if !filepath.IsAbs(f) || strings.HasPrefix(f, "-") {
			t.Fatalf("expected absolute non-flag OrderedFile path, got %q", f)
		}
	}

	var out bytes.Buffer
	if err := Execute(plan, &out, &out); err != nil {
		t.Fatalf("live Execute with relative BinDir and WadRoots failed: %v", err)
	}
	recorded, err := os.ReadFile(outMarker)
	if err != nil {
		t.Fatalf("mock engine binary did not write marker: %v", err)
	}
	if !strings.Contains(string(recorded), filepath.Join(wadsDir, "-tricky.wad")) {
		t.Errorf("recorded args missing absolute -tricky.wad path:\n%s", string(recorded))
	}
}

func TestBuildEngineArgs_ComplevelAndEngineArgsIsolation(t *testing.T) {
	t.Parallel()
	wadsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wadsDir, "DOOM2.WAD"), []byte("iwad"), 0o644)
	_ = os.WriteFile(filepath.Join(wadsDir, "Eviternity.wad"), []byte("pwad"), 0o644)

	cat, err := catalog.LoadCurated()
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	cat.Engines["woof"] = catalog.EngineConfig{
		ID:        "woof",
		Binary:    "woof",
		ArgsStyle: "boom",
	}

	cl11 := 11
	p := catalog.Preset{
		ID:        "eviternity",
		Name:      "Eviternity",
		Engine:    "uzdoom",
		IWAD:      "DOOM2.WAD",
		Files:     []catalog.FileSpec{{Name: "Eviternity.wad"}},
		Complevel: &cl11,
		EngineArgs: map[string][]string{
			"dsda-doom": {"-solonet"},
			"uzdoom":    {"+gl_tonemap", "3"},
		},
		Args: []string{"-skill", "4"},
	}
	idx := catalog.NewDirIndex(wadsDir).WithCatalog(cat)

	// 1. Default uzdoom omits -complevel and dsda-doom engine-args, includes uzdoom engine-args + universal args
	uzPreview := FormatLaunchPreview(p, cat, idx, nil)
	if strings.Contains(uzPreview, "-complevel") || strings.Contains(uzPreview, "-solonet") {
		t.Errorf("expected uzdoom preview to omit Boom complevel/solonet, got %q", uzPreview)
	}
	if !strings.Contains(uzPreview, "+gl_tonemap 3 -skill 4") {
		t.Errorf("expected uzdoom preview to contain uzdoom engine-args and universal args, got %q", uzPreview)
	}

	// 2. Switching to dsda-doom emits -complevel 11 + dsda-doom engine-args (-solonet) + universal args
	pDSDA := p
	pDSDA.Engine = "dsda-doom"
	dsdaPreview := FormatLaunchPreview(pDSDA, cat, idx, nil)
	if !strings.Contains(dsdaPreview, "-complevel 11 -solonet -skill 4") {
		t.Errorf("expected dsda-doom preview to include '-complevel 11 -solonet -skill 4', got %q", dsdaPreview)
	}
	if strings.Contains(dsdaPreview, "+gl_tonemap") {
		t.Errorf("expected dsda-doom preview to omit uzdoom engine-args, got %q", dsdaPreview)
	}

	// 3. Switching to custom Boom engine woof automatically inherits -complevel 11 without dsda-doom's -solonet
	pWoof := p
	pWoof.Engine = "woof"
	woofPreview := FormatLaunchPreview(pWoof, cat, idx, nil)
	if !strings.Contains(woofPreview, "-complevel 11 -skill 4") || strings.Contains(woofPreview, "-solonet") {
		t.Errorf("expected woof preview to inherit '-complevel 11' without dsda-doom '-solonet', got %q", woofPreview)
	}

	// 4. Explicit CLI extraArgs -complevel overrides p.Complevel without duplicating -complevel
	overridePreview := FormatLaunchPreview(pDSDA, cat, idx, []string{"-complevel", "21"})
	if strings.Contains(overridePreview, "-complevel 11") || !strings.Contains(overridePreview, "-complevel 21") {
		t.Errorf("expected CLI -complevel 21 to supersede base -complevel 11, got %q", overridePreview)
	}
}
