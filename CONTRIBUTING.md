# Contributing to doom-cli

Thank you for your interest in contributing to `github.com/lock14/doom-cli`! This document outlines our repository standards, architectural guidelines, development workflow, testing requirements, and pull request conventions.

---

## 1. Repository Design Principles

All contributions must respect our four foundational principles:

*   **Declarative Presets & Single Source of Truth**: All preset definitions, source port engine assignments, IWAD mappings, PWAD file lists, DeHackEd orderings, metadata, and download URLs are defined declaratively in [`data/presets.json`](data/presets.json). Never manually edit generated launcher files (`DoomRunner/linux/options.json` or `DoomRunner/windows/options.json`). Use `doom presets build` to regenerate them and update documentation.
*   **Portability & Path Invariants**:
    *   **Never commit personal user paths, display resolutions, or refresh rates**: Do not commit paths like `/home/<user>/`, hardcoded display resolutions, or fixed monitor refresh rates into configurations, presets, or code.
    *   **Use Placeholder Tokens**: Use `__HOME__`, `__RESOLUTION__`, `__REFRESH_RATE__`, and `__SOUNDFONT__` in configuration templates and presets. The CLI deployment tooling dynamically substitutes these based on runtime display and filesystem detection.
    *   **Standard Directory Structures**: Respect platform standard directories (XDG on Linux, `~/Library/Application Support/` on macOS, and `%LOCALAPPDATA%`/`%APPDATA%` on Windows).
*   **Destructive Safety & Backup Policy**: Any deployment target or command (`doom config install`, `doom setup`) must generate timestamped backups (`.bak.<timestamp>`) before overwriting or modifying any existing user configuration file.
*   **Doom Engine Selection & Preset Hygiene**:
    *   **DSDA-Doom**: Use for classic vanilla, Boom, MBF, and MBF21 maps where demo accuracy, standard physics, and speedrunning precision are desired (e.g. *Alien Vendetta*, *BTSX*, *Sunder*, *Sunlust*, *Legacy of Rust*, *Sigil*).
    *   **UZDoom**: Use for mapsets requiring ZDoom/GZDoom scripting, high-res texture packs like OTEX (*Eviternity I & II*), or Raven Software games (*Heretic*, *Hexen*).
    *   **No Duplicate IWADs**: Never include the base game IWAD in PWAD/mappack lists.
    *   **Filename Normalization**: Maintain case-insensitive and dash/underscore/space-tolerant file resolution so maps and DeHackEd patches resolve reliably regardless of user file naming.

---

## 2. Development Setup & Workflow

### Prerequisites

*   Go 1.23 or higher
*   GNU Make
*   Git

### Clone & Build

```bash
# Clone repository
git clone https://github.com/lock14/doom-cli.git
cd doom-cli

# Compile the static doom CLI binary to bin/doom
make build

# Install locally to ~/.local/bin/doom
make install
```

### Developer Makefile Targets

We maintain a streamlined `Makefile` for developer verification and local builds:

| Target | Command | Description |
| :--- | :--- | :--- |
| `make build` | `go build -o bin/doom ./cmd/doom` | Compiles the `bin/doom` static binary |
| `make install` | `go install ./cmd/doom` | Installs the binary to `~/.local/bin/doom` |
| `make test` | `go test -v -race -shuffle=on ./...` | Runs test suite under race detector with randomized order |
| `make lint` | `go vet && revive` | Performs static analysis via `go vet` and Revive |
| `make format` | `gofmt -s -w .` | Simplifies and formats all Go source files |
| `make tidy` | `go mod tidy && git diff` | Tidies and verifies `go.mod` and `go.sum` hygiene |
| `make check` | Full test & lint pipeline | Runs formatting, tidy, linters, tests, and path invariant audit |
| `make clean` | `rm -rf bin` | Cleans temporary build artifacts |
| `make help` | Target list | Displays available developer targets |

---

## 3. Go Code Style & Quality Standards

*   **Formatting**: Always format code using `gofmt -s -w .` (simplifying slice and composite literal syntax).
*   **Maximum Line Length (120 Columns)**: Enforce a strict 120-character maximum line length limit across the repository via `.editorconfig` and Revive's `line-length-limit` rule in `revive.toml`. Long strings, command invocations, and error messages must be wrapped cleanly.
*   **Receiver Naming**: Use short (1-2 letters), mnemonic receiver names consistently across methods on a type. Never use `this` or `self`.
*   **Avoid Identifier Shadowing**: Never shadow Go built-in identifiers (`min`, `max`, `len`, `cap`, `new`, `clear`, `copy`, `close`, `delete`). Use descriptive identifiers such as `maxVal`, `count`, or `limit`.
*   **Documentation Comments**: All exported packages, types, interfaces, constants, and functions must have comprehensive Go doc comments starting with the symbol name and adhering to standard Go and `revive` conventions.
*   **Static Analysis**: Code must pass `go vet ./...` and `revive -config revive.toml -formatter friendly ./...` with zero warnings.

---

## 4. Testing Conventions

*   **Table-Driven Tests**: Use table-driven tests for unit testing with descriptive test case names.
*   **Hardened Concurrency & Randomization**: All tests must pass cleanly under `-race` (race detector) and `-shuffle=on` (test order randomization).
*   **Filesystem Isolation**: File resolution, deployment, and extraction tests must execute within isolated temporary directories (`t.TempDir()` or `os.MkdirTemp`) and clean up after themselves.
*   **Preset Parity Test**: Any modifications to presets must pass `TestPresetParityAndInvariants`, verifying launcher JSON options and `README.md` match `data/presets.json` and comply with all path invariants.

---

## 5. Documentation Boundaries & Synchronization

*   **`README.md` is for Users**: Focuses strictly on user-facing installation, directory layouts, preset catalog, visual profiles, and `doom` CLI usage.
*   **`CONTRIBUTING.md` is for Contributors**: Covers developer setup, `make` targets, code style, and PR guidelines.
*   **`AGENTS.md` is for Agents & Core Architecture**: Encodes canonical design invariants, compiler mechanics, path rules, and agent workflows.
*   **Mandatory Synchronization**: Whenever presets, engine settings, build targets, directory layouts, or CLI mechanics are modified, all relevant documentation (**Go doc comments**, **`README.md`**, **`CONTRIBUTING.md`**, and **`AGENTS.md`**) MUST be updated within the same pull request.

---

## 6. Pre-Completion Verification Checklist

Before opening a pull request or submitting code, verify that `make check` succeeds completely:

```bash
# 1. Format check
make format-check

# 2. Module hygiene check
make tidy-check

# 3. Static analysis & linting
make lint

# 4. Full validation suite (tests with -race and -shuffle=on, parity, path invariants)
make check
```

---

## 7. Pull Request Guidelines

1. **Descriptive Commits**: Use clean, imperative commit messages (e.g. `Add Sigil II preset and configure DSDA-Doom mapping`).
2. **Path Audit**: Verify `git diff` contains zero hardcoded personal user paths or usernames before committing.
3. **PR Template**: Complete all sections of the [Pull Request Template](.github/PULL_REQUEST_TEMPLATE.md) including the design and verification checklists.
