package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

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

	var (
		addID, addEngine, addIWAD, addCategory  string
		addDesc, addAuthor, addRelease, addArgs string
		addSourceURL, addSourceSHA256           string
		addFiles, addOptionalFiles              []string
	)
	presetsAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a custom WAD or preset to your personal catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("preset name cannot be empty")
			}
			if addSourceSHA256 != "" && addSourceURL == "" {
				return fmt.Errorf("--source-sha256 requires --source-url")
			}

			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			writeDir, err := userCatalogWriteDir(paths)
			if err != nil {
				return err
			}
			curated, err := preset.LoadCuratedCatalog()
			if err != nil {
				return err
			}

			var files []preset.FileSpec
			for _, f := range addFiles {
				if trimmed := strings.TrimSpace(f); trimmed != "" {
					files = append(files, preset.FileSpec{Name: trimmed})
				}
			}
			for _, f := range addOptionalFiles {
				if trimmed := strings.TrimSpace(f); trimmed != "" {
					files = append(files, preset.FileSpec{Name: trimmed, Optional: true})
				}
			}

			var extraArgs []string
			if strings.TrimSpace(addArgs) != "" {
				extraArgs = strings.Fields(addArgs)
			}

			var sources []preset.Source
			if strings.TrimSpace(addSourceURL) != "" {
				sources = append(sources, preset.Source{
					URL:    strings.TrimSpace(addSourceURL),
					SHA256: strings.TrimSpace(addSourceSHA256),
				})
			}

			p := preset.Preset{
				ID:          addID,
				Name:        name,
				Engine:      addEngine,
				IWAD:        addIWAD,
				Files:       files,
				Args:        extraArgs,
				Category:    addCategory,
				Description: addDesc,
				Author:      addAuthor,
				ReleaseDate: addRelease,
				Sources:     sources,
			}

			outPath, err := preset.AddPreset(writeDir, curated, cat, p)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmdOut(cmd), "✓ Successfully added custom preset %q to %s\n", p.Name, outPath)
			return nil
		},
	}
	presetsAddCmd.Flags().StringVar(&addID, "id", "", "Explicit slug ID for this preset (defaults to slugified name)")
	presetsAddCmd.Flags().StringVar(&addEngine, "engine", "uzdoom", "Default engine for this preset")
	presetsAddCmd.Flags().StringVar(&addIWAD, "iwad", "DOOM2.WAD", "Base IWAD required by this preset")
	presetsAddCmd.Flags().StringSliceVar(&addFiles, "files", nil, "Comma-separated list of required PWAD/DEH files")
	presetsAddCmd.Flags().StringSliceVar(
		&addOptionalFiles, "optional-files", nil, "Comma-separated list of optional PWAD files",
	)
	presetsAddCmd.Flags().StringVar(&addCategory, "category", "Custom", "Category or collection name")
	presetsAddCmd.Flags().StringVar(&addDesc, "description", "", "Short description of the mapset")
	presetsAddCmd.Flags().StringVar(&addDesc, "desc", "", "Short description of the mapset")
	_ = presetsAddCmd.Flags().MarkHidden("desc")
	presetsAddCmd.Flags().StringVar(&addAuthor, "author", "", "Map author / team")
	presetsAddCmd.Flags().StringVar(&addRelease, "release", "", "Release year / date")
	presetsAddCmd.Flags().StringVar(&addArgs, "args", "", "Default extra launch arguments")
	presetsAddCmd.Flags().StringVar(&addSourceURL, "source-url", "", "HTTPS download URL for the WAD archive")
	presetsAddCmd.Flags().StringVar(&addSourceURL, "download-url", "", "HTTPS download URL for the WAD archive")
	_ = presetsAddCmd.Flags().MarkHidden("download-url")
	presetsAddCmd.Flags().StringVar(
		&addSourceSHA256, "source-sha256", "", "Expected SHA-256 hex digest for --source-url",
	)

	var cfgEngine, cfgIWAD, cfgArgs string
	var cfgFiles []string
	var cfgReset bool
	presetsConfigCmd := &cobra.Command{
		Use:   "config <preset_name>",
		Short: "Configure per-preset overrides (engine, iwad, extra args, extra files)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			writeDir, err := userCatalogWriteDir(paths)
			if err != nil {
				return err
			}

			out := cmdOut(cmd)
			if cfgReset {
				target, removed, err := preset.ResetPresetOverride(writeDir, cat, name)
				if err != nil {
					return err
				}
				if removed {
					fmt.Fprintf(out, "✓ Reset custom launch options for %q\n", target.Name)
					return nil
				}
				fmt.Fprintf(out, "ℹ No custom launch options configured for %q\n", target.Name)
				return nil
			}

			var req preset.OverridePatchRequest
			changed := false
			if cmd.Flags().Changed("engine") || cfgEngine != "" {
				v := cfgEngine
				req.Engine = &v
				changed = true
			}
			if cmd.Flags().Changed("iwad") || cfgIWAD != "" {
				v := cfgIWAD
				req.IWAD = &v
				changed = true
			}
			if cmd.Flags().Changed("args") || cfgArgs != "" {
				v := strings.Fields(cfgArgs)
				req.AppendArgs = &v
				changed = true
			}
			if cmd.Flags().Changed("files") || len(cfgFiles) > 0 {
				var specs []preset.FileSpec
				for _, f := range cfgFiles {
					if trimmed := strings.TrimSpace(f); trimmed != "" {
						specs = append(specs, preset.FileSpec{Name: trimmed})
					}
				}
				req.AppendFiles = &specs
				changed = true
			}
			if !changed {
				return fmt.Errorf("no option flags provided (use --engine, --args, --iwad, --files, or --reset)")
			}

			outPath, target, err := preset.ConfigPreset(writeDir, cat, name, req)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "✓ Configured launch options for %q in %s\n", target.Name, outPath)
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
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			writeDir, err := userCatalogWriteDir(paths)
			if err != nil {
				return err
			}
			curated, err := preset.LoadCuratedCatalog()
			if err != nil {
				return err
			}

			removed, err := preset.RemovePreset(writeDir, curated, cat, name)
			if err != nil {
				return err
			}
			out := cmdOut(cmd)
			if removed.IsCustom() {
				fmt.Fprintf(out, "✓ Removed custom preset %q (%s)\n", removed.Name, removed.ID)
				return nil
			}
			fmt.Fprintf(
				out,
				"✓ Cleared launch option overrides for built-in preset %q (%s)\n",
				removed.Name,
				removed.ID,
			)
			return nil
		},
	}

	presetsBuildCmd := &cobra.Command{
		Use:   "build [root]",
		Short: "Synchronize catalog/*.json into README.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			if err := preset.SyncReadme(root); err != nil {
				return err
			}
			fmt.Fprintln(cmdOut(cmd), "✓ Successfully synchronized README.md with catalog/*.json")
			return nil
		},
	}

	presetsLintCmd := &cobra.Command{
		Use:   "lint [root]",
		Short: "Validate curated and user catalog entities, schema invariants, and README parity",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			paths := getPaths()
			userDir := paths.CatalogDir
			if flagPresetsFile != "" {
				userDir = flagPresetsFile
			}
			diags, err := preset.LintCatalog(root, userDir)
			if err != nil {
				return err
			}
			out := cmdOut(cmd)
			for _, d := range diags {
				fmt.Fprintf(out, "[%s] %s\n", strings.ToUpper(d.Severity), d.String())
			}
			if preset.HasErrors(diags) {
				return fmt.Errorf("catalog lint failed with errors")
			}
			fmt.Fprintln(out, "✓ Catalog lint passed")
			return nil
		},
	}

	presetsCmd.AddCommand(
		presetsListCmd, presetsShowCmd, presetsAddCmd,
		presetsConfigCmd, presetsRemoveCmd, presetsBuildCmd, presetsLintCmd,
	)
	return presetsCmd
}

