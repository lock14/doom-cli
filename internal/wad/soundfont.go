package wad

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lock14/doom-cli/internal/fsutil"
)

// DefaultSoundFontURL is the pinned commit mirror URL for the GeneralUser GS SoundFont.
const DefaultSoundFontURL = "https://raw.githubusercontent.com/mrbumpy409/GeneralUser-GS/" +
	"684543d5e5efaef08d02be50dcda8d552478fa60/GeneralUser-GS.sf2"

// DefaultSoundFontSHA256 is the expected SHA-256 hex digest of DefaultSoundFontURL.
const DefaultSoundFontSHA256 = "9575028c7a1f589f5770fccc8cff2734566af40cd26ed836944e9a5152688cfe"

// DefaultSoundFontFile is the default filename of the GeneralUser GS SoundFont.
const DefaultSoundFontFile = "GeneralUser-GS.sf2"

// InstallSoundFont downloads and installs the SHA-256 verified Roland SC-55 compatible
// SoundFont from DefaultSoundFontURL.
func InstallSoundFont(destDir string, force bool, out io.Writer) (string, error) {
	return InstallSoundFontContext(context.Background(), destDir, force, out)
}

// InstallSoundFontContext downloads and installs the SHA-256 verified Roland SC-55
// compatible SoundFont under ctx from DefaultSoundFontURL.
func InstallSoundFontContext(
	ctx context.Context,
	destDir string,
	force bool,
	out io.Writer,
) (string, error) {
	return installSoundFontVerified(ctx, DefaultSoundFontURL, DefaultSoundFontSHA256, destDir, force, out)
}

// InstallSoundFontURL downloads and installs a SoundFont from a custom URL.
func InstallSoundFontURL(url, destDir string, force bool, out io.Writer) (string, error) {
	return installSoundFontVerified(context.Background(), url, "", destDir, force, out)
}

func installSoundFontVerified(
	ctx context.Context,
	url, expectedSHA256, destDir string,
	force bool,
	out io.Writer,
) (string, error) {
	if out == nil {
		out = io.Discard
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create soundfonts directory %s: %w", destDir, err)
	}

	targetPath, err := fsutil.SafeJoin(destDir, DefaultSoundFontFile)
	if err != nil {
		return "", err
	}
	if !force {
		if fi, statErr := os.Stat(targetPath); statErr == nil && fi.Size() > 0 {
			fmt.Fprintf(out, "✓ SoundFont already installed: %s\n  (Use --force to re-download)\n", targetPath)
			return targetPath, nil
		}
	}

	fmt.Fprintf(out, "Downloading GeneralUser-GS SoundFont (Roland SC-55 Balanced GM)...\n")
	client := &http.Client{Timeout: 3 * time.Minute}
	written, err := fsutil.DownloadVerifiedContext(ctx, client, url, targetPath, expectedSHA256, 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to download soundfont from %s: %w", url, err)
	}

	fmt.Fprintf(out, "✓ Successfully installed GeneralUser-GS.sf2 (%d bytes) to:\n  %s\n", written, targetPath)
	return targetPath, nil
}
