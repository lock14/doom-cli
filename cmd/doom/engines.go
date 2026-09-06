package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/preset"
)

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

	var addBin, addStyle, addArgs, addDesc string
	enginesAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Register or update a custom source port engine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("engine name cannot be empty")
			}

			binTarget := addBin
			if binTarget == "" {
				binTarget = name
			}

			style := strings.ToLower(strings.TrimSpace(addStyle))
			if style != "" && style != "boom" && style != "zdoom" {
				return fmt.Errorf("invalid args-style %q (must be 'boom' or 'zdoom')", style)
			}

			var defaultArgs []string
			if strings.TrimSpace(addArgs) != "" {
				defaultArgs = strings.Fields(addArgs)
			}

			cfgMeta := preset.EngineConfig{
				Name:        name,
				Binary:      binTarget,
				ArgsStyle:   style,
				DefaultArgs: defaultArgs,
				Description: addDesc,
			}

			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			cfg.AddEngine(cfgMeta)
			if err := config.SaveConfig(paths, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf(
				"✓ Registered engine %q (%s) in %s\n",
				name,
				cfgMeta.EffectiveArgsStyle(),
				paths.ConfigFile,
			)
			return nil
		},
	}
	enginesAddCmd.Flags().StringVar(&addBin, "bin", "", "Executable name or absolute path (defaults to name)")
	enginesAddCmd.Flags().StringVar(&addStyle, "args-style", "", "Argument format: boom (-file/-deh) or zdoom (-file)")
	enginesAddCmd.Flags().StringVar(&addArgs, "args", "", "Default flags always passed to this engine")
	enginesAddCmd.Flags().StringVar(&addDesc, "desc", "", "Description of the engine")

	enginesRemoveCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom engine from user configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if strings.EqualFold(name, "uzdoom") || strings.EqualFold(name, "dsda-doom") {
				return fmt.Errorf("cannot remove built-in engine %q", name)
			}

			paths := getPaths()
			cfg, err := config.LoadConfig(paths)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if !cfg.RemoveEngine(name) {
				return fmt.Errorf("engine %q not found in user configuration", name)
			}
			if err := config.SaveConfig(paths, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Printf("✓ Removed engine %q from %s\n", name, paths.ConfigFile)
			return nil
		},
	}

	enginesInstallCmd := &cobra.Command{
		Use:   "install [uzdoom|dsda-doom|all]",
		Short: "Download and deploy portable engine binaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			ins := engine.NewInstaller(paths.BinDir, os.Stdout)
			target := "all"
			if len(args) > 0 {
				target = strings.ToLower(args[0])
			}

			switch target {
			case "uzdoom":
				return ins.InstallUZDoom()
			case "dsda", "dsda-doom", "dsdadoom":
				return ins.InstallDSDA()
			case "all":
				return ins.InstallAll()
			default:
				return fmt.Errorf("unknown engine target: %s (choose uzdoom, dsda-doom, or all)", target)
			}
		},
	}

	enginesCmd.AddCommand(enginesListCmd, enginesAddCmd, enginesRemoveCmd, enginesInstallCmd)
	return enginesCmd
}

func checkEngineBinary(binary, binDir string) (string, bool) {
	if binary == "" {
		return "", false
	}
	if filepath.IsAbs(binary) || strings.Contains(binary, "/") || strings.Contains(binary, "\\") {
		if fi, err := os.Stat(binary); err == nil && !fi.IsDir() {
			return binary, true
		}
		return binary, false
	}
	cand := filepath.Join(binDir, binary)
	if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
		return cand, true
	}
	if p, err := exec.LookPath(binary); err == nil {
		return p, true
	}
	return binary, false
}

func runEnginesList(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}
	paths := getPaths()

	fmt.Println("=== Available Doom Engines ===")
	keys := make([]string, 0, len(cat.Engines))
	for k := range cat.Engines {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		eng := cat.Engines[k]
		binTarget := eng.Binary
		if binTarget == "" {
			binTarget = eng.Name
		}
		resolvedPath, found := checkEngineBinary(binTarget, paths.BinDir)
		status := "[✗ Missing]"
		if found {
			status = fmt.Sprintf("[✓ Found: %s]", resolvedPath)
		}
		style := eng.EffectiveArgsStyle()
		desc := eng.Description
		if desc == "" {
			desc = eng.Name
		}
		fmt.Printf("  • %-12s [%-5s] %-32s %s\n", eng.Name, style, desc, status)
	}
	return nil
}
