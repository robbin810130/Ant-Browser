# Ant-Browser Desktop Auth Shell Design

Date: 2026-05-11
Status: Proposed
Scope: Ant-Browser desktop client account shell for login, session recovery, logout, account switching, and device rebind

## Background

The current Ant-Browser desktop client can already:

- bootstrap workspace agent access
- fetch authorized shops
- open managed 1688 shop backends through native managed instances
- reuse and recover running managed instances

But the desktop client does not yet behave like a real authenticated product shell:

- there is no desktop login page
- the top-right `Admin` entry is only a static `/profile` link
- logout is not exposed in the desktop UI
- account switching is not exposed
- device rebind is not exposed
- desktop routes depend on an already-prepared server session instead of the client owning session lifecycle

This blocks clean validation of permission revocation and makes the desktop client feel like a local host shell rather than a complete user-facing product.

## Goals

Add a first-class auth shell to the Ant-Browser desktop client so that:

- the client requires login before any workspace or instance feature is accessible
- login supports an optional `remember me` flow
- the client can restore a remembered session on startup
- the top-right account area shows the real current user instead of a static placeholder
- logout, account switch, and device rebind are available from the client
- logout, account switch, and device rebind all perform strong cleanup by stopping every managed instance and clearing local session state
- existing managed instance open/reuse behavior remains intact

## Non-Goals

This design does not include:

- password reset
- registration
- MFA or CAPTCHA
- third-party login
- multi-tenant org switching
- permission editing UI
- server-side revocation management UI

## Product Decisions

Confirmed decisions for this design:

- startup behavior: force login
- session persistence: support `remember me`
- cleanup policy: strongest cleanup on logout, account switch, and device rebind
- target approach: native desktop auth shell inside Ant-Browser, not an external web login workaround

## User Experience

### Entry Flow

On app startup:

1. Read local persisted auth state.
2. If no remembered token exists, route directly to `/login`.
3. If a remembered token exists, validate it via `auth/me`.
4. Only after token validation succeeds may the app enter the protected shell.
5. After auth succeeds, bootstrap desktop session, register device, and start heartbeat before enabling workspace operations.

There is no "enter shell first, then prompt login" flow. Unauthenticated users only see the login screen.

### Login Screen

Add a dedicated `/login` screen with:

- username
- password
- remember me checkbox
- login button
- inline login error state

Deliberately excluded from first release:

- forgot password
- signup
- SMS verification
- QR login

### Account Menu

Replace the current top-right `Admin` static link with a real account menu containing:

- display name
- username
- role summary
- data scope summary
- current device ID summary
- current device status summary
- `Switch Account`
- `Rebind Device`
- `Logout`

The existing static `/profile` page should no longer act as the account entry. If retained, it should be reclassified as an about/developer page, not an auth surface.

### High-Risk Actions

The following actions must require confirmation:

- logout
- switch account
- rebind device

The confirmation must clearly state that the client will:

- stop all managed shop instances
- clear the current desktop session
- require login again before opening any shop backend

Action behavior:

- Logout: cleanup immediately, then go to `/login`
- Switch Account: cleanup immediately, then go to `/login` with a "please sign in with a different account" hint
- Rebind Device: cleanup immediately, clear current device registration state, then go to `/login`; successful login triggers fresh bootstrap/register flow

## Information Architecture

The desktop client becomes two shells:

### Auth Shell

Routes:

- `/login`

Responsibilities:

- collect credentials
- submit login
- optionally persist token
- show login and session-expired messaging

### Protected App Shell

Routes remain under the current application layout, including:

- `/`
- `/browser/list`
- `/settings`
- existing browser tooling pages

Responsibilities:

- render sidebar/topbar/business screens
- expose account menu
- operate workspace and managed instances

### Route Guarding

Rules:

- unauthenticated users may only access `/login`
- protected routes redirect to `/login` when unauthenticated
- expired or invalid sessions trigger cleanup and redirect to `/login`
- topbar and sidebar are not rendered while unauthenticated

## State Model

Introduce a dedicated auth state separate from workspace business state.

### Auth Status

- `anonymous`
- `authenticating`
- `authenticated`
- `session_expired`
- `signing_out`

### Session State

- `accessToken`
- `rememberMe`
- `user`
- `roles`
- `dataScope`
- `deviceId`
- `deviceStatus`

### Desktop Runtime State

- `bootstrapReady`
- `deviceRegistered`
- `heartbeatActive`

Important distinction:

- `authenticated` means the token is valid
- `bootstrapReady` means the desktop client has successfully completed post-login desktop initialization and may safely use workspace flows

## Persistence Model

When `remember me` is enabled, persist only:

- `accessToken`
- `rememberMe = true`

Do not persist derived user/session display data such as:

- `user`
- `roles`
- `dataScope`
- `deviceStatus`

Those must be reloaded from `auth/me` and desktop bootstrap on startup so the client never trusts stale local presentation state.

When `remember me` is disabled:

- keep auth token in memory only
- app close equals logout from the desktop client perspective

## API and Integration Design

### Existing Server APIs To Reuse

No new server auth protocol is required for phase one. Reuse:

- `POST /api/auth/login`
- `GET /api/auth/me`
- `POST /api/auth/logout`

The desktop workspace APIs remain behind authenticated access:

- `POST /api/desktop/session/bootstrap`
- `POST /api/desktop/devices/register`
- `POST /api/desktop/devices/{id}/heartbeat`
- `GET /api/desktop/shops`
- `POST /api/desktop/shops/{shopId}/open`
- related bind/validate/report routes

## Desktop Client Auth Client

Add a dedicated desktop auth client layer that:

- performs login
- performs `auth/me`
- performs logout
- automatically injects bearer token into desktop route requests

This changes the ownership model from:

- "desktop routes assume some external session already exists"

to:

- "Ant-Browser desktop owns token acquisition, persistence, validation, and cleanup"

## Wails / Local Backend Responsibilities

Keep frontend and backend responsibilities clean:

### Frontend

- login UI
- route guard behavior
- account menu
- confirmation dialogs
- auth state orchestration

### Wails / local backend

- read persisted auth token
- write persisted auth token
- clear persisted auth token
- execute strongest cleanup
- stop managed instances
- stop or reset local runtime integrations tied to the current account
- expose current local auth bootstrap status to the frontend

The frontend should not directly own OS-side cleanup behavior.

## Cleanup Semantics

The following events trigger strongest cleanup:

- logout
- switch account
- rebind device
- `auth/me` returns 401 during startup recovery
- any critical desktop route returns an auth-expired condition

Cleanup steps:

1. stop all managed instances
2. stop heartbeat
3. clear local runtime bindings associated with the current account
4. clear in-memory auth state
5. remove persisted token if present
6. route to `/login`

This must prevent any mixed-state outcome where:

- old shop instances keep running after logout
- UI shows one user while device/runtime still belongs to another
- a new account inherits an old account's device registration or managed runtime state

## Error Handling

### 401 Unauthorized

Interpretation:

- session missing
- session revoked
- session expired

Behavior:

- trigger strongest cleanup
- redirect to `/login`
- show one-time message: `登录已失效，请重新登录`

### 403 Forbidden

Interpretation:

- user is authenticated but lacks permission

Behavior:

- keep auth state
- do not force logout
- show explicit permission error

### Desktop Bootstrap / Device Register / Heartbeat Failure

Interpretation:

- login succeeded but the desktop client is not ready for workspace actions

Behavior:

- keep authenticated state
- block access to workspace actions until device init succeeds or user rebinds device
- show explicit blocking message: `设备初始化失败，请重试或重绑设备`

### Logout Failure

Behavior:

- local strongest cleanup wins
- even if `POST /api/auth/logout` fails, the desktop client must not remain half-logged-in

## Testing Strategy

### Frontend State Tests

Cover:

- unauthenticated startup renders only `/login`
- remembered token calls `auth/me`
- valid `auth/me` enters protected shell
- invalid `auth/me` clears local state and returns to `/login`
- `remember me` changes persistence behavior

### Frontend Interaction Tests

Cover:

- account menu shows current user summary
- logout confirmation appears
- switch account confirmation appears
- rebind device confirmation appears
- confirming those actions launches cleanup flow

### Wails / Local Backend Tests

Cover:

- token read/write/clear
- strongest cleanup stops managed instances
- heartbeat stop/reset behavior
- local auth bootstrap state transitions

### Real-Device Acceptance

Required acceptance checks:

1. first launch without session goes directly to login
2. successful login enters the client and shows the real current user
3. remember me enabled restores session after app restart
4. remember me disabled requires login after app restart
5. logout closes all managed instances and returns to login
6. switch account closes all old instances, then the new login sees only the new account's authorized shops
7. rebind device clears old device state, then successful re-login re-registers the device and restores workspace capability
8. existing managed shop open/reuse behavior still works after auth shell integration

## Implementation Boundaries

Expected touch areas:

- frontend app routing and bootstrapping
- topbar account entry
- new login page and auth store
- local persistent session handling
- Wails bindings for local auth token storage and strongest cleanup
- desktop API client request injection

Out of scope for this slice:

- redesign of workspace dashboard business cards
- server-side user or permission model changes
- server-side revocation product UI

## Success Criteria

This design is complete only when all of the following are true:

- Ant-Browser no longer depends on externally prepared login state
- unauthenticated users cannot enter the desktop business shell
- the top-right account area is backed by real authenticated user state
- logout, switch account, and rebind device all execute strongest cleanup
- remember-me behavior is explicit and predictable
- existing native managed instance shop-open behavior remains working

## Risks

### Risk: Auth shell breaks existing workspace initialization

Mitigation:

- treat post-login bootstrap as a separate state from token validity
- preserve existing workspace bootstrap internals, only move their trigger point behind login

### Risk: Cleanup becomes too destructive or inconsistent

Mitigation:

- centralize strongest cleanup in one backend-owned path
- never duplicate cleanup rules independently across multiple frontend screens

### Risk: UI still mixes auth health and business health

Mitigation:

- keep account/session concerns in auth shell and account menu
- keep business status in workspace pages
- do not reuse the current dashboard health card as the source of truth for login state
