import { AlertCircle, CheckCircle2, CircleDot } from 'lucide-react'
import type { RunEvent } from '../types'

function iconForStage(stage: string) {
  if (stage === 'succeeded' || stage === 'completed') return <CheckCircle2 className="h-4 w-4 text-[var(--color-success)]" />
  if (stage === 'failed' || stage === 'expired') return <AlertCircle className="h-4 w-4 text-[var(--color-error)]" />
  return <CircleDot className="h-4 w-4 text-[var(--color-info)]" />
}

export function RunTimeline({ events }: { events: RunEvent[] }) {
  if (events.length === 0) {
    return <div className="py-6 text-center text-sm text-[var(--color-text-muted)]">暂无运行事件</div>
  }

  return (
    <div className="space-y-3">
      {events.map((event) => (
        <div
          key={event.eventId || `${event.stage}-${event.createdAt}`}
          className="flex gap-3 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-3"
        >
          <div className="mt-0.5 shrink-0">{iconForStage(event.stage)}</div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center justify-between gap-3">
              <span className="truncate text-sm font-medium text-[var(--color-text-primary)]">{event.stage || 'event'}</span>
              <span className="shrink-0 text-xs text-[var(--color-text-muted)]">{event.createdAt || '-'}</span>
            </div>
            <p className="mt-1 break-words text-sm text-[var(--color-text-secondary)]">{event.message || '-'}</p>
          </div>
        </div>
      ))}
    </div>
  )
}
