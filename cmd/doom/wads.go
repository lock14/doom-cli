package main

import (
	"fmt"
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
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}

			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			dl := wad.NewDownloaderWithRoots(paths.WadRoots(), flagForce, cmdOut(cmd))
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
			out := cmdOut(cmd)
			fmt.Fprintln(out, "=== Available Downloadable Community Megawads ===")
			for _, p := range cat.Presets {
				if len(p.Sources) > 0 {
					files := wad.FilterExpectedFiles(p.Files)
					fmt.Fprintf(out, "  - %-30s [%s]\n", p.Name, strings.Join(files, ", "))
				}
			}
			return nil
		},
	}

	wadsExtractCmd := &cobra.Command{
		Use:     "extract-steam",
		Aliases: []string{"extract"},
		Short:   "Auto-discover and copy official IWADs and expansions from Steam and GOG",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			_, err = steam.DiscoverAndExtractForCatalog(cat, nil, paths.WadsDir, flagForce, cmdOut(cmd))
			return err
		},
	}
	wadsExtractCmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing files in destination")

	wadsCmd.AddCommand(wadsFetchCmd, wadsListCmd, wadsExtractCmd)
	return wadsCmd
}
