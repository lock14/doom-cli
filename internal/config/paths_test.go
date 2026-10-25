package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveForLinux(t *testing.T) {
	t.Parallel()
	p := ResolveFor("linux")
	if !strings.Contains(filepath.ToSlash(p.UZDoomDir), ".config/uzdoom") {
		t.Errorf("expected .config/uzdoom in UZDoomDir, got %s", p.UZDoomDir)
	}
	if !strings.Contains(filepath.ToSlash(p.DSDADir), ".local/share/dsda-doom") {
		t.Errorf("expected .local/share/dsda-doom in DSDADir, got %s", p.DSDADir)
	}
	if !strings.Contains(filepath.ToSlash(p.WadsDir), ".local/share/games/doom") {
		t.Errorf("expected .local/share/games/doom in WadsDir, got %s", p.WadsDir)
	}
	if !strings.Contains(filepath.ToSlash(p.ConfigDir), ".config/doom-cli") {
		t.Errorf("expected .config/doom-cli in ConfigDir, got %s", p.ConfigDir)
	}
	if !strings.Contains(filepath.ToSlash(p.ConfigFile), "config.json") {
		t.Errorf("expected config.json in ConfigFile, got %s", p.ConfigFile)
	}
	if !strings.Contains(filepath.ToSlash(p.CatalogDir), "catalog.d") {
		t.Errorf("expected catalog.d in CatalogDir, got %s", p.CatalogDir)
	}
}

func TestResolveForDarwin(t *testing.T) {
	t.Parallel()
	p := ResolveFor("darwin")
	if !strings.Contains(filepath.ToSlash(p.UZDoomDir), "Library/Application Support/uzdoom") {
		t.Errorf("expected Library/Application Support/uzdoom in UZDoomDir, got %s", p.UZDoomDir)
	}
	if !strings.Contains(filepath.ToSlash(p.WadsDir), "Library/Application Support/games/doom") {
		t.Errorf("expected Library/Application Support/games/doom in WadsDir, got %s", p.WadsDir)
	}
	if !strings.Contains(filepath.ToSlash(p.ConfigDir), "Library/Application Support/doom-cli") {
		t.Errorf("expected Library/Application Support/doom-cli in ConfigDir, got %s", p.ConfigDir)
	}
}

func TestResolveForWindows(t *testing.T) {
	t.Parallel()
	p := ResolveFor("windows")
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

func TestPaths_SettersAndWadRoots(t *testing.T) {
	t.Parallel()
	p := &Paths{
		WadsDir:  "/wads/primary",
		WadsDirs: []string{"/wads/secondary", "/wads/primary", "  ", "/wads/tertiary"},
	}
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

	roots := p.WadRoots()
	if len(roots) != 3 || roots[0] != "/wads/primary" || roots[1] != "/wads/secondary" || roots[2] != "/wads/tertiary" {
		t.Fatalf("unexpected WadRoots: %v", roots)
	}
}

func TestParsePathList(t *testing.T) {
	t.Parallel()
	got := ParsePathList(" /a/one , /b/two, /a/one ,, ")
	if len(got) != 2 || got[0] != "/a/one" || got[1] != "/b/two" {
		t.Fatalf("ParsePathList got %v", got)
	}
}

func TestResolveFor_DOOMConfigDirAndIgnoredEnvVars(t *testing.T) {
	customCfgDir := filepath.Join(t.TempDir(), "custom-doom-cli")
	t.Setenv("DOOM_CONFIG_DIR", customCfgDir)
	t.Setenv("DOOM_WADS_DIR", "/ignored/env/wads")
	t.Setenv("DOOM_WADS_DIRS", "/ignored/env/extra1,/ignored/env/extra2")
	t.Setenv("DOOM_BIN_DIR", "/ignored/env/bin")
	t.Setenv("DOOM_SF_DIR", "/ignored/env/sf")

	p := ResolveFor("linux")
	if p.ConfigDir != customCfgDir {
		t.Errorf("expected ConfigDir=%q from DOOM_CONFIG_DIR, got %q", customCfgDir, p.ConfigDir)
	}
	if p.ConfigFile != filepath.Join(customCfgDir, "config.json") {
		t.Errorf("expected ConfigFile under DOOM_CONFIG_DIR, got %q", p.ConfigFile)
	}
	if p.CatalogDir != filepath.Join(customCfgDir, "catalog.d") {
		t.Errorf("expected CatalogDir under DOOM_CONFIG_DIR, got %q", p.CatalogDir)
	}
	if p.ThemesDir != filepath.Join(customCfgDir, "themes") {
		t.Errorf("expected ThemesDir under DOOM_CONFIG_DIR, got %q", p.ThemesDir)
	}
	if p.WadsDir == "/ignored/env/wads" {
		t.Errorf("expected DOOM_WADS_DIR to be ignored, got WadsDir=%q", p.WadsDir)
	}
	if len(p.WadsDirs) != 0 {
		t.Errorf("expected DOOM_WADS_DIRS to be ignored, got WadsDirs=%v", p.WadsDirs)
	}
	if p.BinDir == "/ignored/env/bin" {
		t.Errorf("expected DOOM_BIN_DIR to be ignored, got BinDir=%q", p.BinDir)
	}
	if p.SoundFontsDir == "/ignored/env/sf" {
		t.Errorf("expected DOOM_SF_DIR to be ignored, got SoundFontsDir=%q", p.SoundFontsDir)
	}
}
