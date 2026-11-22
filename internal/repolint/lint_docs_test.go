package repolint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lock14/doom-cli/internal/catalog"
)

func TestLintDocumentation_ViolationsDetected(t *testing.T) {
	t.Parallel()
	tmpRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpRoot, "catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	sampleUserPath := "/Users/" + "johndoe"

	badContributing := strings.Join([]string{
		"# Contributing",
		"",
		"Path `" + sampleUserPath + "/doom` and **bold `code`**.",
		"Second stacked line: **[link](missing.md)**.",
		"*(Note: parenthetical aside)*",
		"Emoji: \U0001f41b and :white_check_mark:",
		"Prose arrow: step A -> step B",
		"Second arrow: step C => step D",
		"",
		"| Column | Value |",
		"| :--- | :--- |",
		"| **Bold Cell** | Val |",
		"| **Second Bold Cell** | Val2 |",
		"",
		"---",
		"",
		"[Broken anchor](#nonexistent-heading)",
		"[Broken titled link](missing_titled.md \"Title\")",
		"",
		"```json",
		"{invalid json}",
		"```",
	}, "\n")

	if err := os.WriteFile(filepath.Join(tmpRoot, "CONTRIBUTING.md"), []byte(badContributing), 0o644); err != nil {
		t.Fatal(err)
	}

	wfDir := filepath.Join(tmpRoot, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	badBenchmark := "echo \"## \U0001f4ca Benchmark\" && ls " + sampleUserPath + "/doom\n"
	if err := os.WriteFile(filepath.Join(wfDir, "benchmark.yml"), []byte(badBenchmark), 0o644); err != nil {
		t.Fatal(err)
	}

	diags, err := LintCatalog(tmpRoot, "")
	if err != nil {
		t.Fatalf("LintCatalog error: %v", err)
	}

	var joined strings.Builder
	for _, d := range diags {
		joined.WriteString(d.String() + "\n")
	}
	out := joined.String()

	for _, wantSub := range []string{
		"contains personal path",
		"contains decorative emoji or shortcode",
		"contains stacked bold span \"**bold `code`**\"",
		"contains stacked bold span \"**[link](missing.md)**\"",
		"contains italic parenthetical aside",
		"contains bolded first-column table cell \"| **Bold Cell** |\"",
		"contains bolded first-column table cell \"| **Second Bold Cell** |\"",
		"contains ASCII arrow \"->\"",
		"contains ASCII arrow \"=>\"",
		"contains horizontal rule divider '---'",
		"broken relative file link \"missing.md\"",
		"broken relative file link \"missing_titled.md\"",
		"broken anchor link \"#nonexistent-heading\"",
		"invalid JSON code block",
		"benchmark.yml: contains decorative emoji or shortcode \"\U0001f4ca\"",
		"benchmark.yml: contains personal path \"" + sampleUserPath + "\"",
	} {
		if !strings.Contains(out, wantSub) {
			t.Errorf("expected lint diagnostics to contain %q, got:\n%s", wantSub, out)
		}
	}
}

func TestLintDocumentation_EdgeCasesAndStackedFormatting(t *testing.T) {
	t.Parallel()
	tmpRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpRoot, "catalog"), 0o755); err != nil {
		t.Fatal(err)
	}

	parenFile := filepath.Join(tmpRoot, "file(1).md")
	if err := os.WriteFile(parenFile, []byte("# Paren file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	validContributing := strings.Join([]string{
		"# Contributing",
		"",
		"<!-- HTML comment should not trigger arrow lint -->",
		"",
		"## Diátaxis config_file",
		"",
		"### Examples",
		"",
		"- **First label**: use `doom play` and **Second label**: see [Contributing](CONTRIBUTING.md).",
		"- **Inline code**: do not write `*(Note: aside)*`, ``**`code`**``, `->`, or `<->` in prose.",
		"- **Escaped link**: see [Paren](file\\(1\\).md), [Duplicate](#examples-1), " +
			"and [Slug](#diátaxis-config_file).",
		"",
		"| Column | Value |",
		"| :--- | :--- |",
		"| Plain cell | Val |",
		"",
		"### Examples",
		"",
		"```markdown",
		"**`code inside fenced block`**",
		"*(Note: inside fenced block)*",
		"| **Bold inside fenced block** | Val |",
		"a -> b <-> c",
		"---",
		"```",
	}, "\n")

	if err := os.WriteFile(filepath.Join(tmpRoot, "CONTRIBUTING.md"), []byte(validContributing), 0o644); err != nil {
		t.Fatal(err)
	}

	diags, err := LintCatalog(tmpRoot, "")
	if err != nil {
		t.Fatalf("LintCatalog error: %v", err)
	}
	for _, d := range diags {
		if strings.HasSuffix(d.Path, "CONTRIBUTING.md") {
			t.Errorf("unexpected lint diagnostic on valid CONTRIBUTING.md: %s", d.String())
		}
	}

	stackedCases := []struct {
		name      string
		line      string
		wantMatch string
	}{
		{
			name:      "bold wrapping code",
			line:      "Use **`doom play`** here",
			wantMatch: "**`doom play`**",
		},
		{
			name:      "bold wrapping link",
			line:      "See **[DSDA-Doom](https://example.com)**",
			wantMatch: "**[DSDA-Doom](https://example.com)**",
		},
		{
			name:      "link wrapping bold",
			line:      "See [**DSDA-Doom**](https://example.com)",
			wantMatch: "[**DSDA-Doom**](https://example.com)",
		},
		{
			name:      "triple asterisk bold italics",
			line:      "Avoid ***bold italics*** in prose",
			wantMatch: "***bold italics***",
		},
		{
			name:      "bold wrapping underscore italics",
			line:      "Avoid **_bold italics_** in prose",
			wantMatch: "**_bold italics_**",
		},
		{
			name:      "underscore italics wrapping bold",
			line:      "Avoid _**bold italics**_",
			wantMatch: "_**bold italics**_",
		},
		{
			name:      "two separate bold spans with code between",
			line:      "- **First**: run `doom` and **Second**: done.",
			wantMatch: "",
		},
	}

	for _, tc := range stackedCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := findStackedFormatting(tc.line)
			if got != tc.wantMatch {
				t.Errorf("findStackedFormatting(%q) = %q, want %q", tc.line, got, tc.wantMatch)
			}
		})
	}
}

