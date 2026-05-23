# ASM Shop Operations Closure Design

Date: 2026-05-23
Branch: `codex/windows-phase1-stability`
Status: Written, pending user review and implementation plan

## Context

The Windows and macOS client stability phase is closed for the current product scope. The desktop release, install, runtime update, and application self-update chain now sits in maintenance mode.

The next product gap is business functionality.

The current Ant Browser desktop client already has the first layer of 1688 Workspace integration:

- desktop login and session recovery
- local Workspace agent bootstrap
- authorized shop synchronization
- managed profile reconciliation
- one-click shop backend open
- shared credential update
- local session validation
- managed instance launch and open-result reporting

However, the current business surface is still incomplete:

- the `实例列表` page is already showing authorized shops, but the information architecture still reads like browser instance management
- recent validation and recent open fields are currently hard-coded empty values
- batch open currently only shows a warning toast instead of performing real work
- local agent `runs/events` exist but are not surfaced as first-class evidence in the client
- there is no independent ASM shop profile center after ASM shop onboarding
- shop-level operation tasks do not yet have a clean place in the desktop client

This phase turns the desktop client into a real ASM shop operations console.

## Product Goal

Build a closed loop for ASM shop operations:

1. The user can see ASM shop profiles as business master data.
2. The user can understand whether each shop is executable on this desktop device.
3. The user can open, validate, repair, or retry shop execution from a focused workbench.
4. The user can see operation tasks per shop and across shops without mixing them into execution-state tables.
5. Every user action has traceable run evidence: status, events, failure reason, and next recommended action.

The guiding question for every shop is:

```text
Can this shop be operated now?
If not, where is it blocked?
What should the user do next?
Is there run evidence for the result?
```

## Confirmed Product Direction

Use the shop workbench as the primary execution surface, backed by local agent run evidence.

Confirmed decisions:

- prioritize `A: 店铺工作台优先`
- absorb `B: 任务中心` as the evidence and cross-shop task layer
- avoid the lightweight patch-only path
- add ASM shop profile list and detail pages as must-have scope
- keep shop profile, execution readiness, operation tasks, and run evidence as separate concepts

## Core Information Architecture

The next business surface has three first-class modules and one shared evidence layer.

### 1. Shop Profile Center

The Shop Profile Center owns ASM shop business master data.

It answers:

- What shop is this?
- Which platform and shop identifier does it belong to?
- Is ASM connected?
- What is the shop's business ownership and operating context?
- What is the summarized execution and task status?

Primary routes can be named during implementation, but the intended product shape is:

```text
/shops
/shops/:shopId
```

The list page should show:

- shop name
- platform
- shop id
- ASM connection status
- authorization status
- owner or operator
- tags
- main category or operating category
- data completeness
- execution status summary
- unfinished operation task count
- last sync time
- recommended next action

The detail page should separate tabs or sections:

- `基础资料`
- `ASM 接入`
- `执行状态`
- `运营任务`
- `运行记录`
- `诊断`

The profile detail page can link to the workbench for execution actions, but it should not become the primary place for bulk execution.

### 2. Shop Workbench

The Shop Workbench owns execution readiness and immediate repair actions.

It answers:

- Can this shop be opened from this desktop client?
- Is the local profile mapped?
- Is the fingerprint core ready?
- Is the shared session ready?
- Is the shop currently running?
- Did the last open or validation fail?
- What is the next repair action?

The current `BrowserListPage` should evolve from an instance list into an authorized shop workbench.

Recommended layout:

- left queue column
- center shop action table
- right detail and repair drawer

Left queues:

- ready to open
- awaiting manual verification
- credential stale or missing
- open failed
- currently running
- authorization revoked or pending reclaim

Center table:

- shop name
- platform
- ASM status summary
- execution status
- profile/core/session readiness
- recent open
- recent validation
- recent failure
- unfinished task count
- primary recommended action
- secondary actions

