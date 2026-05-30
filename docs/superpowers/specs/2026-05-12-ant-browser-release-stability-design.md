# Ant-Browser Release Stability Design

Date: 2026-05-12
Status: Proposed
Scope: Ant-Browser desktop release, install, environment consistency, update safety, and supportability across Windows and macOS

## Background

The Ant-Browser desktop client has already gained a real auth shell, desktop-owned session lifecycle, destructive cleanup flows, and managed shop runtime integration. That closes a major product gap, but it does not yet make the desktop client dependable across different machines and environments.

The current rollout risk is no longer "can users log in?" but "can users install, bootstrap, recover, update, and diagnose the desktop environment reliably on Windows and macOS?"

This is especially important because:

- Windows is the primary user base
- the product must support both public internet and proxy-heavy enterprise environments
- the product must support both self-install and IT-assisted deployment
- users should not be required to manually search for external dependency packages

## Goals

Build a release and runtime model that makes Ant-Browser stable enough for real user rollout:

- Windows users can self-install and reach a usable environment with a bundled package
- macOS users can self-install and let the app auto-fetch large missing runtime resources on first launch
- app startup separates auth readiness from environment readiness
- first launch can self-check, auto-repair deterministic issues, and clearly report non-repairable issues
- startup automatically checks for updates but requires explicit user confirmation before upgrading
- runtime resource upgrades are versioned, validated, and rollback-safe
- failures are diagnosable without asking users to manually inspect filesystem state

## Non-Goals

This design does not include:

- a full enterprise fleet management console
- silent auto-upgrades
- complete offline install support for every platform and environment permutation
- full app-binary rollback guarantees on every platform
- deep domain-policy or MDM integrations in the first release stage

## Confirmed Product Decisions

Confirmed decisions for this design:

- support both Windows and macOS
- prioritize Windows stability because most users are on Windows
- support both self-install and centralized deployment, but optimize first for user self-install
- use a mixed dependency strategy
- Windows bundles the fingerprint browser core inside the installer
- macOS uses a lighter app package and auto-fetches large runtime resources on first launch
- startup checks for updates automatically
- users manually confirm upgrades
- the system must work in both direct internet and proxy-heavy environments

## Design Overview

The release stability system is split into five layers:

1. platform-specific release packaging
2. startup bootstrap gate and environment self-check
3. manifest-driven resource and update model
4. auto-repair and rollback behavior
5. diagnostics and supportability

The key principle is that "install success" is not treated as "environment ready." The desktop client becomes usable only when both auth and environment requirements are satisfied.

## Release Architecture

### Windows Release Shape

Windows is the heavy-bundle platform.

The primary Windows installer should include:

- Ant-Browser app binary
- default config
- runtime self-check module
- fingerprint browser core
- critical runtime dependencies
- release manifest metadata
- resource verification metadata

The expected user experience is:

- download one installer
- install once
- pass self-check
- enter login or protected app shell

Windows should minimize first-launch network dependency because this is the main user platform and the primary rollout battleground.

### macOS Release Shape

macOS is the light-package platform.

The primary macOS package should include:

- Ant-Browser app binary
- runtime self-check module
- resource downloader
- resource verifier
- minimum manifest metadata needed for first-launch bootstrap

The expected user experience is:

- install the app package
- launch the app
- let the app fetch and validate large missing runtime resources
- pass self-check
- continue to login or protected app shell

Users should never need to manually browse the internet to find missing runtime packages.

### Release Source Separation

Release infrastructure should separate:

- app release source
- runtime resource source

The app release source publishes installer/app packages, app version metadata, and release notes.

The runtime resource source publishes platform-specific runtime packages, hashes, sizes, compatibility metadata, and manifest files.

This separation allows:

- Windows to prefer full package distribution
- macOS to prefer lightweight package plus runtime fetch
- future mirror sources and enterprise caching without redesigning the whole protocol

## Bootstrap Gate and Environment Readiness

### State Separation

Desktop startup must evaluate two separate readiness states:

- `auth ready`
- `environment ready`

The app may enter the normal product shell only when both are satisfied.

Examples:

- environment ready + auth not ready -> show login
- auth ready + environment not ready -> show environment repair gate
- both not ready -> repair gate first, then login

This prevents the user from entering a broken shell where login succeeded but the desktop runtime cannot actually open or manage browser instances.

### Bootstrap Gate

