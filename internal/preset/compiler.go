package preset

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lock14/doom-cli/internal/fsutil"
)

var readmeTableRegex = regexp.MustCompile(
	`(?i)(## Preconfigured presets\n\n[\s\S]+?\n\n)(\|[\s\S]+?)(\n\n(?:---|## ))`,
)

// GenerateReadmeTable constructs the markdown table for README.md using each
// preset's resolved engine display name, genre, difficulty, quality, and primary link from the catalog.
func GenerateReadmeTable(cat *Catalog) string {
	lines := []string{
		"| Megawad or expansion | Engine | Genre | Difficulty | Quality | Compatibility and details |",
		"| :--- | :--- | :--- | :--- | :--- | :--- |",
	}
	if cat == nil {
		return strings.Join(lines, "\n")
	}
	for _, p := range cat.Presets {
		if p.IsCustom() {
			continue
		}
		nameCell := p.Name
		if primaryURL := p.PrimaryURL(); primaryURL != "" {
			nameCell = fmt.Sprintf("[%s](%s)", p.Name, primaryURL)
		}
		engDisplay := cat.EngineDisplayName(p.Engine)
		lines = append(
			lines,
			fmt.Sprintf(
				"| %s | %s | %s | %s | %s | %s |",
				nameCell, engDisplay, p.Genre, p.Difficulty, p.Quality, p.Description,
			),
		)
	}
	return strings.Join(lines, "\n")
}

// SyncReadme compiles on-disk catalog/*.json in rootDir into the README.md preset table.
func SyncReadme(rootDir string) error {
	if strings.TrimSpace(rootDir) == "" {
		rootDir = "."
	}
	catalogDir := filepath.Join(rootDir, "catalog")
	cat, err := LoadCatalogFromDir(catalogDir)
	if err != nil {
		return fmt.Errorf("loading curated catalog from %s: %w", catalogDir, err)
	}

	readmeFile := filepath.Join(rootDir, "README.md")
	content, err := os.ReadFile(readmeFile)
	if err != nil {
		return err
	}

	text := string(content)
	loc := readmeTableRegex.FindStringSubmatchIndex(text)
	if loc == nil {
		return fmt.Errorf("could not locate '## Preconfigured presets' table in %s", readmeFile)
	}

	table := GenerateReadmeTable(cat)
	updated := text[:loc[4]] + table + text[loc[5]:]
	_, err = fsutil.WriteFileAtomic(readmeFile, []byte(updated), 0o644)
	if err == nil {
		// Remove any .bak file created for README.md in the git checkout
		fsutil.RemoveBackups(readmeFile)
	}
	return err
}
