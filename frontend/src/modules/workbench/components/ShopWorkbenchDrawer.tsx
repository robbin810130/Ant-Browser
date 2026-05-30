import { useEffect, useMemo, useState } from 'react'
import { Badge, Button, Modal } from '../../../shared/components'
import { fetchWorkspaceRunEvents, RunTimeline, type RunEvent, type RunRecord } from '../../runEvidence'
import type { WorkbenchRow } from '../types'

function statusVariant(value: boolean) {
  return value ? 'success' as const : 'warning' as const
}

function runTitle(run: RunRecord | null) {
  if (!run) return '暂无运行记录'
  return `${run.taskType || 'run'} · ${run.statusLabel || run.status || run.runId}`
}

function modalTitle(row: WorkbenchRow) {
  const title = row.shop.shopName || row.shop.shopId
  return title.length > 48 ? `${title.slice(0, 47)}...` : title
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] px-3 py-2">
      <div className="text-xs font-medium text-[var(--color-text-muted)]">{label}</div>
      <div className="mt-1 break-all text-sm text-[var(--color-text-primary)]">{value || '-'}</div>
    </div>
  )
}

export function ShopWorkbenchDrawer({
  row,
  open,
  runningAction,
  onClose,
  onAction,
}: {
  row: WorkbenchRow | null
  open: boolean
  runningAction: { shopId: string; action: WorkbenchRow['recommendedAction'] } | null
  onClose: () => void
  onAction: (row: WorkbenchRow) => void
}) {
  const [events, setEvents] = useState<RunEvent[]>([])
  const [eventsLoading, setEventsLoading] = useState(false)
  const [eventsError, setEventsError] = useState('')

  const selectedRun = useMemo(() => {
    return row?.evidence.activeRun
      || row?.evidence.latestFailure
      || row?.evidence.latestOpen
      || row?.evidence.latestValidation
      || row?.evidence.latestCredential
      || null
  }, [row])

  useEffect(() => {
    let cancelled = false

    async function loadEvents() {
      if (!open || !selectedRun?.runId) {
        setEvents([])
        setEventsError('')
        setEventsLoading(false)
        return
      }

      setEventsLoading(true)
      setEventsError('')
      try {
        const payload = await fetchWorkspaceRunEvents(selectedRun.runId, 50)
        if (!cancelled) setEvents(payload.items)
      } catch (error: any) {
        console.error('load workbench run events failed', error)
        if (!cancelled) {
          setEvents([])
          setEventsError(String(error?.message || '运行事件加载失败'))
        }
      } finally {
        if (!cancelled) setEventsLoading(false)
      }
    }

    void loadEvents()

    return () => {
      cancelled = true
    }
  }, [open, selectedRun?.runId])

  if (!row) return null

  const isRunningThisRow = runningAction?.shopId === row.shop.shopId

  return (
    <Modal open={open} onClose={onClose} title={modalTitle(row)} width="860px">
      <div className="space-y-5">
        <div className="flex flex-wrap gap-2">
          <Badge variant={row.shop.sharedLoginStatus === 'ready' ? 'success' : 'warning'}>
            {row.shop.sharedLoginStatusLabel || row.shop.sharedLoginStatus || 'unknown'}
          </Badge>
          <Badge variant={statusVariant(row.shop.coreReady)}>
            {row.shop.coreReady ? '内核就绪' : '内核不可用'}
          </Badge>
          <Badge variant={statusVariant(row.shop.profileExists)}>
            {row.shop.profileExists ? 'Profile 已映射' : 'Profile 未映射'}
          </Badge>
          {row.shop.reclaimPending ? <Badge variant="error">授权待回收</Badge> : null}
        </div>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <DetailItem label="Shop ID" value={row.shop.shopId} />
          <DetailItem label="Profile ID" value={row.shop.profileId} />
          <DetailItem label="Instance ID" value={row.shop.instanceId} />
          <DetailItem label="平台" value={row.shop.platformCode} />
        </div>

        <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0">
              <div className="text-sm font-semibold text-[var(--color-text-primary)]">推荐动作</div>
              <p className="mt-1 break-words text-sm text-[var(--color-text-secondary)]">
                {row.failureMessage || '当前店铺可按推荐动作继续处理。'}
              </p>
              {row.failureCode ? (
                <p className="mt-2 break-all text-xs text-[var(--color-text-muted)]">失败码：{row.failureCode}</p>
              ) : null}
            </div>
            <Button
              className="w-full shrink-0 whitespace-nowrap sm:w-auto"
              size="sm"
              loading={isRunningThisRow}
              disabled={Boolean(runningAction && !isRunningThisRow)}
              onClick={() => onAction(row)}
            >
              执行推荐动作
            </Button>
          </div>
        </div>

        <div>
          <div className="mb-3 flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-sm font-semibold text-[var(--color-text-primary)]">运行证据</div>
            <span className="min-w-0 truncate text-xs text-[var(--color-text-muted)]" title={runTitle(selectedRun)}>
              {runTitle(selectedRun)}
            </span>
          </div>
          {eventsLoading ? (
            <div className="py-6 text-center text-sm text-[var(--color-text-muted)]">运行事件加载中...</div>
          ) : eventsError ? (
            <div className="rounded-lg border border-[var(--color-warning)]/30 bg-[var(--color-warning)]/10 px-4 py-3 text-sm text-[var(--color-warning)]">
              {eventsError}
            </div>
          ) : (
            <RunTimeline events={events} />
          )}
        </div>
      </div>
    </Modal>
  )
}
