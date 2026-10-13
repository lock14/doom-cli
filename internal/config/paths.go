// Package config provides platform-idiomatic path resolution and user settings for doom-cli.
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Paths holds the resolved platform-idiomatic paths for Doom source ports and tools.
type Paths struct {
	UZDoomDir     string
	DSDADir       string
	WadsDir       string
	WadsDirs      []string
	SoundFontsDir string
	BinDir        string
	DataDir       string
	ConfigDir     string
	ConfigFile    string
	CatalogDir    string
	ThemesDir     string
	SoundFontFile string
}

// GetPaths returns the platform-idiomatic paths for the current running OS.
func GetPaths() *Paths {
	return ResolveFor(runtime.GOOS, "")
}

// ResolveFor returns the platform-idiomatic paths for the given target OS and optional custom WADs dir.
func ResolveFor(targetOS string, customWadsDir string) *Paths {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}

	p := &Paths{}

	switch targetOS {
	case "darwin":
		appSupport := filepath.Join(home, "Library", "Application Support")
		p.UZDoomDir = filepath.Join(appSupport, "uzdoom")
		p.DSDADir = filepath.Join(appSupport, "dsda-doom")
		p.SoundFontsDir = filepath.Join(appSupport, "soundfonts")
		p.DataDir = filepath.Join(appSupport, "doom-cli")
		p.ConfigDir = filepath.Join(appSupport, "doom-cli")
		p.BinDir = filepath.Join(home, ".local", "bin")
		p.WadsDir = filepath.Join(appSupport, "games", "uzdoom")

	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}

		p.DataDir = filepath.Join(localAppData, "doom-cli")
		p.ConfigDir = filepath.Join(localAppData, "doom-cli")

		gamesDoom := filepath.Join(home, "Games", "Doom")
		p.BinDir = filepath.Join(gamesDoom, "bin")
		p.UZDoomDir = p.BinDir
		p.DSDADir = p.BinDir
		p.WadsDir = filepath.Join(gamesDoom, "wads")
		p.SoundFontsDir = filepath.Join(gamesDoom, "soundfonts")

	default: // "linux" and other Unix-like systems following XDG
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(home, ".config")
		}
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(home, ".local", "share")
		}

		p.UZDoomDir = filepath.Join(xdgConfig, "uzdoom")
		p.DSDADir = filepath.Join(xdgData, "dsda-doom")
		p.SoundFontsDir = filepath.Join(xdgData, "soundfonts")
		p.DataDir = filepath.Join(xdgData, "doom-cli")
		p.ConfigDir = filepath.Join(xdgConfig, "doom-cli")
		p.BinDir = filepath.Join(home, ".local", "bin")
		p.WadsDir = filepath.Join(xdgData, "games", "uzdoom")
	}

	p.SetConfigDir(p.ConfigDir)

	if envConfig := os.Getenv("DOOM_CONFIG_DIR"); envConfig != "" {
		p.SetConfigDir(envConfig)
	}

	if envWads := os.Getenv("DOOM_WADS_DIR"); envWads != "" {
		p.WadsDir = envWads
	} else if envWads := os.Getenv("WADS_DIR"); envWads != "" {
		p.WadsDir = envWads
	}
	if envWadsDirs := os.Getenv("DOOM_WADS_DIRS"); envWadsDirs != "" {
		p.WadsDirs = ParsePathList(envWadsDirs)
	}
	if customWadsDir != "" {
		p.WadsDir = customWadsDir
	}
	if envBin := os.Getenv("DOOM_BIN_DIR"); envBin != "" {
		p.SetBinDirForOS(targetOS, envBin)
	} else if envBin := os.Getenv("BIN_DIR"); envBin != "" {
		p.SetBinDirForOS(targetOS, envBin)
	}
	if envSF := os.Getenv("DOOM_SF_DIR"); envSF != "" {
		p.SetSoundFontsDir(envSF)
	} else if envSF := os.Getenv("SF_DIR"); envSF != "" {
		p.SetSoundFontsDir(envSF)
	} else {
		p.SoundFontFile = filepath.Join(p.SoundFontsDir, "GeneralUser-GS.sf2")
	}

	return p
}

// SetConfigDir updates ConfigDir and synchronizes ConfigFile, CatalogDir, and ThemesDir.
func (p *Paths) SetConfigDir(dir string) {
	p.ConfigDir = dir
	p.ConfigFile = filepath.Join(dir, "config.json")
	p.CatalogDir = filepath.Join(dir, "catalog.d")
	p.ThemesDir = filepath.Join(dir, "themes")
}

// SetBinDir updates BinDir and, on Windows, synchronizes UZDoomDir and DSDADir.
func (p *Paths) SetBinDir(dir string) {
	p.SetBinDirForOS(runtime.GOOS, dir)
}

// SetBinDirForOS updates BinDir and, when targetOS is windows, synchronizes UZDoomDir and DSDADir.
func (p *Paths) SetBinDirForOS(targetOS, dir string) {
	p.BinDir = dir
	if targetOS == "windows" {
		p.UZDoomDir = dir
		p.DSDADir = dir
	}
}

// SetSoundFontsDir updates SoundFontsDir and synchronizes SoundFontFile.
func (p *Paths) SetSoundFontsDir(dir string) {
	p.SoundFontsDir = dir
	p.SoundFontFile = filepath.Join(dir, "GeneralUser-GS.sf2")
}

// WadRoots returns the ordered, deduplicated list of WAD root directories,
// starting with WadsDir (root 0) followed by any configured WadsDirs.
func (p *Paths) WadRoots() []string {
	if p == nil {
		return nil
	}
	seen := make(map[string]bool)
	var roots []string
	add := func(dir string) {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			return
		}
		key := filepath.Clean(trimmed)
		if seen[key] {
			return
		}
		seen[key] = true
		roots = append(roots, trimmed)
	}
	add(p.WadsDir)
	for _, dir := range p.WadsDirs {
		add(dir)
	}
	return roots
}

// ParsePathList splits a comma- or OS-path-list-separated string of directories
// into a trimmed, deduplicated slice of non-empty directory paths.
func ParsePathList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, chunk := range strings.Split(raw, ",") {
		for _, part := range filepath.SplitList(chunk) {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			key := filepath.Clean(trimmed)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, trimmed)
		}
	}
	return out
}