Right drawer:

- health summary
- shop profile summary
- local profile mapping
- shared login status
- recent open run
- recent credential run
- run event timeline
- failure diagnosis
- next recommended action
- operation task summary

The workbench should expose execution actions:

- open shop backend
- update shared credentials
- local validation
- retry last failed action
- view run evidence
- open profile detail
- create shop operation task

### 3. Operation Task Center

The Operation Task Center owns cross-shop operation tasks.

It answers:

- What needs to be done today?
- Which shop tasks are waiting, running, blocked, failed, or completed?
- Which tasks are blocked by credentials or local execution readiness?
- Which tasks need manual intervention?
- Which tasks can be retried in bulk?

This phase should introduce the task center as a clean product skeleton, not as a full automation platform.

Task center scope for this phase:

- list operation tasks
- filter by shop, task type, status, failure reason, and blocked reason
- show task summary
- open a task detail
- link to the related shop profile
- link to the related workbench action
- retry failed execution-precondition tasks when safe

The task center should not implement full product sourcing, listing, or report-generation workflows in this phase.

### 4. Run Evidence Layer

The Run Evidence Layer is shared by the profile center, workbench, and task center.

It consumes local agent run data, including:

- `/local/runs`
- `/local/runs/:runId`
- `/local/runs/:runId/events`

It should normalize run evidence into frontend models that can answer:

- latest open run per shop
- latest validation run per shop
- latest credential update run per shop
- active run per shop
- latest failure per shop
- event timeline for a selected run

Run evidence should include:

- run id
- task type
- shop id
- status
- status label
- started at
- finished at
- profile id
- runtime metadata
- failure code
- failure message
- manual action required
- challenge type
- event timeline

## Domain Boundaries

### Shop Profile

Business master data.

Examples:

- shop id
- shop name
- platform code
- ASM connection status
- authorization status
- owner
- tags
- main category
- data completeness
- last ASM sync time

### Authorized Shop Projection

Execution-facing projection from Workspace and local runtime state.

Examples:

- shared login status
- local profile id
- local instance id
- profile exists
- core ready
- instance running
- reclaim pending

This should remain a projection, not the source of shop business truth.

### Operation Task

Business work attached to a shop.

Examples:

- collect shop product status
- inspect opportunity ranking
- run pre-listing checks
- prepare product sourcing workflow
- generate shop operation summary

This phase creates the task surface and lifecycle boundaries. It does not need to implement every future task type.

### Run

Execution evidence produced by local agent or desktop execution flows.

Examples:

- open
- bind
- validate
- diagnose
- retry

Runs are not shop profiles and not business tasks. They are evidence for actions.

## Must Do

### ASM Shop Profile List And Detail

Add or refactor a shop profile surface for ASM-connected shops.

Minimum behavior:

- list ASM shops
- show key business profile fields
- show ASM connection and authorization status
- show execution status summary from the workbench/evidence layer
- show operation task summary
- open shop detail
- navigate from detail to workbench action
- navigate from detail to task center

If backend ASM profile APIs are not yet available, the spec should still define the frontend and Wails/client boundary. Implementation may start with Workspace-provided fields plus explicit unavailable states, but it must not silently treat `WorkspaceAuthorizedShop` as the final ASM profile model.

### Shop Workbench Redesign

Redesign the current shop execution page around execution readiness.

Minimum behavior:

- show queue counts
- show shop action table
- support search and filters
- show recommended action
- show details drawer
- show latest open and validation evidence
- preserve existing open, bind, and validate actions
- replace the current warning-only batch open behavior with real safe batch execution

### Runs And Events Integration

Expose local agent run evidence through the Ant Browser backend and frontend.

Minimum behavior:

- fetch recent runs
- fetch run detail
- fetch run events
- derive latest run per shop and task type
- derive latest failure per shop
- render timeline in detail drawer
- render recent open and recent validation in table rows

