#!/usr/bin/env python3
import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PROMOTION_SCRIPT = ROOT / "tools" / "release" / "promote-windows-release.ps1"
PROMOTION_WORKFLOW = ROOT / ".github" / "workflows" / "windows-stable-promotion.yml"


def read_text(path: Path) -> str:
    if not path.is_file():
        raise AssertionError(f"missing required file: {path.relative_to(ROOT)}")
    return path.read_text(encoding="utf-8-sig")


def require_contains(text: str, needle: str, source: Path) -> None:
    if needle not in text:
        raise AssertionError(f"{source.relative_to(ROOT)} missing expected text: {needle}")


def require_not_contains(text: str, needle: str, source: Path) -> None:
    if needle in text:
        raise AssertionError(f"{source.relative_to(ROOT)} contains forbidden text: {needle}")


def extract_here_string(text: str, variable_name: str) -> str:
    pattern = re.compile(
        rf"^\s*\${re.escape(variable_name)}\s*=\s*@\"\n(?P<body>.*?)\n\"@",
        re.MULTILINE | re.DOTALL,
    )
    match = pattern.search(text)
    if not match:
        raise AssertionError(
            f"{PROMOTION_SCRIPT.relative_to(ROOT)} missing here-string: ${variable_name}"
        )
    return match.group("body")


def powershell_here_string_to_bash(text: str) -> str:
    return text.replace("`$", "$").replace("``", "`")


def require_bash_syntax(script_text: str) -> None:
    if "\r" in script_text:
        raise AssertionError("promotion Bash script contains CR characters")
    result = subprocess.run(
        ["bash", "-n"],
        input=script_text,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise AssertionError(f"promotion Bash script is not valid:\n{result.stderr}")


def main() -> None:
    script = read_text(PROMOTION_SCRIPT)
    workflow = read_text(PROMOTION_WORKFLOW)

    for needle in (
        "Invoke-RemoteBashScript",
        "Normalize-PrivateKeyText",
        'Replace("`r`n", "`n").Replace("`r", "`n")',
        "bash '$RemoteScriptPath'",
        "$remotePromotion = @\"",
        "sha256sum -c",
        ".promoting-$Version-",
        "mv -f",
        "app-update-stable.json",
        "MakaBrowser-Setup-$Version.exe",
        "remote stable promotion script",
    ):
        require_contains(script, needle, PROMOTION_SCRIPT)

    for needle in (
        "$sshBaseArgs",
        'Invoke-Native -FilePath "ssh" -Arguments ($sshBaseArgs',
        '$remotePromotion = "set -eu;',
    ):
        require_not_contains(script, needle, PROMOTION_SCRIPT)

    promotion_bash = powershell_here_string_to_bash(
        extract_here_string(script, "remotePromotion")
    )
    require_bash_syntax(promotion_bash)
    if promotion_bash.index("sha256sum -c") > promotion_bash.index("mv -f"):
        raise AssertionError("promotion publishes before SHA256 verification")
    if promotion_bash.rindex("app-update-stable.json") < promotion_bash.index("mv -f"):
        raise AssertionError("stable alias is not updated after version publication")

    for needle in (
        "name: Windows Stable Promotion",
        "workflow_dispatch:",
        "target_version:",
        "runs-on: [self-hosted, windows, ant-browser-release]",
        "group: windows-release-factory-${{ github.repository }}",
        "cancel-in-progress: false",
        "WINDOWS_RELEASE_SSH_HOST: ${{ secrets.WINDOWS_RELEASE_SSH_HOST }}",
        "tools\\release\\promote-windows-release.ps1",
        "tools\\release\\check-windows-release-url.ps1",
        "/releases/windows/stable/app-update-stable.json",
        "/releases/windows/stable/MakaBrowser-Setup-$env:TARGET_VERSION.exe",
        "/releases/windows/test/$env:TARGET_VERSION/app-update-stable.json",
    ):
        require_contains(workflow, needle, PROMOTION_WORKFLOW)

    for needle in (
        "bat\\publish.ps1",
        "windows-release-factory.yml",
        "upload-windows-release.ps1",
    ):
        require_not_contains(workflow, needle, PROMOTION_WORKFLOW)

    print("[OK] Windows stable promotion contract verified")


if __name__ == "__main__":
    main()
