package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/tui"
)

// newPlayCmd creates the interactive terminal launcher command.
func newPlayCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "play [flags] [-- extra_engine_args...]",
		Short:              "Interactive terminal launcher with fuzzy search and live preview",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE:               runPlay,
	}
}

func runPlay(cmd *cobra.Command, args []string) error {
	paths, cfg, cat, err := loadAppState()
	if err != nil {
		return err
	}

	extraArgs := extractEngineArgs("play", os.Args)

	theme := tui.ResolveTheme(flagTheme, os.Getenv("DOOM_THEME"), cfg.Theme, paths.ThemesDir)
	_, themeSrc := resolveThemeWithSource(cfg)
	nerdFonts, nfSrc := resolveNerdFontsWithSource(cmd, cfg)

	var cfgWads, cfgBin, cfgSF string
	if cfg != nil {
		cfgWads = cfg.WadsDir
		cfgBin = cfg.BinDir
		cfgSF = cfg.SoundFontsDir
	}

	selected, err := tui.RunInteractiveLauncherWithOptions(
		cat,
		paths.WadRoots(),
		theme,
		nerdFonts,
		tui.LauncherOptions{
			Paths:            paths,
			AppConfig:        cfg,
			CatalogDir:       paths.CatalogDir,
			ExtraArgs:        extraArgs,
			ThemeSource:      themeSrc,
			NerdFontsSource:  nfSrc,
			WadsDirSource:    dirSource(flagWadsDir, "DOOM_WADS_DIR", cfgWads),
			WadsDirsSource:   wadsDirsSource(cfg),
			BinDirSource:     dirSource("", "DOOM_BIN_DIR", cfgBin),
			SoundFontsSource: dirSource("", "DOOM_SF_DIR", cfgSF),
		},
		"",
	)
	if err != nil {
		return err
	}
	if selected == nil {
		return nil
	}

	if refreshedPaths, refreshedCfg, refreshedCat, reloadErr := loadAppState(); reloadErr == nil {
		paths = refreshedPaths
		cfg = refreshedCfg
		cat = refreshedCat
	}

	var engOverride string
	if selected.EngineOverridden {
		engOverride = selected.Engine
	}
	opts := engine.LaunchOptions{
		EngineOverride: engOverride,
		WadRoots:       paths.WadRoots(),
		BinDir:         paths.BinDir,
		ExtraArgs:      extraArgs,
		Out:            os.Stdout,
		Catalog:        cat,
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
	if dashDashIdx != -1 {
		if dashDashIdx+1 < len(rawArgs) {
			return rawArgs[dashDashIdx+1:]
		}
		return nil
	}

	knownFlagsWithValue := map[string]bool{
		"--wads-dir": true,
		"--theme":    true,
	}
	knownBoolFlags := map[string]bool{
		"--nerd-fonts": true,
		"-h":           true, "--help": true,
		"--version": true, "-v": true,
	}
	if subcommand == "launch" {
		knownFlagsWithValue["--engine"] = true
		knownFlagsWithValue["-e"] = true
		knownBoolFlags["--dry-run"] = true
	}

	var engineArgs []string
	pastSubcommand := false
	presetSkipped := (subcommand != "launch") // Only skip preset argument on "launch"

	for i := 1; i < len(rawArgs); i++ {
		arg := rawArgs[i]

		if knownBoolFlags[arg] {
			continue
		}
		if knownFlagsWithValue[arg] {
			i++ // skip flag value
			continue
		}
		if flagName, _, hasEq := strings.Cut(arg, "="); hasEq {
			if knownFlagsWithValue[flagName] || knownBoolFlags[flagName] {
				continue
			}
		}

		if !pastSubcommand {
			if arg == subcommand {
				pastSubcommand = true
				continue
			}
			if subcommand == "play" {
				pastSubcommand = true
			} else {
				continue
			}
		}

		if !presetSkipped && !strings.HasPrefix(arg, "-") {
			presetSkipped = true
			continue
		}

		engineArgs = append(engineArgs, arg)
	}

	return engineArgs
}
