## Summary

Explain the problem or limitation in the prior code and why this design was chosen.

## Design and architecture checklist

- [ ] **Declarative catalog**: Curated presets and engines are defined in `catalog/*.json`, validated via `doom wads lint`, and synchronized into `README.md` via `doom wads build`.
- [ ] **Portability and path invariants**: Zero hardcoded usernames or personal paths committed; `__RESOLUTION__`, `__REFRESH_RATE__`, and `__SOUNDFONT__` placeholders used in engine configuration templates.
- [ ] **Destructive safety**: Configuration and entity writes use atomic rename and timestamped backups (`.bak.<timestamp>`).
- [ ] **Strict line length**: Code complies with the 120-character maximum line length limit.
- [ ] **Engine selection**: Engine choices, compatibility levels, and DeHackEd load orders are properly configured.

## Verification

- [ ] Ran `make format-check` (or `gofmt -s -w .`).
- [ ] Ran `make tidy-check` (`go.mod` and `go.sum` verified).
- [ ] Ran `make lint` (`go vet` and `revive` pass with zero warnings).
- [ ] Ran `make check` (all unit tests pass with `-race` and `-shuffle=on`, catalog lint, `README.md` parity, and path invariant audit).
- [ ] Updated and synchronized documentation (`README.md`, `CONTRIBUTING.md`, `AGENTS.md`, `llms.txt`).
