package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/templates"
	"github.com/lock14/doom-cli/internal/wad"
)

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
