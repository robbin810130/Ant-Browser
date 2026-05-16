# Ant Browser Application Self-Update Design

Date: 2026-05-16
Branch: `codex/windows-phase1-stability`
Status: Brainstorming design approved, pending written spec review and implementation plan

## Context

The current release line has a working Windows startup gate and runtime update flow. That flow updates runtime resources and switches `runtime/current.json`; it does not update the installed application binary, install payload, or `ant-chrome.exe` itself.

The next phase adds application self-update so client bugfixes can ship without asking users to manually reinstall. The design must support Windows first while preserving a clean path for macOS.

## Approved Decisions

1. Future Windows installs default to a user-writable install location, such as `%LOCALAPPDATA%\Programs\Ant Browser`.
2. Existing `Program Files` installs are not silently self-updated. They are shown a migration or reinstall prompt.
3. Phase 1 uses full package payloads. Delta patch support is reserved for Phase 2.
4. Failed updates must automatically roll back and relaunch the old version when rollback is possible.
5. The updater uses the existing application binary in a short-lived CLI mode: `ant-chrome.exe --apply-update`.
6. The architecture is cross-platform by design: shared app-update core plus platform-specific apply backends.

## Non-Goals

Phase 1 does not implement delta patches.

Phase 1 does not install a long-running updater service or helper.

Phase 1 does not attempt silent replacement of `Program Files` installs without elevation.

Phase 1 does not merge application self-update into the existing runtime pointer update flow.

## Architecture

Application self-update is split into a shared core and platform backends.

The shared core owns:

- app-update manifest parsing
- target selection
- version comparison and update classification
- manifest source resolution
- package download
- `sha256` and size verification
- staging layout
- persistent state and apply plan files
- user-visible error classification
- backend-facing interfaces

The shared core must not contain Windows-specific path rules, NSIS assumptions, `.exe` replacement details, macOS `.app` bundle details, or signing logic.

Platform backends own:

- install mode validation
- payload shape validation for that OS
- preparation of the apply runner
- file or bundle replacement
- rollback execution
- platform-specific post-update checks

The backend interface should be small enough that Windows and macOS can implement it independently:

```go
type PlatformUpdater interface {
	Target() string
	ValidateInstallMode(layout Layout) error
	PrepareApply(plan ApplyPlan) error
	SpawnApplyRunner(planPath string) error
	RunApply(planPath string) error
	PostUpdateCheck(planPath string) error
}
```

The exact Go type names can follow the local codebase style during implementation, but this boundary is part of the design.

## Windows Backend

Windows Phase 1 supports only user-writable installs.

Expected install root:

```text
%LOCALAPPDATA%\Programs\Ant Browser
```

The backend rejects automatic update when:

- install root is under `Program Files` or `Program Files (x86)`
- install root is not writable by the current user
- the current executable cannot be copied into the update runner directory
- required installed payloads are missing before backup

The running app downloads and stages the update. It then copies the current executable to:

```text
stateRoot/app-update/runner/ant-chrome-updater.exe
```

The copied runner is launched with:

```text
ant-chrome-updater.exe --apply-update <absolute-plan-path>
```

The main GUI process then exits. The runner waits for the main process to exit before touching the install directory.

## macOS Backend

macOS is not implemented in Phase 1, but the shared design must support it without rework.

Expected user-writable install root:

```text
~/Applications/Ant Browser.app
```

The macOS backend will replace the full `.app` bundle rather than individual files. It must validate:

- bundle shape
- executable presence
- bundle identifier and version
- code signature when signing is enabled
- notarization and quarantine behavior in release builds

The shared manifest, staging layout, state machine, and frontend API are the same as Windows. Only the platform backend changes.

## Manifest

Application self-update uses a new app-update manifest. It does not reuse `publish/runtime-manifest.json`.

Example:

```json
{
  "schemaVersion": 1,
  "channel": "stable",
  "version": "1.2.0",
  "minimumRuntimeResourceVersion": "2026.05.16",
  "minimumAppVersion": "1.1.0",
  "publishedAt": "2026-05-16T00:00:00Z",
  "notes": ["Fix login regression"],
  "packages": [
    {
      "target": "windows-amd64",
      "payloadType": "full",
      "url": "https://example.com/AntBrowser-1.2.0-windows-amd64.zip",
      "sha256": "0123456789abcdef",
      "size": 123456789
    },
    {
      "target": "darwin-arm64",
      "payloadType": "full",
      "url": "https://example.com/AntBrowser-1.2.0-darwin-arm64.zip",
      "sha256": "fedcba9876543210",
      "size": 123456789
    }
  ]
}
```

Fields:

- `schemaVersion`: must be `1` for Phase 1.
- `channel`: release channel, initially `stable`.
- `version`: target application version.
- `minimumRuntimeResourceVersion`: minimum runtime resource version the target app expects.
- `minimumAppVersion`: app versions below this floor are treated as required updates.
- `publishedAt`: UTC release timestamp.
- `notes`: short release notes for the UI.
- `packages`: target-specific payloads.

