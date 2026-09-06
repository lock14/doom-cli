package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/preset"
)

// newPresetsCmd creates the preset catalog and custom WAD management command.
func newPresetsCmd() *cobra.Command {
	presetsCmd := &cobra.Command{
		Use:   "presets",
		Short: "Inspect, configure, and manage preset catalog and custom WADs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPresetsList(cmd, args)
		},
	}

	presetsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all presets (built-in and custom) with engine assignments",
		RunE:  runPresetsList,
	}

	presetsShowCmd := &cobra.Command{
		Use:   "show <preset_name>",
		Short: "Display detailed configuration and file status for a preset",
		Args:  cobra.ExactArgs(1),
		RunE:  runPresetsShow,
	}

	var addEngine, addIWAD, addCategory, addDesc, addAuthor, addRelease, addArgs string
	var addFiles []string
	presetsAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a custom WAD or preset to your personal library",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("preset name cannot be empty")
			}

			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			cat, err := getCatalog()
			if err != nil {
				return err
			}

			if !isCustomPreset(name, cfg) && cat.Find(name) != nil {
				return fmt.Errorf(
					"preset %q is a built-in preset; use 'doom presets config %q' to adjust launch options",
					name, name,
				)
			}

			p := preset.Preset{
				Name:           name,
				Engine:         addEngine,
				IWAD:           addIWAD,
				Mappacks:       addFiles,
				Category:       addCategory,
				Description:    addDesc,
				Author:         addAuthor,
				ReleaseDate:    addRelease,
				AdditionalArgs: addArgs,
				Custom:         true,
			}

			cfg.AddPreset(p)
			if err := config.SaveConfig(paths, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("✓ Successfully added custom preset %q to %s\n", name, paths.ConfigFile)
			return nil
		},
	}
	presetsAddCmd.Flags().StringVar(&addEngine, "engine", "uzdoom", "Default engine for this preset")
	presetsAddCmd.Flags().StringVar(&addIWAD, "iwad", "DOOM2.WAD", "Base IWAD required by this preset")
	presetsAddCmd.Flags().StringSliceVar(&addFiles, "files", nil, "Comma-separated list of PWAD and DEH files")
	presetsAddCmd.Flags().StringVar(&addCategory, "category", "Custom", "Category or collection name")
	presetsAddCmd.Flags().StringVar(&addDesc, "desc", "", "Short description of the mapset")
	presetsAddCmd.Flags().StringVar(&addAuthor, "author", "", "Map author / team")
	presetsAddCmd.Flags().StringVar(&addRelease, "release", "", "Release year / date")
	presetsAddCmd.Flags().StringVar(&addArgs, "args", "", "Default extra launch arguments")

	var cfgEngine, cfgIWAD, cfgArgs string
	var cfgFiles []string
	var cfgReset bool
	presetsConfigCmd := &cobra.Command{
		Use:   "config <preset_name>",
		Short: "Configure per-WAD launch options (engine, extra args, files)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cfgReset {
				if cfg.RemoveLaunchOptions(name) {
					if err := config.SaveConfig(paths, cfg); err != nil {
						return fmt.Errorf("saving config: %w", err)
					}
					fmt.Printf("✓ Reset custom launch options for %q in %s\n", name, paths.ConfigFile)
					return nil
				}
				fmt.Printf("ℹ No custom launch options configured for %q\n", name)
				return nil
			}

			var opts preset.WadLaunchOptions
			if existing, ok := cfg.LaunchOptions[name]; ok {
				opts = existing
			}

			changed := false
			if cfgEngine != "" {
				opts.Engine = cfgEngine
				changed = true
			}
			if cfgIWAD != "" {
				opts.IWAD = cfgIWAD
				changed = true
			}
			if cfgArgs != "" {
				opts.AdditionalArgs = cfgArgs
				changed = true
			}
			if len(cfgFiles) > 0 {
				opts.ExtraFiles = cfgFiles
				changed = true
			}

			if !changed {
				return fmt.Errorf("no option flags provided (use --engine, --args, --iwad, --files, or --reset)")
			}

			cfg.SetLaunchOptions(name, opts)
			if err := config.SaveConfig(paths, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("✓ Configured launch options for %q in %s\n", name, paths.ConfigFile)
			return nil
		},
	}
	presetsConfigCmd.Flags().StringVar(&cfgEngine, "engine", "", "Preferred engine override for this preset")
	presetsConfigCmd.Flags().StringVar(&cfgIWAD, "iwad", "", "Custom IWAD override")
	presetsConfigCmd.Flags().StringVar(&cfgArgs, "args", "", "Additional launch arguments (e.g. -skill 4 -warp 01)")
	presetsConfigCmd.Flags().StringSliceVar(&cfgFiles, "files", nil, "Extra PWAD files to load with this preset")
	presetsConfigCmd.Flags().BoolVar(&cfgReset, "reset", false, "Clear all custom launch options for this preset")

	presetsRemoveCmd := &cobra.Command{
		Use:   "remove <preset_name>",
		Short: "Remove a custom preset or reset overrides for a built-in preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cfg.RemovePreset(name) {
				cfg.RemoveLaunchOptions(name)
				if err := config.SaveConfig(paths, cfg); err != nil {
					return fmt.Errorf("saving config: %w", err)
				}
				fmt.Printf("✓ Removed custom preset %q from %s\n", name, paths.ConfigFile)
				return nil
			}

			if cfg.RemoveLaunchOptions(name) {
				if err := config.SaveConfig(paths, cfg); err != nil {
					return fmt.Errorf("saving config: %w", err)
				}
				fmt.Printf(
					"✓ Cleared launch option overrides for built-in preset %q in %s\n",
					name,
					paths.ConfigFile,
				)
				return nil
			}

			return fmt.Errorf("preset %q not found in custom presets or overrides", name)
		},
	}

	presetsBuildCmd := &cobra.Command{
		Use:   "build",
		Short: "Synchronize data/presets.json into README.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			if err := preset.SyncReadme(root); err != nil {
				return err
			}
			fmt.Println("✓ Successfully synchronized README.md with data/presets.json")
			return nil
		},
	}

	presetsCmd.AddCommand(
		presetsListCmd, presetsShowCmd, presetsAddCmd,
		presetsConfigCmd, presetsRemoveCmd, presetsBuildCmd,
	)
	return presetsCmd
}

