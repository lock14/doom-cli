package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/templates"
	"github.com/lock14/doom-cli/internal/wad"
)

// newSetupCmd creates the turnkey automated setup command.
func newSetupCmd() *cobra.Command {
	var force bool
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete 1-command setup (engines, soundfont, configs, steam IWADs, and community megawads)",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}

			out := cmdOut(cmd)
			errOut := cmdErr(cmd)
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

			var failedSteps []string

			// 2. Engines
			fmt.Fprintln(out, ">>> Step 2/5: Installing source ports (UZDoom, DSDA-Doom)...")
			installer := engine.NewInstaller(paths.BinDir, out)
			installer.Force = force
			if err := installer.InstallCatalog(cat); err != nil {
				fmt.Fprintf(errOut, "Warning: engine installer encountered: %v\n", err)
				failedSteps = append(failedSteps, "engines")
			}
			fmt.Fprintln(out)

			// 3. SoundFont
			fmt.Fprintln(out, ">>> Step 3/5: Installing Roland SC-55 SoundFont...")
			if _, err := wad.InstallSoundFont(paths.SoundFontsDir, force, out); err != nil {
				fmt.Fprintf(errOut, "Warning: soundfont installer encountered: %v\n", err)
				failedSteps = append(failedSteps, "soundfont")
			}
			fmt.Fprintln(out)

			// 4. Steam IWADs
			fmt.Fprintln(out, ">>> Step 4/5: Auto-discovering official Steam and GOG game files...")
			if _, err := steam.DiscoverAndExtract(
				cat, nil, paths.WadsDir, force, out, paths.WadsDirs...,
			); err != nil {
				fmt.Fprintf(errOut, "Warning: steam discoverer encountered: %v\n", err)
				failedSteps = append(failedSteps, "steam extract")
			}
			fmt.Fprintln(out)

			// 5. Community megawads
			fmt.Fprintln(out, ">>> Step 5/5: Downloading community megawads...")
			dl := wad.NewDownloader(paths.WadRoots(), force, out)
			if err := dl.DownloadAll(cat); err != nil {
				fmt.Fprintf(errOut, "Warning: some megawads failed to download: %v\n", err)
				failedSteps = append(failedSteps, "megawad downloads")
			}
			fmt.Fprintln(out)

			if len(failedSteps) > 0 {
				summary := strings.Join(failedSteps, ", ")
				fmt.Fprintln(out, "============================================================")
				fmt.Fprintf(out, "  ! Doom setup finished with errors in: %s\n", summary)
				fmt.Fprintln(out, "============================================================")
				return fmt.Errorf("setup completed with errors in: %s", summary)
			}

			fmt.Fprintln(out, "============================================================")
			fmt.Fprintln(out, "  ✓ Doom setup complete!")
			fmt.Fprintln(out, "  Engines, configs, soundfonts, and megawads are ready.")
			fmt.Fprintln(out, "  Run 'doom play' or 'doom' to start playing!")
			fmt.Fprintln(out, "============================================================")
			return nil
		},
	}
	setupCmd.Flags().BoolVar(&force, "force", false, "Force re-download and overwrite existing files")
	return setupCmd
}
