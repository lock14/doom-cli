package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/wad"
)

// newWadsCmd creates the WAD, megawad, and expansion management command.
func newWadsCmd() *cobra.Command {
	wadsCmd := &cobra.Command{
		Use:   "wads",
		Short: "Manage WADs, megawads, and official expansions",
	}

	wadsFetchCmd := &cobra.Command{
		Use:   "fetch [preset_name|all]",
		Short: "Download community megawad files",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			cat, err := getCatalog()
			if err != nil {
				return err
			}

			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			dl := wad.NewDownloader(paths.WadsDir, flagForce, os.Stdout)
			if strings.EqualFold(target, "all") {
				return dl.DownloadAll(cat)
			}

			p := cat.Find(target)
			if p == nil {
				return fmt.Errorf("preset %q not found", target)
			}
			return dl.DownloadPreset(*p)
		},
	}
	wadsFetchCmd.Flags().BoolVar(&flagForce, "force", false, "Re-download and overwrite existing files")

	wadsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List downloadable community megawads",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := getCatalog()
			if err != nil {
				return err
			}
			fmt.Println("=== Available Downloadable Community Megawads ===")
			for _, p := range cat.Presets {
				if len(p.DownloadURLs) > 0 {
					files := wad.FilterExpectedFiles(p.Mappacks)
					fmt.Printf("  - %-30s [%s]\n", p.Name, strings.Join(files, ", "))
				}
			}
			return nil
		},
	}

	wadsExtractCmd := &cobra.Command{
		Use:     "extract-steam",
		Aliases: []string{"extract"},
		Short:   "Auto-discover and copy official IWADs and expansions from Steam/GOG",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			_, err := steam.DiscoverAndExtract(nil, paths.WadsDir, flagForce, os.Stdout)
			return err
		},
	}
	wadsExtractCmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing files in destination")

	wadsCmd.AddCommand(wadsFetchCmd, wadsListCmd, wadsExtractCmd)
	return wadsCmd
}
