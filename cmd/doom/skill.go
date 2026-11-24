package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	doomcli "github.com/lock14/doom-cli"
	"github.com/lock14/doom-cli/internal/fsutil"
)

const (
	embeddedSkillPath = "skills/doom-cli/SKILL.md"
	skillDirName      = "doom-cli"
	skillFileName     = "SKILL.md"
)

var validSkillAgentTargets = []string{
	"agents",
	"antigravity",
	"claude",
	"codex",
	"cursor",
	"gemini",
}

func newSkillCmd() *cobra.Command {
	skillCmd := &cobra.Command{
		Use:          "skill",
		Short:        "Print or install the embedded AI agent skill for doom-cli",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `Print the embedded doom-cli Agent Skill (SKILL.md) to stdout, or install it into
a global or custom AI coding agent skills directory.

Run 'doom skill' with no arguments to write SKILL.md to stdout (similar to
'doom completion'), or run 'doom skill install [agent]' to install it on disk.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, err := doomcli.FS.ReadFile(embeddedSkillPath)
			if err != nil {
				return fmt.Errorf("reading embedded skill: %w", err)
			}
			_, err = cmdOut(cmd).Write(raw)
			return err
		},
	}

	var (
		installDir   string
		installForce bool
	)
	installCmd := &cobra.Command{
		Use:       "install [agent]",
		Short:     "Install the embedded doom-cli SKILL.md into an AI agent skills directory",
		ValidArgs: validSkillAgentTargets,
		Args:      cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := doomcli.FS.ReadFile(embeddedSkillPath)
			if err != nil {
				return fmt.Errorf("reading embedded skill: %w", err)
			}
			destSkillDir, err := resolveSkillDestDir(args, installDir, cmd.Flags().Changed("dir"))
			if err != nil {
				return err
			}
			destFile := filepath.Join(destSkillDir, skillFileName)
			out := cmdOut(cmd)

			info, statErr := os.Stat(destFile)
			if statErr == nil && info.IsDir() {
				return fmt.Errorf("destination %s is a directory", destFile)
			}
			if statErr != nil && !os.IsNotExist(statErr) {
				return statErr
			}
			if statErr == nil && !installForce {
				fmt.Fprintf(
					out,
					"✓ doom-cli skill already installed -> %s (pass --force to overwrite)\n",
					destFile,
				)
				return nil
			}

			bkp, err := fsutil.WriteFileAtomic(destFile, raw, 0o644)
			if err != nil {
				return err
			}
			if bkp != "" {
				fmt.Fprintf(out, "Backing up existing SKILL.md -> %s\n", filepath.Base(bkp))
			}
			fmt.Fprintf(out, "✓ Installed doom-cli skill -> %s\n", destFile)
			return nil
		},
	}
	installCmd.Flags().StringVar(
		&installDir,
		"dir",
		"",
		"Explicit target skills root directory (installs <dir>/doom-cli/SKILL.md)",
	)
	installCmd.Flags().BoolVar(
		&installForce,
		"force",
		false,
		"Overwrite an existing SKILL.md file (creating a timestamped backup)",
	)

	var uninstallDir string
	uninstallCmd := &cobra.Command{
		Use:       "uninstall [agent]",
		Short:     "Remove an installed doom-cli SKILL.md and its backup files",
		ValidArgs: validSkillAgentTargets,
		Args:      cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			destSkillDir, err := resolveSkillDestDir(args, uninstallDir, cmd.Flags().Changed("dir"))
			if err != nil {
				return err
			}
			destFile := filepath.Join(destSkillDir, skillFileName)
			out := cmdOut(cmd)

			info, statErr := os.Lstat(destFile)
			if os.IsNotExist(statErr) {
				fsutil.RemoveBackups(destFile)
				fmt.Fprintf(out, "ℹ doom-cli skill is not installed at %s\n", destFile)
				return nil
			}
			if statErr != nil {
				return statErr
			}
			if info.IsDir() {
				return fmt.Errorf("refusing to remove directory %s", destFile)
			}
			if err := os.Remove(destFile); err != nil {
				return err
			}
			fsutil.RemoveBackups(destFile)
			_ = os.Remove(destSkillDir)
			fmt.Fprintf(out, "✓ Uninstalled doom-cli skill from %s\n", destFile)
			return nil
		},
	}
	uninstallCmd.Flags().StringVar(
		&uninstallDir,
		"dir",
		"",
		"Explicit target skills root directory (removes <dir>/doom-cli/SKILL.md)",
	)

	skillCmd.AddCommand(installCmd, uninstallCmd)
	return skillCmd
}

func resolveSkillDestDir(args []string, customDir string, dirFlagChanged bool) (string, error) {
	if dirFlagChanged {
		if len(args) > 0 {
			return "", errors.New("cannot combine agent target argument with --dir")
		}
		trimmed := strings.TrimSpace(customDir)
		if trimmed == "" {
			return "", errors.New("--dir path cannot be empty")
		}
		return filepath.Join(filepath.Clean(trimmed), skillDirName), nil
	}

	rawTarget := "agents"
	if len(args) > 0 {
		rawTarget = strings.TrimSpace(args[0])
	}
	cleanTarget := strings.ToLower(rawTarget)

	var relParts []string
	switch cleanTarget {
	case "agents":
		relParts = []string{".agents", "skills", skillDirName}
	case "antigravity":
		relParts = []string{".gemini", "config", "skills", skillDirName}
	case "claude":
		relParts = []string{".claude", "skills", skillDirName}
	case "codex":
		relParts = []string{".codex", "skills", skillDirName}
	case "cursor":
		relParts = []string{".cursor", "skills", skillDirName}
	case "gemini":
		relParts = []string{".gemini", "skills", skillDirName}
	default:
		return "", fmt.Errorf(
			"unknown agent target %q (choose %s, or pass --dir <path>)",
			rawTarget,
			strings.Join(validSkillAgentTargets, ", "),
		)
	}

	home, err := resolveUserHomeDir()
	if err != nil {
		return "", err
	}
	parts := append([]string{home}, relParts...)
	return filepath.Join(parts...), nil
}

func resolveUserHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return home, nil
	}
	for _, envKey := range []string{"HOME", "USERPROFILE"} {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			return v, nil
		}
	}
	return "", errors.New("cannot resolve user home directory (pass --dir <path> explicitly)")
}
