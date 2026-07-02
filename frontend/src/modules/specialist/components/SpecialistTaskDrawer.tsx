import { ChangeEvent, useEffect, useMemo, useState } from 'react'
import clsx from 'clsx'
import { CheckCircle2, FileImage, Loader2 } from 'lucide-react'
import {
  Alert,
  Badge,
  Button,
  Card,
  Drawer,
  FormItem,
  Input,
  Progress,
  Select,
  Textarea,
  toast,
} from '../../../shared/components'
import {
  appealSpecialistTask,
  blockSpecialistTask,
  submitSpecialistTask,
  submitSpecialistTaskEvidence,
  updateSpecialistTaskSopStep,
} from '../api'
import {
  evidenceTypeLabel,
  requiredStepSummary,
  specialistTaskPriorityLabel,
  specialistTaskStatusLabel,
} from '../presentation'
import type { SpecialistTaskRecord, SpecialistTaskSopStep } from '../types'

const maxScreenshotSourceBytes = 8 * 1024 * 1024
const maxScreenshotPayloadBytes = 800 * 1024
const maxScreenshotDimension = 1600

type ActionMode = 'submit' | 'appeal' | 'block'

interface ScreenshotEvidence {
  fileName: string
  mimeType: string
  size: number
  dataUrl: string
}

interface EvidenceFeedback {
  typeLabel: string
  summary: string
  submittedAt: string
}

interface SpecialistTaskDrawerProps {
  open: boolean
  task: SpecialistTaskRecord | null
  loading?: boolean
  onClose: () => void
  onTaskUpdated: (task: SpecialistTaskRecord) => void
  onRefreshTask: (taskId: string) => Promise<SpecialistTaskRecord | null>
  onReload: () => void
}

function formatTime(value: string | null | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function firstEvidenceType(step: SpecialistTaskSopStep | undefined) {
  return step?.evidenceTypes?.[0] || 'text_note'
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(new Error('截图读取失败'))
    reader.readAsDataURL(file)
  })
}

function loadImage(dataUrl: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('截图格式无法识别'))
    image.src = dataUrl
  })
}

function dataUrlByteSize(dataUrl: string) {
  const content = dataUrl.split(',', 2)[1] || ''
  return Math.ceil(content.length * 0.75)
}

async function prepareScreenshotEvidence(file: File): Promise<ScreenshotEvidence> {
  if (!file.type.startsWith('image/')) {
    throw new Error('请选择 PNG、JPG 或 WebP 图片')
  }
  if (file.size > maxScreenshotSourceBytes) {
    throw new Error('原始截图不能超过 8 MB')
  }

  const source = await readFileAsDataUrl(file)
  const image = await loadImage(source)
  const scale = Math.min(1, maxScreenshotDimension / Math.max(image.width, image.height))
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.round(image.width * scale))
  canvas.height = Math.max(1, Math.round(image.height * scale))
  const context = canvas.getContext('2d')
  if (!context) {
    throw new Error('当前环境无法处理截图')
  }
  context.fillStyle = '#ffffff'
  context.fillRect(0, 0, canvas.width, canvas.height)
  context.drawImage(image, 0, 0, canvas.width, canvas.height)
  const dataUrl = canvas.toDataURL('image/jpeg', 0.72)
  const size = dataUrlByteSize(dataUrl)
  if (size > maxScreenshotPayloadBytes) {
    throw new Error('截图压缩后仍超过 800 KB，请裁剪后重新选择')
  }
  return {
    fileName: file.name.replace(/\.[^.]+$/, '') + '.jpg',
    mimeType: 'image/jpeg',
    size,
    dataUrl,
  }
}

