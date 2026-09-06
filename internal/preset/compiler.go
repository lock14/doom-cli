// Package preset manages the declarative preset catalog and documentation synchronization.
package preset

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GenerateReadmeTable constructs the markdown table for README.md.
func GenerateReadmeTable(cat *Catalog) string {
	lines := []string{
		"| Megawad / Expansion | Engine | Compatibility / Details |",
		"| :--- | :--- | :--- |",
	}
	for _, p := range cat.Presets {
		engDisplay := "DSDA-Doom"
		if p.Engine == "uzdoom" {
			engDisplay = "UZDoom"
		}
		lines = append(lines, fmt.Sprintf("| **%s** | %s | %s |", p.Name, engDisplay, p.Description))
	}
	return strings.Join(lines, "\n")
}

// SyncReadme synchronizes data/presets.json into README.md.
func SyncReadme(rootDir string) error {
	cat, err := LoadCatalog(filepath.Join(rootDir, "data", "presets.json"))
	if err != nil {
		return err
	}

	readmeFile := filepath.Join(rootDir, "README.md")
	content, err := os.ReadFile(readmeFile)
	if err != nil {
		return err
	}

	table := GenerateReadmeTable(cat)
	pattern := regexp.MustCompile(`(## Preconfigured Presets\n\n[^\n]+\n\n)(\|[\s\S]+?)(\n\n---)`)
	updated := pattern.ReplaceAllString(string(content), fmt.Sprintf("${1}%s${3}", table))
	return os.WriteFile(readmeFile, []byte(updated), 0o644)
}

// CompileOptionsFiles is retained for backward compatibility, delegating to SyncReadme.
func CompileOptionsFiles(rootDir string) error {
	return SyncReadme(rootDir)
}
