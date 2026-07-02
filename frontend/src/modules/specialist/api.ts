import { DesktopWorkspaceRequest } from '../../wailsjs/go/main/App'
import { useDevWorkspaceFallback } from '../workspace/devData'
import { useAuthStore } from '../../store/authStore'
import type {
  AppealSpecialistTaskPayload,
  BlockSpecialistTaskPayload,
  SpecialistTaskEvidenceRecord,
  SpecialistTaskListQuery,
  SpecialistTaskListResponse,
  SpecialistTaskMutationResponse,
  SpecialistTaskRecord,
  SpecialistTaskRuleSnapshotSummary,
  SpecialistTaskSopStep,
  SubmitSpecialistTaskEvidencePayload,
  SubmitSpecialistTaskPayload,
  UpdateSpecialistTaskSopStepPayload,
} from './types'

type Envelope<T> = {
  code?: number
  message?: string
  data?: T
}

type SpecialistRequestInit = {
  method?: string
  body?: Record<string, unknown>
}

function normalizeString(value: unknown): string {
  return String(value ?? '').trim()
}

function normalizeNullableString(value: unknown): string | null {
  const normalized = normalizeString(value)
  return normalized || null
}

function normalizeNumber(value: unknown): number {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : 0
}

function normalizeObject(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}
}

function normalizeStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => normalizeString(item)).filter(Boolean) : []
}

function normalizeSopStep(input: any): SpecialistTaskSopStep {
  return {
    id: normalizeString(input?.id),
    taskId: normalizeString(input?.taskId),
    stepId: normalizeString(input?.stepId),
    actionCode: normalizeString(input?.actionCode),
    title: normalizeString(input?.title),
    description: normalizeString(input?.description),
    required: Boolean(input?.required),
    evidenceTypes: normalizeStringArray(input?.evidenceTypes),
    status: normalizeString(input?.status || 'not_started'),
    evidenceRefs: normalizeStringArray(input?.evidenceRefs),
    operatorNote: normalizeString(input?.operatorNote),
    createdAt: normalizeString(input?.createdAt),
    updatedAt: normalizeString(input?.updatedAt),
  }
}

function normalizeEvidenceRecord(input: any): SpecialistTaskEvidenceRecord {
  return {
    id: normalizeString(input?.id),
    taskId: normalizeString(input?.taskId),
    stepId: normalizeNullableString(input?.stepId),
    evidenceType: normalizeString(input?.evidenceType || 'text_note'),
    payload: normalizeObject(input?.payload ?? {
      text: input?.summary,
      attachments: input?.attachments,
      autoCheckStatus: input?.autoCheckStatus,
      autoCheckNote: input?.autoCheckNote,
    }),
    submittedBy: normalizeString(input?.submittedBy || input?.submitterUserId),
    createdAt: normalizeString(input?.createdAt || input?.submittedAt),
  }
}

function normalizeRuleSnapshotSummary(input: any): SpecialistTaskRuleSnapshotSummary | null {
  if (!input || typeof input !== 'object' || Array.isArray(input)) return null
  return {
    ruleInstanceId: normalizeNullableString(input.ruleInstanceId),
    ruleVersion: input.ruleVersion ?? null,
    publicationId: normalizeNullableString(input.publicationId),
    publishedAt: normalizeNullableString(input.publishedAt),
    source: normalizeNullableString(input.source),
    validationMetrics: Array.isArray(input.validationMetrics) ? input.validationMetrics : [],
  }
}

