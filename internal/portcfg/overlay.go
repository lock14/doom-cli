package portcfg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/fsutil"
)

func resolvePortTarget(target string) (doUZDoom, doDSDA bool, err error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "all":
		return true, true, nil
	case "uzdoom", "zdoom":
		return true, false, nil
	case "dsda-doom", "dsda", "dsdadoom":
		return false, true, nil
	default:
		return false, false, fmt.Errorf(
			"unknown port config target %q (choose dsda-doom, uzdoom, or all)",
			target,
		)
	}
}

func isMachineDSDASaveKey(key string) bool {
	if isMachineDSDAOverlayKey(key) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(key), "snd_device")
}

// UserOverlayCounts returns the count of customized UZDoom command lines and DSDA-Doom keys
// currently saved under paths.PortCfgDir.
func UserOverlayCounts(paths *config.Paths) (uzdoomLines, dsdaKeys int) {
	if paths == nil || strings.TrimSpace(paths.PortCfgDir) == "" {
		return 0, 0
	}
	if autoBytes, err := os.ReadFile(filepath.Join(paths.PortCfgDir, "autoexec.cfg")); err == nil {
		for _, line := range filterAutoexecOverlayLines(string(autoBytes)) {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			uzdoomLines++
		}
	}
	if dsdaBytes, err := os.ReadFile(filepath.Join(paths.PortCfgDir, "dsda-doom.cfg")); err == nil {
		order, _ := parseDSDAOverlayMap(string(dsdaBytes))
		dsdaKeys = len(order)
	}
	return uzdoomLines, dsdaKeys
}

func extractDSDAUserDiff(baseText, liveText string) []string {
	baseVals := make(map[string]string)
	for _, line := range strings.Split(normalizeLineEndings(baseText), "\n") {
		if k, v, ok := parseDSDACfgLine(line); ok {
			baseVals[k] = v
		}
	}

	var diffLines []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(normalizeLineEndings(liveText), "\n") {
		k, v, ok := parseDSDACfgLine(line)
		if !ok || isMachineDSDASaveKey(k) || seen[k] {
			continue
		}
		if baseVal, exists := baseVals[k]; !exists || baseVal != v {
			diffLines = append(diffLines, formatDSDACfgLine(k, v))
			seen[k] = true
		}
	}
	return diffLines
}

func extractAutoexecUserDiff(baseText, liveText string) ([]string, int) {
	baseLines := make(map[string]bool)
	for _, line := range strings.Split(normalizeLineEndings(baseText), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isMachineAutoexecLine(trimmed) {
			continue
		}
		baseLines[trimmed] = true
	}

	var kept []string
	cmdCount := 0
	seenCustom := false
	prevBlank := false
	for _, raw := range strings.Split(normalizeLineEndings(liveText), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if seenCustom && !prevBlank {
				kept = append(kept, "")
				prevBlank = true
			}
			continue
		}
		if isMachineAutoexecLine(trimmed) || baseLines[trimmed] {
			continue
		}
		seenCustom = true
		prevBlank = false
		kept = append(kept, line)
		if !strings.HasPrefix(trimmed, "//") {
			cmdCount++
		}
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	if cmdCount == 0 {
		return nil, 0
	}
	return kept, cmdCount
}

