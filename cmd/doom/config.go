package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/templates"
)

func resolveThemeWithSource(cfg *config.AppConfig) (string, string) {
	if v := strings.TrimSpace(flagTheme); v != "" {
		return v, "flag"
	}
	if v := strings.TrimSpace(os.Getenv("DOOM_THEME")); v != "" {
		return v, "env"
	}
	if cfg != nil && strings.TrimSpace(cfg.Theme) != "" {
		return strings.TrimSpace(cfg.Theme), "config"
	}
	return "classic", "default"
}

func resolveNerdFontsWithSource(cmd *cobra.Command, cfg *config.AppConfig) (bool, string) {
	if (cmd != nil && cmd.Flags().Changed("nerd-fonts")) || flagNerdFonts {
		return flagNerdFonts, "flag"
	}
	if env := strings.TrimSpace(os.Getenv("DOOM_NERD_FONTS")); env != "" {
		if parsed, err := parseBool(env); err == nil {
			return parsed, "env"
		}
	}
	if cfg.HasNerdFonts() {
		return cfg.NerdFonts, "config"
	}
	return false, "default"
}

func resolveNerdFonts(cmd *cobra.Command, cfg *config.AppConfig) bool {
	val, _ := resolveNerdFontsWithSource(cmd, cfg)
	return val
}

func dirSource(flagVal, envKey, cfgVal string) string {
	if strings.TrimSpace(flagVal) != "" {
		return "flag"
	}
	if strings.TrimSpace(os.Getenv(envKey)) != "" {
		return "env"
	}
	if strings.TrimSpace(cfgVal) != "" {
		return "config"
	}
	return "default"
}

func wadsDirsSource(cfg *config.AppConfig) string {
	if strings.TrimSpace(flagWadsDirs) != "" {
		return "flag"
	}
	if strings.TrimSpace(os.Getenv("DOOM_WADS_DIRS")) != "" {
		return "env"
	}
	if cfg != nil && len(cfg.WadsDirs) > 0 {
		return "config"
	}
	return "default"
}

