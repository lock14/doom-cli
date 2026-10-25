package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/preset"
	"github.com/lock14/doom-cli/internal/steam"
	"github.com/lock14/doom-cli/internal/wad"
)

// newWadsCmd creates the WAD, megawad, and expansion management command.
func newWadsCmd() *cobra.Command {
	wadsCmd := &cobra.Command{
		Use:   "wads",
		Short: "Manage WADs, megawads, and official expansions",
	}

	var fetchForce bool
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

			dl := wad.NewDownloader(paths.WadRoots(), fetchForce, cmdOut(cmd))
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
	wadsFetchCmd.Flags().BoolVar(&fetchForce, "force", false, "Re-download and overwrite existing files")

	wadsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List downloadable community megawads",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			dirIdx := preset.NewDirIndex(paths.WadRoots()...).WithCatalog(cat)
			out := cmdOut(cmd)
			fmt.Fprintln(out, "=== Available Downloadable Community Megawads ===")
			for _, p := range cat.Presets {
				if len(p.Sources) == 0 {
					continue
				}
				var fileStatuses []string
				for _, f := range p.Files {
					if f.Optional {
						continue
					}
					resolved, ok := dirIdx.ResolveFile(f.Name)
					if !ok {
						for _, al := range f.Aliases {
							if resolved, ok = dirIdx.ResolveFile(al); ok {
								break
							}
						}
					}
					if ok {
						fileStatuses = append(fileStatuses, fmt.Sprintf("%s [✓ Found: %s]", f.Name, resolved))
					} else {
						fileStatuses = append(fileStatuses, fmt.Sprintf("%s [✗ Missing]", f.Name))
					}
				}
				fmt.Fprintf(out, "  - %-30s [%s]\n", p.Name, strings.Join(fileStatuses, ", "))
			}
			return nil
		},
	}

	var extractForce bool
	wadsExtractCmd := &cobra.Command{
		Use:   "extract",
		Short: "Auto-discover and copy official IWADs and expansions from Steam and GOG",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _, cat, err := loadAppState()
			if err != nil {
				return err
			}
			_, err = steam.DiscoverAndExtract(
				cat, nil, paths.WadsDir, extractForce, cmdOut(cmd), paths.WadsDirs...,
			)
			return err
		},
	}
	wadsExtractCmd.Flags().BoolVar(&extractForce, "force", false, "Overwrite existing files in destination")

	wadsCmd.AddCommand(wadsFetchCmd, wadsListCmd, wadsExtractCmd)
	return wadsCmd
}
