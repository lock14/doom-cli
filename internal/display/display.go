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

// DetectDisplay discovers both the primary monitor's resolution (such as "1920x1080")
// and refresh rate in Hz (such as 60, 144, or 240) in a single query pass where possible.
func DetectDisplay() (string, int) {
	res, rate := detectDisplayOS(runtime.GOOS)
	if !reResFormat.MatchString(res) {
		res = "1920x1080"
	}
	if rate <= 0 {
		rate = 60
	}
	return res, rate
}

// DetectResolution attempts to discover the primary monitor's resolution (such as "1920x1080").
func DetectResolution() string {
	res := detectResolutionOS(runtime.GOOS)
	if !reResFormat.MatchString(res) {
		return "1920x1080"
	}
	return res
}

// DetectRefreshRate attempts to discover the primary monitor's refresh rate in Hz (such as 60, 144, or 240).
func DetectRefreshRate() int {
	rate := detectRefreshRateOS(runtime.GOOS)
	if rate <= 0 {
		return 60
	}
	return rate
}

func parseDarwinDisplay(out string) (string, int) {
	var res string
	var rate int
	if m := reDarwinRes.FindStringSubmatch(out); len(m) >= 3 {
		res = m[1] + "x" + m[2]
	}
	if m := reDarwinRate.FindStringSubmatch(out); len(m) >= 3 {
		if val, err := strconv.Atoi(m[2]); err == nil && val > 0 {
			rate = val
		}
	}
	return res, rate
}

func parseXrandrDisplay(out string) (string, int) {
	var res string
	var rate int
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if res == "" {
			if m := reXrandrRes.FindStringSubmatch(line); len(m) >= 2 {
				res = m[1]
			}
		}
		if rate == 0 {
			if m := reXrandrRate.FindStringSubmatch(line); len(m) >= 2 {
				if f, err := strconv.ParseFloat(m[1], 64); err == nil && f > 0 {
					rate = int(f + 0.5)
				}
			}
		}
		if res != "" && rate > 0 {
			break
		}
	}
	return res, rate
}

func parseWlrDisplay(out string) (string, int) {
	var res string
	var rate int
	if m := reWlrRes.FindStringSubmatch(out); len(m) >= 2 {
		res = m[1]
	}
	if m := reWlrRate.FindStringSubmatch(out); len(m) >= 2 {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil && f > 0 {
			rate = int(f + 0.5)
		}
	}
	return res, rate
}

func detectDisplayOS(goos string) (string, int) {
	switch goos {
	case "darwin":
		if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
			return parseDarwinDisplay(string(out))
		}
		return "1920x1080", 60

	case "windows":
		return detectResolutionOS(goos), detectRefreshRateOS(goos)

	default: // linux and unix
		var res string
		var rate int
		if out, err := exec.Command("xrandr").Output(); err == nil {
			res, rate = parseXrandrDisplay(string(out))
		}
		if res == "" || rate <= 0 {
			if out, err := exec.Command("wlr-randr").Output(); err == nil {
				wRes, wRate := parseWlrDisplay(string(out))
				if res == "" {
					res = wRes
				}
				if rate <= 0 {
					rate = wRate
				}
			}
		}
		if res == "" {
			res = detectDRMResolution()
		}
		if rate <= 0 {
			rate = detectKscreenRate()
		}
		return res, rate
	}
}

func detectResolutionOS(goos string) string {
	switch goos {
	case "darwin":
		if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
			if res, _ := parseDarwinDisplay(string(out)); res != "" {
				return res
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

	default: // linux and unix
		if out, err := exec.Command("xrandr").Output(); err == nil {
			if res, _ := parseXrandrDisplay(string(out)); res != "" {
				return res
			}
		}
		if out, err := exec.Command("wlr-randr").Output(); err == nil {
			if res, _ := parseWlrDisplay(string(out)); res != "" {
				return res
			}
		}
		if res := detectDRMResolution(); res != "" {
			return res
		}
	}

	return "1920x1080"
}

func detectRefreshRateOS(goos string) int {
	switch goos {
	case "darwin":
		if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
			if _, rate := parseDarwinDisplay(string(out)); rate > 0 {
				return rate
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
			if _, rate := parseXrandrDisplay(string(out)); rate > 0 {
				return rate
			}
		}
		if out, err := exec.Command("wlr-randr").Output(); err == nil {
			if _, rate := parseWlrDisplay(string(out)); rate > 0 {
				return rate
			}
		}
		if rate := detectKscreenRate(); rate > 0 {
			return rate
		}
	}

	return 60
}

func detectDRMResolution() string {
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
	return ""
}

func detectKscreenRate() int {
	if out, err := exec.Command("kscreen-doctor", "-o").Output(); err == nil {
		if m := reKscreenRate.FindStringSubmatch(string(out)); len(m) >= 2 {
			if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
				return val
			}
		}
	}
	return 0
}
