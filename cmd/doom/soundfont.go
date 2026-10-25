package main

import (
	"github.com/spf13/cobra"

	"github.com/lock14/doom-cli/internal/wad"
)

// newSoundfontCmd creates the SoundFont management command.
func newSoundfontCmd() *cobra.Command {
	sfCmd := &cobra.Command{
		Use:   "soundfont",
		Short: "Manage MIDI SoundFonts for FluidSynth",
	}
	var force bool
	sfInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Download and deploy Roland SC-55 compatible GeneralUser-GS SoundFont",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := getPaths()
			_, err := wad.InstallSoundFont(paths.SoundFontsDir, force, cmdOut(cmd))
			return err
		},
	}
	sfInstallCmd.Flags().BoolVar(&force, "force", false, "Re-download and overwrite existing SoundFont")
	sfCmd.AddCommand(sfInstallCmd)
	return sfCmd
}
