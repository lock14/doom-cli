package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/preset"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/templates"
	"github.com/lock14/doom-cli/internal/wad"
)

func newLaunchCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "launch <preset_name> [engine_args...]",
		Short:              "Directly launch a preset by name or prefix",
		Args:               cobra.MinimumNArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := getCatalog()
			if err != nil {
				return err
			}

			targetName := args[0]
			extraArgs := extractEngineArgs("launch", os.Args)

			p := cat.Find(targetName)
			if p == nil {
				return fmt.Errorf("preset %q not found. Run 'doom presets list' to see available presets", targetName)
			}

			paths := getPaths()
			opts := engine.LaunchOptions{
				EngineOverride: flagEngineOverride,
				WadsDir:        paths.WadsDir,
				BinDir:         paths.BinDir,
				DryRun:         flagDryRun,
				ExtraArgs:      extraArgs,
				Out:            os.Stdout,
				Engines:        cat.Engines,
			}

			plan, err := engine.PrepareLaunch(*p, opts)
			if err != nil {
				return err
			}

			return engine.Execute(plan, os.Stdout, os.Stderr)
		},
	}
}

func newSetupCmd() *cobra.Command {
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete 1-command setup (engines, soundfont, configs, steam IWADs, and community megawads)",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			cat, err := getCatalog()
			if err != nil {
				return err
			}

			fmt.Println("============================================================")
			fmt.Println("  ⚡ Initiating Doom Setup")
			fmt.Println("============================================================")
			fmt.Println()

			// 1. Configs
			fmt.Println(">>> Step 1/5: Deploying platform configurations...")
			if err := templates.DeployConfigs(paths); err != nil {
				return fmt.Errorf("error deploying configs: %w", err)
			}
			fmt.Println()

			// 2. Engines
			fmt.Println(">>> Step 2/5: Installing source ports (UZDoom, DSDA-Doom)...")
			installer := engine.NewInstaller(paths.BinDir, os.Stdout)
			if err := installer.InstallAll(); err != nil {
				fmt.Printf("Warning: engine installer encountered: %v\n", err)
			}
			fmt.Println()

			// 3. SoundFont
			fmt.Println(">>> Step 3/5: Installing Roland SC-55 SoundFont...")
			if _, err := wad.InstallSoundFont(paths.SoundFontsDir, flagForce, os.Stdout); err != nil {
				fmt.Printf("Warning: soundfont installer encountered: %v\n", err)
			}
			fmt.Println()

			// 4. Steam IWADs
			fmt.Println(">>> Step 4/5: Auto-discovering official Steam/GOG game files...")
			if _, err := steam.DiscoverAndExtract(nil, paths.WadsDir, flagForce, os.Stdout); err != nil {
				fmt.Printf("Warning: steam discoverer encountered: %v\n", err)
			}
			fmt.Println()

			// 5. Community megawads
			fmt.Println(">>> Step 5/5: Downloading community megawads...")
			dl := wad.NewDownloader(paths.WadsDir, flagForce, os.Stdout)
			if err := dl.DownloadAll(cat); err != nil {
				fmt.Printf("Warning: some megawads failed to download: %v\n", err)
			}
			fmt.Println()

			fmt.Println("============================================================")
			fmt.Println("  ✓ Doom setup complete!")
			fmt.Println("  Engines, configs, soundfonts, and megawads are ready.")
			fmt.Println("  Run 'doom play' or 'doom' to start playing!")
			fmt.Println("============================================================")
			return nil
		},
	}
	setupCmd.Flags().BoolVar(&flagForce, "force", false, "Force re-download and overwrite existing files")
	return setupCmd
}

