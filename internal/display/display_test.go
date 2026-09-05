package display

import (
	"regexp"
	"testing"
)

func TestDetectResolution(t *testing.T) {
	res := DetectResolution()
	matched, err := regexp.MatchString(`^\d+x\d+$`, res)
	if err != nil || !matched {
		t.Errorf("DetectResolution() returned invalid format: %s", res)
	}
}

func TestDetectRefreshRate(t *testing.T) {
	rate := DetectRefreshRate()
	if rate <= 0 || rate > 500 {
		t.Errorf("DetectRefreshRate() returned unexpected rate: %d", rate)
	}
}
