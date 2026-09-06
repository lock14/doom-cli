// Package main provides the entry point for the unified doom CLI.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/preset"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/templates"
	"github.com/lock14/doom-cli/internal/tui"
	"github.com/lock14/doom-cli/internal/wad"
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

func main() {
	rootCmd := &cobra.Command{
		Use:   "doom",
		Short: "Unified cross-platform Doom source port manager & launcher",
		Long: `doom is a modern, unified CLI and interactive terminal launcher for classic Doom.
It manages source ports (UZDoom, DSDA-Doom, DoomRunner), official Steam/GOG IWADs,
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
		&flagTheme, "theme", "", "Color theme for launcher (default, cyberpunk, blood, matrix, monochrome)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&flagNerdFonts, "nerd-fonts", false, "Use Powerlevel10k rounded capsule ends (requires a Nerd Font)",
	)

	// Subcommand: play
	playCmd := &cobra.Command{
		Use:                "play [flags] [-- extra_engine_args...]",
		Short:              "Interactive terminal launcher with fuzzy search and live preview",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE:               runPlay,
	}
	playCmd.Flags().BoolVar(&flagOnce, "once", false, "Exit after playing a single preset")
	rootCmd.Flags().BoolVar(&flagOnce, "once", false, "Exit after playing a single preset")
	rootCmd.AddCommand(playCmd)

	// Subcommand: launch
	launchCmd := &cobra.Command{
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
			}

			plan, err := engine.PrepareLaunch(*p, opts)
			if err != nil {
				return err
			}

			return engine.Execute(plan, os.Stdout, os.Stderr)
		},
	}
	rootCmd.AddCommand(launchCmd)

	// Subcommand: setup
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete 1-command setup (engines, soundfont, configs, steam IWADs, and community megawads)",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			cat, err := getCatalog()
			if err != nil {
				return err
			}

			fmt.Println("============================================================")
			fmt.Println("  ⚡ Initiating Doom Setup")
			fmt.Println("============================================================")
			fmt.Println()

			// 1. Configs
			fmt.Println(">>> Step 1/5: Deploying platform configurations...")
			if err := templates.DeployConfigs(paths); err != nil {
				return fmt.Errorf("error deploying configs: %w", err)
			}
			fmt.Println()

			// 2. Engines
			fmt.Println(">>> Step 2/5: Installing source ports (UZDoom, DSDA-Doom, DoomRunner)...")
			installer := engine.NewInstaller(paths.BinDir, os.Stdout)
			if err := installer.InstallAll(); err != nil {
				fmt.Printf("Warning: engine installer encountered: %v\n", err)
			}
			fmt.Println()

			// 3. SoundFont
			fmt.Println(">>> Step 3/5: Installing Roland SC-55 SoundFont...")
			if _, err := wad.InstallSoundFont(paths.SoundFontsDir, flagForce, os.Stdout); err != nil {
				fmt.Printf("Warning: soundfont installer encountered: %v\n", err)
			}
			fmt.Println()

			// 4. Steam IWADs
			fmt.Println(">>> Step 4/5: Auto-discovering official Steam/GOG game files...")
			if _, err := steam.DiscoverAndExtract(nil, paths.WadsDir, flagForce, os.Stdout); err != nil {
				fmt.Printf("Warning: steam discoverer encountered: %v\n", err)
			}
			fmt.Println()

			// 5. Community megawads
			fmt.Println(">>> Step 5/5: Downloading community megawads...")
			dl := wad.NewDownloader(paths.WadsDir, flagForce, os.Stdout)
			if err := dl.DownloadAll(cat); err != nil {
				fmt.Printf("Warning: some megawads failed to download: %v\n", err)
			}
			fmt.Println()

			fmt.Println("============================================================")
			fmt.Println("  ✓ Doom setup complete!")
			fmt.Println("  Engines, configs, soundfonts, and megawads are ready.")
			fmt.Println("  Run 'doom play' or 'doom' to start playing!")
			fmt.Println("============================================================")
			return nil
		},
	}
	setupCmd.Flags().BoolVar(&flagForce, "force", false, "Force re-download and overwrite existing files")
	rootCmd.AddCommand(setupCmd)

	// Subcommand: wads
	wadsCmd := &cobra.Command{
		Use:   "wads",
		Short: "Manage WADs, megawads, and official expansions",
	}

	wadsFetchCmd := &cobra.Command{
		Use:   "fetch [preset_name|all]",
		Short: "Download community megawad files",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			cat, err := getCatalog()
			if err != nil {
				return err
			}

			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			dl := wad.NewDownloader(paths.WadsDir, flagForce, os.Stdout)
			if strings.EqualFold(target, "all") {
				return dl.DownloadAll(cat)
			}

			p := cat.Find(target)
			if p == nil {
				return fmt.Errorf("preset %q not found", target)
			}
			return dl.DownloadPreset(*p)
		},
	}
	wadsFetchCmd.Flags().BoolVar(&flagForce, "force", false, "Re-download and overwrite existing files")

	wadsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List downloadable community megawads",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := getCatalog()
			if err != nil {
				return err
			}
			fmt.Println("=== Available Downloadable Community Megawads ===")
			for _, p := range cat.Presets {
				if len(p.DownloadURLs) > 0 {
					files := wad.FilterExpectedFiles(p.Mappacks)
					fmt.Printf("  - %-30s [%s]\n", p.Name, strings.Join(files, ", "))
				}
			}
			return nil
		},
	}

	wadsExtractCmd := &cobra.Command{
		Use:   "extract-steam",
		Short: "Auto-discover and copy official IWADs and expansions from Steam/GOG",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			_, err := steam.DiscoverAndExtract(nil, paths.WadsDir, flagForce, os.Stdout)
			return err
		},
	}
	wadsExtractCmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing files in destination")

	wadsCmd.AddCommand(wadsFetchCmd, wadsListCmd, wadsExtractCmd)
	rootCmd.AddCommand(wadsCmd)

	// Subcommand: engines
	enginesCmd := &cobra.Command{
		Use:   "engines",
		Short: "Download and manage source port engines (UZDoom, DSDA-Doom, DoomRunner)",
	}
	enginesInstallCmd := &cobra.Command{
		Use:   "install [uzdoom|dsda-doom|doomrunner|all]",
		Short: "Download and deploy engine binaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			ins := engine.NewInstaller(paths.BinDir, os.Stdout)
			target := "all"
			if len(args) > 0 {
				target = strings.ToLower(args[0])
			}

			switch target {
			case "uzdoom":
				return ins.InstallUZDoom()
			case "dsda", "dsda-doom", "dsdadoom":
				return ins.InstallDSDA()
			case "doomrunner":
				return ins.InstallDoomRunner()
			case "all":
				return ins.InstallAll()
			default:
				return fmt.Errorf("unknown engine target: %s (choose uzdoom, dsda-doom, doomrunner, or all)", target)
			}
		},
	}
	enginesCmd.AddCommand(enginesInstallCmd)
	rootCmd.AddCommand(enginesCmd)

	// Subcommand: soundfont
	sfCmd := &cobra.Command{
		Use:   "soundfont",
		Short: "Manage MIDI SoundFonts for FluidSynth",
	}
	sfInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Download and deploy Roland SC-55 compatible GeneralUser-GS SoundFont",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			_, err := wad.InstallSoundFont(paths.SoundFontsDir, flagForce, os.Stdout)
			return err
		},
	}
	sfInstallCmd.Flags().BoolVar(&flagForce, "force", false, "Re-download and overwrite existing SoundFont")
	sfCmd.AddCommand(sfInstallCmd)
	rootCmd.AddCommand(sfCmd)

	// Subcommand: config
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage engine configurations and DoomRunner options",
	}
	configInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Deploy autoexec.cfg, dsda-doom.cfg, and options.json with backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.DeployConfigs(paths)
		},
	}
	configDiffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare installed system configs against repository templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.DiffConfigs(paths, os.Stdout)
		},
	}
	configSyncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync modified in-game settings back into repository templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.SyncConfigs(paths, ".", os.Stdout)
		},
	}
	configCmd.AddCommand(configInstallCmd, configDiffCmd, configSyncCmd)
	rootCmd.AddCommand(configCmd)

	// Subcommand: presets
	presetsCmd := &cobra.Command{
		Use:   "presets",
		Short: "Inspect and build preset catalog",
	}
	presetsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all 32 curated presets with engine assignments and descriptions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := getCatalog()
			if err != nil {
				return err
			}
			fmt.Println("=== Available Doom Presets ===")
			for i, p := range cat.Presets {
				eng := "UZDoom"
				if p.Engine == "dsda-doom" {
					eng = "DSDA-Doom"
				}
				fmt.Printf("%2d. %-30s [%-9s] - %s\n", i+1, p.Name, eng, p.Description)
			}
			return nil
		},
	}
	presetsBuildCmd := &cobra.Command{
		Use:   "build",
		Short: "Compile data/presets.json into DoomRunner options.json and update README.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			if err := preset.CompileOptionsFiles(root); err != nil {
				return err
			}
			fmt.Println("✓ Successfully compiled DoomRunner options.json and updated README.md")
			return nil
		},
	}
	presetsCmd.AddCommand(presetsListCmd, presetsBuildCmd)
	rootCmd.AddCommand(presetsCmd)

	// Subcommand: themes
	themesCmd := &cobra.Command{
		Use:   "themes",
		Short: "Browse, preview, and set color themes for the launcher",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThemesList(cmd, args)
		},
	}
	themesListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all available color themes with visual swatches",
		RunE:  runThemesList,
	}
	themesSetCmd := &cobra.Command{
		Use:   "set <theme_name>",
		Short: "Set the default color theme in user config",
		Args:  cobra.ExactArgs(1),
		RunE:  runThemesSet,
	}
	themesCmd.AddCommand(themesListCmd, themesSetCmd)
	rootCmd.AddCommand(themesCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runThemesList(cmd *cobra.Command, args []string) error {
	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	activeTheme := tui.ResolveTheme(flagTheme, os.Getenv("DOOM_THEME"), cfg.Theme, paths.ThemesDir)
	nerdFonts := flagNerdFonts || cfg.NerdFonts

	fmt.Println("=== Available Doom Themes ===")
	fmt.Println()

	themes := tui.ListBuiltinThemes()
	for _, t := range themes {
		marker := "  "
		if strings.EqualFold(t.Name, activeTheme.Name) {
			marker = "* "
		}

		styles := tui.CompileStyles(t)
		sample := styles.RenderBrandPill(nerdFonts) + " " +
			styles.TagDSDA.Render("[DSDA]") + " " +
			styles.TagUZDoom.Render("[UZDoom]") + " " +
			styles.CursorBar.Render("▎ ") +
			styles.CursorText.Render("Select")

		fmt.Printf("%s%-11s [%-9s] %-42s %s\n", marker, t.Name, t.Type, t.Description, sample)
	}

	fmt.Println()
	fmt.Printf("Active theme: %s\n", activeTheme.Name)
	fmt.Printf("Run 'doom themes set <name>' to save a default theme to %s\n", paths.ConfigFile)
	return nil
}

func runThemesSet(cmd *cobra.Command, args []string) error {
	target := strings.TrimSpace(args[0])
	paths := getPaths()

	if _, ok := tui.GetBuiltinTheme(target); !ok {
		customFile := filepath.Join(paths.ThemesDir, target+".json")
		if _, err := os.Stat(customFile); err != nil {
			if _, errPath := os.Stat(target); errPath != nil {
				return fmt.Errorf("unknown theme %q. Run 'doom themes list' to see available themes", target)
			}
		}
	}

	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cfg.Theme = target
	if err := config.SaveConfig(paths, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("✓ Active theme set to %q in %s\n", target, paths.ConfigFile)
	return nil
}

func runPlay(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}

	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	theme := tui.ResolveTheme(flagTheme, os.Getenv("DOOM_THEME"), cfg.Theme, paths.ThemesDir)
	nerdFonts := flagNerdFonts || cfg.NerdFonts
	extraArgs := extractEngineArgs("play", os.Args)

	isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	singleShot := flagOnce || flagDryRun || !isInteractive

	var lastPreset string
	for {
		selected, err := tui.RunInteractiveLauncher(cat, paths.WadsDir, theme, nerdFonts, lastPreset)
		if err != nil {
			return err
		}
		if selected == nil {
			return nil
		}

		lastPreset = selected.Name
		opts := engine.LaunchOptions{
			EngineOverride: flagEngineOverride,
			WadsDir:        paths.WadsDir,
			BinDir:         paths.BinDir,
			DryRun:         flagDryRun,
			ExtraArgs:      extraArgs,
			Out:            os.Stdout,
		}

		plan, err := engine.PrepareLaunch(*selected, opts)
		if err != nil {
			if singleShot {
				return err
			}
			fmt.Fprintf(os.Stderr, "\nLaunch error: %v\nPress Enter to return to launcher...", err)
			waitForEnter(os.Stdin)
			continue
		}

		if flagDryRun {
			return engine.Execute(plan, os.Stdout, os.Stderr)
		}

		if err := engine.Execute(plan, os.Stdout, os.Stderr); err != nil {
			if singleShot {
				return err
			}
			fmt.Fprintf(os.Stderr, "\nEngine exited with error: %v\nPress Enter to return to launcher...", err)
			waitForEnter(os.Stdin)
		}

		if singleShot {
			return nil
		}
	}
}

// waitForEnter pauses execution until the user presses Enter on the provided reader.
func waitForEnter(r io.Reader) {
	reader := bufio.NewReader(r)
	_, _ = reader.ReadString('\n')
}

func extractEngineArgs(subcommand string, rawArgs []string) []string {
	dashDashIdx := -1
	for i, a := range rawArgs {
		if a == "--" {
			dashDashIdx = i
			break
		}
	}
	if dashDashIdx != -1 && dashDashIdx+1 < len(rawArgs) {
		return rawArgs[dashDashIdx+1:]
	}

	knownFlagsWithValue := map[string]bool{
		"--engine": true, "-e": true,
		"--wads-dir":     true,
		"--bin-dir":      true,
		"--presets-file": true,
		"--theme":        true,
	}
	knownBoolFlags := map[string]bool{
		"--dry-run":    true,
		"--force":      true,
		"--once":       true,
		"--nerd-fonts": true,
		"-h":           true, "--help": true,
	}

	var engineArgs []string
	pastSubcommand := false
	presetSkipped := (subcommand != "launch") // Only skip preset argument on "launch"

	for i := 1; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if !pastSubcommand {
			if arg == subcommand {
				pastSubcommand = true
			}
			continue
		}

		if knownBoolFlags[arg] {
			continue
		}
		if knownFlagsWithValue[arg] {
			i++ // skip flag value
			continue
		}
		if strings.HasPrefix(arg, "--wads-dir=") ||
			strings.HasPrefix(arg, "--bin-dir=") ||
			strings.HasPrefix(arg, "--engine=") ||
			strings.HasPrefix(arg, "--presets-file=") ||
			strings.HasPrefix(arg, "--theme=") ||
			strings.HasPrefix(arg, "--nerd-fonts=") {
			continue
		}

		if !presetSkipped && !strings.HasPrefix(arg, "-") {
			presetSkipped = true
			continue
		}

		engineArgs = append(engineArgs, arg)
	}

	return engineArgs
}
