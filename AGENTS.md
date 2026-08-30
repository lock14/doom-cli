# Agent Guidelines for doom-configs

This repository contains configuration files, presets, build targets, CLI tools, and bootstrap scripts for classic Doom source ports (**DSDA-Doom**, **UZDoom**) and launchers (**DoomRunner**, **`doom-launch`**) across Linux, macOS, and Windows.

Any agent modifying this repository must follow these core principles.

---

## 1. Declarative Presets & Single Source of Truth

- **`data/presets.json` is the Single Source of Truth**: All preset definitions, engine assignments, IWAD mappings, PWAD file lists, DeHackEd orderings, metadata, and download URLs are defined declaratively in [`data/presets.json`](data/presets.json).
- **Never Manually Edit Generated Preset JSONs**: Do not manually edit `DoomRunner/linux/options.json` or `DoomRunner/windows/options.json`. Run `python3 scripts/build-presets.py --build` or `make build-presets` to compile `data/presets.json` into both launcher options files.
- **Parity Verification**: Run `make check` (or `python3 scripts/build-presets.py --check`) to verify that launcher options match `data/presets.json` and comply with all path invariants.

---

## 2. Portability & Path Invariants

- **Never Commit Personal User Paths & Resolutions**: Never commit absolute personal paths like `/home/<username>/` or hardcoded local display resolutions into configuration files, presets, or scripts.
- **Use `__HOME__`, `__RESOLUTION__`, and `__SOUNDFONT__` Placeholders**:
  - In `DoomRunner/linux/options.json` and `data/presets.json`, all user home paths must use the `__HOME__` placeholder.
  - In `dsda-doom/dsda-doom.cfg`, `screen_resolution` must use `__RESOLUTION__` and `snd_soundfont` must use `__SOUNDFONT__`.
  - In `uzdoom/autoexec.cfg`, `fluid_patchset` must use `__SOUNDFONT__`.
  - Deployment tooling (`Makefile`, `setup.sh`) dynamically substitutes `__HOME__`, `__RESOLUTION__` (via `scripts/detect-resolution.sh`), and `__SOUNDFONT__` at installation time.
