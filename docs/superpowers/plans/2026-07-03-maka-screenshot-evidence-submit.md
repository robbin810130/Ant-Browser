# Maka Browser Screenshot Evidence Submission Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Maka Browser 将已压缩的截图证据提交到现有 Maka 专员任务 API，同时保持后台链接和处理摘要行为不变。

**Architecture:** 保留 `SpecialistTaskDrawer` 已有的图片读取、压缩和 data URL 生成逻辑，仅移除 `submitSpecialistTaskEvidence` 中已经过期的截图合同拦截。服务端继续负责证据类型、图片内容、任务状态、权限和审计校验。

**Tech Stack:** React 18、TypeScript、Wails `DesktopWorkspaceRequest`、Node.js contract smoke。

---

## File Map

- Modify: `frontend/scripts/specialist-presentation-smoke.mjs` — 锁定截图证据不能被客户端主动拦截，并确认截图载荷字段会透传。
- Modify: `frontend/src/modules/specialist/api.ts` — 删除遗留的截图拒绝分支及不再使用的辅助函数。

### Task 1: Enable Screenshot Evidence Submission

**Files:**
- Modify: `frontend/scripts/specialist-presentation-smoke.mjs:103-105`
- Modify: `frontend/src/modules/specialist/api.ts:169-171`
- Modify: `frontend/src/modules/specialist/api.ts:226-256`
- Test: `frontend/scripts/specialist-presentation-smoke.mjs`

- [ ] **Step 1: Write the failing contract assertions**

在现有截图和链接断言后加入：

```js
assert.doesNotMatch(
  apiSource,
  /截图证据上传暂不能提交|unsupportedContract/,
  'screenshot evidence must reach the Maka specialist evidence endpoint instead of being blocked in the client',
)
assert.match(
  apiSource,
  /payload:\s*\{\s*\.\.\.payload\.payload,/,
  'screenshot evidence fields should pass through to the Maka specialist evidence payload',
)
```

- [ ] **Step 2: Run the contract smoke and verify RED**

Run:

```bash
cd /Users/robbin/Codex/1688shopManager/desktop-repos/ant-browser-phase1
rtk node frontend/scripts/specialist-presentation-smoke.mjs
```

Expected: FAIL on `screenshot evidence must reach...` because `api.ts` still contains `unsupportedContract('截图证据上传暂不能提交')`.

- [ ] **Step 3: Remove the obsolete client-side blocker**

从 `frontend/src/modules/specialist/api.ts` 删除未再使用的辅助函数：

```ts
function unsupportedContract(message: string): never {
  throw new Error(`${message}。当前客户端已接入任务读取、详情、证据说明和申诉；该动作需要服务端 Maka 专员任务合同继续落地。`)
}
```

并从 `submitSpecialistTaskEvidence` 删除：

```ts
if (payload.evidenceType === 'screenshot') {
  unsupportedContract('截图证据上传暂不能提交')
}
```

保留现有请求体构造，使 `payload.payload` 中的 `fileName`、`mimeType`、`size`、`dataUrl` 和 `note` 继续透传：

```ts
body: {
  stepId: payload.stepId,
  evidenceType: payload.evidenceType,
  payload: {
    ...payload.payload,
    text: text || undefined,
    url: url || undefined,
    fileName: fileName || undefined,
  },
},
```

- [ ] **Step 4: Run the focused contract smoke and verify GREEN**

Run:

```bash
rtk node frontend/scripts/specialist-presentation-smoke.mjs
```

Expected: PASS with `specialist presentation smoke passed`.

- [ ] **Step 5: Run the frontend production build**

Run:

```bash
cd /Users/robbin/Codex/1688shopManager/desktop-repos/ant-browser-phase1/frontend
rtk npm run build
```

Expected: TypeScript and Vite build complete successfully.

- [ ] **Step 6: Verify patch hygiene**

Run:

```bash
cd /Users/robbin/Codex/1688shopManager/desktop-repos/ant-browser-phase1
rtk git diff --check
rtk git status --short
```

Expected: no whitespace errors; only the plan, smoke test and `api.ts` are changed. Existing untracked `images/maka-browser-icon-candidates/` and `node_modules/` remain untouched.

- [ ] **Step 7: Commit the fix**

Run:

```bash
rtk git add frontend/scripts/specialist-presentation-smoke.mjs frontend/src/modules/specialist/api.ts
rtk git commit -m "fix: submit screenshot task evidence"
```

Expected: one focused implementation commit. Do not publish a Maka Browser release.