Startup should enter a dedicated bootstrap gate before routing into the application shell.

The gate is responsible for:

1. loading local app and manifest metadata
2. running platform-specific environment self-checks
3. deciding whether the environment state is:
   - `pass`
   - `repairable`
   - `blocked`

The gate must stay intentionally small. It is a launch-state controller, not a general business screen.

### Environment Self-Checks

Shared checks:

- resource manifest exists and can be read
- local runtime/resource versions satisfy the current app's compatibility floor
- required directories exist and are writable
- required executables exist and are executable
- critical config is present
- stale temporary files, half-installed packages, and lock leftovers are detected

Windows-focused checks:

- bundled runtime resources are present
- key executables have not been quarantined, moved, or locked unexpectedly
- install dir, runtime dir, and user data dir are usable
- old runtime leftovers do not conflict with the current release

macOS-focused checks:

- required large runtime resources have been downloaded
- download origin is reachable
- destination path is writable
- post-download extraction and validation succeeded
- installed runtime binaries pass executable probe checks

### Bootstrap UI States

The user-facing bootstrap flow should have only three main states:

- checking environment
- repairing environment
- unable to repair automatically

The failure state must use human-readable categories, not raw stack traces.

Minimum failure categories:

- network unavailable
- proxy or proxy authentication failed
- checksum or validation failed
- directory not writable
- file locked or in use
- runtime version incompatible
- unknown error

Available user actions:

- retry
- view details
- export diagnostics

## Auto-Repair Model

Auto-repair should only handle deterministic, bounded problems.

Repairable cases:

- missing runtime files
- mismatched runtime version
- corrupted cached packages
- stale temporary leftovers
- permission prompts that can be retried after user authorization
- conflicting old runtime directories that can be safely replaced or isolated

Not automatically repairable by default:

- enterprise proxy authentication policy problems
- persistent security software quarantine or EDR interference
- required update blocked by network policy
- ambiguous unknown failure states

Repair flow:

1. detect failing check items
2. classify each item as auto-repairable or blocked
3. execute the required repair action
4. validate the repaired result
5. rerun the full environment self-check

The system must always perform a full re-check after repair instead of assuming that the one repaired item was the only problem.

## Versioning and Update Model

### Version Layers

The release model should separate:

- `App Version`
- `Resource Manifest Version`
- `Runtime Resource Packages`

Compatibility rules:

- the app declares the minimum acceptable manifest version
- the manifest declares which runtime packages are required for each platform
- runtime packages declare their own platform-specific version, size, checksum, and compatibility metadata

This avoids hard-coding runtime compatibility into the frontend or backend logic and makes release evolution explicit.

### Update Timing

On app startup, the client performs a lightweight update check automatically.

The check flow:

1. run fast local startup/bootstrap checks
2. request remote release metadata
3. compare local app and resource versions with compatibility requirements
4. classify result as:
   - no update
   - soft update available
   - required update

Behavior:

- no update -> continue normally
- soft update available -> prompt user to confirm update
- required update -> block normal entry until the update is completed or the user reaches a controlled failure state

The client must not silently upgrade without explicit user confirmation.

### Update Units

The system should support three update types:

- app-only update
- resource-only update
- combined update

This allows the product to update large runtime assets independently from the app shell when appropriate, especially for macOS and future resource delivery improvements.

### Staged Install and Pointer Switching

Updates must not overwrite the currently active runtime in place.

Required update sequence:

1. download into a staging location
2. validate hash, metadata, and compatibility
3. extract or install into a versioned runtime directory
4. probe the new runtime before activation
5. switch the active runtime pointer only after validation passes
6. keep or prune the previous validated runtime according to retention policy

Recommended runtime layout:

- `runtime/<resource-version>/...`
- `runtime/current -> validated active version`

This makes resource rollback cheap and reliable.

## Rollback Model

The first release stage should guarantee resource-level rollback.

Rules:

- resource-only update failure -> revert to the last validated runtime resource version
- combined update failure during resource stage -> keep old app + old runtime active
- app update success but startup probe failure -> attempt app rollback when platform support is available; otherwise preserve old runtime and show a controlled failure state

The first stage should prioritize robust runtime rollback over full cross-platform app-binary rollback. Runtime rollback is the highest-leverage protection against unusable half-upgraded states.

## Network and Environment Compatibility

The release/runtime model must support three acquisition modes:

- direct internet access
- system proxy / authenticated proxy environments
- seeded local cache or preloaded runtime resources

