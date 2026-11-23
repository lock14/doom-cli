package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseRootLaunchArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rawArgs    []string
		wantPreset string
		wantExtra  []string
	}{
		{
			name:       "naked doom defaults to doom preset",
			rawArgs:    []string{"doom"},
			wantPreset: "doom",
			wantExtra:  nil,
		},
		{
			name:       "naked doom with single-dash engine flags defaults to doom preset",
			rawArgs:    []string{"doom", "-skill", "4", "-warp", "1", "1"},
			wantPreset: "doom",
			wantExtra:  []string{"-skill", "4", "-warp", "1", "1"},
		},
		{
			name:       "naked doom with CLI flags only defaults to doom preset",
			rawArgs:    []string{"doom", "-e", "uzdoom", "--dry-run"},
			wantPreset: "doom",
			wantExtra:  nil,
		},
		{
			name:       "explicit wad preset with engine flags",
			rawArgs:    []string{"doom", "sunlust", "-skill", "4", "-pistolstart"},
			wantPreset: "sunlust",
			wantExtra:  []string{"-skill", "4", "-pistolstart"},
		},
		{
			name:       "multi-word unquoted wad preset before engine flags",
			rawArgs:    []string{"doom", "Doom", "II", "-skill", "4"},
			wantPreset: "Doom II",
			wantExtra:  []string{"-skill", "4"},
		},
		{
			name:       "plus-prefixed console commands treated as engine flags",
			rawArgs:    []string{"doom", "Doom", "II", "+map", "MAP01", "+sv_cheats", "1"},
			wantPreset: "Doom II",
			wantExtra:  []string{"+map", "MAP01", "+sv_cheats", "1"},
		},
		{
			name:       "CLI flags before explicit wad preset and engine flags",
			rawArgs:    []string{"doom", "--dry-run", "-e", "uzdoom", "Eviternity II", "-skill", "4"},
			wantPreset: "Eviternity II",
			wantExtra:  []string{"-skill", "4"},
		},
		{
			name: "CLI flags with equals syntax and dash-dash separator",
			rawArgs: []string{
				"doom", "--engine=uzdoom", "--dry-run=true", "sunlust", "--", "-nomonsters", "-fast",
			},
			wantPreset: "sunlust",
			wantExtra:  []string{"-nomonsters", "-fast"},
		},
		{
			name:       "naked doom with dash-dash separator",
			rawArgs:    []string{"doom", "--dry-run", "--", "-nomonsters", "-fast"},
			wantPreset: "doom",
			wantExtra:  []string{"-nomonsters", "-fast"},
		},
		{
			name:       "trailing dash-dash with no extra args",
			rawArgs:    []string{"doom", "--"},
			wantPreset: "doom",
			wantExtra:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotPreset, gotExtra, _, _, _, err := parseRootInvocation(tt.rawArgs)
			if err != nil {
				t.Fatalf("parseRootInvocation(%v) unexpected error: %v", tt.rawArgs, err)
			}
			if gotPreset != tt.wantPreset {
				t.Errorf("parseRootInvocation(%v) preset = %q, want %q", tt.rawArgs, gotPreset, tt.wantPreset)
			}
			if len(gotExtra) == 0 && len(tt.wantExtra) == 0 {
				return
			}
			if !reflect.DeepEqual(gotExtra, tt.wantExtra) {
				t.Errorf("parseRootInvocation(%v) extra = %v, want %v", tt.rawArgs, gotExtra, tt.wantExtra)
			}
		})
	}
}

func TestParseRootInvocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rawArgs    []string
		wantPreset string
		wantExtra  []string
		wantEngine string
		wantDryRun bool
		wantErr    string
	}{
		{
			name:       "attached -euzdoom shorthand",
			rawArgs:    []string{"doom", "-euzdoom", "--dry-run", "Doom", "II"},
			wantPreset: "Doom II",
			wantEngine: "uzdoom",
			wantDryRun: true,
		},
		{
			name:       "attached -e=uzdoom shorthand",
			rawArgs:    []string{"doom", "-e=uzdoom", "sunlust"},
			wantPreset: "sunlust",
			wantEngine: "uzdoom",
		},
		{
			name:       "single-dash -episode and -extratics forwarded to engine",
			rawArgs:    []string{"doom", "-episode", "2", "-extratics", "1", "--dry-run"},
			wantPreset: "doom",
			wantExtra:  []string{"-episode", "2", "-extratics", "1"},
			wantDryRun: true,
		},
		{
			name:    "unknown double-dash flag before separator rejected",
			rawArgs: []string{"doom", "--dryrun"},
			wantErr: "unknown flag: --dryrun",
		},
		{
			name:       "double-dash token after separator forwarded to engine",
			rawArgs:    []string{"doom", "sunlust", "--", "--custom-engine-arg"},
			wantPreset: "sunlust",
			wantExtra:  []string{"--custom-engine-arg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotPreset, gotExtra, gotEng, _, gotDry, err := parseRootInvocation(tt.rawArgs)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseRootInvocation(%v) error = %v, want %q", tt.rawArgs, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRootInvocation(%v) unexpected error: %v", tt.rawArgs, err)
			}
			if gotPreset != tt.wantPreset || gotEng != tt.wantEngine || gotDry != tt.wantDryRun {
				t.Errorf(
					"parseRootInvocation(%v) = (%q, %v, %q, %v), want (%q, %v, %q, %v)",
					tt.rawArgs, gotPreset, gotExtra, gotEng, gotDry,
					tt.wantPreset, tt.wantExtra, tt.wantEngine, tt.wantDryRun,
				)
			}
			if !(len(gotExtra) == 0 && len(tt.wantExtra) == 0) && !reflect.DeepEqual(gotExtra, tt.wantExtra) {
				t.Errorf("parseRootInvocation(%v) extra = %v, want %v", tt.rawArgs, gotExtra, tt.wantExtra)
			}
		})
	}
}

func TestNewRootCmd(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd()
	if cmd == nil {
		t.Fatal("expected non-nil root command")
	}

	expectedSubcommands := []string{
		"wads", "engines", "config", "setup",
	}

	foundSubcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		foundSubcommands[sub.Name()] = true
	}

	for _, expected := range expectedSubcommands {
		if !foundSubcommands[expected] {
			t.Errorf("expected subcommand %q not found in root command", expected)
		}
	}

	removedSubcommands := []string{
		"play", "launch", "presets", "themes", "soundfont",
	}
	for _, removed := range removedSubcommands {
		if foundSubcommands[removed] {
			t.Errorf("expected legacy subcommand %q to be removed from root command", removed)
		}
	}

	// Verify local root flags (-e/--engine and --dry-run)
	engFlag := cmd.Flags().Lookup("engine")
	if engFlag == nil || engFlag.Shorthand != "e" {
		t.Errorf("expected rootCmd to have local --engine (-e) flag, got %+v", engFlag)
	}
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Error("expected rootCmd to have local --dry-run flag")
	}

	for _, removed := range []string{
		"wads-dir", "theme", "nerd-fonts",
		"wads-dirs", "bin-dir", "soundfonts-dir", "presets-file", "once", "force",
	} {
		if cmd.PersistentFlags().Lookup(removed) != nil || cmd.Flags().Lookup(removed) != nil {
			t.Errorf("expected flag %q to be absent from root command", removed)
		}
	}
	if !cmd.FParseErrWhitelist.UnknownFlags {
		t.Error("expected rootCmd.FParseErrWhitelist.UnknownFlags to be true")
	}

	// Verify all 12 non-positional commands reject unexpected positional arguments
	noArgsCommands := [][]string{
		{"wads"},
		{"wads", "list"},
		{"wads", "scan"},
		{"engines"},
		{"engines", "list"},
		{"engines", "cfg"},
		{"engines", "cfg", "install"},
		{"engines", "cfg", "diff"},
		{"config"},
		{"config", "show"},
		{"config", "themes"},
		{"setup"},
	}
	for _, path := range noArgsCommands {
		sub, _, err := cmd.Find(path)
		if err != nil {
			t.Fatalf("cmd.Find(%v): %v", path, err)
		}
		if sub.Args == nil {
			t.Errorf("expected command %v to set Args: cobra.NoArgs", path)
			continue
		}
		if err := sub.Args(sub, []string{"unexpected"}); err == nil {
			t.Errorf("expected command %v to reject unexpected positional arguments", path)
		}
	}

	// Verify bare `doom -skill 4` routes to rootCmd and passes argument validation
	targetCmd, flags, findErr := cmd.Find([]string{"-skill", "4"})
	if findErr != nil {
		t.Fatalf("cmd.Find([-skill 4]) unexpected error: %v", findErr)
	}
	if targetCmd != cmd {
		t.Fatalf("expected rootCmd to be matched for bare engine flags, got %q", targetCmd.Name())
	}
	if err := targetCmd.ParseFlags(flags); err != nil {
		t.Fatalf("targetCmd.ParseFlags([-skill 4]) unexpected error: %v", err)
	}
	if err := targetCmd.ValidateArgs(targetCmd.Flags().Args()); err != nil {
		t.Fatalf("targetCmd.ValidateArgs([4]) unexpected error: %v", err)
	}
}

