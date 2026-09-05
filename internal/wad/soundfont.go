package wad

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
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
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download soundfont from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("soundfont download failed with HTTP status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp(destDir, "soundfont.*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	written, err := io.Copy(tmpFile, resp.Body)
	_ = tmpFile.Close()
	if err != nil {
		return "", fmt.Errorf("failed writing soundfont: %w", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		// Fallback to copy if cross-device link error
		data, readErr := os.ReadFile(tmpName)
		if readErr != nil {
			return "", fmt.Errorf("failed to move soundfont into place: %w", err)
		}
		if writeErr := os.WriteFile(targetPath, data, 0644); writeErr != nil {
			return "", fmt.Errorf("failed writing soundfont: %w", writeErr)
		}
	}
	_ = os.Chmod(targetPath, 0644)

	fmt.Fprintf(out, "✓ Successfully installed GeneralUser-GS.sf2 (%d bytes) to:\n  %s\n", written, targetPath)
	return targetPath, nil
}
