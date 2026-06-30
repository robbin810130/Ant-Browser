#!/usr/bin/env python3
import hashlib
import json
import re
import subprocess
import tempfile
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


def write_release_fixture(root: Path, version: str) -> Path:
    release_dir = root / "test" / version
    release_dir.mkdir(parents=True)
    zip_name = f"MakaBrowser-{version}-windows-amd64.zip"
    installer_name = f"MakaBrowser-Setup-{version}.exe"
    zip_bytes = b"verified maka browser update package\n"
    installer_bytes = b"verified maka browser installer\n"
    zip_sha = hashlib.sha256(zip_bytes).hexdigest()
    (release_dir / zip_name).write_bytes(zip_bytes)
    (release_dir / installer_name).write_bytes(installer_bytes)
    (release_dir / f"{zip_name}.sha256").write_bytes(
        f"{zip_sha}  {zip_name}\r\n".encode("ascii")
    )
    manifest = {
        "schemaVersion": 1,
        "product": "Maka Browser",
        "channel": "stable",
        "version": version,
        "packages": [
            {
                "target": "windows-amd64",
                "payloadType": "full",
                "url": zip_name,
                "sha256": zip_sha,
                "size": len(zip_bytes),
            }
        ],
    }
    manifest_path = release_dir / "app-update-stable.json"
    manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
    manifest_sha = hashlib.sha256(manifest_path.read_bytes()).hexdigest()
    (release_dir / "app-update-stable.json.sha256").write_bytes(
        f"{manifest_sha}  app-update-stable.json\r\n".encode("ascii")
    )
    return release_dir


def require_promotion_behavior(promotion_template: str) -> None:
    version = "9.8.7"
    with tempfile.TemporaryDirectory(prefix="maka-promotion-contract-") as temp_dir:
        remote_root = Path(temp_dir) / "releases" / "windows"
        test_dir = write_release_fixture(remote_root, version)
        rendered = (
            promotion_template.replace("$RemoteRoot", str(remote_root))
            .replace("$Version", version)
            .replace("$promotionId", "contracttest")
        )
        rendered = powershell_here_string_to_bash(rendered)

        for expected_message in (
            f"[OK] promoted {version} to stable",
            f"[OK] stable release already exists and matches test: {version}",
        ):
            result = subprocess.run(
                ["bash"],
                input=rendered,
                text=True,
                capture_output=True,
                check=False,
            )
            if result.returncode != 0:
                raise AssertionError(
                    f"promotion behavior failed with exit {result.returncode}:\n"
                    f"{result.stdout}\n{result.stderr}"
                )
            if expected_message not in result.stdout:
                raise AssertionError(f"promotion output missing: {expected_message}")

        stable_root = remote_root / "stable"
        stable_dir = stable_root / version
        expected_files = (
            stable_dir / f"MakaBrowser-{version}-windows-amd64.zip",
            stable_dir / f"MakaBrowser-Setup-{version}.exe",
            stable_dir / "app-update-stable.json",
            stable_root / f"MakaBrowser-{version}-windows-amd64.zip",
            stable_root / f"MakaBrowser-Setup-{version}.exe",
            stable_root / "app-update-stable.json",
        )
        for path in expected_files:
            if not path.is_file():
                raise AssertionError(f"promotion did not publish: {path}")
        if not test_dir.is_dir():
            raise AssertionError("promotion removed the original test release")
        if (stable_root / "app-update-stable.json").read_bytes() != (
            stable_dir / "app-update-stable.json"
        ).read_bytes():
            raise AssertionError("stable manifest alias differs from promoted manifest")


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
        "tr -d '\\r'",
        ".promoting-$Version-",
        "stable_zip=",
        "stable_installer=",
        "zip_alias_tmp=",
        "installer_alias_tmp=",
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
    require_promotion_behavior(extract_here_string(script, "remotePromotion"))
    if promotion_bash.index("sha256sum -c") > promotion_bash.index("mv -f"):
        raise AssertionError("promotion publishes before SHA256 verification")
    if promotion_bash.index("stable_zip=") > promotion_bash.rindex("stable_alias"):
        raise AssertionError("stable payload aliases are not prepared before the manifest alias")
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