Package fields:

- `target`: `windows-amd64`, `darwin-arm64`, or `darwin-amd64`.
- `payloadType`: Phase 1 accepts only `full`.
- `url`: HTTP(S), `file://`, or local path source.
- `sha256`: expected payload hash.
- `size`: expected payload size in bytes when available.

## Manifest Source Resolution

Application update sources follow the existing runtime-update pattern but use distinct names:

1. `runtimeDir/config/app-update.json`
2. `DESKTOP_APP_UPDATE_MANIFEST_URL`
3. `config.yaml -> release.app_update_manifest_url`
4. no source, which means no app-update check

The source config file shape is:

```json
{
  "manifestUrl": "https://example.com/app-update-stable.json"
}
```

The app-update source must be reported in update state, diagnostics, and user-visible failures.

## Publish Artifacts

Windows packaging should emit:

```text
publish/output/AntBrowser-Setup-<version>.exe
publish/output/AntBrowser-<version>-windows-amd64.zip
publish/output/app-update-stable.json
publish/output/app-update-stable.json.sha256
```

The full zip payload contains the installed application payload required for a user-mode install:

- `ant-chrome.exe`
- `config.yaml` template behavior compatible with current install rules
- `publish/runtime-manifest.json`
- required `publish/bin/...` runtime packages
- `apps/agent/...`
- `runtime/node/...`
- `bin/xray.exe`
- `bin/sing-box.exe`
- `chrome/...` when the release includes a browser core

The release contract verifier must validate:

- manifest schema
- package target coverage for the current release target
- package hash and size
- zip contains required files
- zip does not include mutable user data
- zip can be unpacked into a valid staging directory

## Staging Layout

Shared app-update state lives under:

```text
stateRoot/app-update/
```

Layout:

```text
stateRoot/app-update/
  state.json
  update-plan.json
  downloads/
  staging/<version>/
  backups/<oldVersion>-<timestamp>/
  runner/
  logs/
```

`state.json` records the current update state, versions, timestamps, source, and the last error.

`update-plan.json` records enough information for the apply runner to finish or roll back without the GUI process:

- install root
- state root
- target platform
- old app version
- new app version
- staged payload path
- backup path
- current executable path
- expected payload hash
- manifest source and URL
- process ID to wait for

## State Machine

States:

- `idle`: no active app-update task
- `available`: newer app version detected
- `downloading`: payload download in progress
- `staged`: payload verified and unpacked
- `applying`: runner is backing up and replacing files
- `verifying`: new version has launched and is running post-update checks
- `succeeded`: new version passed checks
- `rolled_back`: update failed and old version was restored
- `failed_manual_repair`: update failed and automatic rollback did not complete

The state machine is persisted before each destructive step.

Startup behavior:

- `succeeded`: clear stale staging and old backups according to retention rules.
- `rolled_back`: show failure report once, then keep logs for diagnostics.
- `failed_manual_repair`: block further automatic update attempts until the user exports diagnostics or clears the failed state.
- `applying` or `verifying`: inspect plan and backend status. If safe, attempt rollback; otherwise move to `failed_manual_repair`.

## Apply And Rollback Flow

Normal flow:

1. Check app-update manifest.
2. Classify update.
3. Download full payload.
4. Verify size and `sha256`.
5. Unpack into staging.
6. Validate staged payload shape.
7. Write `update-plan.json`.
8. Copy current executable into `runner/`.
9. Spawn runner with `--apply-update`.
10. Exit GUI process.
11. Runner waits for GUI process exit.
12. Runner backs up current install payload.
13. Runner replaces install payload with staged payload.
14. Runner launches new app with `--post-update-check <plan>`.
15. New app validates itself and records `succeeded`.

Rollback flow:

1. If replacement fails, restore backup.
2. If post-update check fails, restore backup.
3. Relaunch the old app.
4. Record `rolled_back` with error details.
5. Show “update failed and old version was restored” on next startup.

Manual repair flow:

1. If rollback fails, record `failed_manual_repair`.
2. Preserve backup and logs.
3. Show install root, backup path, log path, and recommended reinstall action.

## Post-Update Check

The new app is launched with:

```text
ant-chrome.exe --post-update-check <absolute-plan-path>
```

The check passes only when:

- running app version equals the target version
- install layout is valid
- `publish/runtime-manifest.json` is readable
- required bundled workspace agent payload exists
- required bundled Node payload exists
- basic environment gate does not fail due to packaging omissions

Network-dependent workspace host reachability should not by itself roll back an app update, because it can fail for reasons unrelated to packaging. Packaging and local payload defects should roll back.

## Frontend UX

The frontend receives an app-update state separate from runtime-update state.

Kinds:

- `none`
- `soft`
- `required`
- `unsupported_install`
- `failed`

Soft update:

- shows local and remote versions
- shows release notes
- shows manifest source and URL
- provides “download and restart update”
- allows “later”

