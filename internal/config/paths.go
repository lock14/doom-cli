// Package config provides platform-idiomatic path resolution for doom-cli.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Paths holds the resolved platform-idiomatic paths for Doom source ports and tools.
type Paths struct {
	UZDoomDir     string
	DSDADir       string
	WadsDir       string
	SoundFontsDir string
	BinDir        string
	DataDir       string
	ConfigDir     string
	ConfigFile    string
	PresetsFile   string
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

	p.ConfigFile = filepath.Join(p.ConfigDir, "config.json")
	p.PresetsFile = filepath.Join(p.ConfigDir, "presets.json")
	p.ThemesDir = filepath.Join(p.ConfigDir, "themes")

	if envConfig := os.Getenv("DOOM_CONFIG_DIR"); envConfig != "" {
		p.ConfigDir = envConfig
		p.ConfigFile = filepath.Join(p.ConfigDir, "config.json")
		p.PresetsFile = filepath.Join(p.ConfigDir, "presets.json")
		p.ThemesDir = filepath.Join(p.ConfigDir, "themes")
	}

	if envWads := os.Getenv("DOOM_WADS_DIR"); envWads != "" {
		p.WadsDir = envWads
	} else if envWads := os.Getenv("WADS_DIR"); envWads != "" {
		p.WadsDir = envWads
	}
	if customWadsDir != "" {
		p.WadsDir = customWadsDir
	}
	if envBin := os.Getenv("DOOM_BIN_DIR"); envBin != "" {
		p.BinDir = envBin
		if targetOS == "windows" {
			p.UZDoomDir = envBin
			p.DSDADir = envBin
		}
	} else if envBin := os.Getenv("BIN_DIR"); envBin != "" {
		p.BinDir = envBin
		if targetOS == "windows" {
			p.UZDoomDir = envBin
			p.DSDADir = envBin
		}
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

// SetBinDir updates BinDir and, on Windows, synchronizes UZDoomDir and DSDADir.
func (p *Paths) SetBinDir(dir string) {
	p.BinDir = dir
	if runtime.GOOS == "windows" {
		p.UZDoomDir = dir
		p.DSDADir = dir
	}
}

// SetSoundFontsDir updates SoundFontsDir and synchronizes SoundFontFile.
func (p *Paths) SetSoundFontsDir(dir string) {
	p.SoundFontsDir = dir
	p.SoundFontFile = filepath.Join(dir, "GeneralUser-GS.sf2")
}
