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

	"github.com/lock14/doom-cli/internal/preset"
)

// LaunchOptions contains options and overrides for preset execution.
type LaunchOptions struct {
	EngineOverride string
	WadsDir        string
	WadRoots       []string
	BinDir         string
	DryRun         bool
	ExtraArgs      []string
	Out            io.Writer
	Engines        map[string]preset.EngineConfig
	Catalog        *preset.Catalog
}

func (opts LaunchOptions) wadRoots() []string {
	if len(opts.WadRoots) > 0 {
		return opts.WadRoots
	}
	if opts.WadsDir != "" {
		return []string{opts.WadsDir}
	}
	return nil
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

func resolveEngineConfig(name string, opts LaunchOptions) (preset.EngineConfig, error) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		clean = "dsda-doom"
	}

	if opts.Catalog != nil {
		if eng, ok := opts.Catalog.FindEngine(clean); ok {
			return eng, nil
		}
		return preset.EngineConfig{}, fmt.Errorf("unknown engine %q", clean)
	}

	if len(opts.Engines) > 0 {
		for k, eng := range opts.Engines {
			if strings.EqualFold(k, clean) || strings.EqualFold(eng.ID, clean) {
				if eng.ID == "" {
					eng.ID = k
				}
				return eng, nil
			}
			for _, alias := range eng.Aliases {
				if strings.EqualFold(alias, clean) {
					if eng.ID == "" {
						eng.ID = k
					}
					return eng, nil
				}
			}
		}
	}

	if curated, err := preset.LoadCuratedCatalog(); err == nil {
		if eng, ok := curated.FindEngine(clean); ok {
			return eng, nil
		}
	}

	if len(opts.Engines) > 0 {
		return preset.EngineConfig{}, fmt.Errorf("unknown engine %q", clean)
	}

	return preset.EngineConfig{
		ID:        clean,
		Binary:    clean,
		ArgsStyle: preset.GuessArgsStyle(clean, ""),
	}, nil
}

// PrepareLaunch resolves engine binaries, files, and arguments to create a LaunchPlan.
func PrepareLaunch(p preset.Preset, opts LaunchOptions) (*LaunchPlan, error) {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	targetEngine := p.Engine
	if strings.TrimSpace(opts.EngineOverride) != "" {
		targetEngine = strings.TrimSpace(opts.EngineOverride)
	}

	engCfg, err := resolveEngineConfig(targetEngine, opts)
	if err != nil {
		return nil, err
	}
	engineID := engCfg.ID
	if engineID == "" {
		engineID = targetEngine
	}

	binTarget := engCfg.EffectiveBinary()
	engineBin, err := ResolveBinary(binTarget, opts.BinDir)
	if err != nil {
		return nil, err
	}

	roots := opts.wadRoots()
	primaryDir := opts.WadsDir
	if primaryDir == "" && len(roots) > 0 {
		primaryDir = roots[0]
	}
	idx := preset.NewDirIndex(roots...).WithCatalog(opts.Catalog).WithPresets(p)

	iwadPath, ok := idx.ResolveFile(p.IWAD)
	if !ok {
		return nil, fmt.Errorf(
			"base IWAD '%s' not found in %s.\n"+
				"Run 'doom wads extract-steam' if you own the game on Steam or GOG",
			p.IWAD,
			primaryDir,
		)
	}

	type resolvedEntry struct {
		path     string
		optional bool
		isDeh    bool
	}
	var resolved []resolvedEntry

	for _, f := range p.Files {
		clean := strings.TrimSpace(f.Name)
		if clean == "" {
			continue
		}

		fpath, found := idx.ResolveFile(clean)
		if !found {
			if f.Optional {
				fmt.Fprintf(
					out,
					"  ℹ Optional soundtrack '%s' not found in %s; launching with default MIDI.\n",
					clean,
					primaryDir,
				)
				continue
			}
			return nil, fmt.Errorf(
				"required mappack file '%s' not found in %s.\n"+
					"Run 'doom wads fetch %q' to download it",
				clean,
				primaryDir,
				p.Name,
			)
		}

		lower := strings.ToLower(fpath)
		isDeh := strings.HasSuffix(lower, ".deh") || strings.HasSuffix(lower, ".bex")
		resolved = append(resolved, resolvedEntry{
			path:     fpath,
			optional: f.Optional,
			isDeh:    isDeh,
		})
	}

	if p.LoadMapsAfterMods && len(resolved) > 1 {
		var mods []resolvedEntry
		var maps []resolvedEntry
		for _, r := range resolved {
			if r.optional {
				mods = append(mods, r)
			} else {
				maps = append(maps, r)
			}
		}
		resolved = append(mods, maps...)
	}

	var orderedFiles []string
	var wads []string
	var dehs []string
	for _, r := range resolved {
		orderedFiles = append(orderedFiles, r.path)
		if r.isDeh {
			dehs = append(dehs, r.path)
		} else {
			wads = append(wads, r.path)
		}
	}

	var args []string
	if len(engCfg.PrefixArgs) > 0 {
		args = append(args, engCfg.PrefixArgs...)
	}
	args = append(args, "-iwad", iwadPath)

	if engCfg.EffectiveArgsStyle() == "boom" {
		if len(wads) > 0 {
			args = append(args, "-file")
			args = append(args, wads...)
		}
		if len(dehs) > 0 {
			args = append(args, "-deh")
			args = append(args, dehs...)
		}
	} else {
		if len(orderedFiles) > 0 {
			args = append(args, "-file")
			args = append(args, orderedFiles...)
		}
	}

	if len(engCfg.DefaultArgs) > 0 {
		args = append(args, engCfg.DefaultArgs...)
	}
	if len(p.Args) > 0 {
		args = append(args, p.Args...)
	}
	if len(opts.ExtraArgs) > 0 {
		args = append(args, opts.ExtraArgs...)
	}

	return &LaunchPlan{
		Preset:       p,
		Engine:       engineID,
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
	cmd.Dir = filepath.Dir(plan.EngineBin)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

// ResolveBinary searches for an engine executable in an explicit path, binDir, or system PATH.
func ResolveBinary(engine, binDir string) (string, error) {
	if filepath.IsAbs(engine) || strings.ContainsRune(engine, filepath.Separator) ||
		strings.ContainsRune(engine, '/') || strings.ContainsRune(engine, '\\') {
		if fi, err := os.Stat(engine); err == nil && !fi.IsDir() {
			return engine, nil
		}
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(engine), ".exe") {
			if fi, err := os.Stat(engine + ".exe"); err == nil && !fi.IsDir() {
				return engine + ".exe", nil
			}
		}
		return "", fmt.Errorf("engine binary '%s' not found", engine)
	}

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

	if p, err := exec.LookPath(engine); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(engine), ".exe") {
		if p, err := exec.LookPath(engine + ".exe"); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf(
		"engine binary '%s' not found in %s or system PATH.\n"+
			"Run 'doom setup' or 'doom engines install' to install it",
		engine,
		binDir,
	)
}
