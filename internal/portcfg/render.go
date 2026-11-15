// Package portcfg renders, deploys, diffs, and synchronizes source-port configuration templates.
package portcfg

import (
	"fmt"
	"io/fs"
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

// RenderAutoexec returns expected autoexec.cfg content with detected display and soundfont.
func RenderAutoexec(paths *config.Paths) string {
	rate := display.DetectRefreshRate()
	return RenderAutoexecWithRate(paths, rate)
}

// RenderAutoexecWithRate returns expected autoexec.cfg content for a given refresh rate and soundfont path.
func RenderAutoexecWithRate(paths *config.Paths, rate int) string {
	var sfPath string
	if paths != nil {
		sfPath = sanitizeCfgValue(paths.SoundFontFile, true)
	}
	content := normalizeLineEndings(string(embeddedAutoexec))
	content = strings.ReplaceAll(content, "__REFRESH_RATE__", strconv.Itoa(rate))
	content = strings.ReplaceAll(content, "__SOUNDFONT__", sfPath)
	return content
}

// RenderDSDADoom returns expected dsda-doom.cfg content with detected resolution and soundfont.
func RenderDSDADoom(paths *config.Paths) string {
	res := display.DetectResolution()
	return RenderDSDADoomWithRes(paths, res)
}

// RenderDSDADoomWithRes returns expected dsda-doom.cfg content for a given resolution and soundfont path.
func RenderDSDADoomWithRes(paths *config.Paths, res string) string {
	var sfPath string
	if paths != nil {
		sfPath = sanitizeCfgValue(paths.SoundFontFile, false)
	}
	content := normalizeLineEndings(string(embeddedDSDADoom))
	content = strings.ReplaceAll(content, "__RESOLUTION__", sanitizeCfgValue(res, false))
	content = strings.ReplaceAll(content, "__SOUNDFONT__", sfPath)
	return content
}
