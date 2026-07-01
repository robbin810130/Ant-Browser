# Maka Browser Default App Update Source Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every Maka Browser Windows installation discover the stable application update manifest without manual configuration, while preserving explicit configuration precedence.

**Architecture:** Add a final built-in stable manifest fallback to the existing Go source resolver, keep runtime config, environment variables, and `config.yaml` ahead of that fallback, and put the same URL into the Windows release template. Tighten the release and E2E contract checks so CI fails if packaging loses the default URL or recreates the old environment-variable shortcut.

**Tech Stack:** Go, PowerShell, Python 3 contract tests, Wails, NSIS, GitHub Actions.

**Security Boundary:** The stable HTTP URL is for controlled internal-network
use only and must not be exposed publicly. The manifest currently has no
independent signature; HTTPS and Ed25519 verification require a separate
architecture design. This change adds timeout, size, and redirect protections,
but those transport guards do not establish manifest authenticity.

---

### Task 1: Add the Windows-only built-in stable manifest fallback

**Files:**
- Modify: `backend/internal/appupdate/source_test.go`
- Modify: `backend/internal/appupdate/source.go`

- [ ] **Step 1: Write the failing fallback test**

Add this test after `TestResolveManifestSourceUsesConfig`:

```go
func TestResolveManifestSourceUsesDefaultStableManifest(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "")
	t.Setenv("DESKTOP_APP_UPDATE_DISABLED", "")

	resolution := resolveManifestSourceForGOOS(t.TempDir(), &config.Config{}, "windows")

	if resolution.URL != DefaultStableManifestURL {
		t.Fatalf("default stable URL 不正确: got=%q want=%q", resolution.URL, DefaultStableManifestURL)
	}
	if resolution.Source != "default-stable" {
		t.Fatalf("Source 不正确: got=%q", resolution.Source)
	}
	if resolution.ConfigPath != "" {
		t.Fatalf("ConfigPath 应为空: got=%q", resolution.ConfigPath)
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
rtk go test ./backend/internal/appupdate -run TestResolveManifestSourceUsesDefaultStableManifest -count=1
```

Expected: compilation fails because `DefaultStableManifestURL` does not exist, or the assertion fails because the resolver returns an empty URL.

- [ ] **Step 3: Add the minimal fallback implementation**

Add the exported constant beside the existing environment constants:

```go
const DefaultStableManifestURL = "http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json"
```

Have public `ResolveManifestSource` pass `runtime.GOOS` to an internal
`resolveManifestSourceForGOOS` helper. Replace the helper's final empty
resolution only when `goos` is Windows with:

```go
	return ManifestSourceResolution{
		URL:    DefaultStableManifestURL,
		Source: "default-stable",
	}
```

Keep the early `DESKTOP_APP_UPDATE_DISABLED` return unchanged so an explicit disable remains authoritative.
Add a Darwin regression test that calls the helper with `goos="darwin"` and
expects an empty resolution.

- [ ] **Step 4: Run focused and package tests and verify GREEN**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestResolveManifestSource' -count=1
rtk go test ./backend/internal/appupdate/... -count=1
```

Expected: all source precedence tests and all `appupdate` package tests pass.

- [ ] **Step 5: Commit the resolver fix**

```bash
rtk git add backend/internal/appupdate/source.go backend/internal/appupdate/source_test.go
rtk git commit -m "fix: add default stable app update source"
```

### Task 2: Put the stable source into Windows release configuration

**Files:**
- Modify: `tools/release/verify-windows-publish-script.py`
- Modify: `publish/config.init.yaml`

- [ ] **Step 1: Write the failing release contract**

Add this path beside the other constants:

```python
RELEASE_CONFIG = ROOT / "publish" / "config.init.yaml"
```

Add these assertions in `main()` before the success message:

```python
    require_contains(RELEASE_CONFIG, "app_update_manifest_url:")
    require_contains(
        RELEASE_CONFIG,
        "http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json",
    )
```

- [ ] **Step 2: Run the verifier and confirm RED**

Run:

```bash
rtk python3 tools/release/verify-windows-publish-script.py
```

Expected: FAIL because `publish/config.init.yaml` does not contain the stable URL.

- [ ] **Step 3: Add the release configuration value**

Change only the application update field in `publish/config.init.yaml`:

```yaml
release:
  update_manifest_url: ""
  app_update_manifest_url: "http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json"
```

Do not set `update_manifest_url`; that field belongs to the separate runtime payload update mechanism.

- [ ] **Step 4: Run the release contract and verify GREEN**

Run:

```bash
rtk python3 tools/release/verify-windows-publish-script.py
```

Expected: `[OK] windows publish contract verified`.

- [ ] **Step 5: Commit the packaging fix**

```bash
rtk git add publish/config.init.yaml tools/release/verify-windows-publish-script.py
rtk git commit -m "fix: package stable app update source"
```

### Task 3: Remove the E2E environment-variable shortcut

**Files:**
- Modify: `tools/app-update/verify-windows-e2e-script.py`
- Modify: `tools/app-update/windows-app-update-e2e.ps1`

- [ ] **Step 1: Make the E2E contract reject user-level source injection**

Remove `"DESKTOP_APP_UPDATE_MANIFEST_URL"` from `required_fragments`.

After the missing-fragment check, add:

```python
    forbidden_fragments = [
        '[Environment]::SetEnvironmentVariable("DESKTOP_APP_UPDATE_MANIFEST_URL"',
        "$env:DESKTOP_APP_UPDATE_MANIFEST_URL =",
    ]
    present = [fragment for fragment in forbidden_fragments if fragment in text]
    if present:
        fail("script contains forbidden app update source injection: " + ", ".join(present))
