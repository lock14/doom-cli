package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/templates"
)

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
		Use:   "sync",
		Short: "Sync modified in-game settings back into repository templates",
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

	theme := cfg.Theme
	if theme == "" {
		theme = "default"
	}

	nerdStatus := "disabled (universal rectangular badges)"
	if cfg.NerdFonts {
		nerdStatus = "enabled (Powerlevel10k rounded capsules)"
	}

	fmt.Fprintln(out, "=== Doom CLI Configuration ===")
	fmt.Fprintf(out, "  Config File:    %s\n", paths.ConfigFile)
	fmt.Fprintf(out, "  Catalog Dir:    %s\n", paths.CatalogDir)
	fmt.Fprintf(out, "  Theme:          %s\n", theme)
	fmt.Fprintf(out, "  Nerd Fonts:     %s\n", nerdStatus)
	if cfg.WadsDir != "" {
		fmt.Fprintf(out, "  WADs Dir:       %s\n", cfg.WadsDir)
	} else {
		fmt.Fprintf(out, "  WADs Dir:       %s (default)\n", paths.WadsDir)
	}
	if len(paths.WadsDirs) > 0 {
		fmt.Fprintf(out, "  WADs Dirs:      %s\n", strings.Join(paths.WadsDirs, ", "))
	}
	if cfg.BinDir != "" {
		fmt.Fprintf(out, "  Bin Dir:        %s\n", cfg.BinDir)
	} else {
		fmt.Fprintf(out, "  Bin Dir:        %s (default)\n", paths.BinDir)
	}
	if cfg.SoundFontsDir != "" {
		fmt.Fprintf(out, "  SoundFonts Dir: %s\n", cfg.SoundFontsDir)
	} else {
		fmt.Fprintf(out, "  SoundFonts Dir: %s (default)\n", paths.SoundFontsDir)
	}
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
		fmt.Fprintln(out, cfg.Theme)
	case "nerd-fonts":
		fmt.Fprintln(out, cfg.NerdFonts)
	case "wads-dir":
		if cfg.WadsDir != "" {
			fmt.Fprintln(out, cfg.WadsDir)
		} else {
			fmt.Fprintln(out, paths.WadsDir)
		}
	case "wads-dirs":
		fmt.Fprintln(out, strings.Join(paths.WadsDirs, ","))
	case "bin-dir":
		if cfg.BinDir != "" {
			fmt.Fprintln(out, cfg.BinDir)
		} else {
			fmt.Fprintln(out, paths.BinDir)
		}
	case "soundfonts-dir":
		if cfg.SoundFontsDir != "" {
			fmt.Fprintln(out, cfg.SoundFontsDir)
		} else {
			fmt.Fprintln(out, paths.SoundFontsDir)
		}
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
			cfg.NerdFonts = !cfg.NerdFonts
		} else {
			parsed, err := parseBool(val)
			if err != nil {
				return fmt.Errorf("invalid value for %s: %w", canonKey, err)
			}
			cfg.NerdFonts = parsed
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
	case "nerd-fonts", "nerd_fonts", "nerdfonts":
		return "nerd-fonts", nil
	case "wads-dir", "wads_dir", "wadsdir":
		return "wads-dir", nil
	case "wads-dirs", "wads_dirs", "wadsdirs":
		return "wads-dirs", nil
	case "bin-dir", "bin_dir", "bindir":
		return "bin-dir", nil
	case "soundfonts-dir", "soundfonts_dir", "soundfont-dir", "soundfont_dir", "sf-dir", "sf_dir":
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