func newWadsCmd() *cobra.Command {
	wadsCmd := &cobra.Command{
		Use:   "wads",
		Short: "Manage WADs, megawads, and official expansions",
	}

	wadsFetchCmd := &cobra.Command{
		Use:   "fetch [preset_name|all]",
		Short: "Download community megawad files",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			cat, err := getCatalog()
			if err != nil {
				return err
			}

			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			dl := wad.NewDownloader(paths.WadsDir, flagForce, os.Stdout)
			if strings.EqualFold(target, "all") {
				return dl.DownloadAll(cat)
			}

			p := cat.Find(target)
			if p == nil {
				return fmt.Errorf("preset %q not found", target)
			}
			return dl.DownloadPreset(*p)
		},
	}
	wadsFetchCmd.Flags().BoolVar(&flagForce, "force", false, "Re-download and overwrite existing files")

	wadsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List downloadable community megawads",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := getCatalog()
			if err != nil {
				return err
			}
			fmt.Println("=== Available Downloadable Community Megawads ===")
			for _, p := range cat.Presets {
				if len(p.DownloadURLs) > 0 {
					files := wad.FilterExpectedFiles(p.Mappacks)
					fmt.Printf("  - %-30s [%s]\n", p.Name, strings.Join(files, ", "))
				}
			}
			return nil
		},
	}

	wadsExtractCmd := &cobra.Command{
		Use:   "extract-steam",
		Short: "Auto-discover and copy official IWADs and expansions from Steam/GOG",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			_, err := steam.DiscoverAndExtract(nil, paths.WadsDir, flagForce, os.Stdout)
			return err
		},
	}
	wadsExtractCmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing files in destination")

	wadsCmd.AddCommand(wadsFetchCmd, wadsListCmd, wadsExtractCmd)
	return wadsCmd
}

func newEnginesCmd() *cobra.Command {
	enginesCmd := &cobra.Command{
		Use:   "engines",
		Short: "Manage and configure Doom source port engines",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnginesList(cmd, args)
		},
	}

	enginesListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured engines (built-in and custom)",
		RunE:  runEnginesList,
	}

	var addBin, addStyle, addArgs, addDesc string
	enginesAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register or update a custom source port engine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("engine name cannot be empty")
			}

			binTarget := addBin
			if binTarget == "" {
				binTarget = name
			}

			style := strings.ToLower(strings.TrimSpace(addStyle))
			if style != "" && style != "boom" && style != "zdoom" {
				return fmt.Errorf("invalid args-style %q (must be 'boom' or 'zdoom')", style)
			}

			var defaultArgs []string
			if strings.TrimSpace(addArgs) != "" {
				defaultArgs = strings.Fields(addArgs)
			}

			cfgMeta := preset.EngineConfig{
				Name:        name,
				Binary:      binTarget,
				ArgsStyle:   style,
				DefaultArgs: defaultArgs,
				Description: addDesc,
			}

			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			cfg.AddEngine(cfgMeta)
			if err := config.SaveConfig(paths, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf(
				"✓ Registered engine %q (%s) in %s\n",
				name,
				cfgMeta.EffectiveArgsStyle(),
				paths.ConfigFile,
			)
			return nil
		},
	}
	enginesAddCmd.Flags().StringVar(&addBin, "bin", "", "Executable name or absolute path (defaults to name)")
	enginesAddCmd.Flags().StringVar(&addStyle, "args-style", "", "Argument format: boom (-file/-deh) or zdoom (-file)")
	enginesAddCmd.Flags().StringVar(&addArgs, "args", "", "Default flags always passed to this engine")
	enginesAddCmd.Flags().StringVar(&addDesc, "desc", "", "Description of the engine")

	enginesRemoveCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom engine from user configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if strings.EqualFold(name, "uzdoom") || strings.EqualFold(name, "dsda-doom") {
				return fmt.Errorf("cannot remove built-in engine %q", name)
			}

			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if !cfg.RemoveEngine(name) {
				return fmt.Errorf("engine %q not found in user configuration", name)
			}
			if err := config.SaveConfig(paths, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Printf("✓ Removed engine %q from %s\n", name, paths.ConfigFile)
			return nil
		},
	}

	enginesInstallCmd := &cobra.Command{
		Use:   "install [uzdoom|dsda-doom|all]",
		Short: "Download and deploy portable engine binaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			ins := engine.NewInstaller(paths.BinDir, os.Stdout)
			target := "all"
			if len(args) > 0 {
				target = strings.ToLower(args[0])
			}

			switch target {
			case "uzdoom":
				return ins.InstallUZDoom()
			case "dsda", "dsda-doom", "dsdadoom":
				return ins.InstallDSDA()
			case "all":
				return ins.InstallAll()
			default:
				return fmt.Errorf("unknown engine target: %s (choose uzdoom, dsda-doom, or all)", target)
			}
		},
	}

	enginesCmd.AddCommand(enginesListCmd, enginesAddCmd, enginesRemoveCmd, enginesInstallCmd)
	return enginesCmd
}