export function normalizeSpecialistTask(input: any): SpecialistTaskRecord {
  return {
    id: normalizeString(input?.id),
    shopId: normalizeString(input?.shopId),
    shopName: normalizeString(input?.shopName),
    title: normalizeString(input?.title),
    description: normalizeString(input?.description),
    sourceType: normalizeString(input?.sourceType),
    priority: normalizeString(input?.priority || 'medium'),
    status: normalizeString(input?.status || 'pending'),
    timelineBlockCode: normalizeString(input?.timelineBlockCode),
    timelineBlockLabel: normalizeString(input?.timelineBlockLabel),
    assigneeUserId: normalizeString(input?.assigneeUserId),
    assigneeName: normalizeString(input?.assigneeName),
    supervisorUserId: normalizeString(input?.supervisorUserId),
    supervisorName: normalizeString(input?.supervisorName),
    deadlineAt: normalizeNullableString(input?.deadlineAt),
    pauseStartedAt: normalizeNullableString(input?.pauseStartedAt),
    pausedSeconds: normalizeNumber(input?.pausedSeconds),
    validationResult: normalizeNullableString(input?.validationResult),
    metadata: normalizeObject(input?.metadata),
    anomalySignalId: normalizeNullableString(input?.anomalySignalId),
    anomalySignalType: normalizeNullableString(input?.anomalySignalType),
    anomalySignalSourceRef: normalizeNullableString(input?.anomalySignalSourceRef),
    anomalySignalSourceTier: normalizeNullableString(input?.anomalySignalSourceTier),
    anomalySignalSnapshot: input?.anomalySignalSnapshot ? normalizeObject(input.anomalySignalSnapshot) : null,
    ruleSnapshotSummary: normalizeRuleSnapshotSummary(input?.ruleSnapshotSummary),
    createdAt: normalizeString(input?.createdAt),
    updatedAt: normalizeString(input?.updatedAt),
    sopSteps: Array.isArray(input?.sopSteps) ? input.sopSteps.map(normalizeSopStep) : [],
    evidenceRecords: Array.isArray(input?.evidenceRecords)
      ? input.evidenceRecords.map(normalizeEvidenceRecord)
      : Array.isArray(input?.evidences)
        ? input.evidences.map((item: any) => normalizeEvidenceRecord({ ...item, taskId: input?.id }))
        : [],
  }
}

function normalizeListResponse(input: any): SpecialistTaskListResponse {
  const items = Array.isArray(input?.items) ? input.items.map(normalizeSpecialistTask) : []
  return {
    items,
    pagination: {
      page: normalizeNumber(input?.pagination?.page) || 1,
      pageSize: normalizeNumber(input?.pagination?.pageSize) || items.length || 20,
      total: normalizeNumber(input?.pagination?.total) || items.length,
    },
    summary: {
      total: normalizeNumber(input?.summary?.total) || items.length,
      pending: normalizeNumber(input?.summary?.pending),
      inProgress: normalizeNumber(input?.summary?.inProgress),
      submittedPendingValidation: normalizeNumber(input?.summary?.submittedPendingValidation),
      appealInReview: normalizeNumber(input?.summary?.appealInReview),
      overdue: normalizeNumber(input?.summary?.overdue),
      completed: normalizeNumber(input?.summary?.completed),
    },
  }
}

function normalizeMutationResponse(input: any): SpecialistTaskMutationResponse {
  return {
    task: normalizeSpecialistTask(input?.task ?? input),
    evidenceId: normalizeString(input?.evidenceId) || undefined,
    appealId: normalizeString(input?.appealId) || undefined,
  }
}

function appendQuery(path: string, query: SpecialistTaskListQuery = {}): string {
  const params = new URLSearchParams()
  if (query.page) params.set('page', String(query.page))
  if (query.pageSize) params.set('pageSize', String(query.pageSize))
  if (query.status) params.set('status', query.status)
  const search = params.toString()
  return search ? `${path}?${search}` : path
}

function unwrapEnvelope<T>(input: unknown): T {
  const envelope = normalizeObject(input) as Envelope<T>
  if (Number(envelope.code ?? 0) !== 0) {
    throw new Error(envelope.message || '专员任务接口请求失败')
  }
  return envelope.data as T
}

async function specialistRequest<T>(path: string, init: SpecialistRequestInit = {}): Promise<T> {
  if (useDevWorkspaceFallback()) {
    return devSpecialistRequest<T>(path, init)
  }

  const token = useAuthStore.getState().accessToken.trim()
  if (!token) {
    throw new Error('登录态已失效，请重新登录')
  }

  const method = normalizeString(init.method || 'GET').toUpperCase() || 'GET'
  const envelope = await DesktopWorkspaceRequest(token, method, path, init.body ?? {})
  return unwrapEnvelope<T>(envelope)
}

export async function fetchTodaySpecialistTasks(query: SpecialistTaskListQuery = {}): Promise<SpecialistTaskListResponse> {
  const data = await specialistRequest<unknown>(appendQuery('/api/maka/specialist/tasks/today', query))
  return normalizeListResponse(data)
}

