# macOS Internal Deployment Runbook

## Purpose

This runbook is for internal macOS deployment only.

The goal is to make a small number of trusted Macs install, update, roll back, and verify Ant Browser reliably without formal public distribution.

This runbook does not require:

- Developer ID signing
- notarization
- Gatekeeper quarantine validation
- public release hosting
- release channel rollout

Those belong to a later formal distribution phase.

## Supported Internal Install Shape

Install Ant Browser to a user-writable app bundle:

```text
~/Applications/Ant Browser.app
```

Do not use automatic app update for:

```text
/Applications/Ant Browser.app
/System/Applications/Ant Browser.app
```

Protected install locations are intentionally rejected by the updater. Internal users should move or reinstall the app into `~/Applications`.

## Required Artifacts

For each internal rollout, prepare:

```text
publish/output/AntBrowser-<version>-macos-arm64.app
publish/output/AntBrowser-<version>-macos-arm64.zip
publish/output/AntBrowser-<version>-darwin-arm64.zip
publish/output/app-update-stable.json
```

For Intel Macs, use `amd64` equivalents:

```text
publish/output/AntBrowser-<version>-macos-amd64.app
publish/output/AntBrowser-<version>-darwin-amd64.zip
```

## Build And Verify Artifacts

From the repo root:

```bash
rtk python3 tools/runtime/verify-publish-contract.py
rtk proxy env PATH="/Users/robbin/go/bin:$PATH" bash publish/mac/publish-mac.sh --arch arm64
```

Verify the app-update package:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
rtk python3 tools/app-update/verify-app-update-package.py \
  publish/output/app-update-stable.json \
  "publish/output/AntBrowser-${VERSION}-darwin-arm64.zip" \
  darwin-arm64
```

Expected:

```text
[OK] app update package verified
```

Verify the exported app bundle:

```bash
rtk codesign --verify --deep --strict --verbose=2 "publish/output/AntBrowser-${VERSION}-macos-arm64.app"
rtk /usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "publish/output/AntBrowser-${VERSION}-macos-arm64.app/Contents/Info.plist"
```

The current internal build uses ad-hoc signing. That is acceptable for this internal deployment phase.

## Internal Manifest Hosting

Use any internal HTTP location reachable by target Macs.

Recommended layout:

```text
https://<internal-update-host>/ant-browser/macos/stable/app-update-stable.json
https://<internal-update-host>/ant-browser/macos/stable/AntBrowser-<version>-darwin-arm64.zip
```

The manifest package URL may be relative:

```json
{
  "packages": [
    {
      "target": "darwin-arm64",
      "payloadType": "full",
      "url": "AntBrowser-<version>-darwin-arm64.zip"
    }
  ]
}
```

Relative payload URLs are resolved from the manifest URL directory.

## Fresh Install On An Internal Mac

Create the user app directory:

```bash
mkdir -p "$HOME/Applications"
```

Install the app:

```bash
ditto "/path/to/AntBrowser-<version>-macos-arm64.app" "$HOME/Applications/Ant Browser.app"
```

Verify version:

```bash
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$HOME/Applications/Ant Browser.app/Contents/Info.plist"
```

Launch:

```bash
open "$HOME/Applications/Ant Browser.app"
```

If macOS blocks the app because it is not notarized, this is a local trust policy issue, not an app-update logic failure. For this internal phase, resolve it using the team's approved internal Mac trust process.

## Point A Mac At The Internal Update Manifest

Option A: environment variable for smoke tests:

```bash
DESKTOP_APP_UPDATE_MANIFEST_URL="https://<internal-update-host>/ant-browser/macos/stable/app-update-stable.json" \
  "$HOME/Applications/Ant Browser.app/Contents/MacOS/ant-chrome"
```

Option B: runtime config for repeatable internal usage:

```bash
mkdir -p "$HOME/Library/Application Support/ant-browser-runtime/config"
cat > "$HOME/Library/Application Support/ant-browser-runtime/config/app-update.json" <<'JSON'
{
  "manifestUrl": "https://<internal-update-host>/ant-browser/macos/stable/app-update-stable.json"
}
JSON

DESKTOP_RUNTIME_DIR="$HOME/Library/Application Support/ant-browser-runtime" \
  "$HOME/Applications/Ant Browser.app/Contents/MacOS/ant-chrome"
```

Use the runtime config path when asking non-technical internal users to test updates.

## Update Verification

Before clicking update, the prompt should show:

- current client version
- target client version
- `manifestSource`
- manifest URL
- status `available`

Click:

```text
更新并重启
```

Expected state progression:

```text
downloading or applying
verifying
succeeded
idle after relaunch
```

`idle` after relaunch is expected when the new app rechecks the same manifest and finds no further update.

## Version And Hash Verification

Verify app bundle version:

```bash
/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$HOME/Applications/Ant Browser.app/Contents/Info.plist"
```

Verify binary hash against the intended artifact:

```bash
shasum -a 256 \
  "$HOME/Applications/Ant Browser.app/Contents/MacOS/ant-chrome" \
  "/path/to/publish/output/AntBrowser-<version>-macos-arm64.app/Contents/MacOS/ant-chrome"
```

The first two hashes should match.

Check persistent update state:

```bash
cat "$HOME/Library/Application Support/ant-browser/app-update/state.json"
```

Expected successful final state:

```json
{
  "status": "idle",
  "localAppVersion": "<version>",
  "remoteAppVersion": "<version>",
  "lastError": {
    "code": "",
    "message": ""
  }
}
```

## Rollback And Manual Repair Verification

Rollback and manual repair are primarily covered by backend regression tests in this phase.

For internal Mac smoke testing, use a sandbox app path instead of a real user's working app:

```text
/private/tmp/ant-browser-internal-rollback-smoke/home/Applications/Ant Browser.app
```

Expected failure behavior:

- invalid staged payload fails before replacing the installed app
- replace failure restores the previous app when backup exists
- post-check version mismatch writes `failed_manual_repair`
- state file contains a clear `lastError.code`

Do not run destructive rollback experiments against a teammate's daily-use app.

## Cleanup

Remove temporary smoke-test sandboxes:

```bash
rm -rf /private/tmp/ant-browser-mac-regression-*
rm -rf /private/tmp/ant-browser-mac-http-smoke-*
rm -rf /private/tmp/ant-browser-internal-*
```

Remove local build output if Finder or Spotlight shows too many Ant Browser apps:

```bash
rm -rf build/bin/Ant\ Browser.app
```

Do not delete `publish/output` artifacts until the rollout evidence has been recorded.

## Internal Rollout Checklist

For each internal Mac, record:

1. Mac architecture: `arm64` or `amd64`.
2. Install path.
3. Starting version.
4. Manifest URL.
5. Target version.
6. Update result: success, rollback, or manual repair.
7. Final UI version.
8. Final `Info.plist` version.
9. Final binary hash match result.
10. Final `state.json` status and `lastError.code`.