func TestRootCmd_Version(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd()
	if cmd.Version == "" {
		t.Fatalf("expected rootCmd.Version to be non-empty")
	}
	if !strings.Contains(cmd.Version, "commit:") || !strings.Contains(cmd.Version, "built:") {
		t.Errorf("expected rootCmd.Version to contain commit and built metadata, got %q", cmd.Version)
	}
}

func TestSubcommandFlagScoping(t *testing.T) {
	t.Parallel()

	// 1. root command has local --engine (-e) and --dry-run flags, and wads command does not
	rootCmd := newRootCmd()
	engFlag := rootCmd.Flags().Lookup("engine")
	if engFlag == nil || engFlag.Shorthand != "e" {
		t.Errorf("expected rootCmd to have --engine (-e), got %+v", engFlag)
	}
	if rootCmd.Flags().Lookup("dry-run") == nil {
		t.Error("expected rootCmd to have --dry-run flag")
	}

	wadsCmd := newWadsCmd()
	for _, f := range []string{"once", "dry-run", "engine"} {
		if wadsCmd.Flags().Lookup(f) != nil {
			t.Errorf("expected wadsCmd not to have local flag %q", f)
		}
	}

	// 2. engines add and engines set flags
	enginesCmd := newEnginesCmd()
	addEngCmd, _, err := enginesCmd.Find([]string{"add"})
	if err != nil {
		t.Fatalf("enginesCmd.Find(add): %v", err)
	}
	for _, want := range []string{"id", "binary", "style", "args", "prefix-args", "badge", "description"} {
		if addEngCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected engines add to have flag %q", want)
		}
	}

	setEngCmd, _, err := enginesCmd.Find([]string{"set"})
	if err != nil {
		t.Fatalf("enginesCmd.Find(set): %v", err)
	}
	for _, want := range []string{"binary", "style", "args", "prefix-args", "badge", "display-name"} {
		if setEngCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected engines set to have flag %q", want)
		}
	}
	if setEngCmd.Flags().Lookup("reset") != nil {
		t.Error("expected engines set not to have --reset flag (reset is a separate subcommand)")
	}

	// 3. wads add and wads set flags
	addWadCmd, _, err := wadsCmd.Find([]string{"add"})
	if err != nil {
		t.Fatalf("wadsCmd.Find(add): %v", err)
	}
	for _, want := range []string{"id", "engine", "iwad", "files", "args", "description"} {
		if addWadCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected wads add to have flag %q", want)
		}
	}
	if f := addWadCmd.Flags().Lookup("engine"); f == nil || f.Shorthand != "e" {
		t.Errorf("expected wads add --engine to have shorthand 'e', got %+v", f)
	}

	setWadCmd, _, err := wadsCmd.Find([]string{"set"})
	if err != nil {
		t.Fatalf("wadsCmd.Find(set): %v", err)
	}
	if f := setWadCmd.Flags().Lookup("engine"); f == nil || f.Shorthand != "e" {
		t.Errorf("expected wads set --engine to have shorthand 'e', got %+v", f)
	}
	if setWadCmd.Flags().Lookup("reset") != nil {
		t.Error("expected wads set not to have --reset flag (reset is a separate subcommand)")
	}

	// 4. --force is scoped locally to setup, wads fetch, wads scan, and engines install
	setupCmd := newSetupCmd()
	for _, want := range []string{"force", "games", "wads"} {
		if setupCmd.Flags().Lookup(want) == nil {
			t.Errorf("expected setupCmd to have local --%s flag", want)
		}
	}
	if wadsCmd.Flags().Lookup("all") == nil {
		t.Error("expected wadsCmd to have local --all flag")
	}
	listWadCmd, _, err := wadsCmd.Find([]string{"list"})
	if err != nil || listWadCmd.Flags().Lookup("all") == nil {
		t.Errorf("expected wads list to have local --all flag (err=%v)", err)
	}
	for _, sub := range []string{"fetch", "scan"} {
		subCmd, _, findErr := wadsCmd.Find([]string{sub})
		if findErr != nil {
			t.Fatalf("wadsCmd.Find(%s): %v", sub, findErr)
		}
		if subCmd.Flags().Lookup("force") == nil {
			t.Errorf("expected wads %s to have local --force flag", sub)
		}
		if subCmd.Flags().Lookup("games") == nil {
			t.Errorf("expected wads %s to have local --games flag", sub)
		}
	}
	instEngCmd, _, err := enginesCmd.Find([]string{"install"})
	if err != nil {
		t.Fatalf("enginesCmd.Find(install): %v", err)
	}
	if instEngCmd.Flags().Lookup("force") == nil {
		t.Error("expected engines install to have local --force flag")
	}
	if rootCmd.Flags().Lookup("force") != nil || wadsCmd.Flags().Lookup("force") != nil {
		t.Error("expected rootCmd and wadsCmd not to have --force flag")
	}
}

