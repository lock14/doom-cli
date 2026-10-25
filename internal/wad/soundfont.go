package wad

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lock14/doom-cli/internal/fsutil"
)

// DefaultSoundFontURL is the primary mirror URL for the GeneralUser GS SoundFont.
const DefaultSoundFontURL = "https://raw.githubusercontent.com/mrbumpy409/GeneralUser-GS/main/GeneralUser-GS.sf2"

// DefaultSoundFontFile is the default filename of the GeneralUser GS SoundFont.
const DefaultSoundFontFile = "GeneralUser-GS.sf2"

// InstallSoundFont downloads and installs the Roland SC-55 compatible SoundFont from DefaultSoundFontURL.
func InstallSoundFont(destDir string, force bool, out io.Writer) (string, error) {
	return InstallSoundFontURL(DefaultSoundFontURL, destDir, force, out)
}

// InstallSoundFontURL downloads and installs a SoundFont from the given URL.
func InstallSoundFontURL(url, destDir string, force bool, out io.Writer) (string, error) {
	if out == nil {
		out = io.Discard
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create soundfonts directory %s: %w", destDir, err)
	}

	targetPath := filepath.Join(destDir, DefaultSoundFontFile)
	if !force {
		if fi, err := os.Stat(targetPath); err == nil && fi.Size() > 0 {
			fmt.Fprintf(out, "✓ SoundFont already installed: %s\n  (Use --force to re-download)\n", targetPath)
			return targetPath, nil
		}
	}

	fmt.Fprintf(out, "Downloading GeneralUser-GS SoundFont (Roland SC-55 Balanced GM)...\n")
	client := &http.Client{Timeout: 3 * time.Minute}
	written, err := fsutil.DownloadVerified(client, url, targetPath, "", 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to download soundfont from %s: %w", url, err)
	}

	fmt.Fprintf(out, "✓ Successfully installed GeneralUser-GS.sf2 (%d bytes) to:\n  %s\n", written, targetPath)
	return targetPath, nil
}
