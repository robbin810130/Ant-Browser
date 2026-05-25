import { Badge, Button, Modal } from '../../../shared/components'
import {
  isTerminalSharedLoginStatus,
  sharedLoginActionLabel,
  type SharedLoginDialogState,
} from '../sharedLoginSession'

export function SharedLoginSessionModal({
  dialog,
  onClose,
}: {
  dialog: SharedLoginDialogState | null
  onClose: () => void
}) {
  const terminal = Boolean(dialog?.session && isTerminalSharedLoginStatus(dialog.session.status))

  return (
    <Modal
      open={Boolean(dialog)}
      onClose={() => {
        if (terminal) onClose()
      }}
      title={dialog ? `${sharedLoginActionLabel(dialog.action)} · ${dialog.shopName}` : undefined}
      width="560px"
      closable={terminal}
      footer={terminal ? (
        <Button onClick={onClose}>知道了</Button>
      ) : (
        <Button variant="secondary" loading disabled>
          处理中
        </Button>
      )}
    >
      {!dialog || dialog.starting || !dialog.session ? (
        <div className="space-y-3">
          <p className="text-sm text-[var(--color-text-secondary)]">
            正在发起{dialog ? sharedLoginActionLabel(dialog.action) : '操作'}，请稍候。
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-4 py-3">
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-sm font-medium text-[var(--color-text-primary)]">
                  当前状态：{dialog.session.statusLabel || dialog.session.status || '-'}
                </div>
                <div className="mt-1 text-xs text-[var(--color-text-muted)]">
                  会话 ID：{dialog.session.bindSessionId || '-'}
                </div>
              </div>
              <Badge variant={dialog.session.status === 'completed' || dialog.session.status === 'succeeded' ? 'success' : dialog.session.status === 'failed' || dialog.session.status === 'expired' ? 'warning' : 'default'}>
                {dialog.session.sessionType || dialog.action}
              </Badge>
            </div>
          </div>

          <div className="space-y-2 text-sm text-[var(--color-text-secondary)]">
            <p>{dialog.session.message || `${sharedLoginActionLabel(dialog.action)}已发起`}</p>
            {!isTerminalSharedLoginStatus(dialog.session.status) ? (
              <p>如已弹出受控浏览器，请在窗口内完成登录、验证或挑战处理；本弹层会自动刷新状态。</p>
            ) : null}
            {dialog.session.manualActionRequired ? (
              <p className="text-[var(--color-warning)]">当前需要人工完成登录或挑战步骤。</p>
            ) : null}
          </div>

          <div className="grid grid-cols-1 gap-3 rounded-lg border border-[var(--color-border-default)] p-4 md:grid-cols-2">
            <div>
              <div className="text-xs font-medium text-[var(--color-text-muted)]">最后观察 URL</div>
              <div className="mt-1 break-all text-sm text-[var(--color-text-primary)]">
                {dialog.session.lastObservedUrl || '-'}
              </div>
            </div>
            <div>
              <div className="text-xs font-medium text-[var(--color-text-muted)]">挑战类型</div>
              <div className="mt-1 text-sm text-[var(--color-text-primary)]">
                {dialog.session.challengeType || '-'}
              </div>
            </div>
            <div>
              <div className="text-xs font-medium text-[var(--color-text-muted)]">开始时间</div>
              <div className="mt-1 text-sm text-[var(--color-text-primary)]">
                {dialog.session.startedAt || '-'}
              </div>
            </div>
            <div>
              <div className="text-xs font-medium text-[var(--color-text-muted)]">最近更新时间</div>
              <div className="mt-1 text-sm text-[var(--color-text-primary)]">
                {dialog.session.updatedAt || '-'}
              </div>
            </div>
          </div>

          {dialog.errorMessage ? (
            <div className="rounded-lg border border-[var(--color-error)]/30 bg-[var(--color-error)]/10 px-4 py-3 text-sm text-[var(--color-error)]">
              {dialog.errorMessage}
            </div>
          ) : null}
        </div>
      )}
    </Modal>
  )
}
