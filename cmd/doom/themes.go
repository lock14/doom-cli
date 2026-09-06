package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/config"
	"github.com/lock14/doom-cli/internal/tui"
)

// newThemesCmd creates the theme browsing and selection command.
func newThemesCmd() *cobra.Command {
	themesCmd := &cobra.Command{
		Use:   "themes",
		Short: "Browse, preview, and set color themes for the launcher",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThemesList(cmd, args)
		},
	}
	themesListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all available color themes with visual swatches",
		RunE:  runThemesList,
	}
	themesSetCmd := &cobra.Command{
		Use:   "set <theme_name>",
		Short: "Set the default color theme in user config",
		Args:  cobra.ExactArgs(1),
		RunE:  runThemesSet,
	}
	themesCmd.AddCommand(themesListCmd, themesSetCmd)
	return themesCmd
}

func runThemesList(cmd *cobra.Command, args []string) error {
	paths := getPaths()
	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	activeTheme := tui.ResolveTheme(flagTheme, os.Getenv("DOOM_THEME"), cfg.Theme, paths.ThemesDir)
	nerdFonts := flagNerdFonts || cfg.NerdFonts

	fmt.Println("=== Available Doom Themes ===")
	fmt.Println()

	themes := tui.ListBuiltinThemes()
	for _, t := range themes {
		marker := "  "
		if strings.EqualFold(t.Name, activeTheme.Name) {
			marker = "* "
		}

		styles := tui.CompileStyles(t)
		sample := styles.RenderBrandPill(nerdFonts) + " " +
			styles.TagDSDA.Render("[DSDA]") + " " +
			styles.TagUZDoom.Render("[UZDoom]") + " " +
			styles.CursorBar.Render("▎ ") +
			styles.CursorText.Render("Select")

		fmt.Printf("%s%-11s [%-9s] %-42s %s\n", marker, t.Name, t.Type, t.Description, sample)
	}

	fmt.Println()
	fmt.Printf("Active theme: %s\n", activeTheme.Name)
	fmt.Printf("Run 'doom themes set <name>' to save a default theme to %s\n", paths.ConfigFile)
	return nil
}

func runThemesSet(cmd *cobra.Command, args []string) error {
	target := strings.TrimSpace(args[0])
	paths := getPaths()

	if _, ok := tui.GetBuiltinTheme(target); !ok {
		customFile := filepath.Join(paths.ThemesDir, target+".json")
		if _, err := os.Stat(customFile); err != nil {
			if _, errPath := os.Stat(target); errPath != nil {
				return fmt.Errorf("unknown theme %q. Run 'doom themes list' to see available themes", target)
			}
		}
	}

	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cfg.Theme = target
	if err := config.SaveConfig(paths, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("✓ Active theme set to %q in %s\n", target, paths.ConfigFile)
	return nil
}