export async function fetchShopSpecialistTasks(shopId: string, query: SpecialistTaskListQuery = {}): Promise<SpecialistTaskListResponse> {
  const normalizedShopId = shopId.trim()
  if (!normalizedShopId) return normalizeListResponse({ items: [] })
  const data = await specialistRequest<unknown>(
    appendQuery(`/api/maka/specialist/shops/${encodeURIComponent(normalizedShopId)}/tasks`, query),
  )
  return normalizeListResponse(data)
}

export async function fetchSpecialistTaskDetail(taskId: string): Promise<SpecialistTaskRecord> {
  const data = await specialistRequest<unknown>(`/api/maka/specialist/tasks/${encodeURIComponent(taskId.trim())}`)
  return normalizeSpecialistTask((data as any)?.task ?? data)
}

export async function updateSpecialistTaskSopStep(
  taskId: string,
  stepId: string,
  payload: UpdateSpecialistTaskSopStepPayload,
): Promise<SpecialistTaskMutationResponse> {
  const data = await specialistRequest<unknown>(
    `/api/maka/specialist/tasks/${encodeURIComponent(taskId.trim())}/sop-steps/${encodeURIComponent(stepId.trim())}`,
    {
      method: 'POST',
      body: {
        status: payload.status,
        operatorNote: payload.operatorNote ?? '',
        evidenceRefs: payload.evidenceRefs ?? [],
      },
    },
  )
  return normalizeMutationResponse(data)
}

export async function submitSpecialistTaskEvidence(
  taskId: string,
  payload: SubmitSpecialistTaskEvidencePayload,
): Promise<SpecialistTaskMutationResponse> {
  const text = normalizeString(payload.payload.text || payload.payload.note)
  const url = normalizeString(payload.payload.url)
  const fileName = normalizeString(payload.payload.fileName)
  const summary = text || url || fileName
  if (summary.length < 4) {
    throw new Error('证据说明至少需要 4 个字符')
  }
  const data = await specialistRequest<unknown>(
    `/api/maka/specialist/tasks/${encodeURIComponent(taskId.trim())}/evidence`,
    {
      method: 'POST',
      body: {
        stepId: payload.stepId,
        evidenceType: payload.evidenceType,
        payload: payload.payload,
      },
    },
  )
  return normalizeMutationResponse(data)
}

export async function submitSpecialistTask(
  taskId: string,
  payload: SubmitSpecialistTaskPayload,
): Promise<SpecialistTaskMutationResponse> {
  const summary = normalizeString(payload.summary || '处理完成，提交验收')
  const data = await specialistRequest<unknown>(
    `/api/maka/specialist/tasks/${encodeURIComponent(taskId.trim())}/submit`,
    { method: 'POST', body: { summary } },
  )
  return normalizeMutationResponse(data)
}

export async function appealSpecialistTask(
  taskId: string,
  payload: AppealSpecialistTaskPayload,
): Promise<SpecialistTaskMutationResponse> {
  const data = await specialistRequest<unknown>(
    `/api/maka/specialist/tasks/${encodeURIComponent(taskId.trim())}/appeal`,
    { method: 'POST', body: { reason: payload.reason } },
  )
  return normalizeMutationResponse(data)
}

export async function blockSpecialistTask(
  taskId: string,
  payload: BlockSpecialistTaskPayload,
): Promise<SpecialistTaskMutationResponse> {
  const data = await specialistRequest<unknown>(
    `/api/maka/specialist/tasks/${encodeURIComponent(taskId.trim())}/block`,
    {
      method: 'POST',
      body: {
        reasonCode: payload.reasonCode,
        reasonText: payload.reasonText,
      },
    },
  )
  return normalizeMutationResponse(data)
}

