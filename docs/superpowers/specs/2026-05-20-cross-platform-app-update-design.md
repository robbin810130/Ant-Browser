# Cross-Platform App Update Design

Date: 2026-05-20
Branch: `codex/windows-phase1-stability`
Status: Written, pending user review and implementation plan

## Context

Windows App Self-Update Phase 1 is complete on branch `codex/windows-phase1-stability`.

The current implementation has a working shared `appupdate` core for manifest loading, target selection, package download, checksum verification, staging, persistent state, apply plans, and frontend API state. The first backend is Windows-only. The next phase connects macOS to that same core so the project does not grow a second updater architecture.

The current code still has these Windows-specific seams:

- `backend/app_update.go` constructs `appupdate.WindowsBackend` directly.
- `backend/app_update_cli.go` runs `WindowsBackend` directly for `--apply-update` and `--post-update-check`.
- `backend/internal/appupdate/archive.go` rejects `darwin-amd64` and `darwin-arm64` staged payloads.
- `tools/app-update/verify-app-update-package.py` validates only the Windows payload shape.

## Goals

1. Add macOS backend support for application self-update through the existing `appupdate` shared core.
2. Keep Windows Phase 1 behavior stable and covered by existing regression tests.
3. Harden the shared app-update payload contract so Windows and macOS packages are validated consistently.
4. Support `darwin-arm64` and `darwin-amd64` targets.
5. Support user-writable macOS installs such as `~/Applications/Ant Browser.app`.
6. Refuse automatic updates for privileged macOS install locations such as `/Applications/Ant Browser.app`.
7. Document macOS app-update packaging and regression scenarios in the release runbook.

## Non-Goals

This phase does not implement delta patching.

This phase does not implement release channel rollout, staged rollout, or gray release controls.

This phase does not introduce a long-running update service or privileged helper.

This phase does not make code signing or notarization a runtime backend hard gate. Signing, notarization, and Gatekeeper checks are release verification and runbook responsibilities in this phase.

This phase does not redesign the frontend app-update API or merge app update with runtime resource update.

This phase does not refactor the already verified Windows apply flow unless a small extraction is required to select the platform backend safely.

## Approved Approach

Use a minimal cross-platform backend integration.

The implementation should add a `DarwinBackend`, add a small platform selection layer, and extend package validation. It should not rewrite the `Manager` state machine or lift the Windows apply flow into a larger template framework.

This keeps the blast radius narrow:

- shared core remains the coordinator
- Windows backend remains the known-good reference
- macOS backend owns only macOS install, bundle, replacement, rollback, and launch details

## Architecture

Application self-update remains split into a shared core and platform backends.

The shared core owns:

- app-update manifest parsing
- manifest source resolution
- target-specific package selection
- version comparison and update classification
- package download
- `sha256` and size verification
- full payload extraction
- staging layout
- persistent state
- apply plan serialization
- user-visible update state

Platform backends own:

- platform target identity
- install mode validation
- staged payload validation where platform details matter
- apply runner preparation
- process exit waiting
- install backup
- install replacement
- rollback
- post-update checks
- relaunch behavior

The existing `PlatformUpdater` interface remains the main boundary:

```go
type PlatformUpdater interface {
	Target() string
	ValidateInstallMode(Layout) error
	PrepareApply(ApplyPlan) error
	SpawnApplyRunner(planPath string) error
	RunApply(planPath string) error
	PostUpdateCheck(planPath string) error
}
```

## Platform Selection

Add a small factory for platform backends. Exact names can follow local style, but the intended shape is:

```go
type PlatformOptions struct {
	CurrentExePath    string
	CurrentAppVersion string
	ProcessID         int
	SuppressRelaunch  bool
}

func NewPlatformBackend(goos, goarch string, opts PlatformOptions) (PlatformUpdater, error)
```

Target mapping:

