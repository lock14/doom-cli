// Package engine manages source port downloading, execution, and argument synthesis.
package engine

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lock14/doom-configs/internal/preset"
)

// LaunchOptions contains options and overrides for preset execution.
type LaunchOptions struct {
	EngineOverride string
	WadsDir        string
	BinDir         string
	DryRun         bool
	ExtraArgs      []string
	Out            io.Writer
}

// LaunchPlan contains the resolved engine binary and constructed arguments ready for execution.
type LaunchPlan struct {
	Preset       preset.Preset
	Engine       string
	EngineBin    string
	IWADPath     string
	OrderedFiles []string
	Args         []string
	DryRun       bool
}

// PrepareLaunch resolves engine binaries, files, and arguments to create a LaunchPlan.
func PrepareLaunch(p preset.Preset, opts LaunchOptions) (*LaunchPlan, error) {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	engine := p.Engine
	if opts.EngineOverride != "" {
		switch strings.ToLower(opts.EngineOverride) {
		case "dsda", "dsda-doom", "dsdadoom":
			engine = "dsda-doom"
		case "uzdoom", "gzdoom", "zdoom":
			engine = "uzdoom"
		default:
			engine = opts.EngineOverride
		}
	}

	// Resolve engine binary
	engineBin, err := resolveEngineBinary(engine, opts.BinDir)
	if err != nil {
		return nil, err
	}

	// Resolve IWAD
	iwadPath, ok := preset.ResolveFile(opts.WadsDir, p.IWAD)
	if !ok {
		// Fallback check: doom.wad -> doom1.wad
		if strings.EqualFold(p.IWAD, "doom.wad") {
			iwadPath, ok = preset.ResolveFile(opts.WadsDir, "doom1.wad")
		}
	}
	if !ok {
		return nil, fmt.Errorf("base IWAD '%s' not found in %s.\nRun 'doom wads extract-steam' if you own the game on Steam/GOG", p.IWAD, opts.WadsDir)
	}

	// Resolve Mappack and DeHackEd files
	var orderedFiles []string
	var wads []string
	var dehs []string

	for _, m := range p.Mappacks {
		clean := strings.TrimSpace(m)
		if clean == "" {
			continue
		}

		fpath, ok := preset.ResolveFile(opts.WadsDir, clean)
		if !ok {
			if strings.EqualFold(clean, "idkfa 2024.wad") {
				fmt.Fprintf(out, "  ℹ Optional soundtrack '%s' not found in %s; launching with default MIDI.\n", clean, opts.WadsDir)
				continue
			}
			return nil, fmt.Errorf("required mappack file '%s' not found in %s.\nRun 'doom wads fetch %q' to download it", clean, opts.WadsDir, p.Name)
		}

		orderedFiles = append(orderedFiles, fpath)
		if strings.HasSuffix(strings.ToLower(fpath), ".deh") {
			dehs = append(dehs, fpath)
		} else {
			wads = append(wads, fpath)
		}
	}

	// Build argument list
	var args []string
	args = append(args, "-iwad", iwadPath)

	if engine == "dsda-doom" {
		if len(wads) > 0 {
			args = append(args, "-file")
			args = append(args, wads...)
		}
		if len(dehs) > 0 {
			args = append(args, "-deh")
			args = append(args, dehs...)
		}
	} else {
		// UZDoom accepts all files under -file while maintaining declared order
		if len(orderedFiles) > 0 {
			args = append(args, "-file")
			args = append(args, orderedFiles...)
		}
	}

	// Preset additional args
	if strings.TrimSpace(p.AdditionalArgs) != "" {
		tokens := strings.Fields(p.AdditionalArgs)
		args = append(args, tokens...)
	}

	// User-provided extra args
	if len(opts.ExtraArgs) > 0 {
		args = append(args, opts.ExtraArgs...)
	}

	return &LaunchPlan{
		Preset:       p,
		Engine:       engine,
		EngineBin:    engineBin,
		IWADPath:     iwadPath,
		OrderedFiles: orderedFiles,
		Args:         args,
		DryRun:       opts.DryRun,
	}, nil
}

// Execute runs the prepared launch plan or prints it if DryRun is true.
func Execute(plan *LaunchPlan, out, errOut io.Writer) error {
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}

	fmt.Fprintln(out, "==========================================")
	fmt.Fprintf(out, "Launching Preset: %s\n", plan.Preset.Name)
	fmt.Fprintf(out, "Engine: %s (%s)\n", plan.Engine, plan.EngineBin)
	fmt.Fprintf(out, "IWAD:   %s\n", plan.IWADPath)
	if len(plan.OrderedFiles) > 0 {
		fmt.Fprintf(out, "Files:  %s\n", strings.Join(plan.OrderedFiles, " "))
	}
	fmt.Fprintln(out, "==========================================")
	fmt.Fprintf(out, "Command: %s %s\n\n", plan.EngineBin, strings.Join(plan.Args, " "))

	if plan.DryRun {
		return nil
	}

	cmd := exec.Command(plan.EngineBin, plan.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

func resolveEngineBinary(engine, binDir string) (string, error) {
	candidates := []string{
		filepath.Join(binDir, engine),
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(engine), ".exe") {
		candidates = append(candidates, filepath.Join(binDir, engine+".exe"))
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}

	// Check system PATH
	if p, err := exec.LookPath(engine); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(engine), ".exe") {
		if p, err := exec.LookPath(engine + ".exe"); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("engine binary '%s' not found in %s or system PATH.\nRun 'doom setup' or 'doom engines install' to install it", engine, binDir)
}
