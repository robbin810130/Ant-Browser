# Cross-Platform App Update Phase Closeout

Date: 2026-05-22
Branch: `codex/windows-phase1-stability`

## Status

This phase is complete for the current scope.

The completed scope is:

- macOS backend integration with the existing shared `appupdate` core
- macOS full-package application self-update
- shared app-update package contract hardening
- macOS publish artifact generation
- real macOS manual regression for local file and HTTP manifest flows
- non-distribution failure scenario coverage

This phase does not claim formal macOS distribution readiness. Signing, notarization, Gatekeeper quarantine, and public hosting are the next phase.

## Completed Work

### Shared app-update core

The existing app-update core now supports Windows and macOS through platform backend selection.

Covered behavior:

- manifest source resolution
- target package selection
- relative package URL resolution from manifest URL
- sha256 and size verification
- full package extraction
- staged payload validation
- persistent state and apply plans
- frontend app-update prompt flow

### macOS backend

Implemented Darwin app bundle update behavior:

- target selection for `darwin-arm64` and `darwin-amd64`
- user-writable app bundle install support
- protected install rejection for `/Applications` and `/System/Applications`
- staged `.app` bundle validation
- backup old `.app`
- replace with new `.app`
- rollback on replacement failure
- post-update version and bundle validation
- detached apply runner and post-check launch

### Package and filesystem hardening

Real macOS regression exposed Framework symlink issues. The phase fixed and covered them:

- zip extraction accepts safe relative symlinks
- zip extraction rejects absolute or destination-escaping symlinks
- app bundle backup/replace copies safe relative symlinks
- app bundle copy rejects source-root escaping symlinks
- detached state seeding for bundled `chrome/` preserves safe Framework symlinks

### macOS publish artifacts

The macOS publish path now produces app-update artifacts:

- `publish/output/AntBrowser-<version>-macos-<arch>.app`
- `publish/output/AntBrowser-<version>-macos-<arch>.zip`
- `publish/output/AntBrowser-<version>-darwin-<arch>.zip`
- `publish/output/app-update-stable.json`

The publish contract also checks that `publish/config.init.mac.yaml` includes `release.app_update_manifest_url`.

## Manual Regression Evidence

Detailed report:

```text
docs/reports/2026-05-22-macos-app-update-manual-regression.md
```

### Local file manifest

Verified:

- baseline `1.0.0`
- target `1.1.0`
- user-writable `~/Applications/Ant Browser.app` style sandbox
- manifest source `runtime-config`
- UI click `更新并重启`
- state progression `verifying -> succeeded -> idle`
- relaunched UI version `1.1.0`
- installed binary hash matched the `1.1.0` artifact
- Chromium Framework symlink preserved in user state

### HTTP manifest

Verified:

- manifest served over `http://127.0.0.1:18081`
- package URL intentionally relative in manifest
- payload URL resolved to the correct HTTP URL
- HTTP server returned `200` for manifest and payload
- state progression `applying -> verifying -> succeeded -> idle`
- relaunched UI version `1.1.0`
- installed binary hash matched the `1.1.0` artifact

## Automated Verification

Fresh commands run during this phase:

```bash
rtk go test . ./backend ./backend/internal/appupdate ./backend/internal/config ./backend/internal/apppath -count=1
rtk npm --prefix frontend run build
rtk python3 tools/runtime/verify-publish-contract.py
rtk python3 -m py_compile tools/runtime/verify-publish-contract.py
rtk python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json publish/output/AntBrowser-1.1.0-darwin-arm64.zip darwin-arm64
rtk codesign --verify --deep --strict --verbose=2 publish/output/AntBrowser-1.1.0-macos-arm64.app
rtk go test ./backend/internal/appupdate -count=1
rtk git diff --check
```

Key observed results:

- Go scoped suite: `276 passed in 5 packages`
- appupdate suite: `141 passed in 1 packages`
- frontend build passed
- publish contract passed
- app-update package verifier passed
- ad-hoc codesign verification passed
- diff check passed

## Failure Scenario Coverage

Covered:

- protected `/Applications` install rejection
- checksum mismatch
- invalid `.app` payload
- tampered staged payload before replacement
- post-check version mismatch
- app bundle replace rollback
- running process timeout before replacement
- symlink escape rejection

These scenarios are covered by local tests and package-contract checks. They are intentionally not notarized distribution checks.

## Important Commits

- `1a9bc42 Implement darwin app bundle update backend`
- `991baa3 Select app update backend by platform`
- `7d1c45a Verify darwin app update packages`
- `39f60be Document macOS app update regression flow`
- `b620d91 Harden darwin app update apply paths`
- `49177f5 Generate macOS app update publish artifacts`
- `6a59df6 Fix macOS publish bundle signing`
- `21522d3 Harden macOS app update regression path`
- `a2756e7 Document macOS app update manual regression`
- `a1eb8e6 Cover macOS app update failure regressions`
- `5100458 Record macOS HTTP app update smoke`

## Cleanup Performed

Temporary regression sandboxes were removed:

```text
/private/tmp/ant-browser-mac-regression-*
/private/tmp/ant-browser-mac-http-smoke-*
```

The `.app` bundles still visible through Spotlight/Launchpad are repository build or publish artifacts, not `/Applications` installs.

## Out Of Scope

Not done in this phase:

- delta patching
- release channel rollout or gray release
- Developer ID signing
- notarization
- Gatekeeper quarantine validation
- public release hosting
- downloaded-from-public-channel launch validation

## Next Phase

Recommended next phase:

```text
macOS Internal Deployment Readiness
```

Initial checklist:

1. Pick the internal install convention, preferably `~/Applications/Ant Browser.app`.
2. Write a short operator runbook for installing the current internal build on a Mac.
3. Publish an internal-only manifest and payload location reachable by the target Macs.
4. Run one full update from the internal hosted manifest and payload.
5. Verify rollback/manual-repair behavior on an internal sandbox app.
6. Verify the installed app version from UI, `Info.plist`, and binary hash.
7. Document cleanup commands for old build artifacts and test sandboxes.

Formal distribution readiness remains a later phase. Developer ID signing, notarization, Gatekeeper quarantine validation, and public-channel download validation are not required for this internal-only deployment phase.
