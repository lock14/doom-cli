package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/tui"
)

func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage doom-cli user settings and TUI color themes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
				paths, cfg, cat, err := loadAppState()
				if err != nil {
					return err
				}
				theme, nerdFonts, launcherOpts := buildLauncherOptions(
					cmd, paths, cfg, nil, tui.InitialModeConfig,
				)
				_, err = tui.RunInteractiveLauncherWithOptions(
					cat, paths.WadRoots(), theme, nerdFonts, launcherOpts,
				)
				return err
			}
			return runConfigShow(cmd, args)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Display effective doom-cli configuration and setting sources",
		RunE:  runConfigShow,
	}

	getCmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Print the effective value of one or all doom-cli settings",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runConfigGet,
	}

	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a persistent doom-cli setting (theme, nerd-fonts, wads-dir, wads-dirs, bin-dir, soundfonts-dir)",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}

	toggleCmd := &cobra.Command{
		Use:   "toggle <key>",
		Short: "Toggle a boolean doom-cli setting (such as nerd-fonts)",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigToggle,
	}

	themesCmd := &cobra.Command{
		Use:   "themes",
		Short: "List all available built-in and custom TUI color themes",
		RunE:  runConfigThemes,
	}

	configCmd.AddCommand(showCmd, getCmd, setCmd, toggleCmd, themesCmd)
	return configCmd
}

func runConfigShow(cmd *cobra.Command, _ []string) error {
	paths, cfg, _, err := loadAppState()
	if err != nil {
		return err
	}

	effectiveTheme, themeSource := resolveThemeWithSource(cfg)
	effectiveNerdFonts, nfSource := resolveNerdFontsWithSource(cmd, cfg)
	wadsDirSrc := dirSource(flagWadsDir, "DOOM_WADS_DIR", cfg.WadsDir)
	wadsDirsSrc := wadsDirsSource(cfg)
	binDirSrc := dirSource("", "DOOM_BIN_DIR", cfg.BinDir)
	sfDirSrc := dirSource("", "DOOM_SF_DIR", cfg.SoundFontsDir)

	nfState := "disabled (universal rectangular badges)"
	if effectiveNerdFonts {
		nfState = "enabled (Powerlevel10k rounded capsules)"
	}

	extraWadsDisplay := "(none)"
	if len(paths.WadsDirs) > 0 {
		extraWadsDisplay = strings.Join(paths.WadsDirs, ", ")
	}

	out := cmdOut(cmd)
	fmt.Fprintln(out, "=== Doom CLI Configuration ===")
	fmt.Fprintf(out, "Config File:    %s\n", paths.ConfigFile)
	fmt.Fprintf(out, "Catalog Dir:    %s\n", paths.CatalogDir)
	fmt.Fprintf(out, "Themes Dir:     %s\n\n", paths.ThemesDir)
	fmt.Fprintln(out, "Effective Settings:")
	fmt.Fprintf(out, "  Theme:          %s (%s)\n", effectiveTheme, themeSource)
	fmt.Fprintf(out, "  Nerd Fonts:     %s (%s)\n", nfState, nfSource)
	fmt.Fprintf(out, "  WADs Dir:       %s (%s)\n", paths.WadsDir, wadsDirSrc)
	fmt.Fprintf(out, "  WADs Dirs:      %s (%s)\n", extraWadsDisplay, wadsDirsSrc)
	fmt.Fprintf(out, "  Binaries Dir:   %s (%s)\n", paths.BinDir, binDirSrc)
	fmt.Fprintf(out, "  SoundFonts Dir: %s (%s)\n", paths.SoundFontsDir, sfDirSrc)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	paths, cfg, _, err := loadAppState()
	if err != nil {
		return err
	}

	effectiveTheme, _ := resolveThemeWithSource(cfg)
	effectiveNerdFonts, _ := resolveNerdFontsWithSource(cmd, cfg)
	out := cmdOut(cmd)

	if len(args) == 0 {
		fmt.Fprintf(out, "theme=%s\n", effectiveTheme)
		fmt.Fprintf(out, "nerd-fonts=%t\n", effectiveNerdFonts)
		fmt.Fprintf(out, "wads-dir=%s\n", paths.WadsDir)
		fmt.Fprintf(out, "wads-dirs=%s\n", strings.Join(paths.WadsDirs, ","))
		fmt.Fprintf(out, "bin-dir=%s\n", paths.BinDir)
		fmt.Fprintf(out, "soundfonts-dir=%s\n", paths.SoundFontsDir)
		return nil
	}

	key, err := canonicalConfigKey(args[0])
	if err != nil {
		return err
	}

	switch key {
	case "theme":
		fmt.Fprintln(out, effectiveTheme)
	case "nerd-fonts":
		fmt.Fprintln(out, effectiveNerdFonts)
	case "wads-dir":
		fmt.Fprintln(out, paths.WadsDir)
	case "wads-dirs":
		fmt.Fprintln(out, strings.Join(paths.WadsDirs, ","))
	case "bin-dir":
		fmt.Fprintln(out, paths.BinDir)
	case "soundfonts-dir":
		fmt.Fprintln(out, paths.SoundFontsDir)
	}
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, err := canonicalConfigKey(args[0])
	if err != nil {
		return err
	}
	rawVal := args[1]

	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return err
	}

	out := cmdOut(cmd)
	switch key {
	case "nerd-fonts":
		b, err := parseBool(rawVal)
		if err != nil {
			return fmt.Errorf("invalid boolean value %q for nerd-fonts (expected on/off or true/false)", rawVal)
		}
		cfg.SetNerdFonts(b)
		if err := config.SaveConfig(paths, cfg); err != nil {
			return err
		}
		state := "disabled (universal rectangular badges)"
		if b {
			state = "enabled (Powerlevel10k rounded capsules)"
		}
		fmt.Fprintf(out, "✓ Set nerd-fonts = %t (%s) in %s\n", b, state, paths.ConfigFile)
		return nil

	case "theme":
		val := strings.ToLower(strings.TrimSpace(rawVal))
		if val == "" {
			return fmt.Errorf("theme name cannot be empty")
		}
		if _, ok := tui.GetBuiltinTheme(val); !ok {
			customPath := filepath.Join(paths.ThemesDir, val+".json")
			if _, statErr := os.Stat(customPath); statErr != nil {
				if _, pathErr := os.Stat(rawVal); pathErr != nil {
					return fmt.Errorf("unknown theme %q. Run 'doom config themes' to see available themes", val)
				}
			}
		}
		cfg.Theme = val
		if err := config.SaveConfig(paths, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "✓ Set theme = %q in %s\n", val, paths.ConfigFile)
		return nil

	case "wads-dir":
		val := strings.TrimSpace(rawVal)
		cfg.WadsDir = val
		if err := config.SaveConfig(paths, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "✓ Set wads-dir = %q in %s\n", val, paths.ConfigFile)
		return nil

	case "wads-dirs":
		dirs := config.ParsePathList(rawVal)
		cfg.WadsDirs = dirs
		if err := config.SaveConfig(paths, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "✓ Set wads-dirs = %q in %s\n", strings.Join(dirs, ", "), paths.ConfigFile)
		return nil

	case "bin-dir":
		val := strings.TrimSpace(rawVal)
		cfg.BinDir = val
		if err := config.SaveConfig(paths, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "✓ Set bin-dir = %q in %s\n", val, paths.ConfigFile)
		return nil

	case "soundfonts-dir":
		val := strings.TrimSpace(rawVal)
		cfg.SoundFontsDir = val
		if err := config.SaveConfig(paths, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "✓ Set soundfonts-dir = %q in %s\n", val, paths.ConfigFile)
		return nil
	}

	return nil
}