func runPresetsList(cmd *cobra.Command, args []string) error {
	_, _, cat, err := loadAppState()
	if err != nil {
		return err
	}
	out := cmdOut(cmd)

	fmt.Fprintln(out, "=== Available Doom Presets ===")
	for i, p := range cat.Presets {
		tag := formatEngineBadge(cat, p.Engine)
		statusFlag := ""
		if p.IsCustom() {
			statusFlag = " [Custom]"
		} else if p.Overridden {
			statusFlag = " [Overridden]"
		}
		fmt.Fprintf(out, "%2d. %-30s %s%s - %s\n", i+1, p.Name, tag, statusFlag, p.Description)
	}
	return nil
}

func runPresetsShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	paths, _, cat, err := loadAppState()
	if err != nil {
		return err
	}
	p := cat.Find(name)
	if p == nil {
		return fmt.Errorf("preset %q not found", name)
	}
	out := cmdOut(cmd)

	fmt.Fprintf(out, "=== Preset: %s ===\n", p.Name)
	if p.IsCustom() {
		fmt.Fprintf(out, "  Type:          Custom WAD\n")
	}
	if p.Author != "" {
		fmt.Fprintf(out, "  Author:        %s\n", p.Author)
	}
	if p.ReleaseDate != "" {
		fmt.Fprintf(out, "  Released:      %s\n", p.ReleaseDate)
	}
	if p.Category != "" {
		fmt.Fprintf(out, "  Category:      %s\n", p.Category)
	}
	if p.Compatibility != "" {
		fmt.Fprintf(out, "  Compatibility: %s\n", p.Compatibility)
	}
	if p.Description != "" {
		fmt.Fprintf(out, "  Description:   %s\n", p.Description)
	}

	engStr := p.Engine
	if cat.Engines != nil {
		if engCfg, ok := cat.Engines[p.Engine]; ok && engCfg.Description != "" {
			engStr = engCfg.Description
		}
	}
	engineOverridden := ""
	if p.EngineOverridden {
		engineOverridden = " [Overridden in user config]"
	}
	fmt.Fprintf(out, "  Engine:        %s%s\n", engStr, engineOverridden)

	idx := preset.NewDirIndex(paths.WadRoots()...).WithCatalog(cat).WithPresets(*p)
	iwadPath, iwadFound := idx.ResolveFile(p.IWAD)
	iwadStatus := "[✗ Missing]"
	if iwadFound {
		iwadStatus = fmt.Sprintf("[✓ Found: %s]", iwadPath)
	}
	fmt.Fprintf(out, "  Base IWAD:     %s %s\n", p.IWAD, iwadStatus)

	if len(p.Files) > 0 {
		fmt.Fprintln(out, "  Files:")
		for _, f := range p.Files {
			fpath, fFound := idx.ResolveFile(f.Name)
			fStatus := "[✗ Missing]"
			if fFound {
				fStatus = fmt.Sprintf("[✓ Found: %s]", fpath)
			} else if f.Optional {
				fStatus = "[Optional]"
			}
			fmt.Fprintf(out, "    - %-20s %s\n", f.Name, fStatus)
		}
	}

	if len(p.Args) > 0 {
		fmt.Fprintf(out, "  Launch Args:   %s\n", strings.Join(p.Args, " "))
	}

	if len(p.Sources) > 0 {
		fmt.Fprintln(out, "  Sources:")
		for _, s := range p.Sources {
			if s.SHA256 != "" {
				fmt.Fprintf(out, "    - %s (sha256: %s)\n", s.URL, s.SHA256)
			} else {
				fmt.Fprintf(out, "    - %s\n", s.URL)
			}
		}
	}

	if len(p.Links) > 0 {
		fmt.Fprintln(out, "  Links:")
		for _, l := range p.Links {
			if l.Rel != "" {
				fmt.Fprintf(out, "    - [%s] %s\n", l.Rel, l.URL)
			} else {
				fmt.Fprintf(out, "    - %s\n", l.URL)
			}
		}
	}

	return nil
}

func formatEngineBadge(cat *preset.Catalog, engineID string) string {
	if cat != nil {
		return fmt.Sprintf("[%-8s]", cat.EngineBadge(engineID))
	}
	switch strings.ToLower(engineID) {
	case "dsda-doom", "dsda":
		return "[DSDA    ]"
	case "uzdoom", "zdoom":
		return "[UZDoom  ]"
	default:
		tag := strings.ToUpper(engineID)
		runes := []rune(tag)
		if len(runes) > 6 {
			tag = string(runes[:6])
		}
		return fmt.Sprintf("[%-8s]", tag)
	}
}