func TestDocumentationEntityJSONBlocks(t *testing.T) {
	t.Parallel()
	rootDir := filepath.Join("..", "..")
	docTargets := []string{"README.md", "CONTRIBUTING.md", "AGENTS.md"}
	if ruleMatches, err := filepath.Glob(filepath.Join(rootDir, ".agents", "rules", "*.md")); err == nil {
		for _, m := range ruleMatches {
			if rel, relErr := filepath.Rel(rootDir, m); relErr == nil {
				docTargets = append(docTargets, rel)
			}
		}
	}
	for _, relDoc := range docTargets {
		relDoc := relDoc
		t.Run(relDoc, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join(rootDir, relDoc))
			if err != nil {
				t.Fatalf("reading %s: %v", relDoc, err)
			}
			lines := strings.Split(string(raw), "\n")
			inJSON := false
			var buf strings.Builder
			entityCount := 0
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "```") {
					if !inJSON {
						lang := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
						if lang == "json" {
							inJSON = true
							buf.Reset()
						}
					} else {
						inJSON = false
						block := buf.String()
						if strings.Contains(block, `"kind"`) {
							entityCount++
							if _, decErr := catalog.DecodeEntity([]byte(block)); decErr != nil {
								t.Errorf("%s entity JSON block failed DecodeEntity: %v\n%s", relDoc, decErr, block)
							}
						}
					}
					continue
				}
				if inJSON {
					buf.WriteString(line)
					buf.WriteByte('\n')
				}
			}
			if (relDoc == "README.md" || relDoc == "CONTRIBUTING.md") && entityCount == 0 {
				t.Errorf("expected at least one entity JSON block in %s", relDoc)
			}
		})
	}
}

