package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	doomcli "github.com/lock14/doom-cli"
)

func TestSkillCmd_StdoutMatchesEmbedded(t *testing.T) {
	t.Parallel()

	wantBytes, err := doomcli.FS.ReadFile(embeddedSkillPath)
	if err != nil {
		t.Fatalf("reading embedded skill: %v", err)
	}
	if len(wantBytes) == 0 {
		t.Fatal("embedded SKILL.md is empty")
	}

	var out bytes.Buffer
	rootCmd := newRootCmd()
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"skill"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("doom skill failed: %v", err)
	}
	if !bytes.Equal(out.Bytes(), wantBytes) {
		t.Errorf("doom skill stdout (%d bytes) did not match embedded %s (%d bytes)",
			out.Len(), embeddedSkillPath, len(wantBytes))
	}
}

func TestSkillCmd_InstallAndUninstallLifecycle(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	wantBytes, err := doomcli.FS.ReadFile(embeddedSkillPath)
	if err != nil {
		t.Fatalf("reading embedded skill: %v", err)
	}

	targetCases := []struct {
		name     string
		args     []string
		wantFile string
	}{
		{
			name:     "default agents target",
			args:     []string{"skill", "install"},
			wantFile: filepath.Join(homeDir, ".agents", "skills", "doom-cli", "SKILL.md"),
		},
		{
			name:     "claude target",
			args:     []string{"skill", "install", "claude"},
			wantFile: filepath.Join(homeDir, ".claude", "skills", "doom-cli", "SKILL.md"),
		},
		{
			name:     "codex target",
			args:     []string{"skill", "install", "codex"},
			wantFile: filepath.Join(homeDir, ".codex", "skills", "doom-cli", "SKILL.md"),
		},
		{
			name:     "cursor target",
			args:     []string{"skill", "install", "cursor"},
			wantFile: filepath.Join(homeDir, ".cursor", "skills", "doom-cli", "SKILL.md"),
		},
		{
			name:     "gemini target",
			args:     []string{"skill", "install", "gemini"},
			wantFile: filepath.Join(homeDir, ".gemini", "skills", "doom-cli", "SKILL.md"),
		},
		{
			name:     "antigravity target",
			args:     []string{"skill", "install", "antigravity"},
			wantFile: filepath.Join(homeDir, ".gemini", "config", "skills", "doom-cli", "SKILL.md"),
		},
	}

	for _, tc := range targetCases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := newRootCmd()
			cmd.SetOut(&out)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute(%v) error: %v", tc.args, err)
			}
			gotBytes, readErr := os.ReadFile(tc.wantFile)
			if readErr != nil {
				t.Fatalf("reading installed skill %s: %v", tc.wantFile, readErr)
			}
			if !bytes.Equal(gotBytes, wantBytes) {
				t.Errorf("installed skill at %s did not match embedded bytes", tc.wantFile)
			}
		})
	}

	// Custom --dir lifecycle: preserve without --force, overwrite + backup with --force, clean uninstall
	customRoot := filepath.Join(homeDir, "custom-skills")
	customSkillDir := filepath.Join(customRoot, "doom-cli")
	customFile := filepath.Join(customSkillDir, "SKILL.md")

	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"skill", "install", "--dir", customRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install --dir failed: %v", err)
	}

	customEdited := []byte("# Custom local edit\n")
	if err := os.WriteFile(customFile, customEdited, 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-run without --force must preserve local file without creating backups
	out.Reset()
	cmd = newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"skill", "install", "--dir", customRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install --dir without --force failed: %v", err)
	}
	if !strings.Contains(out.String(), "already installed") {
		t.Errorf("expected 'already installed' message, got:\n%s", out.String())
	}
	afterSkip, err := os.ReadFile(customFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterSkip, customEdited) {
		t.Errorf("expected local edit to be preserved without --force, got %q", string(afterSkip))
	}

	// Re-run with --force must create backup and restore embedded SKILL.md
	out.Reset()
	cmd = newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"skill", "install", "--dir", customRoot, "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install --dir --force failed: %v", err)
	}
	if !strings.Contains(out.String(), "Backing up existing SKILL.md") {
		t.Errorf("expected backup message on --force, got:\n%s", out.String())
	}
	afterForce, err := os.ReadFile(customFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterForce, wantBytes) {
		t.Errorf("expected embedded bytes after --force")
	}
	bakMatches, err := filepath.Glob(customFile + ".bak.*")
	if err != nil || len(bakMatches) != 1 {
		t.Fatalf("expected 1 backup file after --force, got %v (err=%v)", bakMatches, err)
	}

	// Uninstall removes SKILL.md, cleans .bak.* files, and removes empty doom-cli directory
	out.Reset()
	cmd = newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"skill", "uninstall", "--dir", customRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("uninstall --dir failed: %v", err)
	}
	if _, statErr := os.Stat(customSkillDir); !os.IsNotExist(statErr) {
		t.Errorf("expected empty skill directory %s to be removed, statErr=%v", customSkillDir, statErr)
	}
	bakAfter, _ := filepath.Glob(customFile + ".bak.*")
	if len(bakAfter) != 0 {
		t.Errorf("expected backups to be removed on uninstall, got %v", bakAfter)
	}

	// Second uninstall is a clean idempotent no-op
	out.Reset()
	cmd = newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"skill", "uninstall", "--dir", customRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("idempotent uninstall --dir failed: %v", err)
	}
	if !strings.Contains(out.String(), "not installed") {
		t.Errorf("expected 'not installed' notice on second uninstall, got:\n%s", out.String())
	}
}