Rules:

- update-check failure does not automatically mean the app is unusable
- if the local version is still compatible, allow continued use and show a weak warning
- if the current version is marked as required-update-only and update verification cannot complete, block entry with a clear explanation that the issue is network or policy related

This allows the client to behave correctly in both consumer and enterprise environments.

## Filesystem Layout and Reinstall Strategy

The runtime must not treat all on-disk data as one undifferentiated install directory.

Recommended layout layers:

- install dir
  - app binaries
- runtime dir
  - fingerprint core and large runtime resources
- user data dir
  - config, logs, diagnostics, auth-local state, caches

Reinstall rules:

- app uninstall should not blindly delete user data by default
- reinstall should compare manifest and runtime state before reusing old assets
- incompatible old runtime state should be isolated or replaced in a clean versioned directory, not overwritten in-place blindly

This significantly reduces reinstall instability and old-version contamination.

## Diagnostics and Supportability

### Goals

The diagnostic system must answer:

- why the current machine cannot use the app
- whether the user can fix the issue alone
- what support or engineering needs to see to resolve it quickly

### Structured Event Model

Install, bootstrap, self-check, repair, update, rollback, and runtime launch should all write into one structured event stream.

Each event should include at least:

- `event_time`
- `stage`
- `result`
- `error_code`
- `platform`
- `app_version`
- `manifest_version`
- `resource_version`
- `machine_scope`
- `summary`

Suggested stage values:

- `install`
- `bootstrap`
- `selfcheck`
- `repair`
- `update`
- `runtime_launch`

Suggested result values:

- `success`
- `warning`
- `failure`

### Error Code Families

The first version must define stable error code families:

- `ENV-*` for local environment, path, and permission failures
- `NET-*` for network, DNS, proxy, and download failures
- `PKG-*` for package integrity, signature, checksum, and compatibility failures
- `UPD-*` for update, activation switch, and rollback failures
- `RUN-*` for runtime launch and process-state failures

The aim is not hyper-granularity. The aim is fast root-cause separation.

### User-Facing Diagnostics Entry Points

Diagnostics should be available in two places:

- the bootstrap failure screen
- a persistent "environment check and repair" entry inside app settings

This allows users to re-run checks even after the app has entered the normal shell.

### Diagnostic Bundle

The app should support exporting a diagnostic bundle that includes:

- OS, architecture, app version, manifest version, resource version
- latest self-check result
- latest repair result
- latest update and rollback result
- key path state
- download, validation, and extraction logs
- runtime probe results
- proxy/network probe summary
- encountered error codes

The bundle must exclude:

- access tokens
- usernames and passwords beyond what is strictly required for identification
- proxy passwords
- cookies and sessions
- shop business payload contents

### Network Diagnostics

Network diagnosis must distinguish at least:

- offline/no network
- DNS or host unreachable
- proxy configured but authentication failed
- proxy blocks release/resource source
- TLS or certificate failure
- release metadata reachable but resource package source unreachable

These cases must not be collapsed into a single "download failed" bucket.

## Release Pipeline Boundaries

### Stage 1 Delivery Priority

First-stage release priority:

- strong self-install path
- startup self-check and bounded auto-repair
- safe update check and user-confirmed update flow
- stable runtime resource versioning and rollback
- exportable diagnostics for support

### Stage 1 Deferred Work

The first stage explicitly does not promise:

- full enterprise device fleet management
- silent zero-touch upgrades
- complete offline matrix support
- automatic negotiation with every complex enterprise proxy policy
- guaranteed app-binary rollback on every platform

These may come later, but they are not required to call the first release stage viable.

## Success Criteria

The first public rollout should not be considered ready until all of the following are true:

- Windows self-install succeeds reliably
- macOS first-launch resource acquisition succeeds reliably
- environment failures classify clearly and export diagnostics
- update failures do not destroy a previously working runtime
- corrupted or partial runtime resources are detected by self-check
- public internet and common proxy environments produce correct behavior
- normal users do not need to manually search for external dependency packages
- support can triage most install/runtime failures from diagnostic bundles and error codes

## Implementation Notes

This design intentionally separates:

- release packaging from business features
- environment readiness from auth readiness
- app version from runtime resource version
- auto-repairable failures from blocked failures
- user-facing error messaging from engineering diagnostics

Those boundaries should remain explicit in the implementation plan.