function buildDevTask(overrides: Partial<SpecialistTaskRecord> = {}): SpecialistTaskRecord {
  return normalizeSpecialistTask({
    id: overrides.id ?? 'dev-specialist-task-1',
    shopId: overrides.shopId ?? 'shop-ready',
    shopName: overrides.shopName ?? '义乌百货样板店',
    title: overrides.title ?? '检查核心商品曝光并回传处置证据',
    description: overrides.description ?? '进入 1688 后台同屏处理异常任务。',
    sourceType: 'dev_mock',
    priority: 'high',
    status: overrides.status ?? 'in_progress',
    timelineBlockCode: 'open-diagnosis',
    timelineBlockLabel: '打开诊断',
    assigneeUserId: 'dev',
    assigneeName: '开发专员',
    supervisorUserId: 'dev-supervisor',
    supervisorName: '开发主管',
    deadlineAt: new Date(Date.now() + 45 * 60 * 1000).toISOString(),
    ruleSnapshotSummary: {
      ruleInstanceId: 'dev-rule-instance-1',
      ruleVersion: 1,
      publicationId: 'dev-publication-1',
      publishedAt: new Date().toISOString(),
      source: 'dev_fallback',
      validationMetrics: [],
    },
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    sopSteps: [
      {
        id: 'dev-step-1',
        taskId: overrides.id ?? 'dev-specialist-task-1',
        stepId: 'step_1_check_exposure',
        actionCode: 'check_exposure',
        title: '检查核心商品曝光',
        description: '进入 1688 后台检查核心商品曝光入口。',
        required: true,
        evidenceTypes: ['text_note', 'screenshot', 'backend_url'],
        status: 'not_started',
        evidenceRefs: [],
        operatorNote: '',
      },
      {
        id: 'dev-step-2',
        taskId: overrides.id ?? 'dev-specialist-task-1',
        stepId: 'step_2_optimize_keyword',
        actionCode: 'optimize_title_keyword',
        title: '复核标题关键词',
        description: '复核标题关键词是否匹配处置目标。',
        required: false,
        evidenceTypes: ['text_note'],
        status: 'not_started',
        evidenceRefs: [],
        operatorNote: '',
      },
    ],
    evidenceRecords: [],
    ...overrides,
  })
}

let devSpecialistTask: SpecialistTaskRecord | null = null
let devEvidenceSequence = 0

function getDevSpecialistTask(): SpecialistTaskRecord {
  if (!devSpecialistTask) {
    devSpecialistTask = buildDevTask()
  }
  return devSpecialistTask
}

function setDevSpecialistTask(task: SpecialistTaskRecord): SpecialistTaskRecord {
  devSpecialistTask = normalizeSpecialistTask(task)
  return devSpecialistTask
}

function devListResponse(task: SpecialistTaskRecord, path: string): SpecialistTaskListResponse {
  const query = path.includes('?') ? new URLSearchParams(path.split('?')[1]) : new URLSearchParams()
  const status = normalizeString(query.get('status'))
  const items = status && task.status !== status ? [] : [task]
  return normalizeListResponse({ items })
}