func checkEngineBinary(binary, binDir string) (string, bool) {
	if binary == "" {
		return "", false
	}
	if filepath.IsAbs(binary) || strings.Contains(binary, "/") || strings.Contains(binary, "\\") {
		if fi, err := os.Stat(binary); err == nil && !fi.IsDir() {
			return binary, true
		}
		return binary, false
	}
	cand := filepath.Join(binDir, binary)
	if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
		return cand, true
	}
	if p, err := exec.LookPath(binary); err == nil {
		return p, true
	}
	return binary, false
}

func runEnginesList(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}
	paths := getPaths()

	fmt.Println("=== Available Doom Engines ===")
	keys := make([]string, 0, len(cat.Engines))
	for k := range cat.Engines {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		eng := cat.Engines[k]
		binTarget := eng.Binary
		if binTarget == "" {
			binTarget = eng.Name
		}
		resolvedPath, found := checkEngineBinary(binTarget, paths.BinDir)
		status := "[✗ Missing]"
		if found {
			status = fmt.Sprintf("[✓ Found: %s]", resolvedPath)
		}
		style := eng.EffectiveArgsStyle()
		desc := eng.Description
		if desc == "" {
			desc = eng.Name
		}
		fmt.Printf("  • %-12s [%-5s] %-32s %s\n", eng.Name, style, desc, status)
	}
	return nil
}

func newSoundfontCmd() *cobra.Command {
	sfCmd := &cobra.Command{
		Use:   "soundfont",
		Short: "Manage MIDI SoundFonts for FluidSynth",
	}
	sfInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Download and deploy Roland SC-55 compatible GeneralUser-GS SoundFont",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			_, err := wad.InstallSoundFont(paths.SoundFontsDir, flagForce, os.Stdout)
			return err
		},
	}
	sfInstallCmd.Flags().BoolVar(&flagForce, "force", false, "Re-download and overwrite existing SoundFont")
	sfCmd.AddCommand(sfInstallCmd)
	return sfCmd
}