function isHttpUrl(value: string) {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

function evidenceRecordSummary(payload: Record<string, unknown>) {
  const fileName = String(payload.fileName || '').trim()
  if (fileName) return fileName
  const url = String(payload.url || '').trim()
  if (url) return url
  const text = String(payload.text || payload.note || '').trim()
  return text || '已提交'
}

function taskFieldRows(task: SpecialistTaskRecord) {
  return [
    ['店铺', task.shopName || task.shopId || '-'],
    ['优先级', specialistTaskPriorityLabel(task.priority)],
    ['截止时间', formatTime(task.deadlineAt)],
    ['负责人', task.assigneeName || task.assigneeUserId || '-'],
    ['任务编号', task.id],
    ['异常类型', task.anomalySignalType || '-'],
  ] as const
}

export function SpecialistTaskDrawer({
  open,
  task,
  loading = false,
  onClose,
  onTaskUpdated,
  onRefreshTask,
  onReload,
}: SpecialistTaskDrawerProps) {
  const [savingAction, setSavingAction] = useState('')
  const [operatorNote, setOperatorNote] = useState('')
  const [evidenceStepId, setEvidenceStepId] = useState('')
  const [evidenceType, setEvidenceType] = useState('text_note')
  const [evidenceText, setEvidenceText] = useState('')
  const [screenshotEvidence, setScreenshotEvidence] = useState<ScreenshotEvidence | null>(null)
  const [screenshotError, setScreenshotError] = useState('')
  const [lastEvidenceFeedback, setLastEvidenceFeedback] = useState<EvidenceFeedback | null>(null)
  const [submitSummary, setSubmitSummary] = useState('')
  const [appealReason, setAppealReason] = useState('')
  const [blockReasonCode, setBlockReasonCode] = useState('backend_unavailable')
  const [blockReasonText, setBlockReasonText] = useState('')
  const [actionMode, setActionMode] = useState<ActionMode>('submit')

  const steps = task?.sopSteps ?? []
  const requiredSteps = steps.filter((step) => step.required)
  const requiredDoneCount = requiredSteps.filter((step) => step.status === 'done').length
  const requiredProgressPercent = requiredSteps.length
    ? Math.round((requiredDoneCount / requiredSteps.length) * 100)
    : 100
  const requiredIncomplete = steps.some((step) => step.required && step.status !== 'done')
  const selectedStep = useMemo(
    () => steps.find((step) => step.stepId === evidenceStepId) ?? steps[0],
    [evidenceStepId, steps],
  )

  useEffect(() => {
    if (!task) return
    const firstStep = task.sopSteps[0]
    setOperatorNote('')
    setEvidenceStepId(firstStep?.stepId || '')
    setEvidenceType(firstEvidenceType(firstStep))
    setEvidenceText('')
    setScreenshotEvidence(null)
    setScreenshotError('')
    setLastEvidenceFeedback(null)
    setSubmitSummary('')
    setAppealReason('')
    setBlockReasonCode('backend_unavailable')
    setBlockReasonText('')
    setActionMode('submit')
  }, [task?.id])

  useEffect(() => {
    setEvidenceType(firstEvidenceType(selectedStep))
    setScreenshotEvidence(null)
    setScreenshotError('')
    setEvidenceText('')
  }, [selectedStep?.stepId])

  async function runAction(
    actionName: string,
    action: () => Promise<SpecialistTaskRecord>,
    successMessage: string,
    refreshFailureMessage = '任务状态已更新，但详情刷新失败，请手动刷新',
  ) {
    setSavingAction(actionName)
    try {
      const nextTask = await action()
      onTaskUpdated(nextTask)
      toast.success(successMessage)
      void onRefreshTask(nextTask.id).catch(() => {
        toast.warning(refreshFailureMessage)
      })
      onReload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '专员任务操作失败，请稍后重试')
    } finally {
      setSavingAction('')
    }
  }

  const evidenceOptions = (selectedStep?.evidenceTypes?.length
    ? selectedStep.evidenceTypes
    : ['text_note']).map((type) => ({ value: type, label: evidenceTypeLabel(type) }))
  const screenshotSelected = evidenceType === 'screenshot'
  const linkSelected = evidenceType === 'backend_url' || evidenceType === 'product_url'
  const evidenceLinkError = linkSelected && evidenceText.trim() && !isHttpUrl(evidenceText.trim())
    ? '请输入 http 或 https 链接'
    : ''
  const evidenceReady = screenshotSelected
    ? Boolean(screenshotEvidence) && !screenshotError
    : linkSelected
      ? isHttpUrl(evidenceText.trim()) && !evidenceLinkError
      : Boolean(evidenceText.trim())
  const busy = Boolean(savingAction)
  const actionHelp = actionMode === 'submit'
    ? '完成必填 SOP 后提交处理结果，服务端会再次校验任务状态。'
    : actionMode === 'appeal'
      ? '用于任务不适用、平台统计延迟等情况，提交后进入主管复核。'
      : '用于 1688 后台不可用、权限缺失、数据缺失等现场阻塞，提交后等待主管处理。'

  function buildEvidencePayload(): Record<string, unknown> {
    if (screenshotSelected && screenshotEvidence) {
      return { ...screenshotEvidence, note: evidenceText.trim() }
    }
    if (linkSelected) {
      return { url: evidenceText.trim() }
    }
    return { text: evidenceText.trim() }
  }

  async function handleScreenshotChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    try {
      setScreenshotError('')
      setScreenshotEvidence(await prepareScreenshotEvidence(file))
    } catch (error) {
      const message = error instanceof Error ? error.message : '截图处理失败，请重新选择截图'
      setScreenshotEvidence(null)
      setScreenshotError(message)
      toast.error(message)
    }
  }

  function renderFooterAction() {
    if (!task) return null
    if (actionMode === 'appeal') {
      return (
        <Button
          variant="secondary"
          size="sm"
          disabled={!appealReason.trim() || busy}
          loading={savingAction === 'appeal'}
          onClick={() => void runAction(
            'appeal',
            async () => (await appealSpecialistTask(task.id, { reason: appealReason.trim() })).task,
            '申诉已提交',
          )}
        >
          提交申诉
        </Button>
      )
    }
    if (actionMode === 'block') {
      return (
        <Button
          variant="danger"
          size="sm"
          disabled={!blockReasonText.trim() || busy}
          loading={savingAction === 'block'}
          onClick={() => void runAction(
            'block',
            async () => (await blockSpecialistTask(task.id, {
              reasonCode: blockReasonCode,
              reasonText: blockReasonText.trim(),
            })).task,
            '阻塞原因已提交',
          )}
        >
          提交无法处理
        </Button>
      )
    }
    return (
      <Button
        size="sm"
        disabled={requiredIncomplete || busy}
        loading={savingAction === 'submit'}
        title={requiredIncomplete ? '完成全部必填 SOP 后才能提交' : '提交处理结果'}
        onClick={() => void runAction(
          'submit',
          async () => (await submitSpecialistTask(task.id, { summary: submitSummary.trim() })).task,
          '任务已提交待验收',
        )}
      >
        提交处理结果
      </Button>
    )
  }

  return (
    <Drawer
      open={open}
      width="920px"
      title={task?.title || '任务详情'}
      subtitle={task ? specialistTaskStatusLabel(task.status) : undefined}
      onClose={onClose}
      footer={task ? (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-sm text-[var(--color-text-muted)]">
            必填 SOP：{requiredStepSummary(steps)}，当前动作：{actionMode === 'submit' ? '提交结果' : actionMode === 'appeal' ? '申诉' : '无法处理'}
          </span>
          <div className="flex justify-end">{renderFooterAction()}</div>
        </div>
      ) : null}
    >
      {loading && !task ? (
        <div className="flex items-center gap-2 py-10 text-sm text-[var(--color-text-muted)]">
          <Loader2 className="h-4 w-4 animate-spin" />
          正在读取任务详情...
        </div>
      ) : null}
      {!loading && !task ? (
        <div className="py-12 text-center text-sm text-[var(--color-text-muted)]">请选择任务</div>
      ) : null}
      {task ? (
        <div className="space-y-4">
          <div className="grid grid-cols-1 overflow-hidden rounded-xl border border-[var(--color-border-default)] sm:grid-cols-2 lg:grid-cols-3">
            {taskFieldRows(task).map(([label, value]) => (
              <div key={label} className="border-b border-r border-[var(--color-border-muted)] p-3 last:border-r-0">
                <div className="text-xs text-[var(--color-text-muted)]">{label}</div>
                <div className="mt-1 truncate text-sm font-medium text-[var(--color-text-primary)]">{value}</div>
              </div>
            ))}
          </div>

          {task.description ? (
            <Alert type="info" title="任务说明" message={task.description} />
          ) : null}

          <Card
            title="SOP 执行"
            actions={(
              <div className="w-40">
                <div className="mb-1 flex justify-between text-xs text-[var(--color-text-muted)]">
                  <span>必填 {requiredStepSummary(steps)}</span>
                  <span>{requiredDoneCount}/{requiredSteps.length || 0}</span>
                </div>
                <Progress percent={requiredProgressPercent} showInfo={false} size="sm" />
              </div>
            )}
            padding="sm"
          >
            {steps.length ? (
              <div className="space-y-2">
                {steps.map((step, index) => {
                  const done = step.status === 'done'
                  const active = selectedStep?.stepId === step.stepId
                  return (
                    <button
                      key={step.stepId}
                      type="button"
                      className={clsx(
                        'w-full rounded-lg border p-3 text-left transition-colors',
                        active
                          ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                          : 'border-[var(--color-border-default)] bg-[var(--color-bg-surface)] hover:bg-[var(--color-bg-muted)]',
                      )}
                      onClick={() => setEvidenceStepId(step.stepId)}
                    >
                      <div className="flex items-start gap-3">
                        <div className={clsx(
                          'mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold',
                          done ? 'bg-[var(--color-success)] text-white' : 'bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)]',
                        )}>
                          {done ? <CheckCircle2 className="h-4 w-4" /> : index + 1}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-medium text-[var(--color-text-primary)]">{step.title || step.stepId}</span>
                            <Badge variant={step.required ? 'warning' : 'default'} size="sm">{step.required ? '必填' : '可选'}</Badge>
                          </div>
                          {step.description ? (
                            <p className="mt-1 text-sm text-[var(--color-text-muted)]">{step.description}</p>
                          ) : null}
                          <p className="mt-1 text-xs text-[var(--color-text-muted)]">
                            证据要求：{step.evidenceTypes.map(evidenceTypeLabel).join('、') || '未限制'}
                          </p>
                          {step.operatorNote ? (
                            <p className="mt-1 text-xs text-[var(--color-text-secondary)]">执行备注：{step.operatorNote}</p>
                          ) : null}
                          <label className="mt-2 inline-flex items-center gap-2 text-sm text-[var(--color-text-secondary)]" onClick={(event) => event.stopPropagation()}>
                            <input
                              type="checkbox"
                              checked={done}
                              disabled={busy}
                              onChange={() => void runAction(
                                `step:${step.stepId}`,
                                async () => {
                                  const result = await updateSpecialistTaskSopStep(task.id, step.stepId, {
                                    status: done ? 'not_started' : 'done',
                                    operatorNote: operatorNote.trim(),
                                    evidenceRefs: step.evidenceRefs,
                                  })
                                  return result.task
                                },
                                done ? 'SOP 步骤已改为未完成' : 'SOP 步骤已完成',
                                'SOP 状态已提交，但任务详情刷新失败，请手动刷新',
                              )}
                            />
                            {done ? '已完成，点此撤回' : '标记完成'}
                          </label>
                        </div>
                      </div>
                    </button>
                  )
                })}
              </div>
            ) : (
              <div className="py-8 text-center text-sm text-[var(--color-text-muted)]">该任务没有 SOP 步骤</div>
            )}
            <div className="mt-3 border-t border-[var(--color-border-muted)] pt-3">
              <FormItem label="本次步骤备注">
                <Input
                  value={operatorNote}
                  maxLength={500}
                  placeholder="勾选步骤时一并提交，说明现场处理情况"
                  onChange={(event) => setOperatorNote(event.target.value)}
                />
              </FormItem>
            </div>
          </Card>

          <Card title="提交证据" padding="sm">
            <div className="grid gap-3 lg:grid-cols-[1fr_180px_1.2fr]">
              <FormItem label="SOP 步骤">
                <Select
                  value={selectedStep?.stepId || ''}
                  options={steps.map((step) => ({ value: step.stepId, label: step.title || step.stepId }))}
                  onChange={(event) => setEvidenceStepId(event.target.value)}
                />
              </FormItem>
              <FormItem label="证据类型">
                <Select
                  value={evidenceType}
                  options={evidenceOptions}
                  onChange={(event) => {
                    setEvidenceType(event.target.value)
                    setEvidenceText('')
                    setScreenshotEvidence(null)
                  }}
                />
              </FormItem>
              {screenshotSelected ? (
                <FormItem label="截图附件" hint="自动压缩为 JPG，单张不超过 800 KB" error={screenshotError}>
                  <div className="space-y-2">
                    <label className="inline-flex">
                      <input
                        type="file"
                        className="sr-only"
                        accept="image/png,image/jpeg,image/webp"
                        onChange={(event) => void handleScreenshotChange(event)}
                      />
                      <span className="inline-flex h-9 cursor-pointer items-center gap-2 rounded-lg border border-[var(--color-border-default)] px-3 text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]">
                        <FileImage className="h-4 w-4" />
                        选择截图
                      </span>
                    </label>
                    <div className="text-xs text-[var(--color-text-muted)]">
                      {screenshotEvidence
                        ? `${screenshotEvidence.fileName}（${Math.ceil(screenshotEvidence.size / 1024)} KB）`
                        : '尚未选择截图'}
                    </div>
                    <Input
                      value={evidenceText}
                      maxLength={500}
                      placeholder="可补充截图说明"
                      onChange={(event) => setEvidenceText(event.target.value)}
                    />
                  </div>
                </FormItem>
              ) : (
                <FormItem label={linkSelected ? '证据链接' : '证据说明'} error={evidenceLinkError}>
                  {linkSelected ? (
                    <Input
                      value={evidenceText}
                      maxLength={2000}
                      placeholder={evidenceType === 'product_url' ? 'https://detail.1688.com/...' : 'https://work.1688.com/...'}
                      onChange={(event) => setEvidenceText(event.target.value)}
                    />
                  ) : (
                    <Textarea
                      rows={2}
                      maxLength={2000}
                      value={evidenceText}
                      placeholder="填写操作结果或说明"
                      onChange={(event) => setEvidenceText(event.target.value)}
                    />
                  )}
                </FormItem>
              )}
            </div>
            <div className="mt-3">
              <Button
                size="sm"
                disabled={!selectedStep || !evidenceReady || busy}
                loading={savingAction === 'evidence'}
                onClick={() => void runAction(
                  'evidence',
                  async () => {
                    const payload = buildEvidencePayload()
                    const result = await submitSpecialistTaskEvidence(task.id, {
                      stepId: selectedStep?.stepId || '',
                      evidenceType,
                      payload,
                    })
                    setLastEvidenceFeedback({
                      typeLabel: evidenceTypeLabel(evidenceType),
                      summary: evidenceRecordSummary(payload),
                      submittedAt: new Date().toISOString(),
                    })
                    setEvidenceText('')
                    setScreenshotEvidence(null)
                    setScreenshotError('')
                    return result.task
                  },
                  '证据已提交',
                  '证据已提交，但任务详情刷新失败，请手动刷新',
                )}
              >
                提交证据
              </Button>
            </div>
            {lastEvidenceFeedback ? (
              <Alert
                type="success"
                title="最近提交反馈"
                message={`${lastEvidenceFeedback.typeLabel}：${lastEvidenceFeedback.summary}，${formatTime(lastEvidenceFeedback.submittedAt)}`}
                className="mt-3"
              />
            ) : null}
            {task.evidenceRecords.length ? (
              <div className="mt-3 divide-y divide-[var(--color-border-muted)] rounded-lg border border-[var(--color-border-muted)]">
                {task.evidenceRecords.map((record) => (
                  <div key={record.id} className="px-3 py-2 text-sm text-[var(--color-text-secondary)]">
                    {evidenceTypeLabel(record.evidenceType)}，{evidenceRecordSummary(record.payload)}，
                    {record.stepId || '任务级'}，{formatTime(record.createdAt)}
                  </div>
                ))}
              </div>
            ) : null}
          </Card>

          <Card title="提交与异常处理" padding="sm">
            <div role="tablist" aria-label="任务提交动作" className="grid grid-cols-3 gap-2 rounded-lg bg-[var(--color-bg-muted)] p-1">
              {([
                ['submit', '提交处理结果'],
                ['appeal', '提交申诉'],
                ['block', '提交无法处理'],
              ] as const).map(([mode, label]) => (
                <button
                  key={mode}
                  type="button"
                  role="tab"
                  aria-selected={actionMode === mode}
                  className={clsx(
                    'rounded-md px-3 py-2 text-sm font-medium transition-colors',
                    actionMode === mode
                      ? 'bg-[var(--color-bg-surface)] text-[var(--color-text-primary)] shadow-sm'
                      : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]',
                  )}
                  onClick={() => setActionMode(mode)}
                >
                  {label}
                </button>
              ))}
            </div>
            <Alert type="info" message={actionHelp} className="mt-3" />
            <div className="mt-3">
              {actionMode === 'submit' ? (
                <FormItem label="处理结果摘要">
                  <Textarea
                    rows={4}
                    maxLength={2000}
                    value={submitSummary}
                    placeholder="填写本次处理结果，提交后进入验收"
                    onChange={(event) => setSubmitSummary(event.target.value)}
                  />
                </FormItem>
              ) : null}
              {actionMode === 'appeal' ? (
                <FormItem label="申诉原因">
                  <Textarea
                    rows={4}
                    maxLength={1000}
                    value={appealReason}
                    placeholder="例如平台统计延迟、任务不适用"
                    onChange={(event) => setAppealReason(event.target.value)}
                  />
                </FormItem>
              ) : null}
              {actionMode === 'block' ? (
                <div className="grid gap-3 md:grid-cols-[220px_1fr]">
                  <FormItem label="无法处理原因">
                    <Select
                      value={blockReasonCode}
                      options={[
                        { value: 'backend_unavailable', label: '1688 后台不可用' },
                        { value: 'permission_missing', label: '缺少后台权限' },
                        { value: 'data_not_found', label: '数据不存在' },
                        { value: 'other', label: '其他原因' },
                      ]}
                      onChange={(event) => setBlockReasonCode(event.target.value)}
                    />
                  </FormItem>
                  <FormItem label="原因说明">
                    <Textarea
                      rows={3}
                      maxLength={1000}
                      value={blockReasonText}
                      placeholder="写清现场情况，方便主管复核"
                      onChange={(event) => setBlockReasonText(event.target.value)}
                    />
                  </FormItem>
                </div>
              ) : null}
            </div>
            {requiredIncomplete ? (
              <p className="mt-3 text-sm text-[var(--color-warning)]">
                完成全部必填 SOP 后才能提交处理结果，服务端会再次校验。
              </p>
            ) : null}
          </Card>
        </div>
      ) : null}
    </Drawer>
  )
}
