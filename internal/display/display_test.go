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

	// ProMotion / VRR range on macOS captures upper bound rather than minimum floor.
	darwinVRR := "Displays:\n  Liquid Retina XDR:\n" +
		"    Resolution: 3456 x 2234\n    UI Looks like: 1728 x 1117 @ 48.00Hz - 120.00Hz\n"
	if res, rate := parseDarwinDisplay(darwinVRR); res != "3456x2234" || rate != 120 {
		t.Errorf("parseDarwinDisplay(VRR) = (%q, %d), want (\"3456x2234\", 120)", res, rate)
	}

	// xrandr prefers primary output over earlier secondary output.
	xrandrMulti := "Screen 0: minimum 8 x 8\n" +
		"HDMI-1 connected 1920x1080+2560+0\n   1920x1080     60.00*\n" +
		"DP-2 connected primary 2560x1440+0+0\n   2560x1440     165.00*\n"
	if res, rate := parseXrandrDisplay(xrandrMulti); res != "2560x1440" || rate != 165 {
		t.Errorf("parseXrandrDisplay(multi) = (%q, %d), want (\"2560x1440\", 165)", res, rate)
	}

	// kscreen-doctor strips ANSI escapes and selects active @144* mode over listed @60 mode.
	kscreenOut := "\x1b[32mOutput: 1 DP-1\x1b[0m\n  Modes: 1:2560x1440@60 2:2560x1440@144*!\n"
	if rate := parseKscreenRate(kscreenOut); rate != 144 {
		t.Errorf("parseKscreenRate = %d, want 144", rate)
	}

	// Windows multi-GPU + virtual adapter output selects active primary display.
	winMulti := "x@\n1920x1080@60\n2560x1440@165\n2560x1440\n"
	if res, rate := parseWindowsDisplay(winMulti); res != "2560x1440" || rate != 165 {
		t.Errorf("parseWindowsDisplay = (%q, %d), want (\"2560x1440\", 165)", res, rate)
	}
}
