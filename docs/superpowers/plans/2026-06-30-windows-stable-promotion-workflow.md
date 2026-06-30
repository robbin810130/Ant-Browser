# Windows Stable Promotion Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a manually dispatched GitHub Actions workflow that promotes a verified Maka Browser Windows test release to stable without rebuilding it.

**Architecture:** Extend the existing PowerShell promotion entrypoint with the proven temporary-script SSH transport used by the release uploader. Execute one fail-closed remote Bash transaction that verifies test artifacts, stages the stable directory, verifies the copy, atomically publishes the version directory, and atomically replaces the stable manifest alias. A separate workflow invokes promotion and performs HTTP package and installer verification.

**Tech Stack:** GitHub Actions YAML, Windows PowerShell 5.1, OpenSSH/SCP, Bash, Python 3 contract verification.

---

### Task 1: Add a Failing Promotion Contract Test

**Files:**
- Create: `tools/release/verify-windows-promotion-script.py`

- [ ] **Step 1: Write the contract verifier**

The verifier must require:

```python
PROMOTION_SCRIPT = ROOT / "tools" / "release" / "promote-windows-release.ps1"
PROMOTION_WORKFLOW = ROOT / ".github" / "workflows" / "windows-stable-promotion.yml"
```

It must assert that the workflow is manually dispatched, uses the Windows
self-hosted release runner, passes repository SSH secrets, invokes only the
promotion and URL-check scripts, and never invokes `bat/publish.ps1` or the
Windows release factory. It must also reject direct multiline SSH command
transport and run `bash -n` on the embedded promotion here-string.

- [ ] **Step 2: Run the verifier and confirm RED**

Run:

```bash
rtk python3 tools/release/verify-windows-promotion-script.py
```

Expected: failure because `.github/workflows/windows-stable-promotion.yml` and
the hardened promotion contract do not exist.

- [ ] **Step 3: Commit the failing verifier**

```bash
rtk git add tools/release/verify-windows-promotion-script.py
rtk git commit -m "test: define Windows stable promotion contract"
```

### Task 2: Harden the Promotion Script

**Files:**
- Modify: `tools/release/promote-windows-release.ps1`

- [ ] **Step 1: Add proven SSH transport helpers**

Add `Normalize-PrivateKeyText`, retry-capable native execution, and
`Invoke-RemoteBashScript`. The helper must write LF-only UTF-8 without BOM,
upload the script to `/tmp`, execute it as:

```powershell
"bash '$RemoteScriptPath'"
```

and remove both local and remote temporary scripts in `finally`.

- [ ] **Step 2: Implement fail-closed remote promotion**

The embedded Bash script must:

```bash
set -eu
test_dir='<root>/test/<version>'
stable_root='<root>/stable'
stable_dir="$stable_root/<version>"
staging_dir="$stable_root/.promoting-<version>-<id>"
stable_alias="$stable_root/app-update-stable.json"
```

Then require the manifest, manifest SHA sidecar, ZIP, ZIP SHA sidecar, and
installer; run `sha256sum -c` for both sidecars; reject an existing
`stable/<version>`; copy test to staging; re-run both checks in staging; move
staging to the version directory; copy the manifest to a temporary alias; and
rename the temporary alias over `stable/app-update-stable.json`.

- [ ] **Step 3: Run the verifier**

Run:

```bash
rtk python3 tools/release/verify-windows-promotion-script.py
```

Expected: still fails only because the workflow is absent.

### Task 3: Add the Manual GitHub Actions Gate

**Files:**
- Create: `.github/workflows/windows-stable-promotion.yml`

- [ ] **Step 1: Add workflow dispatch and concurrency**

The workflow must accept required `target_version`, validate it against
`^[0-9]+\.[0-9]+\.[0-9]+$`, use:

```yaml
runs-on: [self-hosted, windows, ant-browser-release]
```

and serialize with the release factory by using:

```yaml
concurrency:
  group: windows-release-factory-${{ github.repository }}
  cancel-in-progress: false
```

- [ ] **Step 2: Add promotion and HTTP verification**

Invoke:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File tools\release\promote-windows-release.ps1 `
  -ReleaseVersion $env:TARGET_VERSION
```

Then verify:

```text
http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json
http://192.168.210.169:18080/releases/windows/stable/MakaBrowser-Setup-<version>.exe
http://192.168.210.169:18080/releases/windows/test/<version>/app-update-stable.json
```

The stable manifest check must download the ZIP and verify its SHA256 through
`check-windows-release-url.ps1`.

- [ ] **Step 3: Run all release contract checks**

```bash
rtk python3 tools/release/verify-windows-promotion-script.py
rtk python3 tools/release/verify-windows-publish-script.py
rtk python3 tools/runtime/verify-publish-contract.py
rtk python3 tools/app-update/verify-windows-e2e-script.py
rtk git diff --check
```

Expected: every command exits zero.

- [ ] **Step 4: Commit the implementation**

```bash
rtk git add tools/release/promote-windows-release.ps1 \
  .github/workflows/windows-stable-promotion.yml
rtk git commit -m "ci: add Windows stable promotion workflow"
```

### Task 4: Publish and Promote 1.1.23

**Files:**
- No application files.

- [ ] **Step 1: Push and open a PR**

Push `codex/windows-stable-promotion-workflow` and open a PR into
`codex/maka-specialist-task-panel`.

- [ ] **Step 2: Merge after checks**

Confirm required checks pass, mark the PR ready, and merge it.

- [ ] **Step 3: Dispatch promotion**

Dispatch `Windows Stable Promotion` from the merged base branch with:

```text
target_version=1.1.23
```

- [ ] **Step 4: Verify production release evidence**

Require a successful workflow conclusion and log evidence that:

- test SHA256 checks passed;
- stable staging SHA256 checks passed;
- `stable/1.1.23` was published;
- the stable manifest package verification passed;
- the stable installer and original test manifest remained reachable.