func TestRootDirectLauncherDryRunAndEngineOverride(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))
	for _, kv := range [][2]string{
		{"bin-dir", tempDir},
		{"wads-dir", tempDir},
	} {
		if err := runConfigSet(nil, []string{kv[0], kv[1]}); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(tempDir, "DOOM.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "DOOM2.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "TNT.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, binName := range []string{"uzdoom", "uzdoom.exe", "dsda-doom", "dsda-doom.exe"} {
		if err := os.WriteFile(filepath.Join(tempDir, binName), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Naked `doom --dry-run` launches The Ultimate Doom (DOOM.WAD) in dsda-doom
	var out bytes.Buffer
	rootCmd := newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom --dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "dsda-doom") || !strings.Contains(out.String(), "DOOM.WAD") {
		t.Errorf("expected naked doom --dry-run to launch DOOM.WAD in dsda-doom, got:\n%s", out.String())
	}

	// 2. Unquoted `doom --dry-run -euzdoom Doom II +map MAP01` launches DOOM2.WAD in uzdoom with +map MAP01
	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"--dry-run", "-euzdoom", "Doom", "II", "+map", "MAP01"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom --dry-run -euzdoom Doom II +map MAP01 failed: %v", err)
	}
	if !strings.Contains(out.String(), "uzdoom") ||
		!strings.Contains(out.String(), "DOOM2.WAD") ||
		!strings.Contains(out.String(), "+map MAP01") {
		t.Errorf("expected dry-run output to contain uzdoom, DOOM2.WAD, and +map MAP01, got:\n%s", out.String())
	}

	// 2b. Unknown double-dash flag before `--` is rejected with unknown flag error
	rootCmd = newRootCmd()
	rootCmd.SetArgs([]string{"--dryrun"})
	if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag: --dryrun") {
		t.Errorf("expected unknown flag error for --dryrun, got %v", err)
	}

	// 3. `doom -skill 4 -warp 1 1 --dry-run` launches DOOM.WAD with forwarded engine flags and without --dry-run leaked
	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"-skill", "4", "-warp", "1", "1", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom -skill 4 -warp 1 1 --dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "DOOM.WAD") ||
		!strings.Contains(out.String(), "-skill 4 -warp 1 1") ||
		strings.Contains(out.String(), "--dry-run") {
		t.Errorf(
			"expected naked doom with engine flags to launch DOOM.WAD with -skill 4 -warp 1 1 (no --dry-run), got:\n%s",
			out.String(),
		)
	}

	// 4. Single-dash Doom engine flags containing 'e', 'h', or 'v', or ending in 'e'
	// (-nomonsters, -complevel 21, -shorttics, -viddump, -episode 2, -height 1080, -he, -ve, -respawn, -deh, -nomouse)
	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{
		"--dry-run",
		"-nomonsters",
		"-complevel", "21",
		"-shorttics",
		"-viddump", "out.mkv",
		"-episode", "2",
		"-height", "1080",
		"-he",
		"-ve",
		"-respawn",
		"-deh", "foo.deh",
		"-nomouse",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom with single-dash e/h/v flags failed: %v", err)
	}
	wantEngineFlags := "-nomonsters -complevel 21 -shorttics -viddump out.mkv " +
		"-episode 2 -height 1080 -he -ve -respawn -deh foo.deh -nomouse"
	if !strings.Contains(out.String(), wantEngineFlags) {
		t.Errorf("expected dry-run output to contain %q, got:\n%s", wantEngineFlags, out.String())
	}

	// 4b. Multi-word preset followed by -nomonsters -episode 2
	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"--dry-run", "Doom", "II", "-nomonsters", "-episode", "2"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom --dry-run Doom II -nomonsters -episode 2 failed: %v", err)
	}
	if !strings.Contains(out.String(), "DOOM2.WAD") ||
		!strings.Contains(out.String(), "-nomonsters -episode 2") {
		t.Errorf("expected DOOM2.WAD with -nomonsters -episode 2, got:\n%s", out.String())
	}

	// 4c. Empty -e "" and --engine="" are rejected, and -ve --help / -he --version print help/version cleanly
	for _, badArgs := range [][]string{
		{"--dry-run", "-e", ""},
		{"--dry-run", "--engine="},
	} {
		rootCmd = newRootCmd()
		rootCmd.SetArgs(badArgs)
		if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "engine name cannot be empty") {
			t.Errorf("expected empty engine error for %v, got %v", badArgs, err)
		}
	}

	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"-ve", "--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom -ve --help failed: %v", err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("expected help output for doom -ve --help, got:\n%s", out.String())
	}

	out.Reset()
	rootCmd = newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"-he", "--version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom -he --version failed: %v", err)
	}
	if !strings.Contains(out.String(), "doom version") {
		t.Errorf("expected version output for doom -he --version, got:\n%s", out.String())
	}

	// 4d. Overridden engine DisplayName is reflected in `doom wads show doom`
	engCmd := newEnginesCmd()
	engCmd.SetArgs([]string{"set", "dsda-doom", "--display-name", "DSDA"})
	if err := engCmd.Execute(); err != nil {
		t.Fatalf("engines set --display-name DSDA failed: %v", err)
	}
	out.Reset()
	wadsShowCmd := newWadsCmd()
	wadsShowCmd.SetOut(&out)
	wadsShowCmd.SetArgs([]string{"show", "doom"})
	if err := wadsShowCmd.Execute(); err != nil {
		t.Fatalf("wads show doom failed: %v", err)
	}
	if !strings.Contains(out.String(), "DSDA (MBF21 and speedrunning)") {
		t.Errorf("expected wads show doom to display overridden engine name 'DSDA (MBF21 and speedrunning)', got:\n%s",
			out.String())
	}
	engCmd = newEnginesCmd()
	engCmd.SetArgs([]string{"reset", "dsda-doom"})
	if err := engCmd.Execute(); err != nil {
		t.Fatalf("engines reset dsda-doom failed: %v", err)
	}

	// 5. Preset resolution via findPreset (Roman numeral, implicit doom prefix, WAD stem, and token matching)
	for _, tc := range []struct {
		query    string
		wantIWAD string
	}{
		{query: "II", wantIWAD: "DOOM2.WAD"},
		{query: "ii", wantIWAD: "DOOM2.WAD"},
		{query: "2", wantIWAD: "DOOM2.WAD"},
		{query: "doom2", wantIWAD: "DOOM2.WAD"},
		{query: "doom-ii", wantIWAD: "DOOM2.WAD"},
		{query: "1", wantIWAD: "DOOM.WAD"},
		{query: "evilution", wantIWAD: "TNT.WAD"},
		{query: "tnt", wantIWAD: "TNT.WAD"},
	} {
		out.Reset()
		rootCmd = newRootCmd()
		rootCmd.SetOut(&out)
		rootCmd.SetArgs([]string{tc.query, "--dry-run"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("doom %s --dry-run failed: %v", tc.query, err)
		}
		if !strings.Contains(out.String(), tc.wantIWAD) {
			t.Errorf("doom %s --dry-run output = %q, want IWAD %q", tc.query, out.String(), tc.wantIWAD)
		}
	}
}

