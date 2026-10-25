package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/preset"
	"github.com/lock14/doom-cli/internal/templates"
	"github.com/lock14/doom-cli/internal/tui"
	"github.com/lock14/doom-cli/internal/wad"
)

func newEnginesCmd() *cobra.Command {
	enginesCmd := &cobra.Command{
		Use:   "engines",
		Short: "Manage Doom source port binaries, engine overrides, configs, and SoundFonts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
				paths, cfg, cat, err := loadAppState()
				if err != nil {
					return err
				}
				theme, nerdFonts, launcherOpts := buildLauncherOptions(
					paths, cfg, nil, tui.InitialModeEngines,
				)
				_, err = tui.RunInteractiveLauncherWithOptions(
					cat, paths.WadRoots(), theme, nerdFonts, launcherOpts,
				)
				return err
			}
			return runEnginesList(cmd, args)
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all available source port engines and installation status",
		Args:  cobra.NoArgs,
		RunE:  runEnginesList,
	}

	var installForce bool
	installCmd := &cobra.Command{
		Use:   "install [engine|all]",
		Short: "Download and install portable Doom engines, engine configs, and SoundFont",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			out := cmdOut(cmd)

			target := "all"
			if len(args) > 0 {
				target = strings.TrimSpace(args[0])
			}

			ins := engine.NewInstaller(paths.BinDir, out)
			ins.Force = installForce
			if strings.EqualFold(target, "all") {
				if err := ins.InstallCatalog(cat); err != nil {
					return err
				}
				if err := templates.DeployConfigsWithOptions(paths, installForce, out); err != nil {
					return err
				}
				_, err := wad.InstallSoundFont(paths.SoundFontsDir, installForce, out)
				return err
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
	installCmd.Flags().BoolVar(
		&installForce, "force", false, "Re-download and reinstall even if already installed",
	)

	var (
		addID          string
		addBinary      string
		addStyle       string
		addArgs        string
		addPrefixArgs  string
		addBadge       string
		addDescription string
	)
	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register a custom Doom source port in catalog.d",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("engine name cannot be empty")
			}

			paths, _, _, err := loadAppState()
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
			var prefixArgs []string
			if strings.TrimSpace(addPrefixArgs) != "" {
				prefixArgs = strings.Fields(addPrefixArgs)
			}

			id := addID
			if id == "" {
				id = preset.Slug(name)
			}

			eng := preset.EngineConfig{
				ID:          id,
				DisplayName: name,
				Description: addDescription,
				ArgsStyle:   strings.ToLower(strings.TrimSpace(addStyle)),
				Binary:      addBinary,
				DefaultArgs: defaultArgs,
				PrefixArgs:  prefixArgs,
				Badge:       addBadge,
			}

			outPath, err := preset.AddEngine(paths.CatalogDir, curated, eng)
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
	addCmd.Flags().StringVar(&addID, "id", "", "Explicit slug ID for the engine (defaults to slugified name)")
	addCmd.Flags().StringVar(&addBinary, "binary", "", "Executable name or absolute path (defaults to id)")
	addCmd.Flags().StringVar(
		&addStyle, "style", "", "Argument format: boom (-file and -deh) or zdoom (-file)",
	)
	addCmd.Flags().StringVar(&addArgs, "args", "", "Default flags always passed to this engine")
	addCmd.Flags().StringVar(
		&addPrefixArgs, "prefix-args", "",
		"Arguments passed after the engine binary and before -iwad (such as -config /path/to/port.cfg)",
	)
	addCmd.Flags().StringVar(&addBadge, "badge", "", "Short badge label for the launcher")
	addCmd.Flags().StringVar(&addDescription, "description", "", "Description of the engine")

	var (
		setBinary      string
		setStyle       string
		setArgs        string
		setPrefixArgs  string
		setBadge       string
		setDisplayName string
	)
	setCmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Configure binary path, arguments, or metadata overrides for an engine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			out := cmdOut(cmd)

			var opts preset.EngineOverrideOptions
			changed := false
			if cmd.Flags().Changed("binary") || setBinary != "" {
				v := setBinary
				opts.Binary = &v
				changed = true
			}
			if cmd.Flags().Changed("style") || setStyle != "" {
				v := setStyle
				opts.ArgsStyle = &v
				changed = true
			}
			if cmd.Flags().Changed("args") || setArgs != "" {
				v := strings.Fields(setArgs)
				opts.DefaultArgs = &v
				empty := []string{}
				opts.AppendArgs = &empty
				changed = true
			}
			if cmd.Flags().Changed("prefix-args") || setPrefixArgs != "" {
				v := strings.Fields(setPrefixArgs)
				opts.PrefixArgs = &v
				changed = true
			}
			if cmd.Flags().Changed("badge") || setBadge != "" {
				v := setBadge
				opts.Badge = &v
				changed = true
			}
			if cmd.Flags().Changed("display-name") || setDisplayName != "" {
				v := setDisplayName
				opts.DisplayName = &v
				changed = true
			}
			if !changed {
				return fmt.Errorf(
					"no override flags specified; pass --binary, --style, --args, --prefix-args, "+
						"--badge, or --display-name, or run 'doom engines reset %s'",
					name,
				)
			}

			outPath, err := preset.ConfigEngine(paths.CatalogDir, cat, name, opts)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "✓ Updated engine overrides for %q in %s\n", name, outPath)
			return nil
		},
	}
	setCmd.Flags().StringVar(&setBinary, "binary", "", "Override executable binary name or path")
	setCmd.Flags().StringVar(
		&setStyle, "style", "", "Override argument format: boom (-file and -deh) or zdoom (-file)",
	)
	setCmd.Flags().StringVar(&setArgs, "args", "", "Override default flags always passed to this engine")
	setCmd.Flags().StringVar(
		&setPrefixArgs, "prefix-args", "",
		"Arguments passed after the engine binary and before -iwad (such as -config /path/to/port.cfg)",
	)
	setCmd.Flags().StringVar(&setBadge, "badge", "", "Override short badge label for the launcher")
	setCmd.Flags().StringVar(&setDisplayName, "display-name", "", "Override display name for the engine")

	resetCmd := &cobra.Command{
		Use:   "reset <name>",
		Short: "Clear custom engine overrides for a source port",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			if err := preset.ResetEngineOverride(paths.CatalogDir, cat, name); err != nil {
				return err
			}
			fmt.Fprintf(cmdOut(cmd), "✓ Reset custom engine overrides for %q\n", name)
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom engine from user catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			curated, err := preset.LoadCuratedCatalog()
			if err != nil {
				return err
			}

			removedPath, referencedBy, err := preset.RemoveEngine(paths.CatalogDir, curated, cat, name)
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

	cfgCmd := newEnginesCfgCmd()

	enginesCmd.AddCommand(listCmd, installCmd, addCmd, setCmd, resetCmd, removeCmd, cfgCmd)
	return enginesCmd
}