func isCustomPreset(name string, cfg *config.AppConfig) bool {
	if cfg == nil {
		return false
	}
	for _, p := range cfg.Presets {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

func runPresetsList(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}
	paths := getPaths()
	cfg, _ := config.LoadConfig(paths)

	fmt.Println("=== Available Doom Presets ===")
	for i, p := range cat.Presets {
		tag := formatEngineBadge(p.Engine)
		statusFlag := ""
		if p.Custom {
			statusFlag = " [Custom]"
		} else if cfg != nil && cfg.LaunchOptions != nil {
			if _, ok := cfg.LaunchOptions[p.Name]; ok {
				statusFlag = " [Overridden]"
			}
		}
		fmt.Printf("%2d. %-30s %s%s - %s\n", i+1, p.Name, tag, statusFlag, p.Description)
	}
	return nil
}

func runPresetsShow(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}
	name := args[0]
	p := cat.Find(name)
	if p == nil {
		return fmt.Errorf("preset %q not found", name)
	}

	paths := getPaths()
	cfg, _ := config.LoadConfig(paths)

	fmt.Printf("=== Preset: %s ===\n", p.Name)
	if p.Custom {
		fmt.Printf("  Type:          Custom WAD\n")
	}
	if p.Author != "" {
		fmt.Printf("  Author:        %s\n", p.Author)
	}
	if p.ReleaseDate != "" {
		fmt.Printf("  Released:      %s\n", p.ReleaseDate)
	}
	if p.Category != "" {
		fmt.Printf("  Category:      %s\n", p.Category)
	}
	if p.Compatibility != "" {
		fmt.Printf("  Compatibility: %s\n", p.Compatibility)
	}
	if p.Description != "" {
		fmt.Printf("  Description:   %s\n", p.Description)
	}

	engStr := p.Engine
	if cat.Engines != nil {
		if engCfg, ok := cat.Engines[p.Engine]; ok && engCfg.Description != "" {
			engStr = engCfg.Description
		}
	}
	engineOverridden := ""
	if cfg != nil && cfg.LaunchOptions != nil {
		if opt, ok := cfg.LaunchOptions[p.Name]; ok && opt.Engine != "" {
			engineOverridden = " [Overridden in user config]"
		}
	}
	fmt.Printf("  Engine:        %s%s\n", engStr, engineOverridden)

	iwadPath, iwadFound := preset.ResolveFile(paths.WadsDir, p.IWAD)
	iwadStatus := "[✗ Missing]"
	if iwadFound {
		iwadStatus = fmt.Sprintf("[✓ Found: %s]", iwadPath)
	}
	fmt.Printf("  Base IWAD:     %s %s\n", p.IWAD, iwadStatus)

	if len(p.Mappacks) > 0 {
		fmt.Println("  Files:")
		for _, m := range p.Mappacks {
			fpath, fFound := preset.ResolveFile(paths.WadsDir, m)
			fStatus := "[✗ Missing]"
			if fFound {
				fStatus = fmt.Sprintf("[✓ Found: %s]", fpath)
			} else if strings.EqualFold(m, "idkfa 2024.wad") {
				fStatus = "[Optional]"
			}
			fmt.Printf("    - %-20s %s\n", m, fStatus)
		}
	}

	if p.AdditionalArgs != "" {
		fmt.Printf("  Launch Args:   %s\n", p.AdditionalArgs)
	}

	return nil
}

func formatEngineBadge(engine string) string {
	switch strings.ToLower(engine) {
	case "dsda-doom", "dsda":
		return "[DSDA    ]"
	case "uzdoom", "zdoom":
		return "[UZDoom  ]"
	default:
		tag := strings.ToUpper(engine)
		runes := []rune(tag)
		if len(runes) > 6 {
			tag = string(runes[:6])
		}
		return fmt.Sprintf("[%-8s]", tag)
	}
}
