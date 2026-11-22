package repolint

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// MaxRootAgentsLines caps the line count of root AGENTS.md so always-on instructions stay lean.
const MaxRootAgentsLines = 150

// MaxScopedRuleBytes caps .agents/rules/*.md files below the 24,000-byte per-file truncation limit.
const MaxScopedRuleBytes = 24000

const forbiddenRepolintPkg = "github.com/lock14/doom-cli/internal/repolint"

var (
	docPersonalPathRegex = regexp.MustCompile(
		`(?i)(/home/[a-z0-9_-]+|/Users/[a-z0-9_-]+|C:(?:\\+|/)Users(?:\\+|/)[a-z0-9_-]+)`,
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

func discoverDocFiles(rootDir string) []string {
	docFiles := []string{
		"README.md",
		"CONTRIBUTING.md",
		"AGENTS.md",
		"SECURITY.md",
		"llms.txt",
		filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
	}
	var agentDocs []string
	for _, pattern := range []string{
		filepath.Join(rootDir, ".agents", "rules", "*.md"),
		filepath.Join(rootDir, ".agents", "skills", "*", "SKILL.md"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if rel, relErr := filepath.Rel(rootDir, m); relErr == nil {
				agentDocs = append(agentDocs, rel)
			}
		}
	}
	sort.Strings(agentDocs)
	return append(docFiles, agentDocs...)
}

func lintDocumentation(rootDir string, addErr func(where, format string, a ...any)) {
	lintImportBoundaries(rootDir, addErr)
	docFiles := discoverDocFiles(rootDir)
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
		slashRel := filepath.ToSlash(relDoc)
		isAgentDoc := strings.HasPrefix(slashRel, ".agents/")
		if relDoc == "AGENTS.md" {
			trimmedText := strings.TrimRight(text, "\r\n")
			lineCount := 0
			if trimmedText != "" {
				lineCount = strings.Count(trimmedText, "\n") + 1
			}
			if lineCount > MaxRootAgentsLines {
				addErr(
					where,
					"exceeds %d-line root AGENTS.md budget (%d lines); move domain detail to .agents/rules/",
					MaxRootAgentsLines, lineCount,
				)
			}
		}
		if strings.HasPrefix(slashRel, ".agents/rules/") && len(rawBytes) > MaxScopedRuleBytes {
			addErr(
				where,
				"exceeds %d-byte scoped rule limit (%d bytes); split or tighten to prevent truncation",
				MaxScopedRuleBytes, len(rawBytes),
			)
		}
		if isAgentDoc {
			lintAgentFrontmatter(where, slashRel, lines, addErr)
		}
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
					if fenceLang == "json" &&
						(relDoc == "README.md" || relDoc == "CONTRIBUTING.md" || relDoc == "AGENTS.md" || isAgentDoc) {
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

		if relDoc == "README.md" || relDoc == "CONTRIBUTING.md" || relDoc == "AGENTS.md" || isAgentDoc {
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

func isQuotedYAMLString(s string) bool {
	if len(s) < 2 {
		return false
	}
	return (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')
}

func lintAgentFrontmatter(where, slashRel string, lines []string, addErr func(where, format string, a ...any)) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		addErr(where, "missing YAML frontmatter block")
		return
	}
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		addErr(where, "unclosed YAML frontmatter block")
		return
	}

	rawFields := make(map[string]string)
	cleanFields := make(map[string]string)
	lastKey := ""
	for _, fmLine := range lines[1:closeIdx] {
		trimmed := strings.TrimSpace(fmLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if (strings.HasPrefix(fmLine, " ") || strings.HasPrefix(fmLine, "\t")) && lastKey != "" {
			prev := cleanFields[lastKey]
			if prev == ">-" || prev == ">" || prev == "|" {
				cleanFields[lastKey] = trimmed
			} else if prev != "" {
				cleanFields[lastKey] = prev + " " + trimmed
			} else {
				cleanFields[lastKey] = trimmed
			}
			continue
		}
		colon := strings.IndexByte(fmLine, ':')
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(fmLine[:colon])
		rawVal := strings.TrimSpace(fmLine[colon+1:])
		rawFields[key] = rawVal
		cleanFields[key] = strings.TrimSpace(strings.Trim(rawVal, `"'`))
		lastKey = key
	}

	isScalarPlaceholder := func(v string) bool {
		return v == "" || v == ">-" || v == ">" || v == "|"
	}

	if strings.HasPrefix(slashRel, ".agents/rules/") {
		trig := cleanFields["trigger"]
		switch trig {
		case "always_on", "model_decision", "glob", "manual":
		default:
			addErr(
				where,
				"invalid or missing frontmatter trigger %q (want always_on, model_decision, glob, or manual)",
				trig,
			)
		}
		if isScalarPlaceholder(cleanFields["description"]) {
			addErr(where, "missing frontmatter description")
		}
		if trig == "glob" {
			rawGlobs := rawFields["globs"]
			if rawGlobs == "" {
				rawGlobs = rawFields["glob"]
			}
			if strings.TrimSpace(strings.Trim(rawGlobs, `"'`)) == "" {
				addErr(where, "glob rule missing globs pattern")
			} else if strings.Contains(rawGlobs, "*") && !isQuotedYAMLString(rawGlobs) {
				addErr(where, "frontmatter globs containing '*' must be quoted")
			}
		}
	}

	if strings.HasPrefix(slashRel, ".agents/skills/") {
		if isScalarPlaceholder(cleanFields["name"]) {
			addErr(where, "missing frontmatter name")
		}
		if isScalarPlaceholder(cleanFields["description"]) {
			addErr(where, "missing frontmatter description")
		}
	}
}

func lintImportBoundaries(rootDir string, addErr func(where, format string, a ...any)) {
	internalDir := filepath.Join(rootDir, "internal")
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		return
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "repolint" {
			continue
		}
		pkgName := entry.Name()
		goFiles, globErr := filepath.Glob(filepath.Join(internalDir, pkgName, "*.go"))
		if globErr != nil {
			continue
		}
		sort.Strings(goFiles)
		for _, goFile := range goFiles {
			f, parseErr := parser.ParseFile(fset, goFile, nil, parser.ImportsOnly)
			if parseErr != nil {
				continue
			}
			for _, imp := range f.Imports {
				impPath, unqErr := strconv.Unquote(imp.Path.Value)
				if unqErr != nil {
					impPath = strings.Trim(imp.Path.Value, `"`)
				}
				if impPath == forbiddenRepolintPkg || strings.HasPrefix(impPath, forbiddenRepolintPkg+"/") {
					addErr(
						filepath.ToSlash(goFile),
						"runtime package internal/%s must not import internal/repolint",
						pkgName,
					)
				}
			}
		}
	}
}
