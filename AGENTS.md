# Agent Guidelines for doom-configs

This repository contains configuration files, presets, build targets, and bootstrap scripts for classic Doom source ports (**DSDA-Doom**, **UZDoom**) and the **DoomRunner** graphical launcher across Linux and Windows.

Any agent modifying this repository must follow these core principles.

---

## 1. Portability & Path Invariants

- **Never Commit Personal User Paths**: Never commit absolute personal paths like `/home/<username>/` into configuration files or scripts.
- **Use `__HOME__` for Linux Presets**: In `DoomRunner/linux/options.json`, all user home paths must use the `__HOME__` placeholder. Deployment tooling (`Makefile`, `setup.sh`) will dynamically substitute `__HOME__` with `$HOME` at installation time.
- **Respect Standard Directory Structures**:
  - **UZDoom Config (Linux)**: `~/.config/uzdoom/autoexec.cfg`
  - **UZDoom Config (macOS)**: `~/Library/Application Support/uzdoom/autoexec.cfg`
  - **DSDA-Doom Config (Linux)**: `~/.local/share/dsda-doom/dsda-doom.cfg`
  - **DSDA-Doom Config (macOS)**: `~/Library/Application Support/dsda-doom/dsda-doom.cfg`
  - **DoomRunner (Linux)**: `~/.local/share/DoomRunner/options.json`
  - **DoomRunner (macOS)**: `~/Library/Application Support/DoomRunner/options.json`
  - **DoomRunner (Windows)**: `%LOCALAPPDATA%\DoomRunner\options.json` & `%APPDATA%\DoomRunner\options.json`
  - **Linux / macOS Binaries**: `~/.local/bin/` (`uzdoom`, `dsda-doom`, `doomrunner`)
  - **Linux WADs Directory**: `~/.local/share/games/uzdoom/`
  - **macOS WADs Directory**: `~/Library/Application Support/games/uzdoom/`
  - **Windows WADs Directory**: `E:\Doom WADS\` (default, configurable via `-BaseDrive` in `setup.ps1`)

---

## 2. Destructive Safety & Backup Policy

- **Mandatory Non-Destructive Backups**: Any installation target or deployment script (`Makefile`, `setup.sh`, `setup.ps1`) must create timestamped backups (`.bak.<timestamp>`) before overwriting or modifying any existing user configuration file.
- **Preserve Unrelated Configuration**: When adding new engine variables, aliases, or keybindings, avoid modifying or removing unrelated settings unless explicitly requested.

---

## 3. Doom Engine Selection & Preset Hygiene

- **Engine Mapping Rules**:
  - **DSDA-Doom**: Use for classic vanilla, Boom, MBF, and MBF21 maps where demo accuracy, standard physics, and speedrunning precision are desired (e.g., *Alien Vendetta*, *BTSX*, *Sunder*, *Sunlust*, *Legacy of Rust*, *Sigil*).
  - **UZDoom**: Use for mapsets requiring ZDoom/GZDoom features, advanced scripting, high-res texture packs like OTEX (*Eviternity I & II*), or Raven Software games (*Heretic*, *Hexen*).
- **Preset Hygiene in DoomRunner**:
  - **No Duplicate IWADs**: Never include the base game IWAD (`DOOM.WAD`, `DOOM2.WAD`, `PLUTONIA.WAD`, `TNT.WAD`) inside `selected_mappacks`. The IWAD must only be specified in `selected_IWAD`.
  - **Proper Load Ordering**: Ensure DeHackEd patches (`.deh`) and resource files are ordered correctly relative to map PWADs and music wads (`idkfa 2024.wad`).
- **Visual Aesthetic Intent**:
  - Maintain UZDoom’s curated Nightdive "Software-Plus" visual profile (software light mode `gl_lightmode 0`, banded stepping `gl_bandedsw 1`, palette tonemapping `gl_tonemap 3`, and nearest-neighbor texture sampling with 16x anisotropic filtering) unless explicitly instructed otherwise.

---

## 4. Cross-Platform Parity & Tooling Maintenance

- **Cross-Platform Consistency**: When adding or updating presets, ensure changes are mirrored across both Linux (`DoomRunner/linux/options.json`) and Windows (`DoomRunner/windows/options.json`).
- **Maintain Bidirectional Sync Targets**:
  - Every config file tracked in the repository must be integrated into:
    1. `make install` & `make install-<target>`
    2. `make sync` (to pull in-game tweaks from the system back into the repo)
    3. `make diff` (to inspect differences between repo and active system configs)
    4. `setup.sh` (Linux fallback script)
    5. `setup.ps1` (Windows PowerShell script)
    6. `README.md` (documentation and presets table)

---

---

## 5. Documentation Boundaries & Mandatory Updates

- **Mandatory Documentation Synchronization**:
  - **Every agent change must update documentation**: Whenever presets, engine settings, build targets, directory layouts, or script mechanics are modified, the relevant documentation **must** be updated within the same pull request/commit.
- **`README.md` is for Users**:
  - `README.md` must focus purely on user-facing concerns: installation commands, directory setup, presets catalog, engine features, and player-facing usage.
  - Keep `README.md` clear, accessible, and free of internal agent mechanics or maintenance minutiae.
- **`AGENTS.md` is for Agents & Contributors**:
  - Repository principles, path invariant enforcement, internal build architecture, script implementation details, and agent guidelines belong in `AGENTS.md`.
  - Never leak prompt engineering directives, agent-specific meta-instructions, or internal maintenance rules into `README.md`.

---

---

## 6. Continuous Learning & Principle Encoding

- **Persist User Corrections**:
  - Whenever an agent is corrected, redirected, or receives feedback on repository conventions, it **must immediately encode the underlying principle into `AGENTS.md`** before concluding the task.
  - This ensures all future agent sessions and contributors automatically inherit the correction.

---

## 7. Verification Checklist for Agents

Before completing any changes:
1. Run `make check` to execute the full local validation suite (syntax, JSON validity, path invariants, and dry install).
2. Verify `git diff` contains no hardcoded usernames or personal paths.
3. Run `make -n install` or `make help` to verify `Makefile` syntax.
4. If shell scripts were modified, run `bash -n <script.sh>` to verify syntax.
5. **Update Documentation**:
   - Update `README.md` if user-facing behavior changed (presets, commands, defaults, directories).
   - Update `AGENTS.md` if repository principles, build architecture, or agent rules changed.
   - **Encode Corrections**: If the user or a reviewer provided a correction, ensure the principle is codified in `AGENTS.md`.
6. Verify `README.md` tables (presets catalog, Makefile reference) accurately reflect the new state.
