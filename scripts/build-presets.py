#!/usr/bin/env python3
"""
build-presets.py - Single Source of Truth Compiler for Doom Presets

Compiles data/presets.json into:
1. DoomRunner/linux/options.json
2. DoomRunner/windows/options.json
3. README.md presets table (verification & generation)

Validates path invariants and ensures 1:1 cross-platform parity.
"""

import argparse
import json
import os
import re
import sys
from pathlib import Path

# Ensure UTF-8 or safe fallback output on Windows legacy code pages (e.g. cp1252)
if hasattr(sys.stdout, "reconfigure"):
    try:
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass
if hasattr(sys.stderr, "reconfigure"):
    try:
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass


def safe_print(msg, file=sys.stdout):
    try:
        print(msg, file=file)
    except UnicodeEncodeError:
        safe_msg = msg.encode(getattr(file, "encoding", "ascii") or "ascii", errors="replace").decode(
            getattr(file, "encoding", "ascii") or "ascii"
        )
        print(safe_msg, file=file)

ROOT_DIR = Path(__file__).resolve().parent.parent
PRESETS_FILE = ROOT_DIR / "data" / "presets.json"
LINUX_OPTIONS_FILE = ROOT_DIR / "DoomRunner" / "linux" / "options.json"
WINDOWS_OPTIONS_FILE = ROOT_DIR / "DoomRunner" / "windows" / "options.json"
README_FILE = ROOT_DIR / "README.md"


def load_presets():
    if not PRESETS_FILE.exists():
        print(f"Error: Presets file not found at {PRESETS_FILE}", file=sys.stderr)
        sys.exit(1)
    with open(PRESETS_FILE, "r", encoding="utf-8") as f:
        return json.load(f)


def build_linux_presets(presets_data):
    linux_presets = []
    for p in presets_data:
        engine_path = f"__HOME__/.local/bin/{p['engine']}"
        iwad_path = f"__HOME__/.local/share/games/uzdoom/{p['iwad']}"
        mappacks = [
            f"__HOME__/.local/share/games/uzdoom/{m}"
            for m in p.get("mappacks", [])
        ]
        preset_obj = {
            "additional_args": p.get("additional_args", ""),
            "alternative_paths": {
                "config_dir": "",
                "demo_dir": "",
                "save_dir": "",
                "screenshot_dir": "",
            },
            "compatibility_options": {
                "compat_mode": -1,
                "compatflags1": 0,
                "compatflags2": 0,
            },
            "env_vars": {},
            "load_maps_after_mods": p.get("load_maps_after_mods", False),
            "mods": [],
            "name": p["name"],
            "selected_IWAD": iwad_path,
            "selected_config": "",
            "selected_engine": engine_path,
            "selected_mappacks": mappacks,
        }
        linux_presets.append(preset_obj)
    return linux_presets


def build_windows_presets(presets_data, engines_meta):
    win_presets = []
    for p in presets_data:
        engine_id = engines_meta[p["engine"]]["windows_id"]
        iwad_path = f"E:/Doom WADS/{p['iwad']}"
        mappacks = [
            f"E:/Doom WADS/{m}" for m in p.get("mappacks", [])
        ]
        preset_obj = {
            "additional_args": p.get("additional_args", ""),
            "alternative_paths": {
                "config_dir": "",
                "demo_dir": "",
                "save_dir": "",
                "screenshot_dir": "",
            },
            "compatibility_options": {
                "compat_mode": -1,
                "compatflags1": 0,
                "compatflags2": 0,
            },
            "env_vars": {},
            "load_maps_after_mods": p.get("load_maps_after_mods", False),
            "mods": [],
            "name": p["name"],
            "selected_IWAD": iwad_path,
            "selected_config": "",
            "selected_engine": engine_id,
            "selected_mappacks": mappacks,
        }
        win_presets.append(preset_obj)
    return win_presets


def generate_readme_table(presets_data):
    lines = [
        "| Megawad / Expansion | Engine | Compatibility / Details |",
        "| :--- | :--- | :--- |",
    ]
    for p in presets_data:
        engine_display = "DSDA-Doom" if p["engine"] == "dsda-doom" else "UZDoom"
        desc = p.get("description", "")
        lines.append(f"| **{p['name']}** | {engine_display} | {desc} |")
    return "\n".join(lines)


