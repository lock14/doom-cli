package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/engine"
)

// newLaunchCmd creates the direct preset launch command.
func newLaunchCmd() *cobra.Command {
	var (
		engineOverride string
		dryRun         bool
	)
	launchCmd := &cobra.Command{
		Use:                "launch <preset_name> [engine_args...]",
		Short:              "Directly launch a preset by ID, name, or prefix",
		Args:               cobra.MinimumNArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}

			targetName := args[0]
			extraArgs := extractEngineArgs("launch", os.Args)

			p := cat.Find(targetName)
			if p == nil {
				return fmt.Errorf("preset %q not found. Run 'doom presets list' to see available presets", targetName)
			}
			out := cmdOut(cmd)
			errOut := cmdErr(cmd)
			opts := engine.LaunchOptions{
				EngineOverride: engineOverride,
				WadRoots:       paths.WadRoots(),
				BinDir:         paths.BinDir,
				DryRun:         dryRun,
				ExtraArgs:      extraArgs,
				Out:            out,
				Catalog:        cat,
			}

			plan, err := engine.PrepareLaunch(*p, opts)
			if err != nil {
				return err
			}

			return engine.Execute(plan, out, errOut)
		},
	}
	launchCmd.Flags().StringVarP(
		&engineOverride, "engine", "e", "", "Override source port engine (uzdoom or dsda-doom)",
	)
	launchCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print launch command without executing")
	return launchCmd
}
