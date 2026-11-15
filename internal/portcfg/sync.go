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

// DiffConfigs compares installed system configs against embedded templates and prints differences.
func DiffConfigs(paths *config.Paths, out io.Writer) error {
	res, rate := display.DetectDisplay()
	return DiffConfigsWithDisplay(paths, res, rate, out)
}

// DiffConfigsWithDisplay compares installed system configs against embedded templates using
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

func printDiff(title, expected, diskPath string, out io.Writer) {
	diskData, err := os.ReadFile(diskPath)
	if err != nil {
		fmt.Fprintf(out, "File not installed on disk: %s\n", diskPath)
		return
	}

	expLines := strings.Split(strings.TrimSpace(normalizeLineEndings(expected)), "\n")
	actLines := strings.Split(strings.TrimSpace(normalizeLineEndings(string(diskData))), "\n")

	diffCount := 0
	maxLines := len(expLines)
	if len(actLines) > maxLines {
		maxLines = len(actLines)
	}

	for i := 0; i < maxLines; i++ {
		var expLine, actLine string
		if i < len(expLines) {
			expLine = expLines[i]
		}
		if i < len(actLines) {
			actLine = actLines[i]
		}
		if expLine != actLine {
			diffCount++
			if diffCount <= 15 {
				if expLine != "" {
					fmt.Fprintf(out, "- %s\n", expLine)
				}
				if actLine != "" {
					fmt.Fprintf(out, "+ %s\n", actLine)
				}
			}
		}
	}

	if diffCount == 0 {
		fmt.Fprintf(out, "✓ %s is in sync with system (%s)\n", title, diskPath)
	} else if diffCount > 15 {
		fmt.Fprintf(out, "... and %d more line differences\n", diffCount-15)
	}
}

// SyncConfigs normalizes live system configs and writes them back into repo templates if repo exists.
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