func newEnginesCfgCmd() *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   "cfg",
		Short: "Inspect, deploy, or synchronize source port configuration files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return templates.DiffConfigs(getPaths(), cmdOut(cmd))
		},
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Deploy curated autoexec.cfg and dsda-doom.cfg to system directories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return templates.DeployConfigsTo(getPaths(), cmdOut(cmd))
		},
	}

	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare embedded engine templates against live system configurations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return templates.DiffConfigs(getPaths(), cmdOut(cmd))
		},
	}

	syncCmd := &cobra.Command{
		Use:    "sync [repo-root]",
		Short:  "Synchronize live system engine configurations back to repository templates",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir := "."
			if len(args) > 0 {
				rootDir = args[0]
			}
			return templates.SyncConfigs(getPaths(), rootDir, cmdOut(cmd))
		},
	}

	cfgCmd.AddCommand(installCmd, diffCmd, syncCmd)
	return cfgCmd
}

func checkEngineBinary(binary, binDir string) (string, bool) {
	if resolved, err := engine.ResolveBinary(binary, binDir); err == nil {
		return resolved, true
	}
	return binary, false
}

func runEnginesList(cmd *cobra.Command, _ []string) error {
	paths, _, cat, err := loadAppState()
	if err != nil {
		return err
	}
	out := cmdOut(cmd)

	fmt.Fprintln(out, "=== Available Doom Source Port Engines ===")
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
			status = fmt.Sprintf("[✓ Installed: %s]", resolvedPath)
		}
		style := eng.EffectiveArgsStyle()
		displayName := eng.EffectiveDisplayName()
		statusFlag := ""
		if eng.Layer == preset.LayerUser {
			statusFlag = " (custom)"
		} else if eng.Overridden {
			statusFlag = " (overridden)"
		}
		fmt.Fprintf(
			out,
			"  • %-12s [%-5s] %-20s %s%s\n",
			eng.ID, style, displayName, status, statusFlag,
		)
	}
	return nil
}
