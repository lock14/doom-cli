package display

import (
	"regexp"
	"testing"
)

func TestDetectResolution(t *testing.T) {
	t.Parallel()
	res := DetectResolution()
	matched, err := regexp.MatchString(`^\d+x\d+$`, res)
	if err != nil || !matched {
		t.Errorf("DetectResolution() returned invalid format: %s", res)
	}
}

func TestDetectRefreshRate(t *testing.T) {
	t.Parallel()
	rate := DetectRefreshRate()
	if rate <= 0 || rate > 500 {
		t.Errorf("DetectRefreshRate() returned unexpected rate: %d", rate)
	}
}

func TestDetectDisplay(t *testing.T) {
	t.Parallel()
	res, rate := DetectDisplay()
	matched, err := regexp.MatchString(`^\d+x\d+$`, res)
	if err != nil || !matched {
		t.Errorf("DetectDisplay() returned invalid resolution: %s", res)
	}
	if rate <= 0 || rate > 500 {
		t.Errorf("DetectDisplay() returned unexpected rate: %d", rate)
	}
}

func TestParseDisplayOutputs(t *testing.T) {
	t.Parallel()

	darwinOut := "Displays:\n  Studio Display:\n" +
		"    Resolution: 5120 x 2880\n    UI Looks like: 2560 x 1440 @ 120.00Hz\n"
	if res, rate := parseDarwinDisplay(darwinOut); res != "5120x2880" || rate != 120 {
		t.Errorf("parseDarwinDisplay = (%q, %d), want (\"5120x2880\", 120)", res, rate)
	}

	xrandrOut := "Screen 0: minimum 8 x 8\nDP-1 connected primary 2560x1440+0+0\n   2560x1440     59.95 + 143.91*\n"
	if res, rate := parseXrandrDisplay(xrandrOut); res != "2560x1440" || rate != 144 {
		t.Errorf("parseXrandrDisplay = (%q, %d), want (\"2560x1440\", 144)", res, rate)
	}

	wlrOut := "DP-1 \"Monitor\"\n  Modes:\n    3840x2160 px, 239.997009 Hz (current)\n"
	if res, rate := parseWlrDisplay(wlrOut); res != "3840x2160" || rate != 240 {
		t.Errorf("parseWlrDisplay = (%q, %d), want (\"3840x2160\", 240)", res, rate)
	}
}