```

- [ ] **Step 2: Run the E2E contract and confirm RED**

Run:

```bash
rtk python3 tools/app-update/verify-windows-e2e-script.py
```

Expected: FAIL and list both forbidden source-injection fragments.

- [ ] **Step 3: Remove the misleading source injection**

Delete this block from `tools/app-update/windows-app-update-e2e.ps1`:

```powershell
Write-Step "Configure local manifest"
[Environment]::SetEnvironmentVariable("DESKTOP_APP_UPDATE_MANIFEST_URL", $manifestPath, "User")
$env:DESKTOP_APP_UPDATE_MANIFEST_URL = $manifestPath
```

Keep the harness `ManifestProvider` that explicitly points at the local target manifest. That provider isolates Check/Download/Apply from the live stable channel without changing the installed user's environment.

- [ ] **Step 4: Run the E2E contract and verify GREEN**

Run:

```bash
rtk python3 tools/app-update/verify-windows-e2e-script.py
rtk python3 tools/runtime/verify-publish-contract.py
```

Expected: both scripts exit zero and report their contract verification success messages.

- [ ] **Step 5: Commit the E2E correction**

```bash
rtk git add tools/app-update/verify-windows-e2e-script.py tools/app-update/windows-app-update-e2e.ps1
rtk git commit -m "test: verify default app update discovery"
```

### Task 4: Run the full relevant verification set

**Files:**
- Verify only; no production files should change.

- [ ] **Step 1: Run all backend tests**

```bash
rtk go test ./backend/... -count=1
```

Expected: all Go tests pass.

- [ ] **Step 2: Run all Windows release contracts**

```bash
rtk python3 tools/app-update/verify-windows-e2e-script.py
rtk python3 tools/release/verify-windows-publish-script.py
rtk python3 tools/release/verify-windows-promotion-script.py
rtk python3 tools/runtime/verify-publish-contract.py
```

Expected: all four verifiers exit zero.

- [ ] **Step 3: Build the frontend**

From `frontend/`:

```bash
rtk npm run build
```

Expected: TypeScript and Vite production build pass.

- [ ] **Step 4: Verify repository hygiene**

```bash
rtk git diff --check
rtk git status --short --branch
```

Expected: no whitespace errors; only the intended design, implementation, test,
and final-review hardening commits are ahead of
`origin/codex/maka-specialist-task-panel`; existing untracked
`images/maka-browser-icon-candidates/` and `node_modules/` remain untouched.

### Task 5: Integrate and publish the repair

**Files:**
- GitHub PR and workflow state only.

- [ ] **Step 1: Push the repair branch and create a PR**

```bash
rtk git push -u origin codex/fix-default-app-update-source
rtk proxy gh pr create \
  -R robbin810130/Ant-Browser \
  --base codex/maka-specialist-task-panel \
  --head codex/fix-default-app-update-source \
  --title "fix: restore default Maka Browser app updates" \
  --body $'## Summary\n- add a built-in stable app update manifest fallback\n- package the stable update source in Windows config\n- remove the E2E environment-variable shortcut\n\n## Test Plan\n- go test ./backend/... -count=1\n- python3 tools/app-update/verify-windows-e2e-script.py\n- python3 tools/release/verify-windows-publish-script.py\n- python3 tools/release/verify-windows-promotion-script.py\n- python3 tools/runtime/verify-publish-contract.py\n- npm run build (frontend/)'
```

Expected: PR describes the resolver fallback, release configuration, removed E2E shortcut, and exact verification commands.

- [ ] **Step 2: Merge after the PR is clean**

Use squash merge only after the PR is Ready, mergeable, and has no failing checks.

- [ ] **Step 3: Run Windows Release Factory for `1.1.25`**

Trigger from `codex/maka-specialist-task-panel` with:

```text
target_version=1.1.25
baseline_version=1.1.24
channel=test
run_e2e=true
upload_to_server=true
```

Expected: build, artifact validation, E2E, GitHub artifact upload, test-channel upload, and remote package URL verification all pass.

- [ ] **Step 4: Promote the tested build to stable**

Trigger `Windows Stable Promotion` with:

```text
target_version=1.1.25
```

Expected: stable manifest and package URLs report `1.1.25` and HTTP 200; the original test release remains available.

- [ ] **Step 5: Bootstrap the existing `1.1.23` installation once**

Run on the affected Windows user account:

```powershell
[Environment]::SetEnvironmentVariable(
  "DESKTOP_APP_UPDATE_MANIFEST_URL",
  "http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json",
  "User"
)
```

Fully exit Maka Browser, start it again, and click “检查更新”.

Expected: the client discovers `1.1.25`, downloads it, applies it, and restarts on `1.1.25`. Future releases remain discoverable through both the persisted explicit source and the new built-in fallback.
