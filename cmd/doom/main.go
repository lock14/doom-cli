// Package main is the entrypoint for the unified doom CLI.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/catalog"
	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/tui"
)

var (
	// Build-time metadata injected via -ldflags
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		engineOverride string
		dryRun         bool
	)

	rootCmd := &cobra.Command{
		Use:     "doom [wad] [flags]",
		Short:   "Direct Doom launcher and WAD, engine, and configuration manager",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		Long: `Launch classic Doom, commercial expansions, and community megawads directly in
UZDoom or DSDA-Doom, or manage WAD presets, source ports, and CLI settings.

Run 'doom' with no arguments to launch The Ultimate Doom directly, pass a WAD
preset name or ID ('doom doom-ii', 'doom sunlust -skill 4 -warp 1 1'), or run
'doom wads' to open the interactive terminal WAD browser.`,
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		DisableFlagParsing: true,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			rawArgs := append([]string{"doom"}, args...)
			if hasStandaloneFlag(rawArgs, "-h", "--help") {
				return cmd.Help()
			}
			if hasStandaloneFlag(rawArgs, "-v", "--version") {
				fmt.Fprintf(cmdOut(cmd), "%s version %s\n", cmd.DisplayName(), cmd.Version)
				return nil
			}
			targetName, extraArgs, parsedEngine, hasParsedEngine, parsedDryRun, parseErr := parseRootInvocation(rawArgs)
			if parseErr != nil {
				return parseErr
			}

			out := cmdOut(cmd)
			errOut := cmdErr(cmd)
			paths, _, cat, err := loadAppStateWithErr(errOut)
			if err != nil {
				return err
			}

			p := findPreset(cat, targetName)
			if p == nil {
				return fmt.Errorf(
					"WAD preset %q not found. Run 'doom wads list' to see available WADs",
					targetName,
				)
			}

			effectiveEngine := engineOverride
			if hasParsedEngine || len(rawArgs) > 1 {
				effectiveEngine = parsedEngine
			}
			effectiveDryRun := dryRun || parsedDryRun

			opts := engine.LaunchOptions{
				EngineOverride: effectiveEngine,
				WadRoots:       paths.WadRoots(),
				BinDir:         paths.BinDir,
				DryRun:         effectiveDryRun,
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

	rootCmd.Flags().StringVarP(
		&engineOverride, "engine", "e", "", "Override source port engine (uzdoom, dsda-doom, or custom engine ID)",
	)
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print launch command without executing")
	rootCmd.InitDefaultHelpFlag()
	rootCmd.InitDefaultVersionFlag()

	rootCmd.AddCommand(
		newWadsCmd(),
		newEnginesCmd(),
		newConfigCmd(),
		newSetupCmd(),
		newSkillCmd(),
	)

	return rootCmd
}

func hasStandaloneFlag(rawArgs []string, shortFlag, longFlag string) bool {
	for i := 1; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "--" {
			return false
		}
		if arg == shortFlag || arg == longFlag {
			return true
		}
		if flagName, val, hasEq := strings.Cut(arg, "="); hasEq &&
			(flagName == shortFlag || flagName == longFlag) {
			v, err := strconv.ParseBool(val)
			return err == nil && v
		}
	}
	return false
}

func isKnownDoomEFlag(arg string) bool {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "-episode", "-extratics", "-endoom", "-exec", "-errorlog",
		"-export_text_file", "-emulate", "-export", "-exe", "-edf",
		"-ef1", "-ef2", "-empty":
		return true
	default:
		return false
	}
}

func parseRootInvocation(rawArgs []string) (string, []string, string, bool, bool, error) {
	knownFlagsWithValue := map[string]bool{
		"--engine": true,
		"-e":       true,
	}
	knownBoolFlags := map[string]bool{
		"--dry-run": true,
		"-h":        true,
		"--help":    true,
		"-v":        true,
		"--version": true,
	}

	var presetTokens []string
	seenEngineArg := false
	var extraArgs []string
	var engineVal string
	var hasEngine bool
	var dryRunVal bool

	for i := 1; i < len(rawArgs); i++ {
		arg := rawArgs[i]

		if arg == "--" {
			if i+1 < len(rawArgs) {
				extraArgs = append(extraArgs, rawArgs[i+1:]...)
			}
			break
		}

		if knownBoolFlags[arg] {
			if arg == "--dry-run" {
				dryRunVal = true
			}
			continue
		}
		if knownFlagsWithValue[arg] {
			if i+1 >= len(rawArgs) {
				return "", nil, "", false, false, fmt.Errorf("flag needs an argument: %s", arg)
			}
			engineVal = strings.TrimSpace(rawArgs[i+1])
			if engineVal == "" {
				return "", nil, "", false, false, fmt.Errorf("engine name cannot be empty")
			}
			hasEngine = true
			i++ // skip flag value
			continue
		}
		if flagName, val, hasEq := strings.Cut(arg, "="); hasEq {
			if knownFlagsWithValue[flagName] {
				engineVal = strings.TrimSpace(val)
				if engineVal == "" {
					return "", nil, "", false, false, fmt.Errorf("engine name cannot be empty")
				}
				hasEngine = true
				continue
			}
			if knownBoolFlags[flagName] {
				b, err := strconv.ParseBool(val)
				if err != nil {
					return "", nil, "", false, false, fmt.Errorf(
						"invalid argument %q for %q flag: %w", val, flagName, err,
					)
				}
				if flagName == "--dry-run" {
					dryRunVal = b
				}
				continue
			}
			if strings.HasPrefix(flagName, "--") {
				return "", nil, "", false, false, fmt.Errorf("unknown flag: %s", flagName)
			}
		}
		if strings.HasPrefix(arg, "--") {
			return "", nil, "", false, false, fmt.Errorf("unknown flag: %s", arg)
		}
		if strings.HasPrefix(arg, "-e") && len(arg) > 2 && !isKnownDoomEFlag(arg) {
			engineVal = strings.TrimSpace(arg[2:])
			if engineVal == "" {
				return "", nil, "", false, false, fmt.Errorf("engine name cannot be empty")
			}
			hasEngine = true
			continue
		}

		if !seenEngineArg {
			if !strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "+") {
				presetTokens = append(presetTokens, arg)
				continue
			}
			seenEngineArg = true
			extraArgs = append(extraArgs, arg)
			continue
		}

		seenEngineArg = true
		extraArgs = append(extraArgs, arg)
	}

	targetPreset := strings.Join(presetTokens, " ")
	if targetPreset == "" {
		targetPreset = "doom"
	}
	return targetPreset, extraArgs, engineVal, hasEngine, dryRunVal, nil
}

