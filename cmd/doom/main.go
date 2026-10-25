// Package main is the entrypoint for the unified doom CLI.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/preset"
	"github.com/lock14/doom-cli/internal/tui"
)

var (
	// Build-time metadata injected via -ldflags
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
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
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			rawArgs := cmdRawArgs(cmd, args)
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

			paths, _, cat, err := loadAppState()
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

			out := cmdOut(cmd)
			errOut := cmdErr(cmd)
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

	attachStrictHelpAndVersionFlags(rootCmd, true)

	defaultFlagErr := rootCmd.FlagErrorFunc()
	rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		if c == rootCmd && err != nil &&
			strings.Contains(err.Error(), "flag needs an argument: 'e' in -") &&
			!hasDanglingEngineFlag(cmdRawArgs(c, nil)) {
			return rootCmd.RunE(c, c.Flags().Args())
		}
		if defaultFlagErr != nil {
			return defaultFlagErr(c, err)
		}
		return err
	})

	rootCmd.AddCommand(
		newWadsCmd(),
		newEnginesCmd(),
		newConfigCmd(),
		newSetupCmd(),
	)

	return rootCmd
}

type strictShorthandBoolValue struct {
	cmd       *cobra.Command
	shortFlag string
	longFlag  string
	val       bool
}

func (b *strictShorthandBoolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	b.val = v
	return nil
}

func (b *strictShorthandBoolValue) Type() string {
	return "bool"
}

func (b *strictShorthandBoolValue) IsBoolFlag() bool {
	return true
}

func (b *strictShorthandBoolValue) String() string {
	if !b.val {
		return "false"
	}
	if b.cmd != nil && !hasStandaloneFlag(cmdRawArgs(b.cmd, nil), b.shortFlag, b.longFlag) {
		return "false"
	}
	return "true"
}

func attachStrictHelpAndVersionFlags(cmd *cobra.Command, withVersion bool) {
	helpVal := &strictShorthandBoolValue{cmd: cmd, shortFlag: "-h", longFlag: "--help"}
	cmd.Flags().VarP(helpVal, "help", "h", "help for "+cmd.Name())
	if f := cmd.Flags().Lookup("help"); f != nil {
		f.NoOptDefVal = "true"
	}
	if withVersion {
		versionVal := &strictShorthandBoolValue{cmd: cmd, shortFlag: "-v", longFlag: "--version"}
		cmd.Flags().VarP(versionVal, "version", "v", "version for "+cmd.Name())
		if f := cmd.Flags().Lookup("version"); f != nil {
			f.NoOptDefVal = "true"
		}
	}
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

func hasDanglingEngineFlag(rawArgs []string) bool {
	for i := 1; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "--" {
			return false
		}
		if arg == "-e" || arg == "--engine" {
			if i+1 >= len(rawArgs) {
				return true
			}
			i++
		}
	}
	return false
}

func cmdRawArgs(cmd *cobra.Command, cobraArgs []string) []string {
	for c := cmd; c != nil; c = c.Parent() {
		v := reflect.ValueOf(c)
		if v.Kind() == reflect.Ptr && !v.IsNil() {
			f := v.Elem().FieldByName("args")
			if f.IsValid() && f.Kind() == reflect.Slice && !f.IsNil() {
				res := make([]string, f.Len())
				for i := 0; i < f.Len(); i++ {
					res[i] = f.Index(i).String()
				}
				if c != cmd.Root() && c.Name() != "" {
					return append([]string{"doom", c.Name()}, res...)
				}
				return append([]string{"doom"}, res...)
			}
		}
	}
	return rootRawArgs(cobraArgs)
}

func rootRawArgs(cobraArgs []string) []string {
	if len(os.Args) > 0 {
		bin := filepath.Base(os.Args[0])
		if !strings.HasSuffix(bin, ".test") && !strings.HasSuffix(bin, ".test.exe") {
			return os.Args
		}
	}
	return append([]string{"doom"}, cobraArgs...)
}

func isKnownDoomEFlag(arg string) bool {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "-episode", "-extratics", "-endoom", "-emulate", "-export", "-exe", "-ef1", "-ef2":
		return true
	default:
		return false
	}
}

func parseRootLaunchArgs(rawArgs []string) (string, []string) {
	targetPreset, extraArgs, _, _, _, _ := parseRootInvocation(rawArgs)
	return targetPreset, extraArgs
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

func applyConfigPaths(p *config.Paths, cfg *config.AppConfig) {
	if cfg == nil {
		return
	}
	if s := strings.TrimSpace(cfg.WadsDir); s != "" {
		p.WadsDir = s
	}
	if len(cfg.WadsDirs) > 0 {
		p.WadsDirs = append([]string(nil), cfg.WadsDirs...)
	}
	if s := strings.TrimSpace(cfg.BinDir); s != "" {
		p.SetBinDir(s)
	}
	if s := strings.TrimSpace(cfg.SoundFontsDir); s != "" {
		p.SetSoundFontsDir(s)
	}
}

func resolveAppPaths() *config.Paths {
	p := config.GetPaths()
	cfg, cfgErr := config.LoadConfig(p)
	if cfgErr != nil {
		cfg = &config.AppConfig{}
	}
	applyConfigPaths(p, cfg)
	return p
}

func loadAppState() (*config.Paths, *config.AppConfig, *preset.Catalog, error) {
	p := config.GetPaths()
	cfg, cfgErr := config.LoadConfig(p)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not parse %s (%v); using defaults\n", p.ConfigFile, cfgErr)
		cfg = &config.AppConfig{}
	}

	applyConfigPaths(p, cfg)

	cat, err := preset.LoadLayeredCatalog(p.CatalogDir, false)
	if err != nil {
		return p, cfg, nil, err
	}
	for _, d := range cat.Diagnostics {
		fmt.Fprintf(os.Stderr, "warning: %s\n", d.String())
	}

	return p, cfg, cat, nil
}

func getPaths() *config.Paths {
	return resolveAppPaths()
}

func wadsDirsSource(cfg *config.AppConfig) string {
	if cfg != nil && len(cfg.WadsDirs) > 0 {
		return "config"
	}
	return "default"
}

func dirSource(cfgVal string) string {
	if strings.TrimSpace(cfgVal) != "" {
		return "config"
	}
	return "default"
}

func buildLauncherOptions(
	paths *config.Paths,
	cfg *config.AppConfig,
	extraArgs []string,
	initialMode int,
) (tui.Theme, bool, tui.LauncherOptions) {
	theme := tui.ResolveTheme("", "", cfg.Theme, paths.ThemesDir)
	_, themeSrc := resolveThemeWithSource(cfg)
	nerdFonts, nfSrc := resolveNerdFontsWithSource(cfg)

	var cfgWads, cfgBin, cfgSF string
	if cfg != nil {
		cfgWads = cfg.WadsDir
		cfgBin = cfg.BinDir
		cfgSF = cfg.SoundFontsDir
	}

	opts := tui.LauncherOptions{
		Paths:            paths,
		AppConfig:        cfg,
		CatalogDir:       paths.CatalogDir,
		ExtraArgs:        extraArgs,
		ThemeSource:      themeSrc,
		NerdFontsSource:  nfSrc,
		WadsDirSource:    dirSource(cfgWads),
		WadsDirsSource:   wadsDirsSource(cfg),
		BinDirSource:     dirSource(cfgBin),
		SoundFontsSource: dirSource(cfgSF),
		InitialMode:      initialMode,
	}
	return theme, nerdFonts, opts
}
