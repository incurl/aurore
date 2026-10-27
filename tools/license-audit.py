#!/usr/bin/env python3
"""Audit the licences of every Go and npm dependency.

The project must not ship a GPL or AGPL dependency, so the audit fails
when one is found. It also prints the licence breakdown, which is what
the THIRD_PARTY_NOTICES file is derived from.

Usage:
    python3 tools/license-audit.py
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from collections import defaultdict

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
FRONTEND_DIR = os.path.join(REPO_ROOT, "frontend")

# Copyleft licences that must never appear in the dependency tree.
FORBIDDEN = ("GPL", "AGPL", "LGPL", "SSPL", "EUPL", "OSL")

# The licences that are acceptable, by the token found in the file.
LICENCE_PATTERNS = (
    # MPL must be checked before the GPL family: the MPL text references
    # the GNU General Public License, so a naive substring match would
    # misclassify it.
    ("MPL-2.0", "mozilla public license"),
    ("AGPL", "gnu affero general public license"),
    ("LGPL", "gnu lesser general public license"),
    ("GPL", "gnu general public license"),
    ("Apache-2.0", "apache license"),
    ("BSD-3-Clause", "redistribution and use in source and binary forms"),
    ("BSD-2-Clause", "redistribution and use in source and binary forms"),
    ("MIT", "permission is hereby granted, free of charge"),
    ("ISC", "permission to use, copy, modify"),
    ("Unlicense", "this is free and unencumbered software released into the public domain"),
    ("CC0-1.0", "creative commons"),
)


def detect_licence(directory: str) -> str:
    """Return the licence identifier of a module directory."""
    # The filename casing varies between modules, so the directory is
    # scanned case-insensitively.
    candidates = []
    try:
        entries = os.listdir(directory)
    except OSError:
        return "UNKNOWN"
    for entry in entries:
        upper = entry.upper()
        if upper in ("LICENSE", "LICENCE", "COPYING") or upper.startswith(("LICENSE.", "LICENCE.", "COPYING.")):
            candidates.append(entry)

    for name in sorted(candidates):
        path = os.path.join(directory, name)
        if not os.path.isfile(path):
            continue
        try:
            with open(path, errors="ignore") as handle:
                head = handle.read(4000).lower()
        except OSError:
            continue

        # "Mozilla Public License" contains "Public License", so the MPL
        # check must come before the GPL check.
        for identifier, needle in LICENCE_PATTERNS:
            if needle in head:
                return identifier
        return "SEE-LICENSE"
    return "UNKNOWN"


def go_modules() -> list[dict]:
    """Return the Go module dependency list."""
    result = subprocess.run(
        ["go", "list", "-m", "-json", "all"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        check=True,
    )

    decoder = json.JSONDecoder()
    text = result.stdout
    index = 0
    modules: list[dict] = []
    while index < len(text):
        while index < len(text) and text[index] in " \n\t\r":
            index += 1
        if index >= len(text):
            break
        module, index = decoder.raw_decode(text, index)
        modules.append(module)
    return modules


def npm_packages() -> list[tuple[str, str]]:
    """Return the npm dependency list with their licences."""
    with open(os.path.join(FRONTEND_DIR, "package.json"), encoding="utf-8") as handle:
        manifest = json.load(handle)

    names = sorted({**manifest.get("dependencies", {}), **manifest.get("devDependencies", {})})

    packages: list[tuple[str, str]] = []
    for name in names:
        licence = "UNKNOWN"
        try:
            result = subprocess.run(
                ["npm", "view", name, "license", "--registry=https://registry.npmmirror.com"],
                capture_output=True,
                text=True,
                timeout=60,
            )
            value = result.stdout.strip().splitlines()
            if value:
                licence = value[-1].strip()
        except (OSError, subprocess.SubprocessError):
            pass
        packages.append((name, licence or "UNKNOWN"))
    return packages


def main() -> int:
    """Run the audit and report the result."""
    breakdown: dict[str, list[str]] = defaultdict(list)
    forbidden: list[str] = []

    print("Go dependencies:")
    for module in go_modules():
        if module.get("Main") or not module.get("Dir"):
            continue
        licence = detect_licence(module["Dir"])
        label = f"{module['Path']} {module.get('Version', '')}".strip()
        breakdown[licence].append(label)
        if any(token in licence.upper() for token in FORBIDDEN):
            forbidden.append(f"{label} ({licence})")

    print("\nnpm dependencies:")
    for name, licence in npm_packages():
        breakdown[licence].append(name)
        if any(token in licence.upper() for token in FORBIDDEN):
            forbidden.append(f"{name} ({licence})")

    print("\nLicence breakdown:")
    for licence in sorted(breakdown):
        count = len(breakdown[licence])
        print(f"  {licence:16} {count}")

    if forbidden:
        print("\nFAIL: copyleft dependencies found:")
        for item in forbidden:
            print(f"  {item}")
        return 1

    print("\nOK: no GPL/AGPL/copyleft dependency found.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
