// Package templates handles engine configuration templates, backup deployment, and synchronization.
package templates

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lock14/doom-cli"
	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/display"
	"github.com/lock14/doom-cli/internal/fsutil"
)

var (
	embeddedAutoexec = mustReadEmbedded("uzdoom/autoexec.cfg")
	embeddedDSDADoom = mustReadEmbedded("dsda-doom/dsda-doom.cfg")
)

func mustReadEmbedded(name string) []byte {
	data, err := fs.ReadFile(doomcli.FS, name)
	if err != nil {
		panic(fmt.Sprintf("templates: reading embedded %s: %v", name, err))
	}
	return data
}

// BackupFile copies dest to dest.bak.YYYYMMDDHHMMSS if it exists, never
// overwriting an existing backup file.
func BackupFile(dest string) (string, error) {
	return fsutil.BackupFile(dest)
}

var (
	reFluidPatchset = regexp.MustCompile(`fluid_patchset\s+".*"`)
	reVidMaxFPS     = regexp.MustCompile(`vid_maxfps\s+\d+`)
	reScreenRes     = regexp.MustCompile(`screen_resolution\s+"[0-9]+x[0-9]+"`)
	reSndSoundfont  = regexp.MustCompile(`snd_soundfont\s+".*"`)
)

func renderAutoexecWithRate(paths *config.Paths, rate int) string {
	content := string(embeddedAutoexec)
	content = strings.ReplaceAll(content, "__REFRESH_RATE__", strconv.Itoa(rate))
	content = strings.ReplaceAll(content, "__SOUNDFONT__", paths.SoundFontFile)
	return content
}

func renderDSDADoomWithRes(paths *config.Paths, res string) string {
	content := string(embeddedDSDADoom)
	content = strings.ReplaceAll(content, "__RESOLUTION__", res)
	content = strings.ReplaceAll(content, "__SOUNDFONT__", paths.SoundFontFile)
	return content
}

// DeployConfigsTo installs autoexec.cfg and dsda-doom.cfg with backups, reporting output to out.
func DeployConfigsTo(paths *config.Paths, out io.Writer) error {
	return DeployConfigsWithOptions(paths, true, out)
}

// DeployConfigsWithOptions installs autoexec.cfg and dsda-doom.cfg. When force is false,
// existing configuration files on disk are preserved without being overwritten.
func DeployConfigsWithOptions(paths *config.Paths, force bool, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	res, rate := display.DetectDisplay()

	// 1. Deploy UZDoom autoexec.cfg
	if err := os.MkdirAll(paths.UZDoomDir, 0o755); err != nil {
		return err
	}
	autoexecDest := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	if !force {
		if _, err := os.Stat(autoexecDest); err == nil {
			fmt.Fprintf(out, "✓ UZDoom config already installed -> %s\n", autoexecDest)
		} else {
			autoexecContent := renderAutoexecWithRate(paths, rate)
			if _, err := fsutil.WriteFileAtomic(autoexecDest, []byte(autoexecContent), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(out, "✓ Installed UZDoom config -> %s (Refresh: %dHz)\n", autoexecDest, rate)
		}
	} else {
		autoexecContent := renderAutoexecWithRate(paths, rate)
		bkp, err := fsutil.WriteFileAtomic(autoexecDest, []byte(autoexecContent), 0o644)
		if err != nil {
			return err
		}
		if bkp != "" {
			fmt.Fprintf(out, "Backing up existing autoexec.cfg -> %s\n", filepath.Base(bkp))
		}
		fmt.Fprintf(out, "✓ Installed UZDoom config -> %s (Refresh: %dHz)\n", autoexecDest, rate)
	}

	// 2. Deploy DSDA-Doom dsda-doom.cfg
	if err := os.MkdirAll(paths.DSDADir, 0o755); err != nil {
		return err
	}
	dsdaDest := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	if !force {
		if _, err := os.Stat(dsdaDest); err == nil {
			fmt.Fprintf(out, "✓ DSDA-Doom config already installed -> %s\n", dsdaDest)
		} else {
			dsdaContent := renderDSDADoomWithRes(paths, res)
			if _, err := fsutil.WriteFileAtomic(dsdaDest, []byte(dsdaContent), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(out, "✓ Installed DSDA-Doom config -> %s (Resolution: %s)\n", dsdaDest, res)
		}
	} else {
		dsdaContent := renderDSDADoomWithRes(paths, res)
		bkp, err := fsutil.WriteFileAtomic(dsdaDest, []byte(dsdaContent), 0o644)
		if err != nil {
			return err
		}
		if bkp != "" {
			fmt.Fprintf(out, "Backing up existing dsda-doom.cfg -> %s\n", filepath.Base(bkp))
		}
		fmt.Fprintf(out, "✓ Installed DSDA-Doom config -> %s (Resolution: %s)\n", dsdaDest, res)
	}

	return nil
}

// RenderAutoexec returns expected autoexec.cfg content with detected display and soundfont.
func RenderAutoexec(paths *config.Paths) string {
	rate := display.DetectRefreshRate()
	return renderAutoexecWithRate(paths, rate)
}

// RenderDSDADoom returns expected dsda-doom.cfg content with detected resolution and soundfont.
func RenderDSDADoom(paths *config.Paths) string {
	res := display.DetectResolution()
	return renderDSDADoomWithRes(paths, res)
}

// DiffConfigs compares installed system configs against embedded templates and prints differences.
func DiffConfigs(paths *config.Paths, out io.Writer) error {
	if out == nil {
		out = os.Stdout
	}

	fmt.Fprintln(out, "=== UZDoom Diff ===")
	autoDest := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	printDiff("autoexec.cfg", RenderAutoexec(paths), autoDest, out)

	fmt.Fprintln(out, "\n=== DSDA-Doom Diff ===")
	dsdaDest := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	printDiff("dsda-doom.cfg", RenderDSDADoom(paths), dsdaDest, out)

	return nil
}

func printDiff(title, expected, diskPath string, out io.Writer) {
	diskData, err := os.ReadFile(diskPath)
	if err != nil {
		fmt.Fprintf(out, "File not installed on disk: %s\n", diskPath)
		return
	}

	expLines := strings.Split(strings.TrimSpace(expected), "\n")
	actLines := strings.Split(strings.TrimSpace(string(diskData)), "\n")

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
	if out == nil {
		out = os.Stdout
	}

	if repoDir == "" {
		repoDir = "."
	}

	// 1. Sync UZDoom
	autoDest := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	if data, err := os.ReadFile(autoDest); err == nil {
		text := string(data)
		text = reFluidPatchset.ReplaceAllString(text, `fluid_patchset "__SOUNDFONT__"`)
		text = reVidMaxFPS.ReplaceAllString(text, `vid_maxfps __REFRESH_RATE__`)

		repoAuto := filepath.Join(repoDir, "uzdoom", "autoexec.cfg")
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
		text := string(data)
		text = reScreenRes.ReplaceAllString(text, `screen_resolution               "__RESOLUTION__"`)
		text = reSndSoundfont.ReplaceAllString(text, `snd_soundfont                   "__SOUNDFONT__"`)

		repoDSDA := filepath.Join(repoDir, "dsda-doom", "dsda-doom.cfg")
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