- **Respect Standard Directory Structures**:
  - **UZDoom Config (Linux)**: `~/.config/uzdoom/autoexec.cfg`
  - **UZDoom Config (macOS)**: `~/Library/Application Support/uzdoom/autoexec.cfg`
  - **DSDA-Doom Config (Linux)**: `~/.local/share/dsda-doom/dsda-doom.cfg`
  - **DSDA-Doom Config (macOS)**: `~/Library/Application Support/dsda-doom/dsda-doom.cfg`
  - **DoomRunner (Linux)**: `~/.local/share/DoomRunner/options.json`
  - **DoomRunner (macOS)**: `~/Library/Application Support/DoomRunner/options.json`
  - **DoomRunner (Windows)**: `%LOCALAPPDATA%\DoomRunner\options.json` & `%APPDATA%\DoomRunner\options.json`
  - **Linux / macOS Binaries**: `~/.local/bin/` (`uzdoom`, `dsda-doom`, `doomrunner`, `doom-launch`)
  - **Linux WADs Directory**: `~/.local/share/games/uzdoom/`
  - **macOS WADs Directory**: `~/Library/Application Support/games/uzdoom/`
  - **Windows WADs Directory**: `E:\Doom WADS\` (default, configurable via `-BaseDrive` in `setup.ps1`)
  - **SoundFonts Directory**: `~/.local/share/soundfonts/` (Linux) / `~/Library/Application Support/soundfonts/` (macOS)

---

## 3. Destructive Safety & Backup Policy

- **Mandatory Non-Destructive Backups**: Any installation target or deployment script (`Makefile`, `setup.sh`, `setup.ps1`) must create timestamped backups (`.bak.<timestamp>`) before overwriting or modifying any existing user configuration file.
- **Preserve Unrelated Configuration**: When adding new engine variables, aliases, or keybindings, avoid modifying or removing unrelated settings unless explicitly requested.

---

## 4. Doom Engine Selection & Preset Hygiene

- **Engine Mapping Rules**:
  - **DSDA-Doom**: Use for classic vanilla, Boom, MBF, and MBF21 maps where demo accuracy, standard physics, and speedrunning precision are desired (e.g., *Alien Vendetta*, *BTSX*, *Sunder*, *Sunlust*, *Legacy of Rust*, *Sigil*).
  - **UZDoom**: Use for mapsets requiring ZDoom/GZDoom features, advanced scripting, high-res texture packs like OTEX (*Eviternity I & II*), or Raven Software games (*Heretic*, *Hexen*).
- **Preset Hygiene in DoomRunner & `doom-launch`**:
  - **No Duplicate IWADs**: Never include the base game IWAD (`DOOM.WAD`, `DOOM2.WAD`, `PLUTONIA.WAD`, `TNT.WAD`) inside `mappacks`. The IWAD must only be specified in `iwad` / `selected_IWAD`.
  - **Proper Load Ordering**: Ensure DeHackEd patches (`.deh`) and resource files are ordered correctly relative to map PWADs and music wads (`idkfa 2024.wad`).
- **Visual Aesthetic Intent**:
  - Maintain UZDoom’s curated Nightdive "Software-Plus" visual profile (software light mode `gl_lightmode 0`, banded stepping `gl_bandedsw 1`, palette tonemapping `gl_tonemap 3`, and nearest-neighbor texture sampling with 16x anisotropic filtering) unless explicitly instructed otherwise.

---

## 5. Cross-Platform Parity & Tooling Maintenance

- **Cross-Platform Consistency**: When adding or updating presets in `data/presets.json`, ensure changes compile cleanly across Linux (`DoomRunner/linux/options.json`) and Windows (`DoomRunner/windows/options.json`).
- **Maintain Automation & Sync Targets**:
  - Every config file and script tracked in the repository must be integrated into:
    1. `make install` & `make install-<target>`
    2. `make sync` (to pull in-game tweaks from the system back into the repo)
    3. `make diff` (to inspect differences between repo and active system configs)
    4. `make check` (validation suite)
    5. `setup.sh` (Linux fallback script)
    6. `setup.ps1` (Windows PowerShell script)
    7. `README.md` (documentation, commands, and presets table)

---

## 6. Documentation Boundaries & Mandatory Updates

- **Mandatory Documentation Synchronization**:
  - **Every agent change must update documentation**: Whenever presets, engine settings, build targets, directory layouts, or script mechanics are modified, the relevant documentation **must** be updated within the same pull request/commit.
- **`README.md` is for Users**:
  - `README.md` must focus purely on user-facing concerns: installation commands, directory setup, presets catalog, engine features, CLI launcher usage, and player-facing tools.
  - Keep `README.md` clear, accessible, and free of internal agent mechanics or maintenance minutiae.
- **`AGENTS.md` is for Agents & Contributors**:
  - Repository principles, single-source-of-truth compiler mechanics, path invariant enforcement, internal build architecture, and agent guidelines belong in `AGENTS.md`.
  - Never leak prompt engineering directives, agent-specific meta-instructions, or internal maintenance rules into `README.md`.

---

## 7. Continuous Learning & Principle Encoding

- **Persist User Corrections**:
  - Whenever an agent is corrected, redirected, or receives feedback on repository conventions, it **must immediately encode the underlying principle into `AGENTS.md`** before concluding the task.
  - This ensures all future agent sessions and contributors automatically inherit the correction.

---

## 8. Verification Checklist for Agents

Before completing any changes:
1. Run `make check` to execute the full local validation suite:
   - ShellCheck and syntax validation on all shell scripts.
   - Declarative preset invariants and parity checks (`scripts/build-presets.py --check`).
   - JSON format validation.
   - Path invariant inspection.
   - Isolated dry installation and backup verification.
   - Comprehensive CLI suite (`scripts/test-doom-launch.sh` testing all 28 CLI options, aliases, engine overrides, and menu modes).
   - End-to-end turnkey sandbox verification (`scripts/test-turnkey.sh` testing mock Steam libraries, SoundFonts, WAD download, and CLI execution).
2. Verify `git diff` contains no hardcoded usernames or personal paths.
3. Run `make -n install` or `make help` to verify `Makefile` syntax.
4. If shell scripts were modified, run `shellcheck <script.sh>` and `bash -n <script.sh>`.
5. **Update Documentation**:
   - Update `README.md` if user-facing behavior changed (presets, commands, defaults, directories).
   - Update `AGENTS.md` if repository principles, build architecture, or agent rules changed.
   - **Encode Corrections**: If the user or a reviewer provided a correction, ensure the principle is codified in `AGENTS.md`.
6. Verify `README.md` tables (presets catalog, Makefile reference) accurately reflect the new state.
