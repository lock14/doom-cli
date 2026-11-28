package portcfg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/display"
	"github.com/lock14/doom-cli/internal/fsutil"
)

var (
	reFluidPatchset = regexp.MustCompile(`(?m)^(\s*fluid_patchset\s+)"[^"\r\n]*"`)
	reVidMaxFPS     = regexp.MustCompile(`(?m)^(\s*vid_maxfps\s+)\d+`)
	reScreenRes     = regexp.MustCompile(`(?m)^(\s*screen_resolution\s+)"[0-9]+x[0-9]+"`)
	reSndSoundfont  = regexp.MustCompile(`(?m)^(\s*snd_soundfont\s+)"[^"\r\n]*"`)
)

// DiffConfigs compares installed system configs against rendered templates and prints differences.
func DiffConfigs(paths *config.Paths, out io.Writer) error {
	res, rate := display.DetectDisplay()
	return DiffConfigsWithDisplay(paths, res, rate, out)
}

// DiffConfigsWithDisplay compares installed system configs against rendered templates using
// pre-detected display parameters to avoid redundant display probes.
func DiffConfigsWithDisplay(paths *config.Paths, res string, rate int, out io.Writer) error {
	if paths == nil {
		return errors.New("nil configuration paths")
	}
	if out == nil {
		out = os.Stdout
	}
	if strings.TrimSpace(res) == "" {
		res = "1920x1080"
	}
	if rate <= 0 {
		rate = 60
	}

	fmt.Fprintln(out, "=== UZDoom Diff ===")
	autoDest := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	printDiff("autoexec.cfg", RenderAutoexecWithRate(paths, rate), autoDest, out)

	fmt.Fprintln(out, "\n=== DSDA-Doom Diff ===")
	dsdaDest := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	printDiff("dsda-doom.cfg", RenderDSDADoomWithRes(paths, res), dsdaDest, out)

	return nil
}

type diffEdit struct {
	op   byte
	line string
}

func computeLineDiff(expLines, actLines []string) []diffEdit {
	n := len(expLines)
	m := len(actLines)
	cols := m + 1
	dp := make([]int, (n+1)*cols)
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if expLines[i] == actLines[j] {
				dp[i*cols+j] = dp[(i+1)*cols+(j+1)] + 1
			} else if dp[(i+1)*cols+j] >= dp[i*cols+(j+1)] {
				dp[i*cols+j] = dp[(i+1)*cols+j]
			} else {
				dp[i*cols+j] = dp[i*cols+(j+1)]
			}
		}
	}

	var edits []diffEdit
	i, j := 0, 0
	for i < n && j < m {
		if expLines[i] == actLines[j] {
			i++
			j++
		} else if dp[(i+1)*cols+j] >= dp[i*cols+(j+1)] {
			edits = append(edits, diffEdit{op: '-', line: expLines[i]})
			i++
		} else {
			edits = append(edits, diffEdit{op: '+', line: actLines[j]})
			j++
		}
	}
	for ; i < n; i++ {
		edits = append(edits, diffEdit{op: '-', line: expLines[i]})
	}
	for ; j < m; j++ {
		edits = append(edits, diffEdit{op: '+', line: actLines[j]})
	}
	return edits
}

func printDiff(title, expected, diskPath string, out io.Writer) {
	diskData, err := os.ReadFile(diskPath)
	if err != nil {
		fmt.Fprintf(out, "File not installed on disk: %s\n", diskPath)
		return
	}

	normExp := strings.TrimSpace(normalizeLineEndings(expected))
	normAct := strings.TrimSpace(normalizeLineEndings(string(diskData)))
	if normExp == normAct {
		fmt.Fprintf(out, "✓ %s is in sync with system (%s)\n", title, diskPath)
		return
	}

	edits := computeLineDiff(strings.Split(normExp, "\n"), strings.Split(normAct, "\n"))
	const maxShown = 20
	shown := 0
	for _, ed := range edits {
		if strings.TrimSpace(ed.line) == "" {
			continue
		}
		shown++
		if shown <= maxShown {
			fmt.Fprintf(out, "%c %s\n", ed.op, ed.line)
		}
	}
	if shown == 0 {
		fmt.Fprintf(out, "✓ %s is in sync with system (%s)\n", title, diskPath)
		return
	}
	if shown > maxShown {
		fmt.Fprintf(out, "... and %d more line differences\n", shown-maxShown)
	}
}

