## Description

Please provide a summary of the changes introduced in this pull request, including motivation and context.

## Type of Change

- [ ] 🐛 Bug fix (non-breaking change which fixes an issue)
- [ ] ✨ New preset (new community megawad or engine preset in `data/presets.json`)
- [ ] 🚀 New feature (CLI command, engine integration, or asset discovery enhancement)
- [ ] 💥 Breaking change (fix or feature that would alter command line flags or existing config paths)
- [ ] 🎨 Engine config optimization (tweaks to `dsda-doom.cfg` or `autoexec.cfg`)
- [ ] 📝 Documentation update (`README.md`, `CONTRIBUTING.md`, `AGENTS.md`)
- [ ] 🔧 Tooling / CI / Build configuration

## Repository Design & Architecture Checklist

- [ ] **Declarative Presets:** Presets are modified only in `data/presets.json`; generated files were compiled via `doom presets build`.
- [ ] **Portability & Path Invariants:** Zero hardcoded usernames or personal paths committed; `__HOME__`, `__RESOLUTION__`, `__REFRESH_RATE__`, and `__SOUNDFONT__` placeholders used where appropriate.
- [ ] **Destructive Safety:** Configuration deployment includes timestamped backups (`.bak.<timestamp>`).
- [ ] **Strict Line Length:** Code complies with the 120-character maximum line length limit.
- [ ] **Engine Selection:** Engine choices, complevels, and DeHackEd load orders are properly configured.

## Verification & Testing

- [ ] Ran `make format-check` (or `gofmt -s -w .`).
- [ ] Ran `make tidy-check` (`go.mod` and `go.sum` verified).
- [ ] Ran `make lint` (`go vet` and `revive` pass with zero warnings).
- [ ] Ran `make check` (all unit tests pass with `-race` and `-shuffle=on`, preset parity, path invariant audit).
- [ ] Documentation (`README.md`, `CONTRIBUTING.md`, `AGENTS.md`) is updated and synchronized.
