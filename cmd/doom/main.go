// Package main provides the entry point for the unified doom CLI.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/preset"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	flagEngineOverride string
	flagWadsDir        string
	flagWadsDirs       string
	flagBinDir         string
	flagSoundFontsDir  string
	flagDryRun         bool
	flagForce          bool
	flagOnce           bool
	flagPresetsFile    string
	flagTheme          string
	flagNerdFonts      bool
)

func cmdOut(cmd *cobra.Command) io.Writer {
	if cmd == nil {
		return os.Stdout
	}
	return cmd.OutOrStdout()
}

func cmdErr(cmd *cobra.Command) io.Writer {
	if cmd == nil {
		return os.Stderr
	}
	return cmd.ErrOrStderr()
}

func applyFlagOverrides(paths *config.Paths) {
	if flagWadsDir != "" {
		paths.WadsDir = flagWadsDir
	}
	if flagWadsDirs != "" {
		paths.WadsDirs = config.ParsePathList(flagWadsDirs)
	}
	if flagBinDir != "" {
		paths.SetBinDir(flagBinDir)
	}
	if flagSoundFontsDir != "" {
		paths.SetSoundFontsDir(flagSoundFontsDir)
	}
}

func loadPathsAndConfig() (*config.Paths, *config.AppConfig, error) {
	paths := config.GetPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	if cfg != nil {
		if cfg.WadsDir != "" && os.Getenv("DOOM_WADS_DIR") == "" {
			paths.WadsDir = cfg.WadsDir
		}
		if len(cfg.WadsDirs) > 0 && os.Getenv("DOOM_WADS_DIRS") == "" {
			paths.WadsDirs = append([]string(nil), cfg.WadsDirs...)
		}
		if cfg.BinDir != "" && os.Getenv("DOOM_BIN_DIR") == "" {
			paths.SetBinDir(cfg.BinDir)
		}
		if cfg.SoundFontsDir != "" && os.Getenv("DOOM_SF_DIR") == "" {
			paths.SetSoundFontsDir(cfg.SoundFontsDir)
		}
	}
	applyFlagOverrides(paths)
	return paths, cfg, nil
}

func loadAppState() (*config.Paths, *config.AppConfig, *preset.Catalog, error) {
	paths, cfg, err := loadPathsAndConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	userCatalogPath := paths.CatalogDir
	explicitPath := false
	if flagPresetsFile != "" {
		userCatalogPath = flagPresetsFile
		explicitPath = true
	}

	cat, err := preset.LoadLayeredCatalog(userCatalogPath, explicitPath)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, d := range cat.Diagnostics {
		fmt.Fprintf(os.Stderr, "warning: %s\n", d.String())
	}

	return paths, cfg, cat, nil
}

func getCatalog() (*preset.Catalog, error) {
	_, _, cat, err := loadAppState()
	return cat, err
}

func getPaths() *config.Paths {
	paths, _, err := loadPathsAndConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		fallback := config.GetPaths()
		applyFlagOverrides(fallback)
		return fallback
	}
	return paths
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "doom",
		Short: "Unified cross-platform Doom source port manager and launcher",
		Long: `doom is a modern, unified CLI and interactive terminal launcher for classic Doom.
It manages source ports (UZDoom, DSDA-Doom), official Steam and GOG IWADs,
Roland SC-55 SoundFonts, curated community megawads, and platform-native configurations.`,
		Version:            fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		Args:               cobra.ArbitraryArgs,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommands or arguments are passed, run interactive launcher (doom play)
			return runPlay(cmd, args)
		},
	}

	rootCmd.PersistentFlags().StringVarP(
		&flagEngineOverride, "engine", "e", "", "Override source port engine (uzdoom or dsda-doom)",
	)
	rootCmd.PersistentFlags().StringVar(&flagWadsDir, "wads-dir", "", "Custom path to WADs directory")
	rootCmd.PersistentFlags().StringVar(
		&flagWadsDirs, "wads-dirs", "", "Additional WAD search directories (comma or path-list separated)",
	)
	rootCmd.PersistentFlags().StringVar(&flagBinDir, "bin-dir", "", "Custom path to engines binary directory")
	rootCmd.PersistentFlags().StringVar(
		&flagSoundFontsDir, "soundfonts-dir", "", "Custom path to SoundFonts directory",
	)
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Print launch command without executing")
	rootCmd.PersistentFlags().StringVar(
		&flagPresetsFile, "presets-file", "", "Custom user catalog directory or entity JSON file",
	)
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