def build_all(update_readme=True):
    data = load_presets()
    presets_data = data["presets"]
    engines_meta = data["engines"]

    # Update Linux options
    with open(LINUX_OPTIONS_FILE, "r", encoding="utf-8") as f:
        linux_options = json.load(f)
    linux_options["presets"] = build_linux_presets(presets_data)
    with open(LINUX_OPTIONS_FILE, "w", encoding="utf-8") as f:
        json.dump(linux_options, f, indent=4)
        f.write("\n")
    safe_print(f"✓ Generated {LINUX_OPTIONS_FILE} ({len(presets_data)} presets)")

    # Update Windows options
    with open(WINDOWS_OPTIONS_FILE, "r", encoding="utf-8") as f:
        win_options = json.load(f)
    win_options["presets"] = build_windows_presets(presets_data, engines_meta)
    with open(WINDOWS_OPTIONS_FILE, "w", encoding="utf-8") as f:
        json.dump(win_options, f, indent=4)
        f.write("\n")
    safe_print(f"✓ Generated {WINDOWS_OPTIONS_FILE} ({len(presets_data)} presets)")

    if update_readme and README_FILE.exists():
        with open(README_FILE, "r", encoding="utf-8") as f:
            content = f.read()

        table = generate_readme_table(presets_data)
        # Match table between ## Preconfigured Presets and ## Engine Profiles
        pattern = r"(## Preconfigured Presets\n\n[^\n]+\n\n)(\|[\s\S]+?)(\n\n---)"
        new_content = re.sub(
            pattern,
            rf"\g<1>{table}\g<3>",
            content,
        )
        with open(README_FILE, "w", encoding="utf-8") as f:
            f.write(new_content)
        safe_print(f"✓ Updated presets table in {README_FILE}")


def check_invariants():
    data = load_presets()
    presets_data = data["presets"]
    engines_meta = data["engines"]

    errors = []

    # 1. Check path invariants in presets.json
    presets_str = json.dumps(data)
    if re.search(r"/home/[a-zA-Z0-9_-]+", presets_str) or re.search(r"C:\\Users", presets_str):
        errors.append("Hardcoded personal user path detected in data/presets.json")

    # 2. Check duplicate IWADs in mappacks
    for p in presets_data:
        iwad = p["iwad"].upper()
        for m in p.get("mappacks", []):
            if m.upper() == iwad or m.upper() in ["DOOM.WAD", "DOOM2.WAD", "PLUTONIA.WAD", "TNT.WAD", "HERETIC.WAD", "HEXEN.WAD"]:
                errors.append(f"Preset '{p['name']}' includes base IWAD '{m}' in mappacks list")

    # 3. Check parity with Linux options.json
    if LINUX_OPTIONS_FILE.exists():
        with open(LINUX_OPTIONS_FILE, "r", encoding="utf-8") as f:
            linux_options = json.load(f)
        expected_linux = build_linux_presets(presets_data)
        if linux_options.get("presets") != expected_linux:
            errors.append(f"{LINUX_OPTIONS_FILE} is out of sync with data/presets.json. Run 'make build-presets'.")
    else:
        errors.append(f"Missing {LINUX_OPTIONS_FILE}")

    # 4. Check parity with Windows options.json
    if WINDOWS_OPTIONS_FILE.exists():
        with open(WINDOWS_OPTIONS_FILE, "r", encoding="utf-8") as f:
            win_options = json.load(f)
        expected_win = build_windows_presets(presets_data, engines_meta)
        if win_options.get("presets") != expected_win:
            errors.append(f"{WINDOWS_OPTIONS_FILE} is out of sync with data/presets.json. Run 'make build-presets'.")
    else:
        errors.append(f"Missing {WINDOWS_OPTIONS_FILE}")

    # 5. Check parity with README.md presets table
    if README_FILE.exists():
        with open(README_FILE, "r", encoding="utf-8") as f:
            readme_content = f.read()
        expected_table = generate_readme_table(presets_data)
        if expected_table not in readme_content:
            errors.append(f"{README_FILE} presets table is out of sync with data/presets.json. Run 'make build-presets'.")

    if errors:
        safe_print("Validation errors encountered in presets:", file=sys.stderr)
        for err in errors:
            safe_print(f"  - {err}", file=sys.stderr)
        sys.exit(1)
    else:
        safe_print("✓ All preset invariants and cross-platform parity checks passed successfully.")


def main():
    parser = argparse.ArgumentParser(description="Compile and validate Doom presets.")
    parser.add_argument(
        "--build",
        action="store_true",
        help="Compile data/presets.json into Linux/Windows options.json and README.md",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Validate that options.json files and README.md are synchronized with data/presets.json",
    )
    parser.add_argument(
        "--update-readme",
        action="store_true",
        help="Update presets table in README.md from data/presets.json",
    )

    args = parser.parse_args()

    if args.check:
        check_invariants()
    elif args.build or args.update_readme:
        build_all(update_readme=True)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
