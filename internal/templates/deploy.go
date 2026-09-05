package templates

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lock14/doom-configs/internal/config"
	"github.com/lock14/doom-configs/internal/display"
)

//go:embed data/autoexec.cfg
var embeddedAutoexec []byte

//go:embed data/dsda-doom.cfg
var embeddedDSDADoom []byte

//go:embed data/options_linux.json
var embeddedOptionsLinux []byte

//go:embed data/options_windows.json
var embeddedOptionsWindows []byte

// BackupFile copies dest to dest.bak.YYYYMMDDHHMMSS if it exists.
func BackupFile(dest string) (string, error) {
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return "", nil
	}

	timestamp := time.Now().Format("20060102150405")
	backupPath := fmt.Sprintf("%s.bak.%s", dest, timestamp)

	data, err := os.ReadFile(dest)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", err
	}
	return backupPath, nil
}

// DeployConfigs installs autoexec.cfg, dsda-doom.cfg, and DoomRunner options.json with backups.
func DeployConfigs(paths *config.Paths) error {
	res := display.DetectResolution()
	rate := display.DetectRefreshRate()

	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}

	// 1. Deploy UZDoom autoexec.cfg
	if err := os.MkdirAll(paths.UZDoomDir, 0755); err != nil {
		return err
	}
	autoexecDest := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	if bkp, _ := BackupFile(autoexecDest); bkp != "" {
		fmt.Printf("Backing up existing autoexec.cfg -> %s\n", filepath.Base(bkp))
	}
	autoexecContent := string(embeddedAutoexec)
	autoexecContent = strings.ReplaceAll(autoexecContent, "__REFRESH_RATE__", strconv.Itoa(rate))
	autoexecContent = strings.ReplaceAll(autoexecContent, "__SOUNDFONT__", paths.SoundFontFile)
	if err := os.WriteFile(autoexecDest, []byte(autoexecContent), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ Installed UZDoom config -> %s (Refresh: %dHz)\n", autoexecDest, rate)

	// 2. Deploy DSDA-Doom dsda-doom.cfg
	if err := os.MkdirAll(paths.DSDADir, 0755); err != nil {
		return err
	}
	dsdaDest := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	if bkp, _ := BackupFile(dsdaDest); bkp != "" {
		fmt.Printf("Backing up existing dsda-doom.cfg -> %s\n", filepath.Base(bkp))
	}
	dsdaContent := string(embeddedDSDADoom)
	dsdaContent = strings.ReplaceAll(dsdaContent, "__RESOLUTION__", res)
	dsdaContent = strings.ReplaceAll(dsdaContent, "__SOUNDFONT__", paths.SoundFontFile)
	if err := os.WriteFile(dsdaDest, []byte(dsdaContent), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ Installed DSDA-Doom config -> %s (Resolution: %s)\n", dsdaDest, res)

	// 3. Deploy DoomRunner options.json
	if runtime.GOOS == "windows" {
		targetDirs := []string{paths.DoomRunnerDir, paths.DoomRunnerRoam}
		baseDrive := "C:"
		if len(paths.WadsDir) >= 2 && paths.WadsDir[1] == ':' {
			baseDrive = strings.ToUpper(paths.WadsDir[:2])
		}

		winContent := string(embeddedOptionsWindows)
		if baseDrive != "E:" {
			winContent = strings.ReplaceAll(winContent, "E:/", baseDrive+"/")
		}
		if paths.WadsDir != "" && paths.WadsDir != baseDrive+`/Doom WADS` {
			normalizedWads := filepath.ToSlash(paths.WadsDir)
			winContent = strings.ReplaceAll(winContent, baseDrive+"/Doom WADS", normalizedWads)
		}

		for _, dir := range targetDirs {
			if dir == "" {
				continue
			}
			_ = os.MkdirAll(dir, 0755)
			dest := filepath.Join(dir, "options.json")
			if bkp, _ := BackupFile(dest); bkp != "" {
				fmt.Printf("Backing up existing options.json -> %s\n", filepath.Base(bkp))
			}
			if err := os.WriteFile(dest, []byte(winContent), 0644); err != nil {
				return err
			}
			fmt.Printf("✓ Installed DoomRunner options -> %s\n", dest)
		}
	} else {
		if err := os.MkdirAll(paths.DoomRunnerDir, 0755); err != nil {
			return err
		}
		runnerDest := filepath.Join(paths.DoomRunnerDir, "options.json")
		if bkp, _ := BackupFile(runnerDest); bkp != "" {
			fmt.Printf("Backing up existing options.json -> %s\n", filepath.Base(bkp))
		}

		runnerContent := string(embeddedOptionsLinux)
		runnerContent = strings.ReplaceAll(runnerContent, "__HOME__/.local/share/games/uzdoom", paths.WadsDir)
		runnerContent = strings.ReplaceAll(runnerContent, "__HOME__/.config/uzdoom", paths.UZDoomDir)
		runnerContent = strings.ReplaceAll(runnerContent, "__HOME__/.local/share/dsda-doom", paths.DSDADir)
		runnerContent = strings.ReplaceAll(runnerContent, "__HOME__", home)

		if err := os.WriteFile(runnerDest, []byte(runnerContent), 0644); err != nil {
			return err
		}
		fmt.Printf("✓ Installed DoomRunner options -> %s\n", runnerDest)
	}

	return nil
}

// RenderAutoexec returns expected autoexec.cfg content with detected display and soundfont.
func RenderAutoexec(paths *config.Paths) string {
	rate := display.DetectRefreshRate()
	content := string(embeddedAutoexec)
	content = strings.ReplaceAll(content, "__REFRESH_RATE__", strconv.Itoa(rate))
	content = strings.ReplaceAll(content, "__SOUNDFONT__", paths.SoundFontFile)
	return content
}

// RenderDSDADoom returns expected dsda-doom.cfg content with detected resolution and soundfont.
func RenderDSDADoom(paths *config.Paths) string {
	res := display.DetectResolution()
	content := string(embeddedDSDADoom)
	content = strings.ReplaceAll(content, "__RESOLUTION__", res)
	content = strings.ReplaceAll(content, "__SOUNDFONT__", paths.SoundFontFile)
	return content
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
		reSoundfont := regexp.MustCompile(`fluid_patchset\s+".*"`)
		text = reSoundfont.ReplaceAllString(text, `fluid_patchset "__SOUNDFONT__"`)
		reFPS := regexp.MustCompile(`vid_maxfps\s+\d+`)
		text = reFPS.ReplaceAllString(text, `vid_maxfps __REFRESH_RATE__`)

		repoAuto := filepath.Join(repoDir, "uzdoom", "autoexec.cfg")
		if _, err := os.Stat(filepath.Dir(repoAuto)); err == nil {
			_ = os.WriteFile(repoAuto, []byte(text), 0644)
			fmt.Fprintf(out, "✓ Synced %s -> %s\n", autoDest, repoAuto)
		}
	}

	// 2. Sync DSDA-Doom
	dsdaDest := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
	if data, err := os.ReadFile(dsdaDest); err == nil {
		text := string(data)
		reRes := regexp.MustCompile(`screen_resolution\s+"[0-9]+x[0-9]+"`)
		text = reRes.ReplaceAllString(text, `screen_resolution               "__RESOLUTION__"`)
		reSF := regexp.MustCompile(`snd_soundfont\s+".*"`)
		text = reSF.ReplaceAllString(text, `snd_soundfont                   "__SOUNDFONT__"`)

		repoDSDA := filepath.Join(repoDir, "dsda-doom", "dsda-doom.cfg")
		if _, err := os.Stat(filepath.Dir(repoDSDA)); err == nil {
			_ = os.WriteFile(repoDSDA, []byte(text), 0644)
			fmt.Fprintf(out, "✓ Synced %s -> %s\n", dsdaDest, repoDSDA)
		}
	}

	return nil
}

