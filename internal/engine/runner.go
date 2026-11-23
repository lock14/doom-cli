// Package engine manages source port downloading, execution, and argument synthesis.
package engine

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/lock14/doom-cli/internal/catalog"
)

// LaunchOptions contains options and overrides for preset execution.
type LaunchOptions struct {
	EngineOverride string
	WadRoots       []string
	BinDir         string
	DryRun         bool
	ExtraArgs      []string
	Out            io.Writer
	Catalog        *catalog.Catalog
}

// LaunchPlan contains the resolved engine binary and constructed arguments ready for execution.
type LaunchPlan struct {
	Preset       catalog.Preset
	Engine       string
	EngineBin    string
	IWADPath     string
	OrderedFiles []string
	Args         []string
	DryRun       bool
}

type resolvedFile struct {
	path     string
	optional bool
	isDeh    bool
}

func absPath(p string) string {
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func newResolvedFile(path string, optional bool) resolvedFile {
	lower := strings.ToLower(path)
	return resolvedFile{
		path:     path,
		optional: optional,
		isDeh:    strings.HasSuffix(lower, ".deh") || strings.HasSuffix(lower, ".bex"),
	}
}

func resolveEngineConfig(
	name string,
	cat *catalog.Catalog,
	strict bool,
) (catalog.EngineConfig, error) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		clean = "dsda-doom"
	}

	if eng, ok := cat.FindEngine(clean); ok {
		return eng, nil
	}
	if strict && cat != nil {
		return catalog.EngineConfig{}, fmt.Errorf("unknown engine %q", clean)
	}

	return catalog.EngineConfig{
		ID:        clean,
		Binary:    clean,
		ArgsStyle: catalog.GuessArgsStyle(clean, ""),
	}, nil
}

func hasComplevelFlag(args []string) bool {
	for _, tok := range args {
		if strings.EqualFold(strings.TrimSpace(tok), "-complevel") {
			return true
		}
	}
	return false
}

func buildEngineArgs(
	engCfg catalog.EngineConfig,
	p catalog.Preset,
	iwadPath string,
	resolved []resolvedFile,
	extraArgs []string,
) ([]string, []string) {
	if p.LoadMapsAfterMods && len(resolved) > 1 {
		var mods []resolvedFile
		var maps []resolvedFile
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
	if iwadPath != "" {
		args = append(args, "-iwad", iwadPath)
	}

	style := engCfg.EffectiveArgsStyle()
	if style == "boom" {
		if len(wads) > 0 {
			args = append(args, "-file")
			args = append(args, wads...)
		}
		if len(dehs) > 0 {
			args = append(args, "-deh")
			args = append(args, dehs...)
		}
	} else if len(orderedFiles) > 0 {
		args = append(args, "-file")
		args = append(args, orderedFiles...)
	}

	if len(engCfg.DefaultArgs) > 0 {
		args = append(args, engCfg.DefaultArgs...)
	}
	engineArgs := p.ArgsForEngine(engCfg.ID)
	if style == "boom" && p.Complevel != nil &&
		!hasComplevelFlag(engineArgs) &&
		!hasComplevelFlag(p.Args) &&
		!hasComplevelFlag(extraArgs) {
		args = append(args, "-complevel", strconv.Itoa(*p.Complevel))
	}
	if len(engineArgs) > 0 {
		args = append(args, engineArgs...)
	}
	if len(p.Args) > 0 {
		args = append(args, p.Args...)
	}
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}
	return orderedFiles, args
}

