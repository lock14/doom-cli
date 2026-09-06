package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/engine"
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
