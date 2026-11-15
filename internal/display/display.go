// Package display provides native display resolution and refresh rate detection.
package display

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const probeTimeout = 1500 * time.Millisecond

var (
	reResFormat        = regexp.MustCompile(`^\d+x\d+$`)
	reDarwinRes        = regexp.MustCompile(`(?i)Resolution:\s*(\d+)\s*x\s*(\d+)`)
	reDarwinRateRange  = regexp.MustCompile(`(?i)(?:refresh rate|@)\s*:?\s*\d+(?:\.\d+)?\s*(?:hz)?\s*-\s*(\d+)`)
	reDarwinRateSingle = regexp.MustCompile(`(?i)(?:refresh rate\s*:?\s*(\d+)|@\s*(\d+)(?:\.\d+)?\s*hz|(\d+)\s*hertz)`)
	reXrandrRes        = regexp.MustCompile(`^\s*(\d+x\d+)\s+.*\*\+?`)
	reXrandrRate       = regexp.MustCompile(`(\d+(?:\.\d+)?)\*`)
	reWlrRes           = regexp.MustCompile(`(\d+x\d+)\s+px,\s+\d+(?:\.\d+)?\s+Hz\s+\(current\)`)
	reWlrRate          = regexp.MustCompile(`(\d+(?:\.\d+)?)\s+Hz\s+\(current\)`)
	reKscreenActive    = regexp.MustCompile(`@(\d+)(?:\.\d+)?\*`)
	reKscreenAny       = regexp.MustCompile(`@(\d+)`)
	reWinController    = regexp.MustCompile(`^(\d+)x(\d+)@(\d+)$`)
	reANSI             = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

func runProbe(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

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
	res, _ := DetectDisplay()
	return res
}

// DetectRefreshRate attempts to discover the primary monitor's refresh rate in Hz (such as 60, 144, or 240).
func DetectRefreshRate() int {
	_, rate := DetectDisplay()
	return rate
}

func parseDarwinDisplay(out string) (string, int) {
	var res string
	var rate int
	if m := reDarwinRes.FindStringSubmatch(out); len(m) >= 3 {
		res = m[1] + "x" + m[2]
	}
	if m := reDarwinRateRange.FindStringSubmatch(out); len(m) >= 2 {
		if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
			rate = val
		}
	}
	if rate == 0 {
		if m := reDarwinRateSingle.FindStringSubmatch(out); len(m) >= 4 {
			for _, grp := range m[1:4] {
				if grp == "" {
					continue
				}
				if val, err := strconv.Atoi(grp); err == nil && val > 0 {
					rate = val
					break
				}
			}
		}
	}
	return res, rate
}

func parseXrandrDisplay(out string) (string, int) {
	var primRes, fbRes string
	var primRate, fbRate int
	inPrimary := false

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			if strings.Contains(line, " connected") {
				inPrimary = strings.Contains(line, " primary ") || strings.HasSuffix(line, " primary")
			} else {
				inPrimary = false
			}
			continue
		}

		var lineRes string
		var lineRate int
		if m := reXrandrRes.FindStringSubmatch(line); len(m) >= 2 {
			lineRes = m[1]
		}
		if m := reXrandrRate.FindStringSubmatch(line); len(m) >= 2 {
			if f, err := strconv.ParseFloat(m[1], 64); err == nil && f > 0 {
				lineRate = int(f + 0.5)
			}
		}
		if lineRes == "" && lineRate == 0 {
			continue
		}
		if inPrimary {
			if primRes == "" && lineRes != "" {
				primRes = lineRes
			}
			if primRate == 0 && lineRate > 0 {
				primRate = lineRate
			}
		} else {
			if fbRes == "" && lineRes != "" {
				fbRes = lineRes
			}
			if fbRate == 0 && lineRate > 0 {
				fbRate = lineRate
			}
		}
	}

	if primRes != "" && primRate > 0 {
		return primRes, primRate
	}
	if primRes != "" {
		return primRes, fbRate
	}
	return fbRes, fbRate
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

func parseKscreenRate(out string) int {
	clean := reANSI.ReplaceAllString(out, "")
	if m := reKscreenActive.FindStringSubmatch(clean); len(m) >= 2 {
		if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
			return val
		}
	}
	if m := reKscreenAny.FindStringSubmatch(clean); len(m) >= 2 {
		if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
			return val
		}
	}
	return 0
}

func parseWindowsDisplay(out string) (string, int) {
	var bestRes string
	var bestRate int
	var bestPixels int

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if m := reWinController.FindStringSubmatch(line); len(m) >= 4 {
			w, wErr := strconv.Atoi(m[1])
			h, hErr := strconv.Atoi(m[2])
			r, rErr := strconv.Atoi(m[3])
			if wErr == nil && hErr == nil && w > 0 && h > 0 {
				pixels := w * h
				if bestRes == "" || (r > bestRate && pixels >= bestPixels) {
					bestRes = m[1] + "x" + m[2]
					bestPixels = pixels
					if rErr == nil && r > 0 {
						bestRate = r
					}
				} else if bestRate == 0 && rErr == nil && r > 0 {
					bestRate = r
				}
			} else if bestRate == 0 && rErr == nil && r > 0 {
				bestRate = r
			}
			continue
		}
		if bestRes == "" && reResFormat.MatchString(line) {
			bestRes = line
		} else if bestRate == 0 {
			if r, err := strconv.Atoi(line); err == nil && r > 0 {
				bestRate = r
			}
		}
	}
	return bestRes, bestRate
}

func detectDisplayOS(goos string) (string, int) {
	switch goos {
	case "darwin":
		if out, err := runProbe("system_profiler", "SPDisplaysDataType"); err == nil {
			return parseDarwinDisplay(string(out))
		}
		return "1920x1080", 60

	case "windows":
		winScript := "Get-CimInstance Win32_VideoController -ErrorAction SilentlyContinue | " +
			"ForEach-Object { \"$($_.CurrentHorizontalResolution)x$($_.CurrentVerticalResolution)" +
			"@$($_.CurrentRefreshRate)\" }; " +
			"Add-Type -TypeDefinition 'using System.Runtime.InteropServices; " +
			"public class Dpi { [DllImport(\"user32.dll\")] public static extern bool SetProcessDPIAware(); }'; " +
			"[void][Dpi]::SetProcessDPIAware(); " +
			"Add-Type -AssemblyName System.Windows.Forms; " +
			"[System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Width.ToString() + 'x' + " +
			"[System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Height.ToString()"
		if out, err := runProbe("powershell", "-NoProfile", "-Command", winScript); err == nil {
			res, rate := parseWindowsDisplay(string(out))
			if res == "" {
				res = "1920x1080"
			}
			if rate <= 0 {
				rate = 60
			}
			return res, rate
		}
		return "1920x1080", 60

	default: // linux and unix
		var res string
		var rate int
		if out, err := runProbe("xrandr"); err == nil {
			res, rate = parseXrandrDisplay(string(out))
		}
		if res == "" || rate <= 0 {
			if out, err := runProbe("wlr-randr"); err == nil {
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
	if out, err := runProbe("kscreen-doctor", "-o"); err == nil {
		return parseKscreenRate(string(out))
	}
	return 0
}