Required update:

- blocks main app entry
- explains that the app will close and restart
- provides “update and restart”
- does not allow “later”

Unsupported install:

- explains that the current install location cannot self-update
- recommends reinstalling or migrating to the user-writable install location
- does not attempt file replacement

Rolled back:

- old version starts
- user sees the failure reason
- diagnostics export includes app-update state and logs

Manual repair:

- user sees install root, backup path, and log path
- app recommends reinstalling from the latest installer

## Backend API Shape

Names can be adjusted to match Wails binding conventions, but the API should be separate from runtime update APIs.

Suggested methods:

- `CheckDesktopAppUpdate() (appupdate.State, error)`
- `DownloadDesktopAppUpdate() (appupdate.State, error)`
- `ApplyDesktopAppUpdate() (appupdate.State, error)`
- `GetDesktopAppUpdateState() (appupdate.State, error)`
- `ClearDesktopAppUpdateFailure() error`

The runtime methods remain focused on environment and runtime resource updates:

- `CheckDesktopReleaseUpdate`
- `ApplyDesktopReleaseUpdate`

Future cleanup can rename the runtime methods for clarity, but that rename is not required for Phase 1.

## Error Codes

User-visible and diagnostic errors:

- `APP-UPDATE-MANIFEST-LOAD-FAILED`
- `APP-UPDATE-TARGET-MISSING`
- `APP-UPDATE-PAYLOAD-TYPE-UNSUPPORTED`
- `APP-UPDATE-INSTALL-UNSUPPORTED`
- `APP-UPDATE-INSTALL-NOT-WRITABLE`
- `APP-UPDATE-DOWNLOAD-FAILED`
- `APP-UPDATE-CHECKSUM-MISMATCH`
- `APP-UPDATE-PAYLOAD-INVALID`
- `APP-UPDATE-SPAWN-RUNNER-FAILED`
- `APP-UPDATE-APPLY-FAILED-ROLLED-BACK`
- `APP-UPDATE-POST-CHECK-FAILED-ROLLED-BACK`
- `APP-UPDATE-ROLLBACK-FAILED-MANUAL-REPAIR`

Errors must include source details where useful:

- manifest source
- manifest URL
- payload URL
- install root
- state root
- plan path
- log path
- backup path

## Diagnostics

Environment diagnostics should include app-update fields:

- app-update state
- local app version
- remote app version
- manifest source
- manifest URL
- payload URL
- update plan path
- last error code
- last error message
- apply log path
- backup path

Logs must avoid tokens, credentials, cookies, and user session data.

## Tests

Unit and integration tests:

- manifest schema parsing
- package target selection
- unsupported `payloadType`
- version comparison
- soft and required classification
- unsupported install classification
- source resolution priority
- `sha256` mismatch
- invalid zip payload
- state transitions
- fake backend success
- fake backend rollback
- fake backend rollback failure

Windows backend tests:

- user-writable install accepted
- `Program Files` install rejected
- non-writable install rejected
- backup and restore
- staged payload replacement
- runner plan parsing
- post-update check success
- post-update check failure triggers rollback

True Windows regression scenarios:

- soft app update success
- required app update success
- manifest load fail
- unsupported `Program Files` install
- checksum mismatch
- invalid payload
- apply failure rollback
- post-check failure rollback
- crash during apply followed by next-start recovery

Publish tooling tests:

- zip generation
- app-update manifest generation
- package contract verification
- runbook scenario coverage

## Runbook Updates

The Windows release runbook should gain a new “application self-update” section separate from runtime update scenarios.

Required scenarios:

- local file manifest smoke test
- HTTP manifest smoke test
- soft app update
- required app update
- manifest load failure
- checksum mismatch
- unsupported install
- rollback after apply failure
- rollback after post-update failure

The runbook must keep runtime update and app update terminology distinct.

## Open Implementation Notes

The implementation should avoid a large all-in-one updater file. Suggested package structure:

- `backend/internal/appupdate/manifest.go`
- `backend/internal/appupdate/manager.go`
- `backend/internal/appupdate/state.go`
- `backend/internal/appupdate/download.go`
- `backend/internal/appupdate/archive.go`
- `backend/internal/appupdate/platform.go`
- `backend/internal/appupdate/windows_backend.go`
- `backend/app_update.go`

The CLI mode should be dispatched before Wails starts. `main.go` can inspect arguments and call backend updater entrypoints before creating the GUI app.

The frontend should reuse the existing runtime gate style, but the store and types should be separate from `runtimeStore` unless implementation review shows a small shared wrapper is cleaner.

## Implementation Acceptance Criteria

The implementation is accepted when:

- Windows user-mode install can update from one app version to another without manual reinstall.
- The update refuses unsupported install locations without destructive actions.
- The update rolls back after replacement or post-check failure.
- Diagnostics explain what happened.
- Runtime update behavior remains unchanged.
- macOS can be added later by implementing a platform backend and publish artifact generation, without rewriting the shared core.
