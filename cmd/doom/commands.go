package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

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
		Short: "Download and manage source port engines (UZDoom, DSDA-Doom)",
	}
	enginesInstallCmd := &cobra.Command{
		Use:   "install [uzdoom|dsda-doom|all]",
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
			case "all":
				return ins.InstallAll()
			default:
				return fmt.Errorf("unknown engine target: %s (choose uzdoom, dsda-doom, or all)", target)
			}
		},
	}
	enginesCmd.AddCommand(enginesInstallCmd)
	return enginesCmd
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
	presetsCmd.AddCommand(presetsListCmd, presetsBuildCmd)
	return presetsCmd
}
