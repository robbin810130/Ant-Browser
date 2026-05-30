#!/usr/bin/env python3

from __future__ import annotations

import json
import sys
from pathlib import Path


def fail(message: str) -> None:
    print(f"[ERROR] {message}", file=sys.stderr)
    raise SystemExit(1)


def load_json(path: Path) -> dict:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        fail(f"missing required file: {path}")
    except json.JSONDecodeError as exc:
        fail(f"invalid json in {path}: {exc}")


def assert_contains(text: str, needle: str, context: str) -> None:
    if needle not in text:
        fail(f"{context} is missing required text: {needle}")


def collect_required_packages(manifest: dict, target: str) -> list[dict]:
    packages = manifest.get("packages") or []
    normalized = target.strip().lower()
    return [
        pkg
        for pkg in packages
        if bool(pkg.get("required")) and str(pkg.get("target") or "").strip().lower() == normalized
    ]


def main() -> None:
    repo_root = Path(__file__).resolve().parents[2]

    manifest_path = repo_root / "publish" / "runtime-manifest.json"
    sources_path = repo_root / "publish" / "runtime-sources.json"
    installer_path = repo_root / "publish" / "installer.nsi"
    windows_publish_path = repo_root / "bat" / "publish.ps1"
    mac_publish_path = repo_root / "publish" / "mac" / "publish-mac.sh"
    release_readme_path = repo_root / "tools" / "public-release" / "README.md"

    manifest = load_json(manifest_path)
    sources = load_json(sources_path)

    if int(manifest.get("schemaVersion") or 0) != 2:
        fail("publish/runtime-manifest.json must use schemaVersion 2")
    if int(sources.get("schemaVersion") or 0) != 2:
        fail("publish/runtime-sources.json must use schemaVersion 2")

    for target in ("windows-amd64", "darwin-amd64", "darwin-arm64"):
        packages = collect_required_packages(manifest, target)
        if not packages:
            fail(f"runtime manifest is missing required packages for {target}")

    installer_text = installer_path.read_text(encoding="utf-8-sig")
    assert_contains(installer_text, 'File /r "${STAGINGDIR}\\publish\\*"', "publish/installer.nsi")
    assert_contains(installer_text, 'SetOutPath "$INSTDIR\\publish"', "publish/installer.nsi")
    assert_contains(installer_text, 'File /r "${STAGINGDIR}\\apps\\*"', "publish/installer.nsi")
    assert_contains(installer_text, 'File /r "${STAGINGDIR}\\runtime\\*"', "publish/installer.nsi")
    assert_contains(installer_text, 'CreateDirectory "$INSTDIR\\data"', "publish/installer.nsi")
    assert_contains(installer_text, '!define INSTALL_DIR     "$LOCALAPPDATA\\Programs\\Ant Browser"', "publish/installer.nsi")
    assert_contains(installer_text, "RequestExecutionLevel user", "publish/installer.nsi")
    assert_contains(installer_text, 'InstallDirRegKey HKCU "${UNINSTALL_KEY}" "InstallLocation"', "publish/installer.nsi")

    windows_publish_text = windows_publish_path.read_text(encoding="utf-8-sig")
    assert_contains(windows_publish_text, 'Copy-RuntimePublishPayload -Target "windows-amd64"', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'Copy-WindowsChromePayload -ChromeRoot $chromeRoot -StagingDir $stagingDir', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'Copy-WorkspaceAgentPayload -WorkspacePayloadRoot $workspacePayloadRoot -StagingDir $stagingDir', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'Copy-BundledWorkspaceNodePayload -WorkspacePayloadRoot $workspacePayloadRoot -StagingDir $stagingDir', "bat/publish.ps1")
    assert_contains(windows_publish_text, 'runtime/current.json 将在首次通过环境检查后写入用户状态目录', "bat/publish.ps1")
    assert_contains(windows_publish_text, "New-AppUpdateZip -StagingDir $stagingDir", "bat/publish.ps1")
    assert_contains(windows_publish_text, "New-AppUpdateManifest -ZipPath $appUpdateZip", "bat/publish.ps1")
    assert_contains(windows_publish_text, "tools/app-update/verify-app-update-package.py", "bat/publish.ps1")

    mac_publish_text = mac_publish_path.read_text(encoding="utf-8")
    assert_contains(mac_publish_text, 'cp "$ROOT_DIR/publish/runtime-manifest.json" "$APP_PUBLISH_DIR/runtime-manifest.json"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'cp "$XRAY_SRC" "$APP_PUBLISH_DIR/bin/$TARGET/xray"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'cp "$SINGBOX_SRC" "$APP_PUBLISH_DIR/bin/$TARGET/sing-box"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'APP_UPDATE_ZIP_NAME="AntBrowser-${VERSION}-${TARGET}.zip"', "publish/mac/publish-mac.sh")
    assert_contains(mac_publish_text, 'tools/app-update/verify-app-update-package.py', "publish/mac/publish-mac.sh")

    mac_config_text = (repo_root / "publish" / "config.init.mac.yaml").read_text(encoding="utf-8")
    assert_contains(mac_config_text, "app_update_manifest_url:", "publish/config.init.mac.yaml")

    readme_text = release_readme_path.read_text(encoding="utf-8")
    assert_contains(readme_text, "packaged builds must include `publish/runtime-manifest.json`", "tools/public-release/README.md")
    assert_contains(readme_text, "`runtime/current.json` is created in the writable state root", "tools/public-release/README.md")

    print("[OK] publish contract verified")


if __name__ == "__main__":
    main()