func TestLintDocumentation_AgentRuleSizeAndDiscovery(t *testing.T) {
	t.Parallel()
	tmpRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpRoot, "catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	rulesDir := filepath.Join(tmpRoot, ".agents", "rules")
	skillDir := filepath.Join(tmpRoot, ".agents", "skills", "custom-skill")
	runtimePkgDir := filepath.Join(tmpRoot, "internal", "catalog")
	for _, dir := range []string{rulesDir, skillDir, runtimePkgDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	oversizedAgents := "# Root rules\n\nSee [Missing](missing-rule.md).\n\n" +
		strings.Repeat("* Bullet line.\n", MaxRootAgentsLines+5)
	if err := os.WriteFile(filepath.Join(tmpRoot, "AGENTS.md"), []byte(oversizedAgents), 0o644); err != nil {
		t.Fatal(err)
	}

	padding := strings.Repeat("Plain rule text without violations.\n", (MaxScopedRuleBytes/32)+10)
	oversizedScoped := "---\ntrigger: glob\ndescription: \"Scoped rule.\"\nglobs: **/*.go\n---\n\n" +
		"# Scoped rule\n\n```json\n{bad}\n```\n\n" + padding
	if err := os.WriteFile(filepath.Join(rulesDir, "custom.md"), []byte(oversizedScoped), 0o644); err != nil {
		t.Fatal(err)
	}

	badTriggerRule := "---\ntrigger: alwaysOn\n---\n\n# Bad trigger\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "bad-trigger.md"), []byte(badTriggerRule), 0o644); err != nil {
		t.Fatal(err)
	}

	noFmRule := "# Missing frontmatter\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "no-frontmatter.md"), []byte(noFmRule), 0o644); err != nil {
		t.Fatal(err)
	}

	badSkill := "---\nname: custom-skill\n---\n\n# Custom skill\n\nAvoid **bold `code`**.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(badSkill), 0o644); err != nil {
		t.Fatal(err)
	}

	badImportGo := "package catalog\n\nimport _ \"github.com/lock14/doom-cli/internal/repolint\"\n"
	if err := os.WriteFile(filepath.Join(runtimePkgDir, "bad_import.go"), []byte(badImportGo), 0o644); err != nil {
		t.Fatal(err)
	}

	diags, err := LintCatalog(tmpRoot, "")
	if err != nil {
		t.Fatalf("LintCatalog error: %v", err)
	}
	var joined strings.Builder
	for _, d := range diags {
		joined.WriteString(d.String() + "\n")
	}
	out := joined.String()

	for _, wantSub := range []string{
		"AGENTS.md: exceeds 150-line root AGENTS.md budget",
		"AGENTS.md: broken relative file link \"missing-rule.md\"",
		"custom.md: exceeds 24000-byte scoped rule limit",
		"custom.md: frontmatter globs containing '*' must be quoted",
		"custom.md: invalid JSON code block",
		"bad-trigger.md: invalid or missing frontmatter trigger \"alwaysOn\"",
		"bad-trigger.md: missing frontmatter description",
		"no-frontmatter.md: missing YAML frontmatter block",
		"SKILL.md: missing frontmatter description",
		"SKILL.md: contains stacked bold span \"**bold `code`**\"",
		"bad_import.go: runtime package internal/catalog must not import internal/repolint",
	} {
		if !strings.Contains(out, wantSub) {
			t.Errorf("expected lint diagnostics to contain %q, got:\n%s", wantSub, out)
		}
	}

	repoRoot := filepath.Join("..", "..")
	agentsBytes, err := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	agentsLines := strings.Count(strings.TrimRight(string(agentsBytes), "\r\n"), "\n") + 1
	if agentsLines > MaxRootAgentsLines {
		t.Errorf("AGENTS.md has %d lines, exceeding MaxRootAgentsLines (%d)", agentsLines, MaxRootAgentsLines)
	}

	matches, err := filepath.Glob(filepath.Join(repoRoot, ".agents", "rules", "*.md"))
	if err != nil {
		t.Fatalf("globbing .agents/rules: %v", err)
	}
	if len(matches) < 3 {
		t.Fatalf("expected at least 3 scoped rule files in .agents/rules, got %d", len(matches))
	}
	for _, p := range matches {
		raw, readErr := os.ReadFile(p)
		if readErr != nil {
			t.Fatalf("reading %s: %v", p, readErr)
		}
		if len(raw) > MaxScopedRuleBytes {
			t.Errorf("%s is %d bytes, exceeding MaxScopedRuleBytes (%d)", p, len(raw), MaxScopedRuleBytes)
		}
	}
}

