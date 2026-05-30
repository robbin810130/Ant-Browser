# Windows Dev Launcher Follow-ups

Date: 2026-05-15
Owner: desktop runtime / Windows tooling
Status: Open

## Scope

These items are deliberately tracked outside the Windows install-first-run gate work so the release-stability branch can merge on runtime-gate quality, not on dev-only launcher noise.

## Backlog Items

### 1. `cleanup_frontend_dev_processes` residual on native Windows

- Symptom:
  - `bat/dev.bat` previously emitted `The system cannot find the batch label specified - cleanup_frontend_dev_processes`
  - the failure happened in Windows dev-only startup/cleanup flow, not in install-first-run runtime gate
- Current branch mitigation:
  - the cleanup logic has been extracted into `bat/cleanup-frontend-dev-processes.cmd`
  - `bat/dev.bat` now delegates to that helper instead of jumping back into an inline label
- Remaining follow-up:
  - re-run native Windows dev startup against this helperized path
  - if the label error is gone consistently, close this backlog item
  - if any residual retry/noise remains, isolate it as a separate Wails dev-tooling issue instead of reopening runtime-gate work

### 2. Wails dev startup retry noise

- Symptom:
  - on some Windows dev sessions, `wails dev` logs one failed startup attempt before a second successful launch
- Why it is separate:
  - GUI still comes up
  - runtime gate and workspace host readiness are already correct
  - this is a dev-loop ergonomics issue, not an install-stability blocker
- Follow-up:
  - capture one clean repro with full stdout/stderr
  - confirm whether the retry is Wails-internal or launcher-induced