func stripAutoexecOverlayForSync(text string, paths *config.Paths) string {
	if paths == nil || strings.TrimSpace(paths.PortCfgDir) == "" {
		return text
	}
	ovBytes, err := os.ReadFile(filepath.Join(paths.PortCfgDir, "autoexec.cfg"))
	if err != nil {
		return text
	}
	kept := filterAutoexecOverlayLines(string(ovBytes))
	if len(kept) == 0 {
		return text
	}
	suffix := "\n\n" + strings.Join(kept, "\n")
	trimmed := strings.TrimRight(text, "\n")
	if strings.HasSuffix(trimmed, suffix) {
		return strings.TrimSuffix(trimmed, suffix) + "\n"
	}
	return text
}

func stripDSDAOverlayForSync(text string, paths *config.Paths) string {
	if paths == nil || strings.TrimSpace(paths.PortCfgDir) == "" {
		return text
	}
	ovBytes, err := os.ReadFile(filepath.Join(paths.PortCfgDir, "dsda-doom.cfg"))
	if err != nil {
		return text
	}
	_, ovVals := parseDSDAOverlayMap(string(ovBytes))
	if len(ovVals) == 0 {
		return text
	}

	baseVals := make(map[string]string)
	for _, line := range strings.Split(normalizeLineEndings(string(embeddedDSDADoom)), "\n") {
		if k, v, ok := parseDSDACfgLine(line); ok {
			baseVals[k] = v
		}
	}

	var outLines []string
	for _, line := range strings.Split(text, "\n") {
		key, _, ok := parseDSDACfgLine(line)
		if !ok {
			outLines = append(outLines, line)
			continue
		}
		if _, isUserKey := ovVals[key]; isUserKey {
			if baseVal, inBase := baseVals[key]; inBase {
				outLines = append(outLines, formatDSDACfgLine(key, baseVal))
			}
			continue
		}
		outLines = append(outLines, line)
	}
	return strings.Join(outLines, "\n")
}

// SyncConfigs normalizes live system configs, excludes any personal overrides in paths.PortCfgDir,
// and writes them back into repo templates if repo exists.
func SyncConfigs(paths *config.Paths, repoDir string, out io.Writer) error {
	if paths == nil {
		return errors.New("nil configuration paths")
	}
	if out == nil {
		out = os.Stdout
	}
	if repoDir == "" {
		repoDir = "."
	}

	// 1. Sync UZDoom
	autoDest := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	if data, err := os.ReadFile(autoDest); err == nil {
		text := normalizeLineEndings(string(data))
		text = stripAutoexecOverlayForSync(text, paths)
		text = reFluidPatchset.ReplaceAllString(text, `${1}"__SOUNDFONT__"`)
		text = reVidMaxFPS.ReplaceAllString(text, `${1}__REFRESH_RATE__`)

		repoAuto := filepath.Join(repoDir, "configs", "uzdoom", "autoexec.cfg")
		if _, err := os.Stat(filepath.Dir(repoAuto)); err == nil {
			if _, err := fsutil.WriteFileAtomic(repoAuto, []byte(text), 0o644); err != nil {
				return err
			}
			fsutil.RemoveBackups(repoAuto)
			fmt.Fprintf(out, "✓ Synced %s -> %s\n", autoDest, repoAuto)
		}
	}

	// 2. Sync DSDA-Doom
	dsdaDest := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	if data, err := os.ReadFile(dsdaDest); err == nil {
		text := normalizeLineEndings(string(data))
		text = stripDSDAOverlayForSync(text, paths)
		text = reScreenRes.ReplaceAllString(text, `screen_resolution               "__RESOLUTION__"`)
		text = reSndSoundfont.ReplaceAllString(text, `snd_soundfont                   "__SOUNDFONT__"`)

		repoDSDA := filepath.Join(repoDir, "configs", "dsda-doom", "dsda-doom.cfg")
		if _, err := os.Stat(filepath.Dir(repoDSDA)); err == nil {
			if _, err := fsutil.WriteFileAtomic(repoDSDA, []byte(text), 0o644); err != nil {
				return err
			}
			fsutil.RemoveBackups(repoDSDA)
			fmt.Fprintf(out, "✓ Synced %s -> %s\n", dsdaDest, repoDSDA)
		}
	}

	return nil
}
