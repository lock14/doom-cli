package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/preset"
)

func userCatalogWriteDir(paths *config.Paths) (string, error) {
	if flagPresetsFile == "" {
		return paths.CatalogDir, nil
	}
	info, err := os.Stat(flagPresetsFile)
	if err == nil && !info.IsDir() {
		return "", fmt.Errorf("--presets-file %q is a single file; pass a directory to write entities", flagPresetsFile)
	}
	return flagPresetsFile, nil
}

// newEnginesCmd creates the source port engine management command.
func newEnginesCmd() *cobra.Command {
	enginesCmd := &cobra.Command{
		Use:   "engines",
		Short: "Manage and configure Doom source port engines",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnginesList(cmd, args)
		},
	}

	enginesListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured engines (built-in and custom)",
		RunE:  runEnginesList,
	}

	var addID, addBinary, addStyle, addFamily, addArgs, addBadge, addDisplayName, addDescription string
	enginesAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register or update a custom source port engine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("engine name cannot be empty")
			}

			paths, _, err := loadPathsAndConfig()
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

			var defaultArgs []string
			if strings.TrimSpace(addArgs) != "" {
				defaultArgs = strings.Fields(addArgs)
			}

			displayName := addDisplayName
			if displayName == "" {
				displayName = name
			}
			id := addID
			if id == "" {
				id = preset.Slug(name)
			}

			eng := preset.EngineConfig{
				ID:          id,
				DisplayName: displayName,
				Description: addDescription,
				Family:      addFamily,
				ArgsStyle:   strings.ToLower(strings.TrimSpace(addStyle)),
				Binary:      addBinary,
				DefaultArgs: defaultArgs,
				Badge:       addBadge,
			}

			outPath, err := preset.AddEngine(writeDir, curated, eng)
			if err != nil {
				return err
			}

			fmt.Fprintf(
				cmdOut(cmd),
				"✓ Registered engine %q (%s) in %s\n",
				eng.ID,
				eng.EffectiveArgsStyle(),
				outPath,
			)
			return nil
		},
	}
	enginesAddCmd.Flags().StringVar(&addID, "id", "", "Explicit slug ID for the engine (defaults to slugified name)")
	enginesAddCmd.Flags().StringVar(&addBinary, "binary", "", "Executable name or absolute path (defaults to id)")
	enginesAddCmd.Flags().StringVar(
		&addStyle, "style", "", "Argument format: boom (-file and -deh) or zdoom (-file)",
	)
	enginesAddCmd.Flags().StringVar(&addFamily, "family", "", "Engine family (such as boom, mbf21, zdoom)")
	enginesAddCmd.Flags().StringVar(&addArgs, "args", "", "Default flags always passed to this engine")
	enginesAddCmd.Flags().StringVar(&addBadge, "badge", "", "Short badge label for the launcher")
	enginesAddCmd.Flags().StringVar(&addDisplayName, "display-name", "", "Display name for the engine")
	enginesAddCmd.Flags().StringVar(&addDescription, "description", "", "Description of the engine")

	var cfgBinary, cfgStyle, cfgDefaultArgs, cfgPrefixArgs, cfgBadge, cfgDisplayName string
	var cfgAppendArgs []string
	var cfgReset bool
	enginesConfigCmd := &cobra.Command{
		Use:   "config <name>",
		Short: "Configure overrides for a built-in or custom engine",
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
				if err := preset.ResetEngineOverride(writeDir, cat, name); err != nil {
					return err
				}
				fmt.Fprintf(out, "✓ Reset custom engine overrides for %q\n", name)
				return nil
			}

			var opts preset.EngineOverrideOptions
			changed := false
			if cmd.Flags().Changed("binary") || cfgBinary != "" {
				v := cfgBinary
				opts.Binary = &v
				changed = true
			}
			if cmd.Flags().Changed("style") || cfgStyle != "" {
				v := cfgStyle
				opts.ArgsStyle = &v
				changed = true
			}
			if cmd.Flags().Changed("default-args") || cfgDefaultArgs != "" {
				v := strings.Fields(cfgDefaultArgs)
				opts.DefaultArgs = &v
				changed = true
			}
			if cmd.Flags().Changed("append-arg") || len(cfgAppendArgs) > 0 {
				v := append([]string(nil), cfgAppendArgs...)
				opts.AppendArgs = &v
				changed = true
			}
			if cmd.Flags().Changed("prefix-args") || cfgPrefixArgs != "" {
				v := strings.Fields(cfgPrefixArgs)
				opts.PrefixArgs = &v
				changed = true
			}
			if cmd.Flags().Changed("badge") || cfgBadge != "" {
				v := cfgBadge
				opts.Badge = &v
				changed = true
			}
			if cmd.Flags().Changed("display-name") || cfgDisplayName != "" {
				v := cfgDisplayName
				opts.DisplayName = &v
				changed = true
			}
			if !changed {
				return fmt.Errorf(
					"no option flags provided (use --binary, --style, --default-args, " +
						"--append-arg, --prefix-args, --badge, --display-name, or --reset)",
				)
			}

			outPath, err := preset.ConfigEngine(writeDir, cat, name, opts)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "✓ Configured engine overrides for %q in %s\n", name, outPath)
			return nil
		},
	}
	enginesConfigCmd.Flags().StringVar(&cfgBinary, "binary", "", "Override executable name or absolute path")
	enginesConfigCmd.Flags().StringVar(
		&cfgStyle, "style", "", "Override argument format: boom (-file and -deh) or zdoom (-file)",
	)
	enginesConfigCmd.Flags().StringVar(
		&cfgDefaultArgs, "default-args", "", "Replace default flags always passed to this engine",
	)
	enginesConfigCmd.Flags().StringSliceVar(
		&cfgAppendArgs, "append-arg", nil, "Additional flag to append to default engine args (repeatable)",
	)
	enginesConfigCmd.Flags().StringVar(
		&cfgPrefixArgs, "prefix-args", "",
		"Wrapper command tokens prepended before the binary (such as flatpak run ...)",
	)
	enginesConfigCmd.Flags().StringVar(&cfgBadge, "badge", "", "Override short badge label for the launcher")
	enginesConfigCmd.Flags().StringVar(&cfgDisplayName, "display-name", "", "Override display name for the engine")
	enginesConfigCmd.Flags().BoolVar(&cfgReset, "reset", false, "Clear all custom overrides for this engine")

	enginesRemoveCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom engine from user catalog",
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

			removedPath, referencedBy, err := preset.RemoveEngine(writeDir, curated, cat, name)
			if err != nil {
				return err
			}
			if len(referencedBy) > 0 {
				fmt.Fprintf(
					cmdErr(cmd),
					"warning: removed engine %q is still referenced by presets: %s\n",
					name,
					strings.Join(referencedBy, ", "),
				)
			}
			fmt.Fprintf(cmdOut(cmd), "✓ Removed engine %q from %s\n", name, removedPath)
			return nil
		},
	}

	enginesInstallCmd := &cobra.Command{
		Use:   "install [<engine>|all]",
		Short: "Download and deploy portable engine binaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			ins := engine.NewInstaller(paths.BinDir, cmdOut(cmd))
			ins.Force = flagForce
			target := "all"
			if len(args) > 0 {
				target = strings.TrimSpace(args[0])
			}

			if strings.EqualFold(target, "all") {
				return ins.InstallCatalog(cat)
			}

			eng, ok := cat.FindEngine(target)
			if !ok {
				var installable []string
				for id, e := range cat.Engines {
					if e.Install != nil {
						installable = append(installable, id)
					}
				}
				sort.Strings(installable)
				installable = append(installable, "all")
				return fmt.Errorf("unknown engine target: %s (choose %s)", target, strings.Join(installable, ", "))
			}
			return ins.InstallEngine(eng)
		},
	}
	enginesInstallCmd.Flags().BoolVar(&flagForce, "force", false, "Force re-download and overwrite existing files")

	enginesCmd.AddCommand(enginesListCmd, enginesAddCmd, enginesConfigCmd, enginesRemoveCmd, enginesInstallCmd)
	return enginesCmd
}

func checkEngineBinary(binary, binDir string) (string, bool) {
	if resolved, err := engine.ResolveBinary(binary, binDir); err == nil {
		return resolved, true
	}
	return binary, false
}

func runEnginesList(cmd *cobra.Command, args []string) error {
	paths, _, cat, err := loadAppState()
	if err != nil {
		return err
	}
	out := cmdOut(cmd)

	fmt.Fprintln(out, "=== Available Doom Engines ===")
	keys := make([]string, 0, len(cat.Engines))
	for k := range cat.Engines {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		eng := cat.Engines[k]
		binTarget := eng.EffectiveBinary()
		resolvedPath, found := checkEngineBinary(binTarget, paths.BinDir)
		status := "[✗ Missing]"
		if found {
			status = fmt.Sprintf("[✓ Found: %s]", resolvedPath)
		}
		style := eng.EffectiveArgsStyle()
		desc := eng.Description
		if desc == "" {
			desc = eng.EffectiveDisplayName()
		}
		fmt.Fprintf(out, "  • %-12s [%-5s] %-32s %s\n", eng.ID, style, desc, status)
	}
	return nil
}