func cmdOut(cmd *cobra.Command) io.Writer {
	if cmd != nil {
		return cmd.OutOrStdout()
	}
	return os.Stdout
}

func cmdErr(cmd *cobra.Command) io.Writer {
	if cmd != nil {
		return cmd.ErrOrStderr()
	}
	return os.Stderr
}

func resolveAppPaths() *config.Paths {
	p := config.GetPaths()
	cfg, cfgErr := config.LoadConfig(p)
	if cfgErr != nil {
		cfg = &config.AppConfig{}
	}
	config.ApplyConfigPaths(p, cfg)
	return p
}

func loadAppState() (*config.Paths, *config.AppConfig, *catalog.Catalog, error) {
	return loadAppStateWithErr(os.Stderr)
}

func loadAppStateWithErr(errOut io.Writer) (*config.Paths, *config.AppConfig, *catalog.Catalog, error) {
	if errOut == nil {
		errOut = os.Stderr
	}
	p := config.GetPaths()
	cfg, cfgErr := config.LoadConfig(p)
	if cfgErr != nil {
		fmt.Fprintf(errOut, "Warning: could not parse %s (%v); using defaults\n", p.ConfigFile, cfgErr)
		cfg = &config.AppConfig{}
	}

	config.ApplyConfigPaths(p, cfg)

	cat, err := catalog.LoadLayered(p.CatalogDir, false)
	if err != nil {
		return p, cfg, nil, err
	}
	for _, d := range cat.Diagnostics {
		fmt.Fprintf(errOut, "warning: %s\n", d.String())
	}

	return p, cfg, cat, nil
}

func splitCommandArgs(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}

	var args []string
	var cur strings.Builder
	inToken := false
	inSingle := false
	inDouble := false

	runes := []rune(raw)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if inSingle {
			if r == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(r)
			}
			continue
		}

		if inDouble {
			if r == '"' {
				inDouble = false
				continue
			}
			if r == '\\' && i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\') {
				i++
				cur.WriteRune(runes[i])
				continue
			}
			cur.WriteRune(r)
			continue
		}

		if unicode.IsSpace(r) {
			if inToken {
				args = append(args, cur.String())
				cur.Reset()
				inToken = false
			}
			continue
		}

		if r == '\'' {
			inSingle = true
			inToken = true
			continue
		}
		if r == '"' {
			inDouble = true
			inToken = true
			continue
		}
		if r == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			if unicode.IsSpace(next) || next == '\'' || next == '"' {
				i++
				cur.WriteRune(next)
				inToken = true
				continue
			}
		}

		cur.WriteRune(r)
		inToken = true
	}

	if inSingle {
		return nil, fmt.Errorf("unterminated single quote in arguments: %s", raw)
	}
	if inDouble {
		return nil, fmt.Errorf("unterminated double quote in arguments: %s", raw)
	}
	if inToken {
		args = append(args, cur.String())
	}
	return args, nil
}

func buildLauncherOptions(
	paths *config.Paths,
	cfg *config.AppConfig,
	extraArgs []string,
	initialMode int,
) (tui.Theme, bool, tui.LauncherOptions) {
	var cfgTheme, cfgWads, cfgBin, cfgSF string
	if cfg != nil {
		cfgTheme = cfg.Theme
		cfgWads = cfg.WadsDir
		cfgBin = cfg.BinDir
		cfgSF = cfg.SoundFontsDir
	}
	var themesDir, catalogDir string
	if paths != nil {
		themesDir = paths.ThemesDir
		catalogDir = paths.CatalogDir
	}

	theme := tui.ResolveTheme(cfgTheme, themesDir)
	_, themeSrc := resolveThemeWithSource(cfg)
	nerdFonts, nfSrc := config.ResolveNerdFontsWithSource(cfg)

	opts := tui.LauncherOptions{
		Paths:            paths,
		AppConfig:        cfg,
		CatalogDir:       catalogDir,
		ExtraArgs:        extraArgs,
		ThemeSource:      themeSrc,
		NerdFontsSource:  nfSrc,
		WadsDirSource:    config.DirSource(cfgWads),
		WadsDirsSource:   config.WadsDirsSource(cfg),
		BinDirSource:     config.DirSource(cfgBin),
		SoundFontsSource: config.DirSource(cfgSF),
		InitialMode:      initialMode,
	}
	return theme, nerdFonts, opts
}