// newConfigCmd creates the CLI configuration management command.
func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI settings and engine configurations",
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Display current CLI settings (theme, nerd-fonts, directories, catalog path)",
		RunE:  runConfigShow,
	}
	configGetCmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Get a CLI configuration setting (theme, nerd-fonts, wads-dir, wads-dirs, bin-dir, soundfonts-dir)",
		RunE:  runConfigGet,
	}
	configSetCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a CLI configuration setting (theme, nerd-fonts, wads-dir, wads-dirs, bin-dir, soundfonts-dir)",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
	configToggleCmd := &cobra.Command{
		Use:   "toggle <key>",
		Short: "Toggle a boolean CLI configuration setting (such as nerd-fonts)",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigToggle,
	}
	configInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Deploy autoexec.cfg and dsda-doom.cfg with backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.DeployConfigsTo(paths, cmdOut(cmd))
		},
	}
	configDiffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare installed system configs against repository templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.DiffConfigs(paths, cmdOut(cmd))
		},
	}
	configSyncCmd := &cobra.Command{
		Use:    "sync",
		Short:  "Sync modified in-game settings back into repository templates",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			return templates.SyncConfigs(paths, ".", cmdOut(cmd))
		},
	}

	configCmd.AddCommand(
		configShowCmd, configGetCmd, configSetCmd, configToggleCmd,
		configInstallCmd, configDiffCmd, configSyncCmd,
	)
	return configCmd
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	paths, cfg, err := loadPathsAndConfig()
	if err != nil {
		return err
	}
	out := cmdOut(cmd)

	theme, themeSrc := resolveThemeWithSource(cfg)
	nerdFonts, nerdSrc := resolveNerdFontsWithSource(cmd, cfg)
	nerdStatus := "disabled (universal rectangular badges)"
	if nerdFonts {
		nerdStatus = "enabled (Powerlevel10k rounded capsules)"
	}

	wadsSrc := dirSource(flagWadsDir, "DOOM_WADS_DIR", cfg.WadsDir)
	binSrc := dirSource(flagBinDir, "DOOM_BIN_DIR", cfg.BinDir)
	sfSrc := dirSource(flagSoundFontsDir, "DOOM_SF_DIR", cfg.SoundFontsDir)

	fmt.Fprintln(out, "=== Doom CLI Configuration ===")
	fmt.Fprintf(out, "  Config File:    %s\n", paths.ConfigFile)
	fmt.Fprintf(out, "  Catalog Dir:    %s\n", paths.CatalogDir)
	fmt.Fprintf(out, "  Theme:          %s (%s)\n", theme, themeSrc)
	fmt.Fprintf(out, "  Nerd Fonts:     %s (%s)\n", nerdStatus, nerdSrc)
	fmt.Fprintf(out, "  WADs Dir:       %s (%s)\n", paths.WadsDir, wadsSrc)
	if len(paths.WadsDirs) > 0 {
		fmt.Fprintf(out, "  WADs Dirs:      %s (%s)\n", strings.Join(paths.WadsDirs, ", "), wadsDirsSource(cfg))
	}
	fmt.Fprintf(out, "  Bin Dir:        %s (%s)\n", paths.BinDir, binSrc)
	fmt.Fprintf(out, "  SoundFonts Dir: %s (%s)\n", paths.SoundFontsDir, sfSrc)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return runConfigShow(cmd, args)
	}

	canonKey, err := canonicalConfigKey(args[0])
	if err != nil {
		return err
	}

	paths, cfg, err := loadPathsAndConfig()
	if err != nil {
		return err
	}
	out := cmdOut(cmd)

	switch canonKey {
	case "theme":
		theme, _ := resolveThemeWithSource(cfg)
		fmt.Fprintln(out, theme)
	case "nerd-fonts":
		fmt.Fprintln(out, resolveNerdFonts(cmd, cfg))
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
	canonKey, err := canonicalConfigKey(args[0])
	if err != nil {
		return err
	}

	val := strings.TrimSpace(args[1])
	paths, cfg, err := loadPathsAndConfig()
	if err != nil {
		return err
	}
	out := cmdOut(cmd)

	switch canonKey {
	case "theme":
		return runThemesSet(cmd, []string{val})
	case "nerd-fonts":
		if strings.EqualFold(val, "toggle") {
			cfg.SetNerdFonts(!cfg.NerdFonts)
		} else {
			parsed, err := parseBool(val)
			if err != nil {
				return fmt.Errorf("invalid value for %s: %w", canonKey, err)
			}
			cfg.SetNerdFonts(parsed)
		}
		if err := config.SaveConfig(paths, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		status := "disabled"
		if cfg.NerdFonts {
			status = "enabled"
		}
		fmt.Fprintf(out, "✓ Set nerd_fonts to %t (%s) in %s\n", cfg.NerdFonts, status, paths.ConfigFile)
		return nil
	case "wads-dir":
		cfg.WadsDir = val
		if err := config.SaveConfig(paths, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Fprintf(out, "✓ Set wads_dir to %s in %s\n", val, paths.ConfigFile)
		return nil
	case "wads-dirs":
		cfg.WadsDirs = config.ParsePathList(val)
		if err := config.SaveConfig(paths, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Fprintf(out, "✓ Set wads_dirs to [%s] in %s\n", strings.Join(cfg.WadsDirs, ", "), paths.ConfigFile)
		return nil
	case "bin-dir":
		cfg.BinDir = val
		if err := config.SaveConfig(paths, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Fprintf(out, "✓ Set bin_dir to %s in %s\n", val, paths.ConfigFile)
		return nil
	case "soundfonts-dir":
		cfg.SoundFontsDir = val
		if err := config.SaveConfig(paths, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Fprintf(out, "✓ Set soundfonts_dir to %s in %s\n", val, paths.ConfigFile)
		return nil
	}
	return nil
}

func runConfigToggle(cmd *cobra.Command, args []string) error {
	canonKey, err := canonicalConfigKey(args[0])
	if err != nil {
		return err
	}
	if canonKey != "nerd-fonts" {
		return fmt.Errorf("cannot toggle non-boolean config key %q (valid toggleable keys: nerd-fonts)", args[0])
	}
	return runConfigSet(cmd, []string{canonKey, "toggle"})
}

func canonicalConfigKey(rawKey string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawKey))
	switch key {
	case "theme":
		return "theme", nil
	case "nerd-fonts", "nerd_fonts":
		return "nerd-fonts", nil
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
			"unknown config key %q (valid keys: theme, nerd-fonts, wads-dir, wads-dirs, bin-dir, soundfonts-dir)",
			rawKey,
		)
	}
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "t", "1", "on", "yes", "y", "enable", "enabled":
		return true, nil
	case "false", "f", "0", "off", "no", "n", "disable", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("expected boolean (true/false, on/off, yes/no), got %q", s)
	}
}
