#!/usr/bin/env python3

from __future__ import annotations

import sys
from pathlib import Path


def fail(message: str) -> None:
    print(f"[ERROR] {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    root = Path(__file__).resolve().parents[2]
    script = root / "tools" / "app-update" / "windows-app-update-e2e.ps1"
    if not script.is_file():
        fail(f"missing Windows app-update e2e script: {script}")

    text = script.read_text(encoding="utf-8")
    required_fragments = [
        "BaselineVersion",
        "TargetVersion",
        "Win32NT",
        "bat\\publish.bat",
        "verify-app-update-package.py",
        "DESKTOP_APP_UPDATE_MANIFEST_URL",
        "CurrentExePath: currentExe",
        "appupdate.WindowsBackend",
        "cmd\\app-update-e2e",
        "Expand-Archive",
        "Get-FileHash",
        "state.json",
        "localAppVersion",
        "data\\app.db",
    ]
    missing = [fragment for fragment in required_fragments if fragment not in text]
    if missing:
        fail("script missing required fragments: " + ", ".join(missing))

    print("[OK] Windows app-update e2e script contract verified")


if __name__ == "__main__":
    main()