func TestMonospaceTUIOverridesNotStale(t *testing.T) {
	t.Parallel()
	rootDir := filepath.Join("..", "..")
	overridesPath := filepath.Join(rootDir, ".agents", "rules", "monospace-tui.md")
	overridesRaw, err := os.ReadFile(overridesPath)
	if err != nil {
		t.Fatalf("reading %s: %v", overridesPath, err)
	}
	stdRaw, err := os.ReadFile(
		filepath.Join(rootDir, ".agents", "skills", "mono-tui-design", "references", "standard.md"),
	)
	if err != nil {
		t.Fatalf("reading standard.md: %v", err)
	}
	rendRaw, err := os.ReadFile(
		filepath.Join(rootDir, ".agents", "skills", "mono-tui-design", "references", "rendering.md"),
	)
	if err != nil {
		t.Fatalf("reading rendering.md: %v", err)
	}
	patRaw, err := os.ReadFile(
		filepath.Join(rootDir, ".agents", "skills", "mono-tui-design", "references", "patterns.md"),
	)
	if err != nil {
		t.Fatalf("reading patterns.md: %v", err)
	}
	corpus := string(stdRaw) + "\n" + string(rendRaw) + "\n" + string(patRaw)

	var currentHeading string
	var currentHeadingLine int
	currentHasOriginal := false
	checked := 0
	overrideHeadings := 0

	checkPreviousHeading := func() {
		if currentHeading == "" {
			return
		}
		if (strings.HasPrefix(currentHeading, "[OVERRIDE]") || strings.HasPrefix(currentHeading, "[WAIVE]")) &&
			!currentHasOriginal {
			t.Errorf(
				"%s:%d (%s): missing **Original**: quote for override or waiver heading",
				overridesPath, currentHeadingLine, currentHeading,
			)
		}
	}

	for idx, line := range strings.Split(string(overridesRaw), "\n") {
		lineNum := idx + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			checkPreviousHeading()
			currentHeading = strings.TrimPrefix(trimmed, "### ")
			currentHeadingLine = lineNum
			currentHasOriginal = false
			if strings.HasPrefix(currentHeading, "[OVERRIDE]") || strings.HasPrefix(currentHeading, "[WAIVE]") {
				overrideHeadings++
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			checkPreviousHeading()
			currentHeading = ""
			currentHeadingLine = 0
			currentHasOriginal = false
			continue
		}
		bulletBody := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "*"), "-"))
		var quoted string
		switch {
		case strings.HasPrefix(bulletBody, "**Original**: "):
			quoted = strings.TrimSpace(strings.TrimPrefix(bulletBody, "**Original**: "))
		case strings.HasPrefix(bulletBody, "**Original:** "):
			quoted = strings.TrimSpace(strings.TrimPrefix(bulletBody, "**Original:** "))
		default:
			continue
		}
		checked++
		currentHasOriginal = true
		if quoted == "" {
			t.Errorf("%s:%d (%s): empty Original field", overridesPath, lineNum, currentHeading)
			continue
		}
		if !strings.Contains(corpus, quoted) {
			t.Errorf(
				"%s:%d (%s): Original text not found verbatim in standard.md, rendering.md, or patterns.md:\n  %s",
				overridesPath, lineNum, currentHeading, quoted,
			)
		}
	}
	checkPreviousHeading()

	if overrideHeadings == 0 {
		t.Fatalf("expected at least one [OVERRIDE] or [WAIVE] heading in %s, found 0", overridesPath)
	}
	if checked == 0 {
		t.Fatalf("expected at least one Original entry in %s, found 0", overridesPath)
	}
}