func TestSplitCommandArgsAndQuotedCLIFlags(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "empty string returns empty slice",
			input: "   ",
			want:  []string{},
		},
		{
			name:  "double quoted path with spaces",
			input: `-config "/opt/my doom/woof.cfg" -complevel 21`,
			want:  []string{"-config", "/opt/my doom/woof.cfg", "-complevel", "21"},
		},
		{
			name:  "single quoted path with spaces",
			input: `-file '/opt/wad collection/test map.wad'`,
			want:  []string{"-file", "/opt/wad collection/test map.wad"},
		},
		{
			name:  "backslash escaped space and quotes",
			input: `-config /opt/my\ doom/port.cfg +name \"Doomer\"`,
			want:  []string{"-config", "/opt/my doom/port.cfg", "+name", `"Doomer"`},
		},
		{
			name:  "windows paths inside and outside double quotes preserved",
			input: `-config "C:\Games\My Doom\woof.cfg" -iwad D:\WADs\DOOM2.WAD`,
			want:  []string{"-config", `C:\Games\My Doom\woof.cfg`, "-iwad", `D:\WADs\DOOM2.WAD`},
		},
		{
			name:    "unterminated single quote rejected",
			input:   `-config '/opt/unterminated`,
			wantErr: true,
		},
		{
			name:    "unterminated double quote rejected",
			input:   `-config "/opt/unterminated`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommandArgs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("splitCommandArgs(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitCommandArgs(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}

	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(tempDir, "doom-cli"))

	engCmd := newEnginesCmd()
	engCmd.SetArgs([]string{
		"add", "Crispy",
		"--id", "crispy",
		"--binary", "crispy-doom",
		"--style", "boom",
		"--prefix-args", `-config "/opt/my doom/crispy.cfg"`,
		"--args", `-file "/opt/extra mods/hud.wad"`,
	})
	if err := engCmd.Execute(); err != nil {
		t.Fatalf("engines add with quoted args failed: %v", err)
	}

	wadCmd := newWadsCmd()
	wadCmd.SetArgs([]string{"set", "doom", "--args", `-record "/opt/my demos/run 1.lmp"`})
	if err := wadCmd.Execute(); err != nil {
		t.Fatalf("wads set with quoted args failed: %v", err)
	}

	_, _, cat, err := loadAppState()
	if err != nil {
		t.Fatalf("loadAppState: %v", err)
	}
	crispy, ok := cat.FindEngine("crispy")
	if !ok {
		t.Fatal("expected crispy engine in catalog")
	}
	wantPrefix := []string{"-config", "/opt/my doom/crispy.cfg"}
	if !reflect.DeepEqual(crispy.PrefixArgs, wantPrefix) {
		t.Errorf("crispy.PrefixArgs = %#v, want %#v", crispy.PrefixArgs, wantPrefix)
	}
	wantDefault := []string{"-file", "/opt/extra mods/hud.wad"}
	if !reflect.DeepEqual(crispy.DefaultArgs, wantDefault) {
		t.Errorf("crispy.DefaultArgs = %#v, want %#v", crispy.DefaultArgs, wantDefault)
	}
	doomPreset := cat.Find("doom")
	wantWadArgs := []string{"-record", "/opt/my demos/run 1.lmp"}
	if doomPreset == nil || !reflect.DeepEqual(doomPreset.Args, wantWadArgs) {
		t.Errorf("doomPreset.Args = %#v, want %#v", doomPreset, wantWadArgs)
	}

	// Verify 'doom wads show' displays [Disabled] for default-off soundtrack and '--include-file' enables it
	var showBefore bytes.Buffer
	showCmd1 := newWadsCmd()
	showCmd1.SetOut(&showBefore)
	showCmd1.SetArgs([]string{"show", "tnt-evilution"})
	if err := showCmd1.Execute(); err != nil {
		t.Fatalf("wads show tnt-evilution failed: %v", err)
	}
	if !strings.Contains(showBefore.String(), "[Disabled]") {
		t.Errorf("expected [Disabled] in wads show tnt-evilution, got:\n%s", showBefore.String())
	}

	incCmd := newWadsCmd()
	incCmd.SetArgs([]string{"set", "tnt-evilution", "--include-file", "idkfa 2024.wad"})
	if err := incCmd.Execute(); err != nil {
		t.Fatalf("wads set --include-file failed: %v", err)
	}

	var showAfter bytes.Buffer
	showCmd2 := newWadsCmd()
	showCmd2.SetOut(&showAfter)
	showCmd2.SetArgs([]string{"show", "tnt-evilution"})
	if err := showCmd2.Execute(); err != nil {
		t.Fatalf("wads show tnt-evilution after include-file failed: %v", err)
	}
	if strings.Contains(showAfter.String(), "[Disabled]") || !strings.Contains(showAfter.String(), "[Optional]") {
		t.Errorf("expected [Optional] instead of [Disabled] after --include-file, got:\n%s", showAfter.String())
	}
}
