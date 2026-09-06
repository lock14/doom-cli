package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveForLinux(t *testing.T) {
	p := ResolveFor("linux", "")
	if !strings.Contains(filepath.ToSlash(p.UZDoomDir), ".config/uzdoom") {
		t.Errorf("expected .config/uzdoom in UZDoomDir, got %s", p.UZDoomDir)
	}
	if !strings.Contains(filepath.ToSlash(p.DSDADir), ".local/share/dsda-doom") {
		t.Errorf("expected .local/share/dsda-doom in DSDADir, got %s", p.DSDADir)
	}
	if !strings.Contains(filepath.ToSlash(p.WadsDir), ".local/share/games/uzdoom") {
		t.Errorf("expected .local/share/games/uzdoom in WadsDir, got %s", p.WadsDir)
	}
	if !strings.Contains(filepath.ToSlash(p.ConfigDir), ".config/doom-cli") {
		t.Errorf("expected .config/doom-cli in ConfigDir, got %s", p.ConfigDir)
	}
	if !strings.Contains(filepath.ToSlash(p.ConfigFile), "config.json") {
		t.Errorf("expected config.json in ConfigFile, got %s", p.ConfigFile)
	}
}

func TestResolveForDarwin(t *testing.T) {
	p := ResolveFor("darwin", "")
	if !strings.Contains(filepath.ToSlash(p.UZDoomDir), "Library/Application Support/uzdoom") {
		t.Errorf("expected Library/Application Support/uzdoom in UZDoomDir, got %s", p.UZDoomDir)
	}
	if !strings.Contains(filepath.ToSlash(p.WadsDir), "Library/Application Support/games/uzdoom") {
		t.Errorf("expected Library/Application Support/games/uzdoom in WadsDir, got %s", p.WadsDir)
	}
	if !strings.Contains(filepath.ToSlash(p.ConfigDir), "Library/Application Support/doom-cli") {
		t.Errorf("expected Library/Application Support/doom-cli in ConfigDir, got %s", p.ConfigDir)
	}
}

func TestResolveForWindows(t *testing.T) {
	p := ResolveFor("windows", "")
	if !strings.Contains(filepath.ToSlash(p.UZDoomDir), "Games/Doom/bin") {
		t.Errorf("expected Games/Doom/bin in UZDoomDir, got %s", p.UZDoomDir)
	}
	if !strings.Contains(filepath.ToSlash(p.DSDADir), "Games/Doom/bin") {
		t.Errorf("expected Games/Doom/bin in DSDADir, got %s", p.DSDADir)
	}
	if !strings.Contains(filepath.ToSlash(p.BinDir), "Games/Doom/bin") {
		t.Errorf("expected Games/Doom/bin in BinDir, got %s", p.BinDir)
	}
	if !strings.Contains(filepath.ToSlash(p.WadsDir), "Games/Doom/wads") {
		t.Errorf("expected Games/Doom/wads in WadsDir, got %s", p.WadsDir)
	}
	if !strings.Contains(filepath.ToSlash(p.SoundFontsDir), "Games/Doom/soundfonts") {
		t.Errorf("expected Games/Doom/soundfonts in SoundFontsDir, got %s", p.SoundFontsDir)
	}
	if !strings.Contains(filepath.ToSlash(p.ConfigDir), "doom-cli") {
		t.Errorf("expected doom-cli in ConfigDir, got %s", p.ConfigDir)
	}
}

func TestCustomWadsDirOverride(t *testing.T) {
	custom := "/my/custom/doom/wads"
	p := ResolveFor("linux", custom)
	if p.WadsDir != custom {
		t.Errorf("expected custom WadsDir %s, got %s", custom, p.WadsDir)
	}
}

func TestPaths_Setters(t *testing.T) {
	p := &Paths{}
	p.SetBinDir("/opt/doom/bin")
	if p.BinDir != "/opt/doom/bin" {
		t.Errorf("expected BinDir /opt/doom/bin, got %s", p.BinDir)
	}
	if runtime.GOOS == "windows" {
		if p.UZDoomDir != "/opt/doom/bin" || p.DSDADir != "/opt/doom/bin" {
			t.Errorf("expected engine dirs to sync on windows")
		}
	}

	p.SetSoundFontsDir("/opt/doom/soundfonts")
	if p.SoundFontsDir != "/opt/doom/soundfonts" {
		t.Errorf("expected SoundFontsDir /opt/doom/soundfonts, got %s", p.SoundFontsDir)
	}
	expectedSF := filepath.Join("/opt/doom/soundfonts", "GeneralUser-GS.sf2")
	if p.SoundFontFile != expectedSF {
		t.Errorf("expected SoundFontFile %s, got %s", expectedSF, p.SoundFontFile)
	}
}
