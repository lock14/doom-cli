package portcfg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/display"
	"github.com/lock14/doom-cli/internal/fsutil"
)

// DeployConfigsTo installs autoexec.cfg and dsda-doom.cfg with backups, reporting output to out.
func DeployConfigsTo(paths *config.Paths, out io.Writer) error {
	return DeployConfigsWithOptions(paths, true, out)
}

// DeployConfigsWithOptions installs autoexec.cfg and dsda-doom.cfg. When force is false,
// existing configuration files on disk are preserved without being overwritten.
func DeployConfigsWithOptions(paths *config.Paths, force bool, out io.Writer) error {
	res, rate := display.DetectDisplay()
	return DeployConfigsWithDisplay(paths, res, rate, force, out)
}

// DeployConfigsWithDisplay installs autoexec.cfg and dsda-doom.cfg using pre-detected display
// parameters so callers that already probed the display avoid spawning duplicate subprocesses.
func DeployConfigsWithDisplay(paths *config.Paths, res string, rate int, force bool, out io.Writer) error {
	if paths == nil {
		return errors.New("nil configuration paths")
	}
	if out == nil {
		out = io.Discard
	}
	if strings.TrimSpace(res) == "" {
		res = "1920x1080"
	}
	if rate <= 0 {
		rate = 60
	}

	// 1. Deploy UZDoom autoexec.cfg
	if err := os.MkdirAll(paths.UZDoomDir, 0o755); err != nil {
		return err
	}
	autoexecDest := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
	if !force {
		if _, err := os.Stat(autoexecDest); err == nil {
			fmt.Fprintf(out, "✓ UZDoom config already installed -> %s\n", autoexecDest)
		} else {
			autoexecContent := RenderAutoexecWithRate(paths, rate)
			if _, err := fsutil.WriteFileAtomic(autoexecDest, []byte(autoexecContent), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(out, "✓ Installed UZDoom config -> %s (Refresh: %dHz)\n", autoexecDest, rate)
		}
	} else {
		autoexecContent := RenderAutoexecWithRate(paths, rate)
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
			dsdaContent := RenderDSDADoomWithRes(paths, res)
			if _, err := fsutil.WriteFileAtomic(dsdaDest, []byte(dsdaContent), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(out, "✓ Installed DSDA-Doom config -> %s (Resolution: %s)\n", dsdaDest, res)
		}
	} else {
		dsdaContent := RenderDSDADoomWithRes(paths, res)
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