async function devSpecialistRequest<T>(path: string, init: SpecialistRequestInit = {}): Promise<T> {
  const method = String(init.method || 'GET').toUpperCase()
  const task = getDevSpecialistTask()
  if (method === 'GET' && path.startsWith('/api/maka/specialist/tasks/today')) {
    return devListResponse(task, path) as T
  }
  const shopTasksMatch = path.match(/^\/api\/maka\/specialist\/shops\/([^/]+)\/tasks(?:\?|$)/)
  if (method === 'GET' && shopTasksMatch) {
    const shopId = decodeURIComponent(shopTasksMatch[1])
    if (shopId && shopId !== task.shopId) {
      return normalizeListResponse({ items: [] }) as T
    }
    const filtered = devListResponse(task, path)
    return normalizeListResponse({ ...filtered, items: filtered.items.length ? [task] : [] }) as T
  }
  const detailMatch = path.match(/^\/api\/maka\/specialist\/tasks\/([^/?]+)(?:\?|$)/)
  if (method === 'GET' && detailMatch) {
    const taskId = decodeURIComponent(detailMatch[1])
    if (taskId !== task.id) {
      throw new Error('开发任务不存在')
    }
    return task as T
  }
  const sopStepMatch = path.match(/^\/api\/maka\/specialist\/tasks\/([^/]+)\/sop-steps\/([^/?]+)/)
  if (method === 'POST' && sopStepMatch) {
    const taskId = decodeURIComponent(sopStepMatch[1])
    const stepId = decodeURIComponent(sopStepMatch[2])
    const body = normalizeObject(init.body)
    if (taskId !== task.id) {
      throw new Error('开发任务不存在')
    }
    const now = new Date().toISOString()
    const nextTask = setDevSpecialistTask({
      ...task,
      updatedAt: now,
      sopSteps: task.sopSteps.map((step) => step.stepId === stepId
        ? {
            ...step,
            status: normalizeString(body.status || step.status),
            operatorNote: normalizeString(body.operatorNote),
            evidenceRefs: normalizeStringArray(body.evidenceRefs),
            updatedAt: now,
          }
        : step),
    })
    return normalizeMutationResponse({ task: nextTask }) as T
  }
  const evidenceMatch = path.match(/^\/api\/maka\/specialist\/tasks\/([^/]+)\/evidence$/)
  if (method === 'POST' && evidenceMatch) {
    const taskId = decodeURIComponent(evidenceMatch[1])
    if (taskId !== task.id) {
      throw new Error('开发任务不存在')
    }
    const now = new Date().toISOString()
    const body = normalizeObject(init.body)
    const evidencePayload = normalizeObject(body.payload)
    const evidenceId = `dev-evidence-${devEvidenceSequence + 1}`
    devEvidenceSequence += 1
    const nextTask = setDevSpecialistTask({
      ...task,
      updatedAt: now,
      sopSteps: task.sopSteps.map((step) => step.stepId === normalizeString(body.stepId)
        ? {
            ...step,
            evidenceRefs: [...new Set([...step.evidenceRefs, evidenceId])],
            updatedAt: now,
          }
        : step),
      evidenceRecords: [
        ...task.evidenceRecords,
        {
          id: evidenceId,
          taskId: task.id,
          stepId: normalizeNullableString(body.stepId),
          evidenceType: normalizeString(body.evidenceType || 'text_note'),
          payload: evidencePayload,
          submittedBy: 'dev',
          createdAt: now,
        },
      ],
    })
    return normalizeMutationResponse({ task: nextTask, evidenceId }) as T
  }
  const submitMatch = path.match(/^\/api\/maka\/specialist\/tasks\/([^/]+)\/submit$/)
  if (method === 'POST' && submitMatch) {
    const taskId = decodeURIComponent(submitMatch[1])
    if (taskId !== task.id) {
      throw new Error('开发任务不存在')
    }
    const now = new Date().toISOString()
    const nextTask = setDevSpecialistTask({
      ...task,
      status: 'submitted_pending_validation',
      updatedAt: now,
    })
    devEvidenceSequence += 1
    return normalizeMutationResponse({ task: nextTask, evidenceId: `dev-evidence-${devEvidenceSequence}` }) as T
  }
  const appealMatch = path.match(/^\/api\/maka\/specialist\/tasks\/([^/]+)\/appeal$/)
  if (method === 'POST' && appealMatch) {
    const taskId = decodeURIComponent(appealMatch[1])
    if (taskId !== task.id) {
      throw new Error('开发任务不存在')
    }
    const now = new Date().toISOString()
    const nextTask = setDevSpecialistTask({
      ...task,
      status: 'appeal_in_review',
      updatedAt: now,
    })
    return normalizeMutationResponse({ task: nextTask, appealId: `dev-appeal-${Date.now()}` }) as T
  }
  const blockMatch = path.match(/^\/api\/maka\/specialist\/tasks\/([^/]+)\/block$/)
  if (method === 'POST' && blockMatch) {
    const taskId = decodeURIComponent(blockMatch[1])
    if (taskId !== task.id) {
      throw new Error('开发任务不存在')
    }
    const now = new Date().toISOString()
    const body = normalizeObject(init.body)
    const nextTask = setDevSpecialistTask({
      ...task,
      status: 'appeal_in_review',
      metadata: {
        ...task.metadata,
        makaBlockReason: {
          reasonCode: normalizeString(body.reasonCode),
          reasonText: normalizeString(body.reasonText),
          blockedBy: 'dev',
          blockedAt: now,
        },
      },
      updatedAt: now,
    })
    return normalizeMutationResponse({ task: nextTask, appealId: `dev-appeal-${Date.now()}` }) as T
  }
  return normalizeMutationResponse({ task }) as T
}
