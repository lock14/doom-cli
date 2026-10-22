package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/templates"
	"github.com/lock14/doom-cli/internal/wad"
)

// newSetupCmd creates the turnkey automated setup command.
func newSetupCmd() *cobra.Command {
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete 1-command setup (engines, soundfont, configs, steam IWADs, and community megawads)",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}

			out := cmdOut(cmd)
			fmt.Fprintln(out, "============================================================")
			fmt.Fprintln(out, "  Initiating Doom Setup")
			fmt.Fprintln(out, "============================================================")
			fmt.Fprintln(out)

			// 1. Configs
			fmt.Fprintln(out, ">>> Step 1/5: Deploying platform configurations...")
			if err := templates.DeployConfigsTo(paths, out); err != nil {
				return fmt.Errorf("error deploying configs: %w", err)
			}
			fmt.Fprintln(out)

			// 2. Engines
			fmt.Fprintln(out, ">>> Step 2/5: Installing source ports (UZDoom, DSDA-Doom)...")
			installer := engine.NewInstaller(paths.BinDir, out)
			installer.Force = flagForce
			if err := installer.InstallCatalog(cat); err != nil {
				fmt.Fprintf(out, "Warning: engine installer encountered: %v\n", err)
			}
			fmt.Fprintln(out)

			// 3. SoundFont
			fmt.Fprintln(out, ">>> Step 3/5: Installing Roland SC-55 SoundFont...")
			if _, err := wad.InstallSoundFont(paths.SoundFontsDir, flagForce, out); err != nil {
				fmt.Fprintf(out, "Warning: soundfont installer encountered: %v\n", err)
			}
			fmt.Fprintln(out)

			// 4. Steam IWADs
			fmt.Fprintln(out, ">>> Step 4/5: Auto-discovering official Steam and GOG game files...")
			if _, err := steam.DiscoverAndExtract(cat, nil, paths.WadsDir, flagForce, out); err != nil {
				fmt.Fprintf(out, "Warning: steam discoverer encountered: %v\n", err)
			}
			fmt.Fprintln(out)

			// 5. Community megawads
			fmt.Fprintln(out, ">>> Step 5/5: Downloading community megawads...")
			dl := wad.NewDownloader(paths.WadRoots(), flagForce, out)
			if err := dl.DownloadAll(cat); err != nil {
				fmt.Fprintf(out, "Warning: some megawads failed to download: %v\n", err)
			}
			fmt.Fprintln(out)

			fmt.Fprintln(out, "============================================================")
			fmt.Fprintln(out, "  ✓ Doom setup complete!")
			fmt.Fprintln(out, "  Engines, configs, soundfonts, and megawads are ready.")
			fmt.Fprintln(out, "  Run 'doom play' or 'doom' to start playing!")
			fmt.Fprintln(out, "============================================================")
			return nil
		},
	}
	setupCmd.Flags().BoolVar(&flagForce, "force", false, "Force re-download and overwrite existing files")
	return setupCmd
}
