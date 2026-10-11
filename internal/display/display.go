// Package display provides native display resolution and refresh rate detection.
package display

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var (
	reResFormat   = regexp.MustCompile(`^\d+x\d+$`)
	reDarwinRes   = regexp.MustCompile(`(?i)Resolution:\s*(\d+)\s*x\s*(\d+)`)
	reXrandrRes   = regexp.MustCompile(`^\s*(\d+x\d+)\s+.*\*\+?`)
	reWlrRes      = regexp.MustCompile(`(\d+x\d+)\s+px,\s+\d+(\.\d+)?\s+Hz\s+\(current\)`)
	reDarwinRate  = regexp.MustCompile(`(?i)(refresh rate|hertz|@)\s*:?\s*(\d+)`)
	reXrandrRate  = regexp.MustCompile(`(\d+(\.\d+)?)\*`)
	reWlrRate     = regexp.MustCompile(`(\d+(\.\d+)?)\s+Hz\s+\(current\)`)
	reKscreenRate = regexp.MustCompile(`@(\d+)`)
)

// DetectResolution attempts to discover the primary monitor's resolution (e.g. "1920x1080").
func DetectResolution() string {
	res := detectResolutionOS(runtime.GOOS)
	if !reResFormat.MatchString(res) {
		return "1920x1080"
	}
	return res
}

// DetectRefreshRate attempts to discover the primary monitor's refresh rate in Hz (e.g. 60, 144, 240).
func DetectRefreshRate() int {
	rate := detectRefreshRateOS(runtime.GOOS)
	if rate <= 0 {
		return 60
	}
	return rate
}

func detectResolutionOS(goos string) string {
	switch goos {
	case "darwin":
		if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
			if m := reDarwinRes.FindStringSubmatch(string(out)); len(m) >= 3 {
				return m[1] + "x" + m[2]
			}
		}

	case "windows":
		// Windows PowerShell query for primary monitor resolution
		psScript := "Add-Type -AssemblyName System.Windows.Forms; " +
			"[System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Width.ToString() + 'x' + " +
			"[System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Height.ToString()"
		cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
		if out, err := cmd.Output(); err == nil {
			str := strings.TrimSpace(string(out))
			if reResFormat.MatchString(str) {
				return str
			}
		}

	default: // linux / unix
		// 1. Try xrandr
		if out, err := exec.Command("xrandr").Output(); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				if m := reXrandrRes.FindStringSubmatch(scanner.Text()); len(m) >= 2 {
					return m[1]
				}
			}
		}

		// 2. Try wlr-randr (Wayland)
		if out, err := exec.Command("wlr-randr").Output(); err == nil {
			if m := reWlrRes.FindStringSubmatch(string(out)); len(m) >= 2 {
				return m[1]
			}
		}

		// 3. Try /sys/class/drm/*/modes
		modes, _ := filepath.Glob("/sys/class/drm/*/modes")
		for _, modeFile := range modes {
			if data, err := os.ReadFile(modeFile); err == nil {
				lines := strings.Split(string(data), "\n")
				if len(lines) > 0 {
					candidate := strings.TrimSpace(lines[0])
					if reResFormat.MatchString(candidate) {
						return candidate
					}
				}
			}
		}
	}

	return "1920x1080"
}

func detectRefreshRateOS(goos string) int {
	switch goos {
	case "darwin":
		if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
			if m := reDarwinRate.FindStringSubmatch(string(out)); len(m) >= 3 {
				if val, err := strconv.Atoi(m[2]); err == nil && val > 0 {
					return val
				}
			}
		}

	case "windows":
		psScript := "Get-CimInstance Win32_VideoController | " +
			"Select-Object -ExpandProperty CurrentRefreshRate -ErrorAction SilentlyContinue"
		cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
		if out, err := cmd.Output(); err == nil {
			str := strings.TrimSpace(string(out))
			if val, err := strconv.Atoi(str); err == nil && val > 0 {
				return val
			}
		}

	default: // linux
		if out, err := exec.Command("xrandr").Output(); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				if m := reXrandrRate.FindStringSubmatch(scanner.Text()); len(m) >= 2 {
					if f, err := strconv.ParseFloat(m[1], 64); err == nil {
						return int(f + 0.5)
					}
				}
			}
		}

		if out, err := exec.Command("wlr-randr").Output(); err == nil {
			if m := reWlrRate.FindStringSubmatch(string(out)); len(m) >= 2 {
				if f, err := strconv.ParseFloat(m[1], 64); err == nil {
					return int(f + 0.5)
				}
			}
		}

		if out, err := exec.Command("kscreen-doctor", "-o").Output(); err == nil {
			if m := reKscreenRate.FindStringSubmatch(string(out)); len(m) >= 2 {
				if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
					return val
				}
			}
		}
	}

	return 60
}
