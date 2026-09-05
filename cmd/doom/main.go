// Package main provides the entry point for the unified doom CLI.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-configs/internal/config"
	"github.com/lock14/doom-configs/internal/engine"
	"github.com/lock14/doom-configs/internal/preset"
	"github.com/lock14/doom-configs/internal/steam"
	"github.com/lock14/doom-configs/internal/templates"
	"github.com/lock14/doom-configs/internal/tui"
	"github.com/lock14/doom-configs/internal/wad"
)

var (
	flagEngineOverride string
	flagWadsDir        string
	flagBinDir         string
	flagDryRun         bool
	flagForce          bool
	flagPresetsFile    string
)

func getCatalog() (*preset.Catalog, error) {
	return preset.LoadCatalog(flagPresetsFile)
}

func getPaths() *config.Paths {
	paths := config.GetPaths()
	if flagWadsDir != "" {
		paths.WadsDir = flagWadsDir
	}
	if flagBinDir != "" {
		paths.BinDir = flagBinDir
	}
	return paths
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "doom",
		Short: "Unified cross-platform Doom source port manager & launcher",
		Long: `doom is a modern, unified CLI and interactive terminal launcher for classic Doom.
It manages source ports (UZDoom, DSDA-Doom, DoomRunner), official Steam/GOG IWADs,
Roland SC-55 SoundFonts, curated community megawads, and platform-native configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommands or arguments are passed, run interactive launcher (doom play)
			return runPlay(cmd, args)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&flagEngineOverride, "engine", "e", "", "Override source port engine (uzdoom or dsda-doom)")
	rootCmd.PersistentFlags().StringVar(&flagWadsDir, "wads-dir", "", "Custom path to WADs directory")
	rootCmd.PersistentFlags().StringVar(&flagBinDir, "bin-dir", "", "Custom path to engines binary directory")
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Print launch command without executing")
	rootCmd.PersistentFlags().StringVar(&flagPresetsFile, "presets-file", "", "Custom presets.json file path")

	// Subcommand: play
	playCmd := &cobra.Command{
		Use:                "play [flags] [-- extra_engine_args...]",
		Short:              "Interactive terminal launcher with fuzzy search and live preview",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE:               runPlay,
	}
	rootCmd.AddCommand(playCmd)

	// Subcommand: launch
	launchCmd := &cobra.Command{
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
			}

			plan, err := engine.PrepareLaunch(*p, opts)
			if err != nil {
				return err
			}

			return engine.Execute(plan, os.Stdout, os.Stderr)
		},
	}
	rootCmd.AddCommand(launchCmd)

	// Subcommand: setup
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
			fmt.Println(">>> Step 2/5: Installing source ports (UZDoom, DSDA-Doom, DoomRunner)...")
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
	rootCmd.AddCommand(setupCmd)

	// Subcommand: wads
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
	rootCmd.AddCommand(wadsCmd)

	// Subcommand: engines
	enginesCmd := &cobra.Command{
		Use:   "engines",
		Short: "Download and manage source port engines (UZDoom, DSDA-Doom, DoomRunner)",
	}
	enginesInstallCmd := &cobra.Command{
		Use:   "install [uzdoom|dsda-doom|doomrunner|all]",
		Short: "Download and deploy engine binaries",
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
			case "doomrunner":
				return ins.InstallDoomRunner()
			case "all":
				return ins.InstallAll()
			default:
				return fmt.Errorf("unknown engine target: %s (choose uzdoom, dsda-doom, doomrunner, or all)", target)
			}
		},
	}
	enginesCmd.AddCommand(enginesInstallCmd)
	rootCmd.AddCommand(enginesCmd)

	// Subcommand: soundfont
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
	rootCmd.AddCommand(sfCmd)

	// Subcommand: config
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage engine configurations and DoomRunner options",
	}
	configInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Deploy autoexec.cfg, dsda-doom.cfg, and options.json with backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.DeployConfigs(paths)
		},
	}
	configDiffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare installed system configs against repository templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.DiffConfigs(paths, os.Stdout)
		},
	}
	configSyncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync modified in-game settings back into repository templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.SyncConfigs(paths, ".", os.Stdout)
		},
	}
	configCmd.AddCommand(configInstallCmd, configDiffCmd, configSyncCmd)
	rootCmd.AddCommand(configCmd)

	// Subcommand: presets
	presetsCmd := &cobra.Command{
		Use:   "presets",
		Short: "Inspect and build preset catalog",
	}
	presetsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all 32 curated presets with engine assignments and descriptions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := getCatalog()
			if err != nil {
				return err
			}
			fmt.Println("=== Available Doom Presets ===")
			for i, p := range cat.Presets {
				eng := "UZDoom"
				if p.Engine == "dsda-doom" {
					eng = "DSDA-Doom"
				}
				fmt.Printf("%2d. %-30s [%-9s] - %s\n", i+1, p.Name, eng, p.Description)
			}
			return nil
		},
	}
	presetsBuildCmd := &cobra.Command{
		Use:   "build",
		Short: "Compile data/presets.json into DoomRunner options.json and update README.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			if err := preset.CompileOptionsFiles(root); err != nil {
				return err
			}
			fmt.Println("✓ Successfully compiled DoomRunner options.json and updated README.md")
			return nil
		},
	}
	presetsCmd.AddCommand(presetsListCmd, presetsBuildCmd)
	rootCmd.AddCommand(presetsCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runPlay(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}

	paths := getPaths()
	selected, err := tui.RunInteractiveLauncher(cat, paths.WadsDir)
	if err != nil {
		return err
	}
	if selected == nil {
		return nil
	}

	opts := engine.LaunchOptions{
		EngineOverride: flagEngineOverride,
		WadsDir:        paths.WadsDir,
		BinDir:         paths.BinDir,
		DryRun:         flagDryRun,
		ExtraArgs:      extractEngineArgs("play", os.Args),
		Out:            os.Stdout,
	}

	plan, err := engine.PrepareLaunch(*selected, opts)
	if err != nil {
		return err
	}

	return engine.Execute(plan, os.Stdout, os.Stderr)
}

func extractEngineArgs(subcommand string, rawArgs []string) []string {
	dashDashIdx := -1
	for i, a := range rawArgs {
		if a == "--" {
			dashDashIdx = i
			break
		}
	}
	if dashDashIdx != -1 && dashDashIdx+1 < len(rawArgs) {
		return rawArgs[dashDashIdx+1:]
	}

	knownFlagsWithValue := map[string]bool{
		"--engine": true, "-e": true,
		"--wads-dir":     true,
		"--bin-dir":      true,
		"--presets-file": true,
	}
	knownBoolFlags := map[string]bool{
		"--dry-run": true,
		"--force":   true,
		"-h":        true, "--help": true,
	}

	var engineArgs []string
	pastSubcommand := false
	presetSkipped := (subcommand != "launch") // Only skip preset argument on "launch"

	for i := 1; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if !pastSubcommand {
			if arg == subcommand {
				pastSubcommand = true
			}
			continue
		}

		if knownBoolFlags[arg] {
			continue
		}
		if knownFlagsWithValue[arg] {
			i++ // skip flag value
			continue
		}
		if strings.HasPrefix(arg, "--wads-dir=") ||
			strings.HasPrefix(arg, "--bin-dir=") ||
			strings.HasPrefix(arg, "--engine=") ||
			strings.HasPrefix(arg, "--presets-file=") {
			continue
		}

		if !presetSkipped && !strings.HasPrefix(arg, "-") {
			presetSkipped = true
			continue
		}

		engineArgs = append(engineArgs, arg)
	}

	return engineArgs
}
