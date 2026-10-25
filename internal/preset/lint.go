package preset

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/lock14/doom-cli"
)

var (
	personalPathRegex = regexp.MustCompile(
		`(?i)(/home/[a-z0-9_-]+|/Users/[a-z0-9_-]+|C:\\Users\\|%USERPROFILE%)`,
	)
	docPersonalPathRegex = regexp.MustCompile(
		`(?i)(/home/[a-z0-9_-]+|/Users/[a-z0-9_-]+|C:\\Users\\[a-z0-9_-]+)`,
	)
	decorativeEmojiRegex = regexp.MustCompile(
		`([\x{1F300}-\x{1FAFF}\x{26A1}\x{2705}\x{2728}\x{274C}]|:(?:white_check_mark|x):)`,
	)
	italicAsideRegex = regexp.MustCompile(
		`(?i)(\*|_)\((?:note|tip|alternatively)\b`,
	)
	boldTableCellRegex = regexp.MustCompile(`^\|\s*\*\*[^*]+\*\*\s*\|`)
	htmlCommentRegex   = regexp.MustCompile(`<!--.*?-->`)
	asciiArrowRegex    = regexp.MustCompile(`(<->|->|<-|=>)`)
	headingLinkRegex   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
)

// GitHubHeadingSlug computes the GitHub-flavored Markdown anchor slug for a heading.
func GitHubHeadingSlug(heading string) string {
	stripped := headingLinkRegex.ReplaceAllString(heading, "$1")
	s := strings.ToLower(strings.TrimSpace(stripped))
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// IsSupportedSourceURL reports whether url has a supported downloadable extension (.zip, .wad, .pk3).
func IsSupportedSourceURL(url string) bool {
	lower := strings.ToLower(strings.TrimSpace(url))
	if idx := strings.IndexAny(lower, "?#"); idx >= 0 {
		lower = lower[:idx]
	}
	return strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".wad") ||
		strings.HasSuffix(lower, ".pk3")
}