- `windows/amd64` maps to `WindowsBackend` and target `windows-amd64`.
- `darwin/arm64` maps to `DarwinBackend` and target `darwin-arm64`.
- `darwin/amd64` maps to `DarwinBackend` and target `darwin-amd64`.
- unsupported OS or architecture values return an explicit unsupported-platform error.

Replace direct Windows backend construction in:

- `backend/app_update.go`
- `backend/app_update_cli.go`

Both GUI API calls and CLI updater modes must use the same platform selection path.

## macOS Install Model

The macOS install root is the `.app` bundle root:

```text
~/Applications/Ant Browser.app
```

Automatic app update is supported only when:

- install root ends in `.app`
- install root is an app bundle directory
- install root is not inside `/Applications`
- install root is not inside `/System/Applications`
- install root parent directory is writable by the current user
- state root is outside the app bundle

Automatic app update is refused when:

- install root is `/Applications/Ant Browser.app`
- install root is under `/Applications/`
- install root is under `/System/Applications/`
- install root is not a `.app` bundle root
- install root or its parent cannot be inspected safely
- install root parent is not writable

Refused installs return the existing unsupported-install kind:

```text
unsupported_install
```

The UI should continue to explain that the current install location cannot self-update and that the user should reinstall or move to a user-writable install location.

## macOS State Root

The app-update state root must not live inside the `.app` bundle.

The implementation should reuse the existing detached state behavior already used for macOS bundle installs. The effective app-update layout should place update state under a user-writable application support or config directory, then under:

```text
app-update/
```

The state root must survive replacing `Ant Browser.app`.

## macOS Payload Contract

The app-update manifest target must use Go-style target names:

```text
darwin-arm64
darwin-amd64
```

The macOS full package zip must contain one top-level bundle:

```text
Ant Browser.app/
  Contents/
    Info.plist
    MacOS/
      ant-chrome
      config.yaml
      publish/
        runtime-manifest.json
      bin/
        xray
        sing-box
      chrome/
        ...
```

Required staged payload checks:

- top-level `Ant Browser.app` exists
- `Ant Browser.app/Contents/Info.plist` exists
- `Ant Browser.app/Contents/MacOS/ant-chrome` exists
- `Ant Browser.app/Contents/MacOS/ant-chrome` is executable
- `Ant Browser.app/Contents/MacOS/publish/runtime-manifest.json` exists
- `Ant Browser.app/Contents/MacOS/bin/xray` exists and is executable
- `Ant Browser.app/Contents/MacOS/bin/sing-box` exists and is executable
- mutable user data is absent

Forbidden payload content:

- `data/`
- files ending in `.db`
- files ending in `.sqlite`
- files ending in `.sqlite3`

The backend should reject payloads that fail this contract before any destructive install step.

## macOS Apply Flow

Normal apply flow:

1. GUI process downloads, verifies, extracts, and validates the full macOS payload.
2. GUI process writes `update-plan.json`.
3. GUI process spawns the apply runner with `--apply-update <plan>`.
4. GUI process exits.
5. Runner waits for the GUI process ID from the plan to exit.
6. Runner writes `applying`.
7. Runner backs up the old bundle to the backup path.
8. Runner removes the old install bundle.
9. Runner replaces it with staged `Ant Browser.app`.
10. Runner writes `verifying`.
11. Runner launches the new bundle executable:

```text
<installRoot>/Contents/MacOS/ant-chrome --post-update-check <plan>
```

12. New app validates itself and writes `succeeded`.

The backup path should contain the old app bundle, not only the contents of `Contents/MacOS`.

## macOS Rollback Flow

If replacement fails:

1. Remove any partially copied new bundle when possible.
2. Restore the backup bundle to the install root.
3. Write `rolled_back` with the apply error.
4. Relaunch the old app when possible.

If post-update check fails:

1. Restore the backup bundle to the install root.
2. Write `rolled_back` with the post-check error.
3. Relaunch the old app when possible.

If rollback fails:

1. Write `failed_manual_repair`.
2. Preserve backup path and plan path in state.
3. Include install root, backup path, plan path, and error details for diagnostics.
4. Do not continue automatic update attempts until the failure state is cleared.

## macOS Post-Update Check

Post-update check passes only when:

- running app version equals the target version in the plan
- install root exists and ends in `.app`
- `Contents/Info.plist` exists
- `Contents/MacOS/ant-chrome` exists and is executable
- `Contents/MacOS/publish/runtime-manifest.json` exists
- required local runtime payload exists
- no forbidden mutable user data exists inside the app-update payload paths

Post-update check does not require network reachability.

Post-update check does not require codesign or notarization verification in this phase.

## Runner Strategy

The macOS backend should reuse the current executable as the short-lived apply runner, matching the Windows Phase 1 architecture.

Runner preparation must ensure:

- the runner path is outside the app bundle
- the runner is executable
- the runner survives deletion of the old app bundle
- `--apply-update` can read the plan from the detached state root

The runner must not be placed inside `Ant Browser.app`, because the app bundle is the object being replaced.

## Manifest Contract

The existing app-update manifest schema remains version `1`.

Example package entries:

```json
{
  "packages": [
    {
      "target": "windows-amd64",
      "payloadType": "full",
      "url": "AntBrowser-1.2.0-windows-amd64.zip",
      "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      "size": 123456789
    },
    {
      "target": "darwin-arm64",
      "payloadType": "full",
      "url": "AntBrowser-1.2.0-darwin-arm64.zip",
      "sha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
      "size": 123456789
    }
  ]
}
```

Only `payloadType: "full"` is accepted in this phase.

Relative package URLs continue to resolve relative to the manifest source URL or manifest file location.

## Publish Artifacts

The macOS publish flow currently creates a zip with a `macos-<arch>` artifact name. The app-update contract uses manifest target names and should produce these canonical app-update artifacts:

```text
publish/output/AntBrowser-<version>-darwin-arm64.zip
publish/output/AntBrowser-<version>-darwin-amd64.zip
publish/output/app-update-stable.json
publish/output/app-update-stable.json.sha256
```

If the existing `macos-<arch>` artifact names are retained temporarily, the manifest target must still be `darwin-arm64` or `darwin-amd64`.

## App-Update Verifier

Extend `tools/app-update/verify-app-update-package.py` to be target-aware.

For `windows-amd64`, preserve the current checks:

- manifest schema is `1`
- package target exists
- `payloadType` is `full`
- zip `sha256` matches manifest
- zip size matches manifest when manifest size is non-zero
- Windows required files exist
- mutable user data is absent

For `darwin-arm64` and `darwin-amd64`, add checks:

- manifest schema is `1`
- package target exists
- `payloadType` is `full`
- zip `sha256` matches manifest
- zip size matches manifest when manifest size is non-zero
- zip contains `Ant Browser.app/Contents/Info.plist`
- zip contains `Ant Browser.app/Contents/MacOS/ant-chrome`
- zip contains `Ant Browser.app/Contents/MacOS/publish/runtime-manifest.json`
- zip contains `Ant Browser.app/Contents/MacOS/bin/xray`
- zip contains `Ant Browser.app/Contents/MacOS/bin/sing-box`
- mutable user data is absent

The verifier should fail fast with clear messages and remain usable from the release runbook.

## Release Runbook Updates

Extend `docs/release/windows-packaging-and-update-runbook.md` with a macOS app-update section in this phase. Do not split or rename the runbook as part of this phase unless a later review explicitly approves that documentation change. The runbook must keep runtime update and app update terms distinct.

Add macOS app-update scenarios:

- local file manifest smoke test
- HTTP manifest smoke test
- soft update from `~/Applications/Ant Browser.app`
- required update from `~/Applications/Ant Browser.app`
- unsupported install at `/Applications/Ant Browser.app`
- checksum mismatch
- invalid `.app` payload
- replace failure rollback
- post-check version mismatch rollback
- manual repair state after rollback failure

