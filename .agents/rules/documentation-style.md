---
trigger: glob
description: "Technical writing, Markdown typography, surface boundaries, commit, and pull request standards."
globs: "**/*.md, llms.txt, .github/**/*"
---

# Technical writing, typography, and documentation rules

Follow these standards whenever editing Markdown documentation (`README.md`, `CONTRIBUTING.md`, `AGENTS.md`,
`SECURITY.md`, `.agents/**/*.md`, `skills/**/*.md`), `llms.txt`, GitHub templates and workflows (`.github/**/*`), Go
comments, commit messages, or pull request descriptions.

## Typography and formatting restraint

*   **One format per span**: Never combine bold, italics, code font, or hyperlinks on the same text (no bolded code
    spans such as ``**`flag`**`` or ``**Heading `key`**:``, no bolded links, and no bold-italics). Enforced mechanically
    by `findStackedFormatting` in `internal/repolint/lint_docs.go`.
*   **Strict role separation**:
    *   **Code font**: Use backticks strictly for literal code, CLI commands, flags, file paths, configuration keys, and
        keyboard shortcuts. Keep sentence punctuation outside backticks.
    *   **Bold**: Use bold (`**...**`) only for UI labels and run-in list headings followed by a colon (`:`). Never use
        a dash after a run-in heading, never bold first-column table cells in `README.md` or `CONTRIBUTING.md`, and
        never use bold for mid-sentence emphasis or whole clauses.
    *   **Italics**: Use italics (`_..._` or `*...*`) only when introducing a term on first use or for titles of
        standalone works (such as game and megawad titles in prose).
*   **Sentence-case headings and table headers**: Use sentence case for all document titles, section headings, and table
    column headers (capitalizing only the first word and proper nouns like `CLI`, `TUI`, `WAD`, `WADs`, `IWAD`, `PWAD`,
    `UZDoom`, `DSDA-Doom`, `Linux`, `macOS`, `Windows`, `JSON`, `ANSI`, `Nerd Fonts`, `README`, `Go`, `Makefile`, and
    `Git`), with no trailing punctuation.
*   **No decorative symbols or arrows in prose**: Omit decorative emojis, GitHub emoji shortcodes, LaTeX math
    expressions, ASCII arrows (`->`, `<-`, `<->`, `=>`), and inequality signs as prose connectors outside code blocks
    and diagrams. Write plain English verbs ("maps to", "resolves to", "takes precedence over") instead.
*   **No horizontal section dividers**: Never place horizontal rules (`---`) between sections; Markdown headings provide
    structural hierarchy (YAML frontmatter `---` at line 1 of `.agents/` and `skills/` files is permitted).
*   **Clean punctuation and list discipline**: Never use italic parenthetical asides (`*(Note: ...)*`, `*(Tip: ...)*`,
    `*(Alternatively ...)*`); state details as normal sentences or table entries. Do not use slashes (`/`) in prose to
    join alternatives (write "or" or "and" instead), reserving `/` for file paths, URLs, and literal commands or keys.
*   **Maximum line length (120 columns)**: Wrap prose, list items, and code snippets within 120 columns (excluding wide
    generated Markdown table rows in `README.md` and verbatim `- **Original**:` specification quotes in
    [monospace-tui.md](monospace-tui.md)).
*   **Validated JSON blocks and links**: Every ```json fenced block in `README.md`, `CONTRIBUTING.md`, `AGENTS.md`, and
    `.agents/` must be valid JSON (and `"kind"` entity examples must pass `catalog.DecodeEntity`), and every relative
    file link and `#anchor` slug must resolve on disk.

## Documentation roles, boundaries, and surface standards

*   **Player reference**: Reserve `README.md` strictly for players and end-users of the `doom` CLI. Follow Diátaxis
    structure (concise summary, quick start, task guides, reference tables, and troubleshooting) and dual representation
    so interactive keybindings, CLI flags, and environment variables appear both conceptually in prose and tabularly in
    reference tables. Document only behavior verified against the current codebase (zero aspirational features), and
    never expose internal developer details or hidden maintainer commands (`doom wads build`, `doom engines cfg sync`).
*   **Developer guide**: Dedicate `CONTRIBUTING.md` to human open-source contributors covering architecture, Makefile
    targets, Go code style, how-to workflows, testing requirements, and pull request guidelines.
*   **Agent constitution**: Keep [AGENTS.md](../../AGENTS.md) at the repository root (with zero `.agents/AGENTS.md`
    copies or `CLAUDE.md` symlinks) as the universal always-on summary, delegating contributor rules and TUI workflows
    to `.agents/rules/*.md` and `.agents/skills/*/SKILL.md` while keeping the player-facing CLI skill isolated at
    [SKILL.md](../../skills/doom-cli/SKILL.md).
*   **LLM summary**: Keep [llms.txt](../../llms.txt) synchronized whenever CLI commands, flags, environment variables,
    packages, or core invariants change.
*   **Pull request descriptions**: Follow the Google CL Description style using
    [PULL_REQUEST_TEMPLATE.md](../../.github/PULL_REQUEST_TEMPLATE.md): standalone imperative title, summary explaining
    "why" before "what" grouped by subsystem, and completed checklists without decorative emojis. Proactively update the
    PR body via `gh pr edit` whenever follow-up commits refine the changeset.
*   **Issues, comments, and commits**: Frame bug reports around environment, numbered reproduction steps, expected
    behavior, and actual behavior with logs; frame feature requests around the user problem before proposing CLI or
    schema syntax. Start every exported Go doc comment with the symbol name, focus inline comments on "why" rather than
    "what", prefix non-blocking review polish with `Nit:`, and follow Chris Beams' commit rules (imperative subject of
    50 characters or fewer, hard cap 72, no trailing period, blank line, and 72-column wrapped body).
