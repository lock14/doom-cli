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
	enginesAddCmd.Flags().StringVar(&addBinary, "bin", "", "Executable name or absolute path (defaults to id)")
	_ = enginesAddCmd.Flags().MarkHidden("bin")
	enginesAddCmd.Flags().StringVar(&addStyle, "style", "", "Argument format: boom (-file/-deh) or zdoom (-file)")
	enginesAddCmd.Flags().StringVar(&addStyle, "args-style", "", "Argument format: boom (-file/-deh) or zdoom (-file)")
	_ = enginesAddCmd.Flags().MarkHidden("args-style")
	enginesAddCmd.Flags().StringVar(&addFamily, "family", "", "Engine family (e.g. boom, mbf21, zdoom)")
	enginesAddCmd.Flags().StringVar(&addArgs, "args", "", "Default flags always passed to this engine")
	enginesAddCmd.Flags().StringVar(&addBadge, "badge", "", "Short badge label for the launcher")
	enginesAddCmd.Flags().StringVar(&addDisplayName, "display-name", "", "Display name for the engine")
	enginesAddCmd.Flags().StringVar(&addDescription, "description", "", "Description of the engine")
	enginesAddCmd.Flags().StringVar(&addDescription, "desc", "", "Description of the engine")
	_ = enginesAddCmd.Flags().MarkHidden("desc")

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
		Use:   "install [uzdoom|dsda-doom|all]",
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
				return fmt.Errorf("unknown engine target: %s (choose uzdoom, dsda-doom, or all)", target)
			}
			return ins.InstallEngine(eng)
		},
	}
	enginesInstallCmd.Flags().BoolVar(&flagForce, "force", false, "Force re-download and overwrite existing files")

	enginesCmd.AddCommand(enginesListCmd, enginesAddCmd, enginesRemoveCmd, enginesInstallCmd)
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
