## Description

Please provide a summary of the changes introduced in this pull request, including motivation and context.

## Type of Change

- [ ] 🐛 Bug fix (non-breaking change which fixes an issue)
- [ ] ✨ New preset or engine entity (new community megawad or engine in `catalog/*.json`)
- [ ] 🚀 New feature (CLI command, engine integration, or asset discovery enhancement)
- [ ] 💥 Breaking change (fix or feature that would alter command line flags or existing config paths)
- [ ] 🎨 Engine config optimization (tweaks to `dsda-doom.cfg` or `autoexec.cfg`)
- [ ] 📝 Documentation update (`README.md`, `CONTRIBUTING.md`, `AGENTS.md`, `llms.txt`)
- [ ] 🔧 Tooling / CI / Build configuration

## Repository Design & Architecture Checklist

- [ ] **Declarative Catalog:** Curated presets and engines are defined in `catalog/*.json`; validated via `doom presets lint` and synchronized into `README.md` via `doom presets build`.
- [ ] **Portability & Path Invariants:** Zero hardcoded usernames or personal paths committed; `__RESOLUTION__`, `__REFRESH_RATE__`, and `__SOUNDFONT__` placeholders used in engine config templates.
- [ ] **Destructive Safety:** Configuration and entity writes use atomic rename and timestamped backups (`.bak.<timestamp>`).
- [ ] **Strict Line Length:** Code complies with the 120-character maximum line length limit.
- [ ] **Engine Selection:** Engine choices, complevels, and DeHackEd load orders are properly configured.

## Verification & Testing

- [ ] Ran `make format-check` (or `gofmt -s -w .`).
- [ ] Ran `make tidy-check` (`go.mod` and `go.sum` verified).
- [ ] Ran `make lint` (`go vet` and `revive` pass with zero warnings).
- [ ] Ran `make check` (all unit tests pass with `-race` and `-shuffle=on`, catalog lint, README parity, path invariant audit).
- [ ] Documentation (`README.md`, `CONTRIBUTING.md`, `AGENTS.md`, `llms.txt`) is updated and synchronized.