### Failure-To-Recovery Mapping

Map known failure states to next actions.

Minimum mappings:

- missing fingerprint core -> go to core management
- core unavailable -> inspect core management or retry after repair
- shared login not ready -> update credentials
- awaiting verification -> local validation
- validation failed -> update credentials or retry validation
- authorization revoked -> disable execution and show reclaim state
- local profile missing -> refresh/reconcile authorized shops
- workspace agent unavailable -> show connection repair guidance
- workspace server unreachable -> show server connection state
- ant runtime unreachable -> show runtime repair guidance
- unknown failure -> show run evidence and diagnostic export

### Safe Batch Operations

Implement safe batch operations for execution actions.

Minimum behavior:

- batch open ready shops
- batch validate eligible shops
- batch retry failed eligible shops
- skip ineligible shops with explicit reasons
- limit concurrency
- show progress
- show result summary
- preserve per-shop run evidence

Batch behavior must not silently attempt actions on revoked, missing-core, missing-profile, or not-ready shops.

### Single-Shop Operation Task Skeleton

Add a single-shop operation task tab or section in the shop detail drawer/page.

Minimum behavior:

- show operation task summary for the selected shop
- show waiting, running, blocked, failed, and completed counts
- show latest task rows
- show blocked reason when execution readiness prevents a task
- expose create-task entry as a controlled skeleton

This skeleton should be ready for future sourcing, listing, collection, and reporting tasks.

### Global Operation Task Center Skeleton

Add a cross-shop operation task center skeleton.

Minimum behavior:

- list operation tasks
- filter by status
- filter by shop
- filter by blocked reason
- open related shop profile
- open related workbench action
- show failure or blocked reason

This is a product foundation. It is not a full automation scheduler in this phase.

## Should Do

### Diagnostic Export

Support exporting selected diagnostic context for support and regression.

Suggested contents:

- selected shop profile summary
- local execution projection
- latest runs
- selected run events
- app version
- platform
- workspace agent health
- ant runtime health
- relevant failure codes

### Navigation Cleanup

Rename and reorganize navigation so users see business concepts first.

Suggested navigation:

- `店铺资料`
- `店铺工作台`
- `运营任务`
- `运行记录`
- `系统维护`

The older `指纹浏览器` group can remain for lower-level tools such as core management, proxy pool, default bookmarks, tags, logs, and API docs.

### Status Language Cleanup

Use user-facing Chinese labels for status while preserving raw codes in details.

Examples:

- `ready` -> `可执行`
- `awaiting_verification` -> `待人工验证`
- `validation_failed` -> `验证失败`
- `reclaim_pending` -> `授权失效，待回收`
- `ANT_CORE_UNAVAILABLE` -> `指纹内核不可用`

## Can Wait

These items should not block the phase:

- full product sourcing workflow
- full listing/publishing workflow
- AI shop operation daily report generation
- advanced operation-task scheduling
- cross-shop automation orchestration
- long-running queue migration to a durable external job system
- release channel or gray rollout
- delta app updates

## Data Flow

### Shop Profile Flow

```text
Workspace / ASM source
  -> local workspace agent or desktop backend API boundary
  -> Ant Browser Wails backend
  -> frontend shop profile API
  -> Shop Profile Center
```

The frontend must not directly query databases or bypass the Wails/backend boundary.

### Execution Workbench Flow

```text
Workspace authorized shops
  -> local agent /local/shops
  -> Ant Browser workspace service
  -> managed profile reconcile
  -> shop execution projection
  -> Shop Workbench
```

Actions:

```text
Shop Workbench action
  -> Wails backend
  -> workspace service / local agent
  -> managed instance service
  -> open / bind / validate
  -> run evidence
  -> frontend state refresh
```

### Run Evidence Flow

```text
local agent runs/events
  -> Wails backend run evidence API
  -> frontend run evidence module
  -> workbench table summaries
  -> shop drawer timeline
  -> task center evidence links
```

