// Package portcfg renders, deploys, diffs, and synchronizes source-port configuration templates.
package portcfg

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lock14/doom-cli"
	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/display"
)

var (
	embeddedAutoexec = mustReadEmbedded("configs/uzdoom/autoexec.cfg")
	embeddedDSDADoom = mustReadEmbedded("configs/dsda-doom/dsda-doom.cfg")
)

func mustReadEmbedded(name string) []byte {
	data, err := fs.ReadFile(doomcli.FS, name)
	if err != nil {
		panic(fmt.Sprintf("portcfg: reading embedded %s: %v", name, err))
	}
	return data
}

func sanitizeCfgValue(val string, slashPaths bool) string {
	clean := strings.Map(func(r rune) rune {
		if r == '"' || r == '\r' || r == '\n' || r == 0 {
			return -1
		}
		return r
	}, val)
	if slashPaths {
		clean = strings.ReplaceAll(filepath.ToSlash(clean), `\`, `/`)
	}
	return clean
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// ConfigsInSync reports whether expected and actual configuration text match after
// normalizing CRLF line endings and surrounding whitespace.
func ConfigsInSync(expected, actual string) bool {
	return strings.TrimSpace(normalizeLineEndings(expected)) ==
		strings.TrimSpace(normalizeLineEndings(actual))
}

func parseDSDACfgLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	idx := strings.IndexAny(trimmed, " \t")
	if idx <= 0 {
		return "", "", false
	}
	key := trimmed[:idx]
	val := strings.TrimSpace(trimmed[idx+1:])
	if val == "" {
		return "", "", false
	}
	return key, val, true
}

func formatDSDACfgLine(key, val string) string {
	if len(key) < 31 {
		return fmt.Sprintf("%-31s %s", key, val)
	}
	return key + " " + val
}

func isMachineDSDAOverlayKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "screen_resolution", "snd_soundfont":
		return true
	default:
		return false
	}
}

func isMachineAutoexecLine(trimmed string) bool {
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "fluid_patchset", "vid_maxfps":
		return true
	default:
		return false
	}
}

func renderBaseAutoexec(paths *config.Paths, rate int) string {
	var sfPath string
	if paths != nil {
		sfPath = sanitizeCfgValue(paths.SoundFontFile, true)
	}
	content := normalizeLineEndings(string(embeddedAutoexec))
	content = strings.ReplaceAll(content, "__REFRESH_RATE__", strconv.Itoa(rate))
	content = strings.ReplaceAll(content, "__SOUNDFONT__", sfPath)
	return content
}

func renderBaseDSDADoom(paths *config.Paths, res string) string {
	var sfPath string
	if paths != nil {
		sfPath = sanitizeCfgValue(paths.SoundFontFile, false)
	}
	content := normalizeLineEndings(string(embeddedDSDADoom))
	content = strings.ReplaceAll(content, "__RESOLUTION__", sanitizeCfgValue(res, false))
	content = strings.ReplaceAll(content, "__SOUNDFONT__", sfPath)
	return content
}

func filterAutoexecOverlayLines(overlayText string) []string {
	var kept []string
	for _, raw := range strings.Split(normalizeLineEndings(overlayText), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if isMachineAutoexecLine(trimmed) {
			continue
		}
		kept = append(kept, line)
	}
	for len(kept) > 0 && strings.TrimSpace(kept[0]) == "" {
		kept = kept[1:]
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	return kept
}

func applyAutoexecOverlay(baseRendered, overlayText string) string {
	kept := filterAutoexecOverlayLines(overlayText)
	if len(kept) == 0 {
		return baseRendered
	}
	base := strings.TrimRight(baseRendered, "\n")
	return base + "\n\n" + strings.Join(kept, "\n") + "\n"
}

func parseDSDAOverlayMap(overlayText string) ([]string, map[string]string) {
	var order []string
	vals := make(map[string]string)
	for _, line := range strings.Split(normalizeLineEndings(overlayText), "\n") {
		key, val, ok := parseDSDACfgLine(line)
		if !ok || isMachineDSDAOverlayKey(key) {
			continue
		}
		if _, exists := vals[key]; !exists {
			order = append(order, key)
		}
		vals[key] = val
	}
	return order, vals
}

func applyDSDAOverlay(baseRendered, overlayText string) string {
	order, vals := parseDSDAOverlayMap(overlayText)
	if len(order) == 0 {
		return baseRendered
	}

	applied := make(map[string]bool, len(order))
	lines := strings.Split(baseRendered, "\n")
	for i, line := range lines {
		key, _, ok := parseDSDACfgLine(line)
		if !ok {
			continue
		}
		if ov, has := vals[key]; has {
			lines[i] = formatDSDACfgLine(key, ov)
			applied[key] = true
		}
	}

	var extra []string
	for _, key := range order {
		if !applied[key] {
			extra = append(extra, formatDSDACfgLine(key, vals[key]))
		}
	}
	out := strings.Join(lines, "\n")
	if len(extra) > 0 {
		out = strings.TrimRight(out, "\n") + "\n" + strings.Join(extra, "\n") + "\n"
	}
	return out
}

// RenderAutoexec returns expected autoexec.cfg content with detected display, soundfont,
// and any personal overrides from <PortCfgDir>/autoexec.cfg.
func RenderAutoexec(paths *config.Paths) string {
	rate := display.DetectRefreshRate()
	return RenderAutoexecWithRate(paths, rate)
}

// RenderAutoexecWithRate returns expected autoexec.cfg content for a given refresh rate,
// soundfont path, and any personal overrides from <PortCfgDir>/autoexec.cfg.
func RenderAutoexecWithRate(paths *config.Paths, rate int) string {
	content := renderBaseAutoexec(paths, rate)
	if paths != nil && strings.TrimSpace(paths.PortCfgDir) != "" {
		ovPath := filepath.Join(paths.PortCfgDir, "autoexec.cfg")
		if ovBytes, err := os.ReadFile(ovPath); err == nil {
			content = applyAutoexecOverlay(content, string(ovBytes))
		}
	}
	return content
}

// RenderDSDADoom returns expected dsda-doom.cfg content with detected resolution, soundfont,
// and any personal overrides from <PortCfgDir>/dsda-doom.cfg.
func RenderDSDADoom(paths *config.Paths) string {
	res := display.DetectResolution()
	return RenderDSDADoomWithRes(paths, res)
}

// RenderDSDADoomWithRes returns expected dsda-doom.cfg content for a given resolution,
// soundfont path, and any personal overrides from <PortCfgDir>/dsda-doom.cfg.
func RenderDSDADoomWithRes(paths *config.Paths, res string) string {
	content := renderBaseDSDADoom(paths, res)
	if paths != nil && strings.TrimSpace(paths.PortCfgDir) != "" {
		ovPath := filepath.Join(paths.PortCfgDir, "dsda-doom.cfg")
		if ovBytes, err := os.ReadFile(ovPath); err == nil {
			content = applyDSDAOverlay(content, string(ovBytes))
		}
	}
	return content
}
