package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/engine"
	"github.com/lock14/doom-cli/internal/tui"
)

func newPlayCmd() *cobra.Command {
	playCmd := &cobra.Command{
		Use:                "play [flags] [-- extra_engine_args...]",
		Short:              "Interactive terminal launcher with fuzzy search and live preview",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE:               runPlay,
	}
	playCmd.Flags().BoolVar(&flagOnce, "once", false, "Exit after playing a single preset")
	return playCmd
}

func runPlay(cmd *cobra.Command, args []string) error {
	cat, err := getCatalog()
	if err != nil {
		return err
	}

	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	theme := tui.ResolveTheme(flagTheme, os.Getenv("DOOM_THEME"), cfg.Theme, paths.ThemesDir)
	nerdFonts := flagNerdFonts || cfg.NerdFonts
	extraArgs := extractEngineArgs("play", os.Args)

	isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	singleShot := flagOnce || flagDryRun || !isInteractive

	var lastPreset string
	for {
		selected, err := tui.RunInteractiveLauncher(cat, paths.WadsDir, theme, nerdFonts, lastPreset)
		if err != nil {
			return err
		}
		if selected == nil {
			return nil
		}

		lastPreset = selected.Name
		opts := engine.LaunchOptions{
			EngineOverride: flagEngineOverride,
			WadsDir:        paths.WadsDir,
			BinDir:         paths.BinDir,
			DryRun:         flagDryRun,
			ExtraArgs:      extraArgs,
			Out:            os.Stdout,
			Engines:        cat.Engines,
		}

		plan, err := engine.PrepareLaunch(*selected, opts)
		if err != nil {
			if singleShot {
				return err
			}
			fmt.Fprintf(os.Stderr, "\nLaunch error: %v\nPress Enter to return to launcher...", err)
			waitForEnter(os.Stdin)
			continue
		}

		if flagDryRun {
			return engine.Execute(plan, os.Stdout, os.Stderr)
		}

		if err := engine.Execute(plan, os.Stdout, os.Stderr); err != nil {
			if singleShot {
				return err
			}
			fmt.Fprintf(os.Stderr, "\nEngine exited with error: %v\nPress Enter to return to launcher...", err)
			waitForEnter(os.Stdin)
		}

		if singleShot {
			return nil
		}
	}
}

// waitForEnter pauses execution until the user presses Enter on the provided reader.
func waitForEnter(r io.Reader) {
	reader := bufio.NewReader(r)
	_, _ = reader.ReadString('\n')
}

func extractEngineArgs(subcommand string, rawArgs []string) []string {
	dashDashIdx := -1
	for i, a := range rawArgs {
		if a == "--" {
			dashDashIdx = i
			break
		}
	}
	if dashDashIdx != -1 && dashDashIdx+1 < len(rawArgs) {
		return rawArgs[dashDashIdx+1:]
	}

	knownFlagsWithValue := map[string]bool{
		"--engine": true, "-e": true,
		"--wads-dir":     true,
		"--bin-dir":      true,
		"--presets-file": true,
		"--theme":        true,
	}
	knownBoolFlags := map[string]bool{
		"--dry-run":    true,
		"--force":      true,
		"--once":       true,
		"--nerd-fonts": true,
		"-h":           true, "--help": true,
	}

	var engineArgs []string
	pastSubcommand := false
	presetSkipped := (subcommand != "launch") // Only skip preset argument on "launch"

	for i := 1; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if !pastSubcommand {
			if arg == subcommand {
				pastSubcommand = true
			}
			continue
		}

		if knownBoolFlags[arg] {
			continue
		}
		if knownFlagsWithValue[arg] {
			i++ // skip flag value
			continue
		}
		if strings.HasPrefix(arg, "--wads-dir=") ||
			strings.HasPrefix(arg, "--bin-dir=") ||
			strings.HasPrefix(arg, "--engine=") ||
			strings.HasPrefix(arg, "--presets-file=") ||
			strings.HasPrefix(arg, "--theme=") ||
			strings.HasPrefix(arg, "--nerd-fonts=") {
			continue
		}

		if !presetSkipped && !strings.HasPrefix(arg, "-") {
			presetSkipped = true
			continue
		}

		engineArgs = append(engineArgs, arg)
	}

	return engineArgs
}