func newPresetsCmd() *cobra.Command {
	presetsCmd := &cobra.Command{
		Use:   "presets",
		Short: "Inspect, configure, and manage preset catalog and custom WADs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPresetsList(cmd, args)
		},
	}

	presetsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all presets (built-in and custom) with engine assignments",
		RunE:  runPresetsList,
	}

	presetsShowCmd := &cobra.Command{
		Use:   "show <preset_name>",
		Short: "Display detailed configuration and file status for a preset",
		Args:  cobra.ExactArgs(1),
		RunE:  runPresetsShow,
	}

	var addEngine, addIWAD, addCategory, addDesc, addAuthor, addRelease, addArgs string
	var addFiles []string
	presetsAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a custom WAD or preset to your personal library",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("preset name cannot be empty")
			}

			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			cat, err := getCatalog()
			if err != nil {
				return err
			}

			if !isCustomPreset(name, cfg) && cat.Find(name) != nil {
				return fmt.Errorf(
					"preset %q is a built-in preset; use 'doom presets config %q' to adjust launch options",
					name, name,
				)
			}

			p := preset.Preset{
				Name:           name,
				Engine:         addEngine,
				IWAD:           addIWAD,
				Mappacks:       addFiles,
				Category:       addCategory,
				Description:    addDesc,
				Author:         addAuthor,
				ReleaseDate:    addRelease,
				AdditionalArgs: addArgs,
				Custom:         true,
			}

			cfg.AddPreset(p)
			if err := config.SaveConfig(paths, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("✓ Successfully added custom preset %q to %s\n", name, paths.ConfigFile)
			return nil
		},
	}
	presetsAddCmd.Flags().StringVar(&addEngine, "engine", "uzdoom", "Default engine for this preset")
	presetsAddCmd.Flags().StringVar(&addIWAD, "iwad", "DOOM2.WAD", "Base IWAD required by this preset")
	presetsAddCmd.Flags().StringSliceVar(&addFiles, "files", nil, "Comma-separated list of PWAD and DEH files")
	presetsAddCmd.Flags().StringVar(&addCategory, "category", "Custom", "Category or collection name")
	presetsAddCmd.Flags().StringVar(&addDesc, "desc", "", "Short description of the mapset")
	presetsAddCmd.Flags().StringVar(&addAuthor, "author", "", "Map author / team")
	presetsAddCmd.Flags().StringVar(&addRelease, "release", "", "Release year / date")
	presetsAddCmd.Flags().StringVar(&addArgs, "args", "", "Default extra launch arguments")

	var cfgEngine, cfgIWAD, cfgArgs string
	var cfgFiles []string
	var cfgReset bool
	presetsConfigCmd := &cobra.Command{
		Use:   "config <preset_name>",
		Short: "Configure per-WAD launch options (engine, extra args, files)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cfgReset {
				if cfg.RemoveLaunchOptions(name) {
					if err := config.SaveConfig(paths, cfg); err != nil {
						return fmt.Errorf("saving config: %w", err)
					}
					fmt.Printf("✓ Reset custom launch options for %q in %s\n", name, paths.ConfigFile)
					return nil
				}
				fmt.Printf("ℹ No custom launch options configured for %q\n", name)
				return nil
			}

			var opts preset.WadLaunchOptions
			if existing, ok := cfg.LaunchOptions[name]; ok {
				opts = existing
			}

			changed := false
			if cfgEngine != "" {
				opts.Engine = cfgEngine
				changed = true
			}
			if cfgIWAD != "" {
				opts.IWAD = cfgIWAD
				changed = true
			}
			if cfgArgs != "" {
				opts.AdditionalArgs = cfgArgs
				changed = true
			}
			if len(cfgFiles) > 0 {
				opts.ExtraFiles = cfgFiles
				changed = true
			}

			if !changed {
				return fmt.Errorf("no option flags provided (use --engine, --args, --iwad, --files, or --reset)")
			}

			cfg.SetLaunchOptions(name, opts)
			if err := config.SaveConfig(paths, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("✓ Configured launch options for %q in %s\n", name, paths.ConfigFile)
			return nil
		},
	}
	presetsConfigCmd.Flags().StringVar(&cfgEngine, "engine", "", "Preferred engine override for this preset")
	presetsConfigCmd.Flags().StringVar(&cfgIWAD, "iwad", "", "Custom IWAD override")
	presetsConfigCmd.Flags().StringVar(&cfgArgs, "args", "", "Additional launch arguments (e.g. -skill 4 -warp 01)")
	presetsConfigCmd.Flags().StringSliceVar(&cfgFiles, "files", nil, "Extra PWAD files to load with this preset")
	presetsConfigCmd.Flags().BoolVar(&cfgReset, "reset", false, "Clear all custom launch options for this preset")

	presetsRemoveCmd := &cobra.Command{
		Use:   "remove <preset_name>",
		Short: "Remove a custom preset or reset overrides for a built-in preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cfg.RemovePreset(name) {
				cfg.RemoveLaunchOptions(name)
				if err := config.SaveConfig(paths, cfg); err != nil {
					return fmt.Errorf("saving config: %w", err)
				}
				fmt.Printf("✓ Removed custom preset %q from %s\n", name, paths.ConfigFile)
				return nil
			}

			if cfg.RemoveLaunchOptions(name) {
				if err := config.SaveConfig(paths, cfg); err != nil {
					return fmt.Errorf("saving config: %w", err)
				}
				fmt.Printf(
					"✓ Cleared launch option overrides for built-in preset %q in %s\n",
					name,
					paths.ConfigFile,
				)
				return nil
			}

			return fmt.Errorf("preset %q not found in custom presets or overrides", name)
		},
	}

	presetsBuildCmd := &cobra.Command{
		Use:   "build",
		Short: "Synchronize data/presets.json into README.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			if err := preset.SyncReadme(root); err != nil {
				return err
			}
			fmt.Println("✓ Successfully synchronized README.md with data/presets.json")
			return nil
		},
	}

	presetsCmd.AddCommand(
		presetsListCmd, presetsShowCmd, presetsAddCmd,
		presetsConfigCmd, presetsRemoveCmd, presetsBuildCmd,
	)
	return presetsCmd
}

