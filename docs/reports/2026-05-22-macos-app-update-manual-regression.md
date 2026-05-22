# macOS App Update Manual Regression Report

Date: 2026-05-22
Branch: `codex/windows-phase1-stability`
Commit: `21522d3 Harden macOS app update regression path`

## Scope

This pass closed real macOS manual regression for application self-update only.

Formal distribution checks were intentionally not run in this pass:

- Developer ID signing
- notarization
- Gatekeeper quarantine validation
- public release upload

## Artifacts

Generated local artifacts:

- `publish/output/AntBrowser-1.0.0-macos-arm64.app`
- `publish/output/AntBrowser-1.0.0-darwin-arm64.zip`
- `publish/output/AntBrowser-1.1.0-macos-arm64.app`
- `publish/output/AntBrowser-1.1.0-darwin-arm64.zip`
- `publish/output/app-update-stable.json`

Final regression sandbox:

```text
/tmp/ant-browser-mac-regression-20260522-final
```

Installed app path:

```text
/tmp/ant-browser-mac-regression-20260522-final/home/Applications/Ant Browser.app
```

Runtime manifest config:

```text
/tmp/ant-browser-mac-regression-20260522-final/runtime/config/app-update.json
```

## Real UI Regression

Setup:

1. Built a local `1.0.0` baseline app.
2. Built a local `1.1.0` app-update payload and manifest.
3. Copied the `1.0.0` app into a user-writable `~/Applications`-style sandbox.
4. Pointed `DESKTOP_RUNTIME_DIR` at a sandbox runtime config whose app-update manifest used a local `file://` URL.
5. Launched the app with sandboxed `HOME` and `DESKTOP_RUNTIME_DIR`.

Observed before update:

- Gate page showed client version `1.0.0`.
- Required client update prompt appeared.
- Manifest source was `runtime-config`.
- Target version was `1.1.0`.
- Update button was available.

Action:

- Clicked `更新并重启` in the real macOS app UI.

Observed state progression:

```text
verifying 1.0.0 -> 1.1.0
succeeded 1.1.0 -> 1.1.0
idle      1.1.0 -> 1.1.0
```

The final `idle` state is expected because the relaunched `1.1.0` app rechecked the same manifest and found no further update.

Observed after update:

- Relaunched app UI showed client version `1.1.0`.
- Installed app `Info.plist` reported `1.1.0`.
- Installed binary hash matched `publish/output/AntBrowser-1.1.0-macos-arm64.app`.
- Installed binary hash differed from the `1.0.0` baseline.
- Final `state.json` had empty `lastError`.

Final state:

```json
{
  "status": "idle",
  "localAppVersion": "1.1.0",
  "remoteAppVersion": "1.1.0",
  "manifestSource": "runtime-config",
  "target": "darwin-arm64",
  "lastError": {
    "code": "",
    "message": ""
  }
}
```

## Bugs Found And Fixed

### 1. macOS update zip symlinks failed during staging

Failure:

```text
APP-UPDATE-STAGE-FAILED
zip symlink entries are not supported
```

Cause:

Chromium Framework inside the macOS `.app` bundle contains relative symlinks such as:

```text
Resources -> Versions/Current/Resources
Versions/Current -> 142.0.7444.175
```

Fix:

- `ExtractFullPayload` now supports safe relative zip symlinks.
- Absolute symlink targets and destination-escaping targets are rejected.
- Regression tests cover safe and escaping symlink entries.

### 2. macOS bundle backup/replace failed on symlinks

Failure:

```text
app update apply failed: symlink copy is not supported
```

Cause:

The bundle copy path used by backup, replace, and rollback rejected symlinks.

Fix:

- `copyDir` now copies safe relative symlinks.
- Absolute symlink targets and source-root escaping targets are rejected.
- Regression tests cover safe and escaping symlink copies.

### 3. Detached post-check launch was too fragile

Symptom:

Manual runner and manual post-check could complete, but the automatic detached post-check path did not reliably leave a completed state.

Fix:

- Darwin detached commands now set the executable directory as `cmd.Dir`.
- stdin/stdout/stderr are connected to `/dev/null`.
- Darwin launches detach into a new process group.
- Apply runner and post-check both use the same detached launch helper.

### 4. User-state chrome seeding broke Framework symlinks

Failure:

On first launch, detached state initialization logged a user data preparation error and the browser core path was invalid.

Cause:

`apppath.EnsureWritableLayout` copied the bundled `chrome/` directory into the user state root but did not preserve macOS Framework symlinks.

Fix:

- User-state chrome seeding now copies safe relative symlinks.
- Escaping symlinks are rejected.
- Final manual regression confirmed the copied Framework link:

```text
Resources -> Versions/Current/Resources
```

## Unsupported Install Regression

Automatic app update remains unsupported for protected macOS install locations:

```text
/Applications/Ant Browser.app
/System/Applications/...
```

Verification:

```bash
rtk go test ./backend/internal/appupdate -run 'TestDarwinBackendValidateInstallModeRejectsApplicationsInstall|TestDarwinBackendValidateInstallModeRejectsSystemApplicationsInstall' -count=1
```

Expected behavior:

- The backend rejects these install roots before staging or replacement.
- No app bundle files are deleted or replaced.
- The user must reinstall or move the app into a user-writable location such as `~/Applications/Ant Browser.app`.

## Failure Scenario Regression

These scenarios were verified without formal distribution. They are intentionally local backend/package-contract checks, not notarized release checks.

### Checksum mismatch

Verification:

```bash
rtk go test ./backend/internal/appupdate -run TestManagerDownloadRejectsDarwinChecksumMismatch -count=1
```

Expected behavior:

- The package is downloaded from the selected `darwin-arm64` manifest package.
- Hash mismatch is reported as `APP-UPDATE-DOWNLOAD-FAILED`.
- Persistent state remains `available` with `lastError.code` set.
- The update does not create the target staging directory.
- The apply runner is not launched.

### Invalid `.app` payload

Verification:

```bash
rtk go test ./backend/internal/appupdate -run 'TestValidateStagedPayloadRejectsDarwinMissingInfoPlist|TestValidateStagedPayloadRejectsDarwinNonExecutableMainBinary|TestDarwinBackendRunApplyRejectsTamperedStagedPayloadBeforeRemovingInstall' -count=1
```

Expected behavior:

- Invalid staged bundles fail validation before replacement.
- A tampered staged payload does not remove the existing installed app.
- The user remains on the previously installed app.

### Post-check version mismatch

Verification:

```bash
rtk go test ./backend/internal/appupdate -run TestDarwinBackendPostUpdateCheckRejectsVersionMismatch -count=1
```

Expected behavior:

- Post-check compares the running app version with the apply plan target version.
- A mismatch writes `failed_manual_repair`.
- `lastError.code` is `APP-UPDATE-POST-CHECK-VERSION-MISMATCH`.

## Verification Commands

Fresh verification run:

```bash
rtk go test . ./backend ./backend/internal/appupdate ./backend/internal/config ./backend/internal/apppath -count=1
rtk npm --prefix frontend run build
rtk python3 tools/runtime/verify-publish-contract.py
rtk python3 -m py_compile tools/runtime/verify-publish-contract.py
rtk python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json publish/output/AntBrowser-1.1.0-darwin-arm64.zip darwin-arm64
rtk codesign --verify --deep --strict --verbose=2 publish/output/AntBrowser-1.1.0-macos-arm64.app
rtk git diff --check
```

Results:

- Go tests: `276 passed in 5 packages`
- Frontend build: passed
- Publish contract: passed
- App-update package verifier: passed
- Codesign verification: valid on disk and satisfies designated requirement
- Diff check: passed

## Remaining Before Formal Distribution

Do not treat this pass as formal macOS distribution readiness. Before public distribution, still run:

1. Developer ID signing.
2. Notarization.
3. Gatekeeper quarantine validation.
4. Launch after download from the intended distribution channel.
5. Release-channel manifest hosting validation.