// HasErrors reports whether any diagnostic in diags has severity "error".
func HasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// LintCatalog validates curated entities (from <rootDir>/catalog if present,
// otherwise embedded), optional user entities in userDir, template files, and
// README.md table synchronization when <rootDir>/README.md exists.
func LintCatalog(rootDir, userDir string) ([]Diagnostic, error) {
	var diags []Diagnostic
	addErr := func(where, format string, a ...any) {
		diags = append(diags, Diagnostic{
			Path:     where,
			Severity: "error",
			Message:  fmt.Sprintf(format, a...),
		})
	}
	addWarn := func(where, format string, a ...any) {
		diags = append(diags, Diagnostic{
			Path:     where,
			Severity: "warn",
			Message:  fmt.Sprintf(format, a...),
		})
	}

	var curatedFS fs.FS
	curatedPrefix := "catalog"
	hasDiskCatalog := false
	if strings.TrimSpace(rootDir) != "" {
		diskCat := filepath.Join(rootDir, "catalog")
		if info, err := os.Stat(diskCat); err == nil && info.IsDir() {
			curatedFS = os.DirFS(diskCat)
			curatedPrefix = filepath.ToSlash(diskCat)
			hasDiskCatalog = true
		}
	}
	if curatedFS == nil {
		sub, err := fs.Sub(doomcli.FS, "catalog")
		if err != nil {
			return nil, err
		}
		curatedFS = sub
	}

	curatedEnts, curatedDiags, _ := scanEntityFS(curatedFS, dirScanOptions{
		displayPrefix:      curatedPrefix,
		strict:             false,
		checkPersonalPaths: true,
		severity:           "error",
	})
	diags = append(diags, curatedDiags...)

	var userEnts []Entity
	if strings.TrimSpace(userDir) != "" {
		if info, err := os.Stat(userDir); err == nil && info.IsDir() {
			uEnts, uDiags, _ := scanEntityFS(os.DirFS(userDir), dirScanOptions{
				displayPrefix:      filepath.ToSlash(userDir),
				strict:             false,
				checkPersonalPaths: true,
				severity:           "error",
			})
			userEnts = uEnts
			diags = append(diags, uDiags...)
		}
	}

	engineIDs := make(map[string]string)
	presetIDs := make(map[string]string)
	presetIWADs := make(map[string]string)
	sortOwners := make(map[int]string)
	fileAliasTargets := make(map[string]string)
	canonicalFiles := make(map[string]string)

	type taggedEnt struct {
		ent     Entity
		curated bool
	}
	var all []taggedEnt
	for _, e := range curatedEnts {
		all = append(all, taggedEnt{ent: e, curated: true})
	}
	for _, e := range userEnts {
		all = append(all, taggedEnt{ent: e, curated: false})
	}

	for _, te := range all {
		if te.ent.Preset == nil {
			continue
		}
		for _, f := range te.ent.Preset.Files {
			base := filepath.Base(strings.TrimSpace(f.Name))
			if norm := NormalizeFilename(base); norm != "" {
				if _, exists := canonicalFiles[norm]; !exists {
					canonicalFiles[norm] = base
				}
			}
		}
	}

	validateFiles := func(where, ownerID, iwad string, files []FileSpec) {
		iwadNorm := NormalizeFilename(iwad)
		for _, f := range files {
			if strings.TrimSpace(f.Name) == "" {
				addErr(where, "preset %s has an empty file name", ownerID)
				continue
			}
			fileNorm := NormalizeFilename(filepath.Base(f.Name))
			if iwadNorm != "" && fileNorm == iwadNorm {
				addErr(where, "iwad %q must not be repeated in files", iwad)
			}
			seenAliases := make(map[string]bool)
			for _, al := range f.Aliases {
				trimmedAl := strings.TrimSpace(al)
				if trimmedAl == "" {
					addErr(where, "file %s has an empty alias", f.Name)
					continue
				}
				if strings.ContainsAny(trimmedAl, `/\`) {
					addErr(where, "file %s alias %q must not contain path separators", f.Name, al)
					continue
				}
				alNorm := NormalizeFilename(trimmedAl)
				if (iwadNorm != "" && alNorm == iwadNorm) || isIWADName(trimmedAl) {
					matchedIWAD := iwad
					if matchedIWAD == "" || alNorm != iwadNorm {
						matchedIWAD = trimmedAl
					}
					addErr(where, "file %s alias %q must not match iwad %q", f.Name, al, matchedIWAD)
				}
				if alNorm == fileNorm {
					addErr(
						where,
						"file %s alias %q is redundant with normalized filename",
						f.Name, al,
					)
				}
				if seenAliases[alNorm] {
					addErr(where, "file %s has duplicate normalized alias %q", f.Name, al)
				}
				seenAliases[alNorm] = true
				if canonName, exists := canonicalFiles[alNorm]; exists && alNorm != fileNorm {
					addErr(
						where,
						"file %s alias %q conflicts with canonical target %q",
						f.Name, al, canonName,
					)
				} else if prevTarget, exists := fileAliasTargets[alNorm]; exists && prevTarget != fileNorm {
					addErr(
						where,
						"file %s alias %q conflicts with canonical target %q",
						f.Name, al, prevTarget,
					)
				} else {
					fileAliasTargets[alNorm] = fileNorm
				}
			}
		}
	}

	for _, te := range all {
		if te.ent.Engine == nil {
			continue
		}
		eng := te.ent.Engine
		where := te.ent.Path
		if prev, exists := engineIDs[eng.ID]; exists && te.curated {
			addErr(where, "duplicate engine id %q (already in %s)", eng.ID, prev)
		}
		engineIDs[eng.ID] = where
		for _, al := range eng.Aliases {
			cleanAl := strings.ToLower(strings.TrimSpace(al))
			if err := ValidateID(cleanAl); err != nil {
				addErr(where, "invalid engine alias %q: %v", al, err)
				continue
			}
			if prev, exists := engineIDs[cleanAl]; exists && prev != where {
				addErr(where, "engine alias %q conflicts with %s", al, prev)
			}
			engineIDs[cleanAl] = where
		}
		if eng.Install != nil {
			for plat, pr := range eng.Install.Platforms {
				if !strings.Contains(plat, "/") {
					addErr(where, "invalid platform key %q (want goos/goarch)", plat)
				}
				for _, pat := range pr.Assets {
					if _, err := regexp.Compile(pat); err != nil {
						addErr(where, "invalid asset regex %q for %s: %v", pat, plat, err)
					}
				}
				if pr.Pinned != nil {
					if !strings.HasPrefix(pr.Pinned.URL, "https://") {
						addErr(where, "pinned URL for %s must start with https://", plat)
					}
					if err := ValidateSHA256(pr.Pinned.SHA256); err != nil {
						addErr(where, "pinned sha256 for %s: %v", plat, err)
					}
				}
			}
		}
	}

	for _, te := range all {
		if te.ent.Preset == nil {
			continue
		}
		p := te.ent.Preset
		where := te.ent.Path
		if prev, exists := presetIDs[p.ID]; exists && te.curated {
			addErr(where, "duplicate preset id %q (already in %s)", p.ID, prev)
		}
		presetIDs[p.ID] = where
		presetIWADs[p.ID] = p.IWAD

		engKey := strings.ToLower(strings.TrimSpace(p.Engine))
		if _, ok := engineIDs[engKey]; !ok {
			addErr(where, "preset %s references unknown engine %q", p.ID, p.Engine)
		}

		validateFiles(where, p.ID, p.IWAD, p.Files)

		for _, s := range p.Sources {
			if !strings.HasPrefix(s.URL, "https://") {
				addErr(where, "source URL %q must use https://", s.URL)
			}
			if !IsSupportedSourceURL(s.URL) {
				addErr(where, "source URL %q must end with .zip, .wad, or .pk3", s.URL)
			}
			if err := ValidateSHA256(s.SHA256); err != nil {
				addErr(where, "source %s: %v", s.URL, err)
			}
		}

		for _, l := range p.Links {
			if !strings.HasPrefix(l.URL, "https://") && !strings.HasPrefix(l.URL, "http://") {
				addErr(where, "link URL %q must use http:// or https://", l.URL)
			}
		}

		if te.curated {
			if strings.TrimSpace(p.Author) == "" ||
				strings.TrimSpace(p.ReleaseDate) == "" ||
				strings.TrimSpace(p.Category) == "" {
				addErr(where, "curated preset %s requires author, release_date, and category", p.ID)
			}
			if p.Sort <= 0 {
				addWarn(where, "curated preset %s has no positive sort order", p.ID)
			} else if prev, dup := sortOwners[p.Sort]; dup {
				addErr(where, "sort %d on preset %s collides with %s", p.Sort, p.ID, prev)
			} else {
				sortOwners[p.Sort] = p.ID
			}
		}
	}

	for _, te := range all {
		if te.ent.Override == nil {
			continue
		}
		ov := te.ent.Override
		where := te.ent.Path
		switch ov.Target.Kind {
		case KindPreset:
			if _, ok := presetIDs[ov.Target.ID]; !ok {
				addErr(where, "override target preset/%s does not exist", ov.Target.ID)
			}
			ovIWAD := presetIWADs[ov.Target.ID]
			if ov.Set != nil {
				if ov.Set.IWAD != nil && *ov.Set.IWAD != "" {
					ovIWAD = *ov.Set.IWAD
				}
				if ov.Set.Files != nil {
					validateFiles(where, ov.Target.ID, ovIWAD, *ov.Set.Files)
				}
			}
			if len(ov.AppendFiles) > 0 {
				validateFiles(where, ov.Target.ID, ovIWAD, ov.AppendFiles)
			}
			for _, ex := range ov.ExcludeFiles {
				if strings.TrimSpace(ex) == "" {
					addErr(where, "preset %s has an empty exclude_files entry", ov.Target.ID)
				}
			}
		case KindEngine:
			if _, ok := engineIDs[ov.Target.ID]; !ok {
				addErr(where, "override target engine/%s does not exist", ov.Target.ID)
			}
		}
		if ov.Set != nil && ov.Set.Engine != nil && *ov.Set.Engine != "" {
			if _, ok := engineIDs[strings.ToLower(strings.TrimSpace(*ov.Set.Engine))]; !ok {
				addErr(where, "override engine %q does not exist", *ov.Set.Engine)
			}
		}
	}

	if hasDiskCatalog {
		for _, relCfg := range []string{
			filepath.Join("dsda-doom", "dsda-doom.cfg"),
			filepath.Join("uzdoom", "autoexec.cfg"),
		} {
			cfgPath := filepath.Join(rootDir, relCfg)
			if raw, err := os.ReadFile(cfgPath); err == nil {
				if match := personalPathRegex.FindString(string(raw)); match != "" {
					addErr(filepath.ToSlash(cfgPath), "contains personal path %q", match)
				}
			}
		}

		readmePath := filepath.Join(rootDir, "README.md")
		if readmeBytes, err := os.ReadFile(readmePath); err == nil && !HasErrors(curatedDiags) {
			curatedCat := buildCatalog(curatedEnts, nil)
			wantTable := GenerateReadmeTable(curatedCat)
			matches := readmeTableRegex.FindStringSubmatch(string(readmeBytes))
			if len(matches) < 3 {
				addErr(filepath.ToSlash(readmePath), "could not locate '## Preconfigured presets' markdown table")
			} else if strings.TrimSpace(matches[2]) != strings.TrimSpace(wantTable) {
				addErr(
					filepath.ToSlash(readmePath),
					"README.md preset table is out of sync with catalog/; run 'doom wads build'",
				)
			}
		}

		lintDocumentation(rootDir, addErr)
	}

	sort.SliceStable(diags, func(i, j int) bool {
		if diags[i].Path != diags[j].Path {
			return diags[i].Path < diags[j].Path
		}
		return diags[i].Message < diags[j].Message
	})
	return diags, nil
}

func lintDocumentation(rootDir string, addErr func(where, format string, a ...any)) {
	docFiles := []string{
		"README.md",
		"CONTRIBUTING.md",
		"AGENTS.md",
		"SECURITY.md",
		"llms.txt",
		filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
		filepath.Join(".agents", "rules", "monospace-tui.md"),
		filepath.Join(".agents", "skills", "mono-tui-design", "SKILL.md"),
		filepath.Join(".agents", "skills", "mono-tui-audit", "SKILL.md"),
	}
	yamlFiles := []string{
		filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.yml"),
		filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.yml"),
		filepath.Join(".github", "ISSUE_TEMPLATE", "config.yml"),
		filepath.Join(".github", "workflows", "benchmark.yml"),
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join(".github", "workflows", "security.yml"),
	}

	for _, relYAML := range yamlFiles {
		yamlPath := filepath.Join(rootDir, relYAML)
		rawBytes, err := os.ReadFile(yamlPath)
		if err != nil {
			continue
		}
		where := filepath.ToSlash(yamlPath)
		text := string(rawBytes)
		if match := docPersonalPathRegex.FindString(text); match != "" {
			addErr(where, "contains personal path %q", match)
		}
		if match := decorativeEmojiRegex.FindString(text); match != "" {
			addErr(where, "contains decorative emoji or shortcode %q", match)
		}
	}

	for _, relDoc := range docFiles {
		docPath := filepath.Join(rootDir, relDoc)
		rawBytes, err := os.ReadFile(docPath)
		if err != nil {
			continue
		}
		where := filepath.ToSlash(docPath)
		text := string(rawBytes)

		if match := docPersonalPathRegex.FindString(text); match != "" {
			addErr(where, "contains personal path %q", match)
		}
		if match := decorativeEmojiRegex.FindString(text); match != "" {
			addErr(where, "contains decorative emoji or shortcode %q", match)
		}

		lines := strings.Split(text, "\n")
		isAgentDoc := strings.HasPrefix(filepath.ToSlash(relDoc), ".agents/")
		inFrontmatter := false
		inFence := false
		fenceLang := ""
		fenceStartLine := 0
		var fenceBuf strings.Builder
		headingSlugs := make(map[string]bool)
		slugCounts := make(map[string]int)

		for idx, line := range lines {
			lineNum := idx + 1
			trimmed := strings.TrimSpace(line)
			if isAgentDoc && idx == 0 && trimmed == "---" {
				inFrontmatter = true
				continue
			}
			if inFrontmatter {
				if trimmed == "---" {
					inFrontmatter = false
				}
				continue
			}
			if strings.HasPrefix(trimmed, "```") {
				if !inFence {
					inFence = true
					fenceLang = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
					fenceStartLine = lineNum
					fenceBuf.Reset()
				} else {
					if fenceLang == "json" && (relDoc == "README.md" || relDoc == "CONTRIBUTING.md") {
						if !json.Valid([]byte(fenceBuf.String())) {
							addErr(where, "invalid JSON code block starting at line %d", fenceStartLine)
						}
					}
					inFence = false
					fenceLang = ""
				}
				continue
			}
			if inFence {
				fenceBuf.WriteString(line)
				fenceBuf.WriteByte('\n')
				continue
			}

			if trimmed == "---" {
				addErr(where, "contains horizontal rule divider '---' at line %d", lineNum)
			}

			if relDoc == "README.md" || relDoc == "CONTRIBUTING.md" {
				if match := boldTableCellRegex.FindString(trimmed); match != "" {
					addErr(
						where,
						"contains bolded first-column table cell %q at line %d",
						strings.TrimSpace(match), lineNum,
					)
				}
			}

			if match := findStackedFormatting(line); match != "" {
				addErr(where, "contains stacked bold span %q", match)
			}

			cleanedLine := htmlCommentRegex.ReplaceAllString(stripInlineCode(line), "")
			if match := italicAsideRegex.FindString(cleanedLine); match != "" {
				addErr(where, "contains italic parenthetical aside %q", match)
			}
			if match := asciiArrowRegex.FindString(cleanedLine); match != "" {
				addErr(where, "contains ASCII arrow %q at line %d", match, lineNum)
			}

			if strings.HasPrefix(trimmed, "#") {
				title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
				if title != "" {
					baseSlug := GitHubHeadingSlug(title)
					if baseSlug != "" {
						count := slugCounts[baseSlug]
						slugCounts[baseSlug] = count + 1
						slug := baseSlug
						if count > 0 {
							slug = fmt.Sprintf("%s-%d", baseSlug, count)
						}
						headingSlugs[slug] = true
					}
				}
			}
		}

		if relDoc == "README.md" || relDoc == "CONTRIBUTING.md" || isAgentDoc {
			inFrontmatter = false
			inFence = false
			docDir := filepath.Dir(docPath)
			for idx, line := range lines {
				trimmed := strings.TrimSpace(line)
				if isAgentDoc && idx == 0 && trimmed == "---" {
					inFrontmatter = true
					continue
				}
				if inFrontmatter {
					if trimmed == "---" {
						inFrontmatter = false
					}
					continue
				}
				if strings.HasPrefix(trimmed, "```") {
					inFence = !inFence
					continue
				}
				if inFence {
					continue
				}
				for _, target := range extractMarkdownLinkTargets(line) {
					if strings.HasPrefix(target, "#") {
						slug := strings.TrimPrefix(target, "#")
						if !headingSlugs[slug] {
							addErr(where, "broken anchor link %q", target)
						}
						continue
					}
					if strings.HasPrefix(target, "http://") ||
						strings.HasPrefix(target, "https://") ||
						strings.HasPrefix(target, "mailto:") {
						continue
					}
					relTarget := target
					if hashIdx := strings.IndexByte(relTarget, '#'); hashIdx >= 0 {
						relTarget = relTarget[:hashIdx]
					}
					if relTarget == "" {
						continue
					}
					targetPath := filepath.Join(docDir, filepath.FromSlash(relTarget))
					if _, statErr := os.Stat(targetPath); statErr != nil {
						addErr(where, "broken relative file link %q", target)
					}
				}
			}
		}
	}
}

func findClosingBackticks(s string, tickCount int) int {
	for i := 0; i <= len(s)-tickCount; {
		if s[i] != '`' {
			i++
			continue
		}
		run := 0
		for i+run < len(s) && s[i+run] == '`' {
			run++
		}
		if run == tickCount {
			return i
		}
		i += run
	}
	return -1
}

func stripInlineCode(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	for i := 0; i < len(line); {
		if line[i] == '`' {
			tickCount := 0
			for i+tickCount < len(line) && line[i+tickCount] == '`' {
				tickCount++
			}
			if closeRel := findClosingBackticks(line[i+tickCount:], tickCount); closeRel >= 0 {
				i += tickCount + closeRel + tickCount
				continue
			}
			b.WriteString(line[i : i+tickCount])
			i += tickCount
			continue
		}
		b.WriteByte(line[i])
		i++
	}
	return b.String()
}

func findStackedFormatting(line string) string {
	for i := 0; i < len(line); {
		if line[i] == '`' {
			tickCount := 0
			for i+tickCount < len(line) && line[i+tickCount] == '`' {
				tickCount++
			}
			if closeRel := findClosingBackticks(line[i+tickCount:], tickCount); closeRel >= 0 {
				i += tickCount + closeRel + tickCount
				continue
			}
			i += tickCount
			continue
		}

		if strings.HasPrefix(line[i:], "***") {
			if end := strings.Index(line[i+3:], "***"); end >= 0 {
				return line[i : i+3+end+3]
			}
		}

		if strings.HasPrefix(line[i:], "_**") {
			if end := strings.Index(line[i+3:], "**_"); end >= 0 {
				return line[i : i+3+end+3]
			}
		}

		if strings.HasPrefix(line[i:], "**") {
			if end := strings.Index(line[i+2:], "**"); end >= 0 {
				inner := line[i+2 : i+2+end]
				full := line[i : i+2+end+2]
				if strings.Contains(inner, "`") {
					return full
				}
				if strings.Contains(inner, "[") && strings.Contains(inner, "](") {
					return full
				}
				if (strings.HasPrefix(inner, "_") && strings.HasSuffix(inner, "_") && len(inner) > 2) ||
					(strings.HasPrefix(inner, "*") && strings.HasSuffix(inner, "*") && len(inner) > 2) {
					return full
				}
				i += 2 + end + 2
				continue
			}
		}

		if line[i] == '[' {
			if closeBracket := strings.IndexByte(line[i+1:], ']'); closeBracket >= 0 {
				afterBracket := i + 1 + closeBracket + 1
				if afterBracket < len(line) && line[afterBracket] == '(' {
					linkText := line[i+1 : i+1+closeBracket]
					if strings.Contains(linkText, "**") {
						if closeParen := strings.IndexByte(line[afterBracket+1:], ')'); closeParen >= 0 {
							return line[i : afterBracket+1+closeParen+1]
						}
					}
				}
			}
		}

		i++
	}
	return ""
}

func extractMarkdownLinkTargets(line string) []string {
	cleaned := stripInlineCode(line)
	var targets []string
	for pos := 0; pos < len(cleaned); {
		idx := strings.Index(cleaned[pos:], "](")
		if idx < 0 {
			break
		}
		start := pos + idx + 2
		depth := 1
		var raw strings.Builder
		j := start
		for j < len(cleaned) && depth > 0 {
			if cleaned[j] == '\\' && j+1 < len(cleaned) {
				raw.WriteByte(cleaned[j+1])
				j += 2
				continue
			}
			if cleaned[j] == '(' {
				depth++
				raw.WriteByte('(')
				j++
				continue
			}
			if cleaned[j] == ')' {
				depth--
				if depth == 0 {
					break
				}
				raw.WriteByte(')')
				j++
				continue
			}
			raw.WriteByte(cleaned[j])
			j++
		}
		if depth != 0 {
			break
		}
		target := strings.TrimSpace(raw.String())
		if fields := strings.Fields(target); len(fields) > 0 {
			target = fields[0]
		}
		target = strings.TrimPrefix(strings.TrimSuffix(target, ">"), "<")
		if target != "" {
			targets = append(targets, target)
		}
		pos = j + 1
	}
	return targets
}