Add release verification steps:

- run app-update verifier for `darwin-arm64` or `darwin-amd64`
- confirm manifest target matches payload target
- confirm app bundle launches before packaging
- confirm signing status for release candidates
- confirm notarization status for release candidates
- confirm Gatekeeper and quarantine behavior for distributed artifacts

Signing, notarization, and Gatekeeper checks are required runbook checks for release readiness, not backend runtime gates.

## Tests

Required Go tests:

- platform factory maps `windows/amd64` to `windows-amd64`
- platform factory maps `darwin/arm64` to `darwin-arm64`
- platform factory maps `darwin/amd64` to `darwin-amd64`
- platform factory rejects unsupported OS and architecture pairs
- `backend/app_update.go` uses the selected backend
- `backend/app_update_cli.go` uses the selected backend
- `ValidateStagedPayload` accepts a valid macOS app bundle
- `ValidateStagedPayload` rejects missing `Info.plist`
- `ValidateStagedPayload` rejects missing app executable
- `ValidateStagedPayload` rejects non-executable app executable
- `ValidateStagedPayload` rejects missing runtime manifest
- `ValidateStagedPayload` rejects forbidden mutable user data
- `DarwinBackend.ValidateInstallMode` accepts a fake user-writable app under `~/Applications`
- `DarwinBackend.ValidateInstallMode` rejects `/Applications`
- `DarwinBackend.ValidateInstallMode` rejects non-`.app` roots
- `DarwinBackend` backs up and replaces a fake app bundle
- `DarwinBackend` restores backup on replacement failure
- `DarwinBackend.PostUpdateCheck` writes `succeeded` for matching version and valid bundle
- `DarwinBackend.PostUpdateCheck` rejects version mismatch

Required verifier tests:

- Windows fixture still passes
- Windows fixture still rejects missing required files
- Darwin fixture passes
- Darwin fixture rejects missing `Info.plist`
- Darwin fixture rejects missing executable
- Darwin fixture rejects missing runtime manifest
- Darwin fixture rejects forbidden user data
- Darwin fixture rejects checksum mismatch

Required manual regression checks on macOS:

- soft update pass
- required update pass
- checksum mismatch pass
- invalid payload pass
- unsupported `/Applications` install pass
- user data preserved after update
- runner replacement and restart pass
- post-check pass

## Acceptance Criteria

This phase is complete when:

1. Windows app-update tests still pass.
2. The app-update manager no longer hardcodes `WindowsBackend`.
3. CLI updater modes no longer hardcode `WindowsBackend`.
4. `darwin-arm64` and `darwin-amd64` targets are accepted by the shared payload validator.
5. macOS user-writable app bundle installs can stage and apply a full app-update payload.
6. macOS privileged app bundle installs are reported as unsupported without destructive actions.
7. macOS invalid payloads fail before install replacement.
8. macOS replacement failures roll back when backup restore is possible.
9. macOS post-check failures roll back when backup restore is possible.
10. App-update verifier validates both Windows and macOS payload contracts.
11. Release runbook documents macOS app-update packaging and regression flow.

## Implementation Notes

Likely files:

- `backend/app_update.go`
- `backend/app_update_cli.go`
- `backend/app_update_test.go`
- `backend/internal/appupdate/platform.go`
- `backend/internal/appupdate/archive.go`
- `backend/internal/appupdate/darwin_backend.go`
- `backend/internal/appupdate/darwin_backend_test.go`
- `backend/internal/appupdate/windows_backend.go`
- `backend/internal/appupdate/windows_backend_test.go`
- `tools/app-update/verify-app-update-package.py`
- `docs/release/windows-packaging-and-update-runbook.md`

Keep the Windows backend behavior stable. Any shared helper extraction should be small, covered by tests, and justified by actual duplication between Windows and Darwin backends.

Prefer fake app bundles in Go tests instead of relying on local signing, notarization, or a real Wails macOS build.