// SaveUserConfigs captures personal customizations from live installed dsda-doom.cfg and
// autoexec.cfg files into paths.PortCfgDir, excluding machine-specific resolution, refresh
// rate, and SoundFont path values so the resulting files are portable across systems.
func SaveUserConfigs(paths *config.Paths, target string, out io.Writer) error {
	if paths == nil || strings.TrimSpace(paths.PortCfgDir) == "" {
		return errors.New("portcfg override directory is not configured")
	}
	if out == nil {
		out = io.Discard
	}
	doUZDoom, doDSDA, err := resolvePortTarget(target)
	if err != nil {
		return err
	}

	processedAny := false
	if doUZDoom {
		liveAutoPath := filepath.Join(paths.UZDoomDir, "autoexec.cfg")
		liveBytes, readErr := os.ReadFile(liveAutoPath)
		if readErr != nil {
			if !os.IsNotExist(readErr) {
				return readErr
			}
			if !doDSDA {
				return fmt.Errorf(
					"autoexec.cfg not found at %s (run 'doom engines cfg install' first)",
					liveAutoPath,
				)
			}
		} else {
			processedAny = true
			ovAutoPath := filepath.Join(paths.PortCfgDir, "autoexec.cfg")
			lines, cmdCount := extractAutoexecUserDiff(renderBaseAutoexec(paths, 60), string(liveBytes))
			if cmdCount > 0 {
				payload := strings.Join(lines, "\n") + "\n"
				if _, wErr := fsutil.WriteFileAtomic(ovAutoPath, []byte(payload), 0o644); wErr != nil {
					return wErr
				}
				fmt.Fprintf(out, "✓ Saved %d UZDoom override line(s) -> %s\n", cmdCount, ovAutoPath)
			} else {
				if rmErr := os.Remove(ovAutoPath); rmErr == nil {
					fsutil.RemoveBackups(ovAutoPath)
					fmt.Fprintf(out, "ℹ Cleared %s (live autoexec.cfg matches curated defaults)\n", ovAutoPath)
				} else {
					fmt.Fprintf(out, "ℹ UZDoom autoexec.cfg matches curated defaults (0 overrides to save)\n")
				}
			}
		}
	}

	if doDSDA {
		liveDSDAPath := filepath.Join(paths.DSDADir, "dsda-doom.cfg")
		liveBytes, readErr := os.ReadFile(liveDSDAPath)
		if readErr != nil {
			if !os.IsNotExist(readErr) {
				return readErr
			}
			if !doUZDoom {
				return fmt.Errorf(
					"dsda-doom.cfg not found at %s (run 'doom engines cfg install' first)",
					liveDSDAPath,
				)
			}
		} else {
			processedAny = true
			ovDSDAPath := filepath.Join(paths.PortCfgDir, "dsda-doom.cfg")
			diffLines := extractDSDAUserDiff(renderBaseDSDADoom(paths, "1920x1080"), string(liveBytes))
			if len(diffLines) > 0 {
				header := "# Personal DSDA-Doom overrides (managed by 'doom engines cfg save')\n"
				payload := header + strings.Join(diffLines, "\n") + "\n"
				if _, wErr := fsutil.WriteFileAtomic(ovDSDAPath, []byte(payload), 0o644); wErr != nil {
					return wErr
				}
				fmt.Fprintf(out, "✓ Saved %d DSDA-Doom override key(s) -> %s\n", len(diffLines), ovDSDAPath)
			} else {
				if rmErr := os.Remove(ovDSDAPath); rmErr == nil {
					fsutil.RemoveBackups(ovDSDAPath)
					fmt.Fprintf(out, "ℹ Cleared %s (live dsda-doom.cfg matches curated defaults)\n", ovDSDAPath)
				} else {
					fmt.Fprintf(out, "ℹ DSDA-Doom dsda-doom.cfg matches curated defaults (0 overrides to save)\n")
				}
			}
		}
	}

	if !processedAny {
		return errors.New("no installed port configuration files found (run 'doom engines cfg install' first)")
	}
	return nil
}

// ResetUserConfigs removes saved personal port-configuration overrides under paths.PortCfgDir
// along with any .bak.* backup siblings.
func ResetUserConfigs(paths *config.Paths, target string, out io.Writer) (bool, error) {
	if paths == nil || strings.TrimSpace(paths.PortCfgDir) == "" {
		return false, errors.New("portcfg override directory is not configured")
	}
	if out == nil {
		out = io.Discard
	}
	doUZDoom, doDSDA, err := resolvePortTarget(target)
	if err != nil {
		return false, err
	}

	removedAny := false
	removeTarget := func(filename, label string) error {
		ovPath := filepath.Join(paths.PortCfgDir, filename)
		if rmErr := os.Remove(ovPath); rmErr != nil {
			if os.IsNotExist(rmErr) {
				return nil
			}
			return rmErr
		}
		fsutil.RemoveBackups(ovPath)
		fmt.Fprintf(out, "✓ Removed personal %s overrides (%s)\n", label, ovPath)
		removedAny = true
		return nil
	}

	if doUZDoom {
		if err := removeTarget("autoexec.cfg", "UZDoom"); err != nil {
			return removedAny, err
		}
	}
	if doDSDA {
		if err := removeTarget("dsda-doom.cfg", "DSDA-Doom"); err != nil {
			return removedAny, err
		}
	}
	if !removedAny {
		fmt.Fprintf(out, "ℹ No personal port configuration overrides found in %s\n", paths.PortCfgDir)
	}
	return removedAny, nil
}