## API Boundary

The implementation plan should define exact names after code inspection, but the intended backend boundary is:

```text
WorkspaceShopProfiles()
WorkspaceShopProfile(shopId)
WorkspaceAuthorizedShops()
WorkspaceRuns(query)
WorkspaceRun(runId)
WorkspaceRunEvents(runId)
WorkspaceOpenShop(shopId)
StartDesktopSharedLoginBind(accessToken, shopId)
StartDesktopSharedLoginValidate(accessToken, shopId)
```

Existing Wails APIs should be reused where they already fit. New APIs should remain thin adapters over Workspace/local-agent contracts.

## Frontend Module Boundaries

Suggested modules:

```text
frontend/src/modules/shops/
frontend/src/modules/workbench/
frontend/src/modules/operations/
frontend/src/modules/runEvidence/
```

The exact directory layout can follow local conventions during implementation. The key rule is boundary clarity:

- `shops` owns business profile pages and profile DTOs
- `workbench` owns execution readiness screens and actions
- `operations` owns operation task skeletons and global task views
- `runEvidence` owns run/event fetching, derivation, and timeline components

The existing `workspace` module can remain the transport/integration layer if that matches current code better.

## UI Design Requirements

The UI should feel like an operations console, not a marketing dashboard.

Requirements:

- dense but readable tables
- stable row heights
- clear status badges
- action buttons with icons
- details drawers for drill-down
- no nested cards inside cards
- no decorative hero treatment
- no one-note color theme
- no hidden fake actions that only toast "later"
- no status-only page without next actions

Primary surfaces:

- shop profile list
- shop profile detail
- shop workbench
- shop detail drawer
- operation task center
- run evidence timeline

## Error Handling

Errors must resolve into user-facing categories and actionable next steps.

Categories:

- workspace server unreachable
- workspace agent unavailable
- ant runtime unavailable
- fingerprint core missing
- fingerprint core unavailable
- local profile missing
- shared login not ready
- manual verification required
- credential update failed
- authorization revoked
- open failed
- report failed
- unknown error

Every category should provide:

- short label
- detail message
- raw code when available
- recommended action
- whether the action can be retried
- whether batch execution should skip it

## Testing Strategy

Backend tests:

- shop profile DTO normalization
- run evidence API adapters
- run/event derivation by shop and task type
- failure-to-recovery mapping
- batch eligibility and skip reasons
- existing open/bind/validate behavior remains stable

Frontend tests or verification:

- shop profile list renders empty, loading, normal, and error states
- shop profile detail displays ASM, execution, task, and evidence sections
- workbench filters and queues derive correct counts
- recent open and validation are derived from run evidence
- failure mapping shows the correct next action
- batch operation summary includes success, skipped, and failed rows
- operation task center skeleton handles empty, loading, normal, blocked, and failed states

Manual regression:

- login and session recovery
- workspace agent bootstrap
- authorized shop sync
- open ready shop
- update credentials
- local validation
- open failure shows evidence
- missing core maps to core repair action
- batch open skips ineligible shops
- shop profile detail links to workbench and task center

## Non-Goals

This phase does not:

- implement a complete product sourcing system
- implement a complete listing or publishing system
- implement AI-generated operation reports
- implement a full durable scheduler
- store product or order business entities in Ant Browser local state
- let frontend access databases directly
- redesign release/update infrastructure
- reopen macOS signing or notarization work

## Success Criteria

The phase is successful when:

- ASM shops have a dedicated profile list and detail surface
- users can distinguish shop business profile from execution readiness
- the shop workbench shows real recent open and validation state
- warning-only batch open behavior is replaced by safe batch execution
- every open/bind/validate failure can be inspected through run evidence
- known failure codes map to concrete next actions
- single-shop operation tasks have a clear home
- cross-shop operation tasks have a clear home
- the release/update chain remains untouched except for normal compatibility checks
