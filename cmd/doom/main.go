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
	flagWadsDir   string
	flagTheme     string
	flagNerdFonts bool

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
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}

			rawArgs := cmdRawArgs(cmd, args)
			targetName, extraArgs, parsedEngine, hasParsedEngine, parsedDryRun := parseRootInvocation(rawArgs)
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

	rootCmd.PersistentFlags().StringVar(
		&flagWadsDir, "wads-dir", "", "Directory containing WAD files (overrides config and DOOM_WADS_DIR)",
	)
	rootCmd.PersistentFlags().StringVar(
		&flagTheme,
		"theme",
		"",
		"Color theme (classic, blood, toxic, inferno, frost, plasma, heretic, amber, sigil, "+
			"monochrome, or custom theme name)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&flagNerdFonts,
		"nerd-fonts",
		false,
		"Enable Nerd Font glyphs (Powerlevel10k rounded capsules; default: universal rectangular badges)",
	)

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

func parseRootLaunchArgs(rawArgs []string) (string, []string) {
	targetPreset, extraArgs, _, _, _ := parseRootInvocation(rawArgs)
	return targetPreset, extraArgs
}

func parseRootInvocation(rawArgs []string) (string, []string, string, bool, bool) {
	knownFlagsWithValue := map[string]bool{
		"--wads-dir": true,
		"--theme":    true,
		"--engine":   true,
		"-e":         true,
	}
	knownBoolFlags := map[string]bool{
		"--nerd-fonts": true,
		"--dry-run":    true,
		"-h":           true,
		"--help":       true,
		"-v":           true,
		"--version":    true,
	}

	targetPreset := ""
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
			if (arg == "-e" || arg == "--engine") && i+1 < len(rawArgs) {
				engineVal = rawArgs[i+1]
				hasEngine = true
			}
			i++ // skip flag value
			continue
		}
		if flagName, val, hasEq := strings.Cut(arg, "="); hasEq {
			if knownFlagsWithValue[flagName] || knownBoolFlags[flagName] {
				if flagName == "-e" || flagName == "--engine" {
					engineVal = val
					hasEngine = true
				} else if flagName == "--dry-run" {
					if b, err := strconv.ParseBool(val); err == nil {
						dryRunVal = b
					}
				}
				continue
			}
		}

		if targetPreset == "" && !seenEngineArg {
			if !strings.HasPrefix(arg, "-") {
				targetPreset = arg
				continue
			}
			seenEngineArg = true
			extraArgs = append(extraArgs, arg)
			continue
		}

		seenEngineArg = true
		extraArgs = append(extraArgs, arg)
	}

	if targetPreset == "" {
		targetPreset = "doom"
	}
	return targetPreset, extraArgs, engineVal, hasEngine, dryRunVal
}

func extractEngineArgs(subcommand string, rawArgs []string) []string {
	if subcommand == "" || subcommand == "root" {
		_, extra := parseRootLaunchArgs(rawArgs)
		return extra
	}

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
		"-h":           true,
		"--help":       true,
		"--version":    true,
		"-v":           true,
	}

	var engineArgs []string
	pastSubcommand := false

	for i := 1; i < len(rawArgs); i++ {
		arg := rawArgs[i]

		if knownBoolFlags[arg] {
			continue
		}
		if knownFlagsWithValue[arg] {
			i++
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
			}
			continue
		}

		engineArgs = append(engineArgs, arg)
	}

	return engineArgs
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

func loadAppState() (*config.Paths, *config.AppConfig, *preset.Catalog, error) {
	p := config.GetPaths()
	cfg, cfgErr := config.LoadConfig(p)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not parse %s (%v); using defaults\n", p.ConfigFile, cfgErr)
		cfg = &config.AppConfig{}
	}

	p.WadsDir = resolveStringValue(flagWadsDir, "DOOM_WADS_DIR", cfg.WadsDir, p.WadsDir)
	p.WadsDirs = resolveWadsDirs(cfg)
	p.SetBinDir(resolveStringValue("", "DOOM_BIN_DIR", cfg.BinDir, p.BinDir))
	p.SetSoundFontsDir(resolveStringValue("", "DOOM_SF_DIR", cfg.SoundFontsDir, p.SoundFontsDir))

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
	p, _, _, _ := loadAppState()
	return p
}

func resolveStringValue(flagVal, envKey, cfgVal, defVal string) string {
	if s := strings.TrimSpace(flagVal); s != "" {
		return s
	}
	if envKey != "" {
		if s := strings.TrimSpace(os.Getenv(envKey)); s != "" {
			return s
		}
	}
	if s := strings.TrimSpace(cfgVal); s != "" {
		return s
	}
	return defVal
}

func resolveWadsDirs(cfg *config.AppConfig) []string {
	if envVal := strings.TrimSpace(os.Getenv("DOOM_WADS_DIRS")); envVal != "" {
		return config.ParsePathList(envVal)
	}
	if cfg != nil && len(cfg.WadsDirs) > 0 {
		return append([]string(nil), cfg.WadsDirs...)
	}
	return nil
}

func wadsDirsSource(cfg *config.AppConfig) string {
	if strings.TrimSpace(os.Getenv("DOOM_WADS_DIRS")) != "" {
		return "env"
	}
	if cfg != nil && len(cfg.WadsDirs) > 0 {
		return "config"
	}
	return "default"
}

func dirSource(flagVal, envKey, cfgVal string) string {
	if strings.TrimSpace(flagVal) != "" {
		return "flag"
	}
	if envKey != "" && strings.TrimSpace(os.Getenv(envKey)) != "" {
		return "env"
	}
	if strings.TrimSpace(cfgVal) != "" {
		return "config"
	}
	return "default"
}

func buildLauncherOptions(
	cmd *cobra.Command,
	paths *config.Paths,
	cfg *config.AppConfig,
	extraArgs []string,
	initialMode int,
) (tui.Theme, bool, tui.LauncherOptions) {
	theme := tui.ResolveTheme(flagTheme, os.Getenv("DOOM_THEME"), cfg.Theme, paths.ThemesDir)
	_, themeSrc := resolveThemeWithSource(cfg)
	nerdFonts, nfSrc := resolveNerdFontsWithSource(cmd, cfg)

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
		WadsDirSource:    dirSource(flagWadsDir, "DOOM_WADS_DIR", cfgWads),
		WadsDirsSource:   wadsDirsSource(cfg),
		BinDirSource:     dirSource("", "DOOM_BIN_DIR", cfgBin),
		SoundFontsSource: dirSource("", "DOOM_SF_DIR", cfgSF),
		InitialMode:      initialMode,
	}
	return theme, nerdFonts, opts
}
