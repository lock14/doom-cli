package config

import (
	"path/filepath"
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
	if !strings.Contains(p.DoomRunnerRoam, "AppData") && !strings.Contains(p.DoomRunnerRoam, "DoomRunner") {
		t.Errorf("expected AppData/DoomRunner in DoomRunnerRoam, got %s", p.DoomRunnerRoam)
	}
	if !strings.Contains(p.WadsDir, "Doom WADS") {
		t.Errorf("expected Doom WADS in WadsDir, got %s", p.WadsDir)
	}
	if !strings.Contains(p.ConfigDir, "doom-cli") {
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