func TestSkillCmd_ValidationAndRootSkillFlagIsolation(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("XDG_CONFIG_HOME", homeDir)
	t.Setenv("XDG_DATA_HOME", homeDir)
	t.Setenv("DOOM_CONFIG_DIR", filepath.Join(homeDir, "doom-cli"))

	errCases := []struct {
		name    string
		args    []string
		wantSub string
	}{
		{
			name:    "unknown agent target",
			args:    []string{"skill", "install", "unknown-agent"},
			wantSub: "unknown agent target",
		},
		{
			name:    "mutually exclusive target and --dir on install",
			args:    []string{"skill", "install", "claude", "--dir", filepath.Join(homeDir, "x")},
			wantSub: "cannot combine agent target argument with --dir",
		},
		{
			name:    "empty --dir rejected",
			args:    []string{"skill", "install", "--dir", "   "},
			wantSub: "--dir path cannot be empty",
		},
		{
			name:    "extra positional arg on bare doom skill rejected",
			args:    []string{"skill", "extra"},
			wantSub: "unknown command",
		},
	}

	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCmd()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Execute(%v) error = %v, want substring %q", tc.args, err, tc.wantSub)
			}
		})
	}

	// Verify engine '-skill 4' flag on root 'doom' still routes to game launch synthesis
	wadsDir := filepath.Join(homeDir, "games", "doom")
	binDir := filepath.Join(homeDir, ".local", "bin")
	for _, d := range []string{wadsDir, binDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(wadsDir, "DOOM.WAD"), []byte("IWAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(binDir, "dsda-doom")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Ensure Windows also finds dsda-doom.exe in binDir
	if err := os.WriteFile(fakeBin+".exe", []byte("MZ"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, kv := range [][2]string{
		{"wads-dir", wadsDir},
		{"bin-dir", binDir},
	} {
		cfgCmd := newRootCmd()
		cfgCmd.SetOut(new(bytes.Buffer))
		cfgCmd.SetArgs([]string{"config", "set", kv[0], kv[1]})
		if err := cfgCmd.Execute(); err != nil {
			t.Fatalf("config set %s failed: %v", kv[0], err)
		}
	}

	var launchOut bytes.Buffer
	launchCmd := newRootCmd()
	launchCmd.SetOut(&launchOut)
	launchCmd.SetArgs([]string{"-skill", "4", "--dry-run"})
	if err := launchCmd.Execute(); err != nil {
		t.Fatalf("doom -skill 4 --dry-run failed: %v", err)
	}
	if !strings.Contains(launchOut.String(), "-skill 4") {
		t.Errorf("expected '-skill 4' in dry-run launch output, got:\n%s", launchOut.String())
	}
}
