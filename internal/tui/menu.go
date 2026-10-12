package tui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/lock14/doom-cli/internal/preset"
)

func engineDisplayLabel(catalog *preset.Catalog, engineKey string) string {
	if catalog != nil {
		if eng, ok := catalog.FindEngine(engineKey); ok {
			return eng.EffectiveDisplayName()
		}
	}
	if curated, err := preset.LoadCuratedCatalog(); err == nil {
		if eng, ok := curated.FindEngine(engineKey); ok {
			return eng.EffectiveDisplayName()
		}
	}
	if strings.TrimSpace(engineKey) == "" {
		return "UZDoom"
	}
	return engineKey
}

// RunNumberedMenu provides a standard interactive menu without full-screen TUI.
func RunNumberedMenu(catalog *preset.Catalog, in io.Reader, out io.Writer) (*preset.Preset, error) {
	fmt.Fprintln(out, "======================================================")
	fmt.Fprintln(out, "               DOOM PRESET LAUNCHER                   ")
	fmt.Fprintln(out, "======================================================")

	for i, p := range catalog.Presets {
		eng := engineDisplayLabel(catalog, p.Engine)
		fmt.Fprintf(out, "%2d. %-30s [%s]\n", i+1, p.Name, eng)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Enter preset number to launch (or 'q' to quit): ")

	scanner := bufio.NewScanner(in)
	if scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(text, "q") || text == "" {
			return nil, nil
		}
		idx, err := strconv.Atoi(text)
		if err == nil && idx >= 1 && idx <= len(catalog.Presets) {
			return &catalog.Presets[idx-1], nil
		}
	}

	return nil, fmt.Errorf("invalid menu selection")
}
