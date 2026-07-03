import assert from 'node:assert/strict'
import { readFile, rm } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { spawnSync } from 'node:child_process'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const outDir = resolve(root, '.tmp', 'specialist-presentation-smoke')
const tsc = resolve(root, 'node_modules', 'typescript', 'bin', 'tsc')

await rm(outDir, { recursive: true, force: true })

const compile = spawnSync(
  process.execPath,
  [
    tsc,
    '--target',
    'ES2020',
    '--module',
    'NodeNext',
    '--moduleResolution',
    'NodeNext',
    '--strict',
    '--skipLibCheck',
    '--outDir',
    outDir,
    '--rootDir',
    resolve(root, 'src'),
    resolve(root, 'src', 'modules', 'specialist', 'presentation.ts'),
  ],
  { cwd: root, stdio: 'inherit' },
)

if (compile.status !== 0) {
  process.exit(compile.status ?? 1)
}

const presentation = await import(
  pathToFileURL(resolve(outDir, 'modules', 'specialist', 'presentation.js')).href
)

assert.equal(presentation.specialistTaskStatusLabel('pending'), '待开始')
assert.equal(presentation.specialistTaskStatusLabel('submitted_pending_validation'), '已提交待校验')
assert.equal(presentation.specialistTaskStatusLabel('appeal_in_review'), '申诉中')
assert.equal(presentation.specialistTaskPriorityLabel('critical'), '紧急')
assert.equal(presentation.specialistTaskPriorityLabel('high'), '高')
assert.equal(presentation.specialistTaskPriorityLabel('medium'), '中')
assert.equal(presentation.specialistTaskPriorityLabel('low'), '低')

const past = new Date(Date.now() - 60_000).toISOString()
assert.equal(presentation.specialistTaskDeadlineTone(past), 'danger')

assert.equal(
  presentation.requiredStepSummary([
    { required: true, status: 'done' },
    { required: true, status: 'not_started' },
    { required: false, status: 'not_started' },
  ]),
  '1/2',
)

const pageSource = await readFile(
  resolve(root, 'src', 'modules', 'specialist', 'pages', 'SpecialistTaskPanelPage.tsx'),
  'utf8',
)
const drawerSource = await readFile(
  resolve(root, 'src', 'modules', 'specialist', 'components', 'SpecialistTaskDrawer.tsx'),
  'utf8',
)
const apiSource = await readFile(
  resolve(root, 'src', 'modules', 'specialist', 'api.ts'),
  'utf8',
)

assert.match(pageSource, /shared\/components/, 'specialist task page should use the Maka Browser shared components')
assert.match(drawerSource, /shared\/components/, 'specialist task drawer should use the Maka Browser shared components')
assert.doesNotMatch(pageSource, /from 'antd'/, 'specialist task page must stay aligned with the Maka Browser client component system')
assert.doesNotMatch(drawerSource, /from 'antd'/, 'specialist task drawer must stay aligned with the Maka Browser client component system')
assert.doesNotMatch(
  pageSource,
  /@ant-design\/icons/,
  'specialist task page must not depend on Ant Design icons',
)
assert.doesNotMatch(
  drawerSource,
  /@ant-design\/icons/,
  'specialist task drawer must not depend on Ant Design icons',
)
assert.doesNotMatch(
  pageSource,
  /requiredStepSummary\(row\.sopSteps\)/,
  'task list must not claim SOP progress when list responses omit SOP steps',
)
assert.match(drawerSource, /type="file"/, 'specialist task drawer should expose a native screenshot file picker')
assert.match(drawerSource, /\bProgress\b/, 'specialist task drawer should expose required SOP completion progress with the shared component')
assert.match(drawerSource, /role="tablist"/, 'specialist task drawer should separate submit, appeal, and blocked actions with client tabs')
assert.match(drawerSource, /lastEvidenceFeedback/, 'evidence submission should leave local feedback inside the drawer')
assert.match(drawerSource, /最近提交反馈/, 'evidence submission feedback should be visible to the specialist')
assert.match(drawerSource, /onRefreshTask/, 'SOP step changes should refresh the selected task detail from the server contract')
assert.match(drawerSource, /await onRefreshTask\(task\.id\)/, 'SOP step changes should re-read task detail after a successful mutation')
assert.match(drawerSource, /screenshotError/, 'screenshot validation should provide field-level feedback')
assert.match(drawerSource, /evidenceLinkError/, 'link evidence validation should provide field-level feedback')
assert.match(drawerSource, /dataUrl/, 'screenshot evidence should include portable image data')
assert.match(drawerSource, /return \{ url:/, 'link evidence should submit a URL payload')
assert.doesNotMatch(
  apiSource,
  /SOP 步骤状态暂不能在客户端直接更新/,
  'SOP updates must call the Maka specialist server contract instead of being blocked in the renderer',
)
assert.match(
  apiSource,
  /appendQuery\('\/api\/maka\/specialist\/tasks\/today', query\)/,
  'specialist task list should use the Maka specialist task contract path',
)
assert.match(
  apiSource,
  /`\/api\/maka\/specialist\/tasks\/\$\{encodeURIComponent\(taskId\.trim\(\)\)\}`/,
  'specialist task detail should use the Maka specialist task contract path so SOP snapshots are returned',
)
assert.match(
  apiSource,
  /`\/api\/maka\/specialist\/shops\/\$\{encodeURIComponent\(normalizedShopId\)\}\/tasks`/,
  'shop specialist task list should use the Maka specialist shop task contract path',
)
assert.match(
  apiSource,
  /\/api\/maka\/specialist\/tasks\/\$\{encodeURIComponent\(taskId\.trim\(\)\)\}\/sop-steps\/\$\{encodeURIComponent\(stepId\.trim\(\)\)\}/,
  'SOP updates should use the Maka specialist task contract path',
)
assert.match(apiSource, /devSpecialistTask/, 'dev fallback should preserve local specialist task state across detail refreshes')
assert.match(apiSource, /DesktopWorkspaceRequest/, 'specialist task API should proxy through the Wails backend')
assert.doesNotMatch(apiSource, /\bfetch\(/, 'specialist task API must not fetch the workspace server directly from the renderer')

await rm(outDir, { recursive: true, force: true })
console.log('specialist presentation smoke passed')
