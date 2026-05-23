# Client Stability Phase Closeout

Date: 2026-05-23
Branch: `codex/windows-phase1-stability`

## Status

The Windows and macOS client stability work can be closed for the current product scope.

The closed scope is:

- Windows formal package, installation, startup, login, runtime update, and app self-update stability.
- macOS internal deployment installation and app self-update stability.
- Shared app-update contract for Windows and macOS full-package updates.
- Regression evidence for install, update, rollback, failure handling, and version verification.

This closeout does not claim public macOS distribution readiness. Developer ID signing, notarization, Gatekeeper quarantine validation, and public download-channel validation remain outside the current scope.

## Product Decision

The current client platform work satisfies the immediate need:

- Windows users can use the formal Windows installer and update chain.
- Internal macOS users can install to a user-writable app location and receive online app updates from an internal manifest and payload host.
- Both platforms use the shared app-update contract instead of separate future-only update systems.

Given that, the next major engineering focus should return to business feature development unless a new distribution requirement is introduced.

## Windows Closeout

Windows is complete for the release-stability scope.

Completed and verified areas:

- startup environment gate
- runtime/resource update
- formal installer generation
- installer install and first launch regression
- login regression
- application self-update full-package flow
- soft update
- required update
- checksum mismatch handling
- unsupported/protected install behavior
- user data preservation
- runner replacement
- restart and post-check
- current client version display on the gate page

Key report:

```text
docs/reports/2026-05-16-windows-release-stability-phase-report.md
```

Primary runbook:

```text
docs/release/windows-packaging-and-update-runbook.md
```

## macOS Internal Closeout

macOS is complete for internal deployment readiness.

Supported internal install shape:

```text
~/Applications/Ant Browser.app
```

Automatic app update intentionally does not support:

```text
/Applications/Ant Browser.app
/System/Applications/Ant Browser.app
```

Completed and verified areas:

- Darwin backend integration with shared `appupdate` core
- `darwin-arm64` and `darwin-amd64` target contract
- user-writable `.app` bundle install validation
- protected install rejection
- `.app` staged payload validation
- backup, replace, rollback, and post-check
- safe relative symlink extraction and bundle copy
- macOS publish artifacts and manifest generation
- online HTTP manifest and relative payload URL resolution
- real UI update from `1.0.0` to `1.1.0`
- final UI, `Info.plist`, binary hash, and `state.json` verification

Key reports:

```text
docs/reports/2026-05-22-cross-platform-app-update-phase-closeout.md
docs/reports/2026-05-22-macos-app-update-manual-regression.md
```

Primary runbook:

```text
docs/release/macos-internal-deployment-runbook.md
```

## Online Update Readiness

The current implementation supports online automatic app updates for the intended internal deployment model.

Required operational shape:

```text
https://<internal-update-host>/ant-browser/macos/stable/app-update-stable.json
https://<internal-update-host>/ant-browser/macos/stable/AntBrowser-<version>-darwin-arm64.zip
```

The manifest package URL may be relative. The client resolves it from the manifest directory and verifies package size and sha256 before staging.

The verified macOS UI flow observed:

```text
1.0.0 available from runtime-config
downloading
applying
verifying
succeeded
idle after relaunch
1.1.0 verified by UI, Info.plist, binary hash, and state.json
```

The verified Windows flow covers the same app-update contract with Windows-specific installer and runner behavior.

## Remaining Technical Scope

These items are intentionally not part of this closeout:

- macOS Developer ID signing
- macOS notarization
- Gatekeeper quarantine validation after internet download
- public release download channel validation
- release channel or gray rollout strategy
- delta patch updates

They should only be reopened when the product needs broader public macOS distribution or lower-bandwidth update economics.

## Recommended Next Focus

Return the main engineering lane to business feature development.

Suggested guardrails:

1. Keep client release work in maintenance mode.
2. Use the current Windows and macOS runbooks for internal or release validation.
3. Reopen distribution work only when there is a concrete public macOS distribution requirement.
4. Before major business releases, run the existing package/update smoke suite instead of redesigning the update system.

## Final Closeout Checklist

- Windows release stability scope: closed.
- Windows app self-update scope: closed.
- Cross-platform shared app-update scope: closed.
- macOS internal deployment scope: closed.
- Formal public macOS distribution scope: deferred.
- Next product focus: business functionality.