func runConfigToggle(cmd *cobra.Command, args []string) error {
	key, err := canonicalConfigKey(args[0])
	if err != nil {
		return err
	}
	if key != "nerd-fonts" {
		return fmt.Errorf(
			"setting %q is not a boolean toggle (use 'doom config set %s <value>')",
			key, key,
		)
	}

	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return err
	}

	next := !cfg.NerdFonts
	cfg.SetNerdFonts(next)
	if err := config.SaveConfig(paths, cfg); err != nil {
		return err
	}

	state := "disabled (universal rectangular badges)"
	if next {
		state = "enabled (Powerlevel10k rounded capsules)"
	}
	fmt.Fprintf(cmdOut(cmd), "✓ Toggled nerd-fonts = %t (%s) in %s\n", next, state, paths.ConfigFile)
	return nil
}

func runConfigThemes(cmd *cobra.Command, _ []string) error {
	paths, cfg, _, err := loadAppState()
	if err != nil {
		return err
	}
	out := cmdOut(cmd)

	activeTheme := tui.ResolveTheme(flagTheme, os.Getenv("DOOM_THEME"), cfg.Theme, paths.ThemesDir)
	nerdFonts, _ := resolveNerdFontsWithSource(cmd, cfg)

	fmt.Fprintln(out, "=== Available Doom Themes ===")
	fmt.Fprintln(out)

	themes := append(tui.ListBuiltinThemes(), tui.ListCustomThemes(paths.ThemesDir)...)
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

		fmt.Fprintf(out, "%s%-11s [%-9s] %-42s %s\n", marker, t.Name, t.Type, t.Description, sample)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Active theme: %s\n", activeTheme.Name)
	fmt.Fprintf(out, "Run 'doom config set theme <name>' to save a default theme to %s\n", paths.ConfigFile)
	return nil
}

func canonicalConfigKey(raw string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(raw))
	switch k {
	case "nerd-fonts", "nerd_fonts":
		return "nerd-fonts", nil
	case "theme":
		return "theme", nil
	case "wads-dir", "wads_dir":
		return "wads-dir", nil
	case "wads-dirs", "wads_dirs":
		return "wads-dirs", nil
	case "bin-dir", "bin_dir":
		return "bin-dir", nil
	case "soundfonts-dir", "soundfonts_dir":
		return "soundfonts-dir", nil
	default:
		return "", fmt.Errorf(
			"unknown setting %q (valid keys: theme, nerd-fonts, wads-dir, wads-dirs, bin-dir, soundfonts-dir)",
			raw,
		)
	}
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "t", "1", "on", "yes", "y", "enable", "enabled":
		return true, nil
	case "false", "f", "0", "off", "no", "n", "disable", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", raw)
	}
}

func resolveThemeWithSource(cfg *config.AppConfig) (string, string) {
	if s := strings.TrimSpace(flagTheme); s != "" {
		return s, "flag"
	}
	if s := strings.TrimSpace(os.Getenv("DOOM_THEME")); s != "" {
		return s, "env"
	}
	if cfg != nil && strings.TrimSpace(cfg.Theme) != "" {
		return strings.TrimSpace(cfg.Theme), "config"
	}
	return tui.ClassicTheme.Name, "default"
}

func resolveNerdFontsWithSource(cmd *cobra.Command, cfg *config.AppConfig) (bool, string) {
	if flagNerdFonts || (cmd != nil && cmd.Flags().Changed("nerd-fonts")) {
		return flagNerdFonts, "flag"
	}
	if envVal := strings.TrimSpace(os.Getenv("DOOM_NERD_FONTS")); envVal != "" {
		if b, err := parseBool(envVal); err == nil {
			return b, "env"
		}
	}
	if cfg != nil && cfg.HasNerdFonts() {
		return cfg.NerdFonts, "config"
	}
	return false, "default"
}
