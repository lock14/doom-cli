package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/portcfg"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/wad"
)

// newSetupCmd creates the turnkey automated setup command.
func newSetupCmd() *cobra.Command {
	var (
		force     bool
		gamesFlag []string
		wadsFlag  []string
	)
	setupCmd := &cobra.Command{
		Use:          "setup",
		Short:        "Turnkey core setup (configs, engines, soundfont, and IWADs; optional --wads and --games)",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			errOut := cmdErr(cmd)
			paths, _, cat, err := loadAppStateWithErr(errOut)
			if err != nil {
				return err
			}

			dirIdx := catalog.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
			scope, err := resolveGameScope(cat, dirIdx, gamesFlag, cmd.Flags().Changed("games"))
			if err != nil {
				return err
			}
			wadSel, err := resolveWadSelection(cat, scope.allowed, wadsFlag, cmd.Flags().Changed("wads"))
			if err != nil {
				return err
			}

			totalSteps := 4
			if wadSel.mode != wadsNone {
				totalSteps = 5
			}

			out := cmdOut(cmd)
			ctx := cmd.Context()
			fmt.Fprintln(out, "============================================================")
			fmt.Fprintln(out, "  Initiating Doom Setup")
			fmt.Fprintln(out, "============================================================")
			fmt.Fprintln(out)

			// 1. Configs
			fmt.Fprintf(out, ">>> Step 1/%d: Deploying platform configurations...\n", totalSteps)
			if err := portcfg.DeployConfigsWithOptions(paths, force, out); err != nil {
				return fmt.Errorf("error deploying configs: %w", err)
			}
			fmt.Fprintln(out)
			if err := ctx.Err(); err != nil {
				return err
			}

			var failedSteps []string

			// 2. Engines
			fmt.Fprintf(out, ">>> Step 2/%d: Installing source ports (UZDoom, DSDA-Doom)...\n", totalSteps)
			installer := engine.NewInstaller(paths.BinDir, out)
			installer.Ctx = ctx
			installer.Force = force
			if err := installer.InstallCatalog(cat); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				fmt.Fprintf(errOut, "Warning: engine installer encountered: %v\n", err)
				failedSteps = append(failedSteps, "engines")
			}
			fmt.Fprintln(out)
			if err := ctx.Err(); err != nil {
				return err
			}

			// 3. SoundFont
			fmt.Fprintf(out, ">>> Step 3/%d: Installing Roland SC-55 SoundFont...\n", totalSteps)
			if _, err := wad.InstallSoundFontContext(ctx, paths.SoundFontsDir, force, out); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				fmt.Fprintf(errOut, "Warning: soundfont installer encountered: %v\n", err)
				failedSteps = append(failedSteps, "soundfont")
			}
			fmt.Fprintln(out)
			if err := ctx.Err(); err != nil {
				return err
			}

			// 4. Steam IWADs
			scopedCat := cat.FilterByGames(scope.allowed)
			if len(scope.includedOptional) > 0 {
				fmt.Fprintf(
					out,
					">>> Step 4/%d: Auto-discovering official Steam and GOG game files (games: doom, %s)...\n",
					totalSteps,
					strings.Join(scope.includedOptional, ", "),
				)
			} else {
				fmt.Fprintf(out, ">>> Step 4/%d: Auto-discovering official Steam and GOG game files...\n", totalSteps)
			}
			if _, err := steam.DiscoverAndExtractContext(
				ctx, scopedCat, nil, paths.WadsDir, force, out, paths.WadsDirs...,
			); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				fmt.Fprintf(errOut, "Warning: steam discoverer encountered: %v\n", err)
				failedSteps = append(failedSteps, "steam extract")
			}
			fmt.Fprintln(out)
			if err := ctx.Err(); err != nil {
				return err
			}

			// 5. Community megawads (optional)
			if wadSel.mode != wadsNone {
				fmt.Fprintf(out, ">>> Step 5/%d: Downloading community megawads...\n", totalSteps)
				dl := wad.NewDownloader(paths.WadRoots(), force, out)
				dl.Ctx = ctx
				var dlErr error
				if wadSel.mode == wadsAll {
					dlErr = dl.DownloadAll(scopedCat)
				} else {
					dlErr = dl.DownloadSelected(wadSel.selected, cat)
				}
				if dlErr != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					fmt.Fprintf(errOut, "Warning: some megawads failed to download: %v\n", dlErr)
					failedSteps = append(failedSteps, "megawad downloads")
				}
				fmt.Fprintln(out)
			}

			if len(failedSteps) > 0 {
				summary := strings.Join(failedSteps, ", ")
				fmt.Fprintln(out, "============================================================")
				fmt.Fprintf(out, "  ! Doom setup finished with errors in: %s\n", summary)
				fmt.Fprintln(out, "============================================================")
				return fmt.Errorf("setup completed with errors in: %s", summary)
			}

			fmt.Fprintln(out, "============================================================")
			fmt.Fprintln(out, "  ✓ Doom setup complete!")
			if wadSel.mode == wadsNone {
				fmt.Fprintln(out, "  Engines, configs, soundfonts, and official IWADs are ready.")
				fmt.Fprintln(
					out,
					"  Browse and fetch community megawads in 'doom wads' (F3 + Ctrl+F), "+
						"or run 'doom wads fetch <name|all>'.",
				)
			} else {
				fmt.Fprintln(out, "  Engines, configs, soundfonts, and megawads are ready.")
			}
			if len(scope.skippedOptional) > 0 {
				fmt.Fprintf(
					out,
					"  Optional non-Doom games skipped: %s (pass --games <list|all> to include).\n",
					strings.Join(scope.skippedOptional, ", "),
				)
			}
			fmt.Fprintln(out, "  Run 'doom' or 'doom wads' to start playing!")
			fmt.Fprintln(out, "============================================================")
			return nil
		},
	}
	setupCmd.Flags().BoolVar(&force, "force", false, "Force re-download and overwrite existing files")
	setupCmd.Flags().StringSliceVar(
		&gamesFlag,
		"games",
		nil,
		"Include optional non-Doom games (heretic, hexen, strife, all, or doom)",
	)
	setupCmd.Flags().StringSliceVar(
		&wadsFlag,
		"wads",
		nil,
		"Community megawads to download during setup (none, all, or comma-separated preset names; default none)",
	)
	return setupCmd
}