// PrepareLaunch resolves engine binaries, files, and arguments to create a LaunchPlan.
// Resolved binary and asset paths are canonicalized to absolute paths so child working
// directory changes in Execute never break relative bin-dir or wads-dir configurations
// and relative filenames starting with '-', '+', or '@' cannot inject engine flags.
func PrepareLaunch(p catalog.Preset, opts LaunchOptions) (*LaunchPlan, error) {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	targetEngine := p.Engine
	if strings.TrimSpace(opts.EngineOverride) != "" {
		targetEngine = strings.TrimSpace(opts.EngineOverride)
	}

	engCfg, err := resolveEngineConfig(targetEngine, opts.Catalog, opts.Catalog != nil)
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

	roots := opts.WadRoots
	primaryDir := ""
	if len(roots) > 0 {
		primaryDir = roots[0]
	}
	idx := catalog.NewDirIndex(roots...)
	if opts.Catalog != nil {
		idx = idx.WithCatalog(opts.Catalog)
	} else {
		idx = idx.WithPresets(p)
	}

	iwadPath, ok := idx.ResolveFile(p.IWAD)
	if !ok {
		if !p.IsCoreGame() {
			return nil, fmt.Errorf(
				"base IWAD '%s' not found in %s.\n"+
					"Run 'doom wads scan --games %s' or 'doom wads fetch %s' if you own the game on Steam or GOG",
				p.IWAD,
				primaryDir,
				p.EffectiveGame(),
				p.ID,
			)
		}
		return nil, fmt.Errorf(
			"base IWAD '%s' not found in %s.\n"+
				"Run 'doom wads scan' or 'doom wads fetch %s' if you own the game on Steam or GOG",
			p.IWAD,
			primaryDir,
			p.ID,
		)
	}
	iwadPath = absPath(iwadPath)

	var resolved []resolvedFile
	for _, f := range p.Files {
		if f.Disabled {
			continue
		}
		clean := strings.TrimSpace(f.Name)
		if clean == "" {
			continue
		}

		fpath, found := idx.ResolveFileSpec(f)
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

		resolved = append(resolved, newResolvedFile(absPath(fpath), f.Optional))
	}

	orderedFiles, args := buildEngineArgs(engCfg, p, iwadPath, resolved, opts.ExtraArgs)

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

	binPath := absPath(plan.EngineBin)
	cmd := exec.Command(binPath, plan.Args...)
	cmd.Dir = filepath.Dir(binPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

// ResolveBinary searches for an engine executable in an explicit path, binDir, or system PATH,
// returning a canonicalized absolute path so callers can safely set cmd.Dir.
func ResolveBinary(engine, binDir string) (string, error) {
	if filepath.IsAbs(engine) || strings.ContainsRune(engine, filepath.Separator) ||
		strings.ContainsRune(engine, '/') || strings.ContainsRune(engine, '\\') {
		if fi, err := os.Stat(engine); err == nil && !fi.IsDir() {
			return absPath(engine), nil
		}
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(engine), ".exe") {
			if fi, err := os.Stat(engine + ".exe"); err == nil && !fi.IsDir() {
				return absPath(engine + ".exe"), nil
			}
		}
		return "", fmt.Errorf("engine binary '%s' not found", engine)
	}

	if strings.TrimSpace(binDir) != "" {
		candidates := []string{
			filepath.Join(binDir, engine),
		}
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(engine), ".exe") {
			candidates = append(candidates, filepath.Join(binDir, engine+".exe"))
		}

		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
				return absPath(c), nil
			}
		}
	}

	if p, err := exec.LookPath(engine); err == nil {
		return absPath(p), nil
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(engine), ".exe") {
		if p, err := exec.LookPath(engine + ".exe"); err == nil {
			return absPath(p), nil
		}
	}

	return "", fmt.Errorf(
		"engine binary '%s' not found in %s or system PATH.\n"+
			"Run 'doom setup' or 'doom engines install' to install it",
		engine,
		binDir,
	)
}

// FormatLaunchPreview synthesizes a pure, zero-I/O command-line preview string for p
// using the pre-constructed idx and catalog, falling back to raw filenames when required
// files are not resolved in idx and omitting missing optional files when idx is non-nil.
func FormatLaunchPreview(
	p catalog.Preset,
	cat *catalog.Catalog,
	idx *catalog.DirIndex,
	extraArgs []string,
) string {
	engCfg, _ := resolveEngineConfig(p.Engine, cat, false)
	binTarget := engCfg.EffectiveBinary()
	iwadPath := strings.TrimSpace(p.IWAD)
	if idx != nil && iwadPath != "" {
		if resolved, found := idx.ResolveFile(iwadPath); found {
			iwadPath = resolved
		}
	}

	var resolved []resolvedFile
	for _, f := range p.Files {
		if f.Disabled {
			continue
		}
		clean := strings.TrimSpace(f.Name)
		if clean == "" {
			continue
		}
		fpath := clean
		if idx != nil {
			if rpath, found := idx.ResolveFileSpec(f); found {
				fpath = rpath
			} else if f.Optional {
				continue
			}
		}
		resolved = append(resolved, newResolvedFile(fpath, f.Optional))
	}

	_, args := buildEngineArgs(engCfg, p, iwadPath, resolved, extraArgs)
	tokens := append([]string{binTarget}, args...)
	return strings.Join(tokens, " ")
}