func isCustomPreset(name string, cfg *config.AppConfig) bool {
	if cfg == nil {
		return false
	}
	for _, p := range cfg.Presets {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

func runPresetsList(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}
	paths := getPaths()
	cfg, _ := config.LoadConfig(paths)

	fmt.Println("=== Available Doom Presets ===")
	for i, p := range cat.Presets {
		tag := formatEngineBadge(p.Engine)
		statusFlag := ""
		if p.Custom {
			statusFlag = " [Custom]"
		} else if cfg != nil && cfg.LaunchOptions != nil {
			if _, ok := cfg.LaunchOptions[p.Name]; ok {
				statusFlag = " [Overridden]"
			}
		}
		fmt.Printf("%2d. %-30s %s%s - %s\n", i+1, p.Name, tag, statusFlag, p.Description)
	}
	return nil
}

func runPresetsShow(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}
	name := args[0]
	p := cat.Find(name)
	if p == nil {
		return fmt.Errorf("preset %q not found", name)
	}

	paths := getPaths()
	cfg, _ := config.LoadConfig(paths)

	fmt.Printf("=== Preset: %s ===\n", p.Name)
	if p.Custom {
		fmt.Printf("  Type:          Custom WAD\n")
	}
	if p.Author != "" {
		fmt.Printf("  Author:        %s\n", p.Author)
	}
	if p.ReleaseDate != "" {
		fmt.Printf("  Released:      %s\n", p.ReleaseDate)
	}
	if p.Category != "" {
		fmt.Printf("  Category:      %s\n", p.Category)
	}
	if p.Compatibility != "" {
		fmt.Printf("  Compatibility: %s\n", p.Compatibility)
	}
	if p.Description != "" {
		fmt.Printf("  Description:   %s\n", p.Description)
	}

	engStr := p.Engine
	if cat.Engines != nil {
		if engCfg, ok := cat.Engines[p.Engine]; ok && engCfg.Description != "" {
			engStr = engCfg.Description
		}
	}
	engineOverridden := ""
	if cfg != nil && cfg.LaunchOptions != nil {
		if opt, ok := cfg.LaunchOptions[p.Name]; ok && opt.Engine != "" {
			engineOverridden = " [Overridden in user config]"
		}
	}
	fmt.Printf("  Engine:        %s%s\n", engStr, engineOverridden)

	iwadPath, iwadFound := preset.ResolveFile(paths.WadsDir, p.IWAD)
	iwadStatus := "[✗ Missing]"
	if iwadFound {
		iwadStatus = fmt.Sprintf("[✓ Found: %s]", iwadPath)
	}
	fmt.Printf("  Base IWAD:     %s %s\n", p.IWAD, iwadStatus)

	if len(p.Mappacks) > 0 {
		fmt.Println("  Files:")
		for _, m := range p.Mappacks {
			fpath, fFound := preset.ResolveFile(paths.WadsDir, m)
			fStatus := "[✗ Missing]"
			if fFound {
				fStatus = fmt.Sprintf("[✓ Found: %s]", fpath)
			} else if strings.EqualFold(m, "idkfa 2024.wad") {
				fStatus = "[Optional]"
			}
			fmt.Printf("    - %-20s %s\n", m, fStatus)
		}
	}

	if p.AdditionalArgs != "" {
		fmt.Printf("  Launch Args:   %s\n", p.AdditionalArgs)
	}

	return nil
}

func formatEngineBadge(engine string) string {
	switch strings.ToLower(engine) {
	case "dsda-doom", "dsda":
		return "[DSDA    ]"
	case "uzdoom", "zdoom":
		return "[UZDoom  ]"
	default:
		tag := strings.ToUpper(engine)
		runes := []rune(tag)
		if len(runes) > 6 {
			tag = string(runes[:6])
		}
		return fmt.Sprintf("[%-8s]", tag)
	}
}
