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

	var addFlags engineOptionFlags
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
			if strings.TrimSpace(addFlags.args) != "" {
				defaultArgs = strings.Fields(addFlags.args)
			}
			var prefixArgs []string
			if strings.TrimSpace(addFlags.prefixArgs) != "" {
				prefixArgs = strings.Fields(addFlags.prefixArgs)
			}

			id := addFlags.id
			if id == "" {
				id = preset.Slug(name)
			}

			eng := preset.EngineConfig{
				ID:          id,
				DisplayName: name,
				Description: addFlags.description,
				ArgsStyle:   strings.ToLower(strings.TrimSpace(addFlags.style)),
				Binary:      addFlags.binary,
				DefaultArgs: defaultArgs,
				PrefixArgs:  prefixArgs,
				Badge:       addFlags.badge,
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
	bindEngineCommonFlags(addCmd, &addFlags)
	addCmd.Flags().StringVar(
		&addFlags.id, "id", "", "Explicit slug ID for the engine (defaults to slugified name)",
	)
	addCmd.Flags().StringVar(&addFlags.description, "description", "", "Description of the engine")

	var setFlags engineOptionFlags
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
			if cmd.Flags().Changed("binary") || setFlags.binary != "" {
				v := setFlags.binary
				opts.Binary = &v
				changed = true
			}
			if cmd.Flags().Changed("style") || setFlags.style != "" {
				v := setFlags.style
				opts.ArgsStyle = &v
				changed = true
			}
			if cmd.Flags().Changed("args") || setFlags.args != "" {
				v := strings.Fields(setFlags.args)
				opts.DefaultArgs = &v
				empty := []string{}
				opts.AppendArgs = &empty
				changed = true
			}
			if cmd.Flags().Changed("prefix-args") || setFlags.prefixArgs != "" {
				v := strings.Fields(setFlags.prefixArgs)
				opts.PrefixArgs = &v
				changed = true
			}
			if cmd.Flags().Changed("badge") || setFlags.badge != "" {
				v := setFlags.badge
				opts.Badge = &v
				changed = true
			}
			if cmd.Flags().Changed("display-name") || setFlags.displayName != "" {
				v := setFlags.displayName
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
	bindEngineCommonFlags(setCmd, &setFlags)
	setCmd.Flags().StringVar(
		&setFlags.displayName, "display-name", "", "Override display name for the engine",
	)

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

type engineOptionFlags struct {
	id          string
	binary      string
	style       string
	args        string
	prefixArgs  string
	badge       string
	description string
	displayName string
}

func bindEngineCommonFlags(cmd *cobra.Command, f *engineOptionFlags) {
	cmd.Flags().StringVar(&f.binary, "binary", "", "Executable binary name or path")
	cmd.Flags().StringVar(
		&f.style, "style", "", "Argument format: boom (-file and -deh) or zdoom (-file)",
	)
	cmd.Flags().StringVar(&f.args, "args", "", "Default flags always passed to this engine")
	cmd.Flags().StringVar(
		&f.prefixArgs, "prefix-args", "",
		"Arguments passed after the engine binary and before -iwad (such as -config /path/to/port.cfg)",
	)
	cmd.Flags().StringVar(&f.badge, "badge", "", "Short badge label for the launcher")
}

func newEnginesCfgCmd() *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   "cfg",
		Short: "Inspect, deploy, or synchronize source port configuration files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return templates.DiffConfigs(resolveAppPaths(), cmdOut(cmd))
		},
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Deploy curated autoexec.cfg and dsda-doom.cfg to system directories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return templates.DeployConfigsTo(resolveAppPaths(), cmdOut(cmd))
		},
	}

	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare embedded engine templates against live system configurations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return templates.DiffConfigs(resolveAppPaths(), cmdOut(cmd))
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
			return templates.SyncConfigs(resolveAppPaths(), rootDir, cmdOut(cmd))
		},
	}

	cfgCmd.AddCommand(installCmd, diffCmd, syncCmd)
	return cfgCmd
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
		status := "[✗ Missing]"
		if resolvedPath, resolveErr := engine.ResolveBinary(binTarget, paths.BinDir); resolveErr == nil {
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
