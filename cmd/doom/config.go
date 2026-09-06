package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/templates"
)

func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI settings and engine configurations",
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Display current CLI settings (theme, nerd-fonts, config path)",
		RunE:  runConfigShow,
	}
	configGetCmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Get a CLI configuration setting (theme, nerd-fonts)",
		RunE:  runConfigGet,
	}
	configSetCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a CLI configuration setting (theme, nerd-fonts)",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
	configToggleCmd := &cobra.Command{
		Use:   "toggle <key>",
		Short: "Toggle a boolean CLI configuration setting (e.g. nerd-fonts)",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigToggle,
	}
	configInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Deploy autoexec.cfg and dsda-doom.cfg with backups",
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

	configCmd.AddCommand(
		configShowCmd, configGetCmd, configSetCmd, configToggleCmd,
		configInstallCmd, configDiffCmd, configSyncCmd,
	)
	return configCmd
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	theme := cfg.Theme
	if theme == "" {
		theme = "default"
	}

	nerdStatus := "disabled (universal rectangular badges)"
	if cfg.NerdFonts {
		nerdStatus = "enabled (Powerlevel10k rounded capsules)"
	}

	fmt.Println("=== Doom CLI Configuration ===")
	fmt.Printf("  Config File: %s\n", paths.ConfigFile)
	fmt.Printf("  Theme:       %s\n", theme)
	fmt.Printf("  Nerd Fonts:  %s\n", nerdStatus)

	if len(cfg.Engines) > 0 {
		fmt.Printf("\nCustom Engines (%d):\n", len(cfg.Engines))
		for name, eng := range cfg.Engines {
			binTarget := eng.Binary
			if binTarget == "" {
				binTarget = name
			}
			fmt.Printf("  • %-12s [%-5s] (binary: %s)\n", name, eng.EffectiveArgsStyle(), binTarget)
		}
	}

	if len(cfg.Presets) > 0 {
		fmt.Printf("\nCustom Presets (%d):\n", len(cfg.Presets))
		for _, p := range cfg.Presets {
			fmt.Printf("  • %-20s [%s] (IWAD: %s)\n", p.Name, p.Engine, p.IWAD)
		}
	}

	if len(cfg.LaunchOptions) > 0 {
		fmt.Printf("\nLaunch Options Overrides (%d):\n", len(cfg.LaunchOptions))
		for name, opt := range cfg.LaunchOptions {
			var details []string
			if opt.Engine != "" {
				details = append(details, "engine: "+opt.Engine)
			}
			if opt.IWAD != "" {
				details = append(details, "iwad: "+opt.IWAD)
			}
			if opt.AdditionalArgs != "" {
				details = append(details, "args: "+opt.AdditionalArgs)
			}
			if len(opt.ExtraFiles) > 0 {
				details = append(details, fmt.Sprintf("extra files: %d", len(opt.ExtraFiles)))
			}
			fmt.Printf("  • %-20s (%s)\n", name, strings.Join(details, ", "))
		}
	}
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return runConfigShow(cmd, args)
	}

	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	key := strings.ToLower(strings.TrimSpace(args[0]))
	switch key {
	case "theme":
		fmt.Println(cfg.Theme)
		return nil
	case "nerd-fonts", "nerd_fonts", "nerdfonts":
		fmt.Println(cfg.NerdFonts)
		return nil
	default:
		return fmt.Errorf("unknown config key %q (valid keys: theme, nerd-fonts)", key)
	}
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := strings.ToLower(strings.TrimSpace(args[0]))
	val := strings.TrimSpace(args[1])
	paths := getPaths()

	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	switch key {
	case "theme":
		return runThemesSet(cmd, []string{val})
	case "nerd-fonts", "nerd_fonts", "nerdfonts":
		if strings.EqualFold(val, "toggle") {
			cfg.NerdFonts = !cfg.NerdFonts
		} else {
			parsed, err := parseBool(val)
			if err != nil {
				return fmt.Errorf("invalid value for %s: %w", key, err)
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
		fmt.Printf("✓ Set nerd_fonts to %t (%s) in %s\n", cfg.NerdFonts, status, paths.ConfigFile)
		return nil
	default:
		return fmt.Errorf("unknown config key %q (valid keys: theme, nerd-fonts)", key)
	}
}

func runConfigToggle(cmd *cobra.Command, args []string) error {
	key := strings.ToLower(strings.TrimSpace(args[0]))
	switch key {
	case "nerd-fonts", "nerd_fonts", "nerdfonts":
		return runConfigSet(cmd, []string{key, "toggle"})
	default:
		return fmt.Errorf("cannot toggle non-boolean config key %q (valid toggleable keys: nerd-fonts)", key)
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
