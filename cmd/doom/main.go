// Package main provides the entry point for the unified doom CLI.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/preset"
)

var (
	flagEngineOverride string
	flagWadsDir        string
	flagBinDir         string
	flagDryRun         bool
	flagForce          bool
	flagOnce           bool
	flagPresetsFile    string
	flagTheme          string
	flagNerdFonts      bool
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

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "doom",
		Short: "Unified cross-platform Doom source port manager & launcher",
		Long: `doom is a modern, unified CLI and interactive terminal launcher for classic Doom.
It manages source ports (UZDoom, DSDA-Doom), official Steam/GOG IWADs,
Roland SC-55 SoundFonts, curated community megawads, and platform-native configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommands or arguments are passed, run interactive launcher (doom play)
			return runPlay(cmd, args)
		},
	}

	rootCmd.PersistentFlags().StringVarP(
		&flagEngineOverride, "engine", "e", "", "Override source port engine (uzdoom or dsda-doom)",
	)
	rootCmd.PersistentFlags().StringVar(&flagWadsDir, "wads-dir", "", "Custom path to WADs directory")
	rootCmd.PersistentFlags().StringVar(&flagBinDir, "bin-dir", "", "Custom path to engines binary directory")
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Print launch command without executing")
	rootCmd.PersistentFlags().StringVar(&flagPresetsFile, "presets-file", "", "Custom presets.json file path")
	rootCmd.PersistentFlags().StringVar(
		&flagTheme, "theme", "",
		"Color theme for launcher (classic, blood, toxic, inferno, frost, plasma, heretic, amber, sigil, monochrome)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&flagNerdFonts, "nerd-fonts", false, "Use Powerlevel10k rounded capsule ends (requires a Nerd Font)",
	)
	rootCmd.Flags().BoolVar(&flagOnce, "once", false, "Exit after playing a single preset")

	rootCmd.AddCommand(
		newPlayCmd(),
		newLaunchCmd(),
		newSetupCmd(),
		newWadsCmd(),
		newEnginesCmd(),
		newSoundfontCmd(),
		newConfigCmd(),
		newPresetsCmd(),
		newThemesCmd(),
	)

	return rootCmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
