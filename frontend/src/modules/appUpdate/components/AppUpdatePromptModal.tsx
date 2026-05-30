import { AlertCircle, Download, RotateCcw } from 'lucide-react'
import { Alert, Button, Modal } from '../../../shared/components'
import { useAppUpdateStore } from '../../../store/appUpdateStore'

export function AppUpdatePromptModal() {
  const open = useAppUpdateStore((state) => state.promptOpen)
  const update = useAppUpdateStore((state) => state.state)
  const applying = useAppUpdateStore((state) => state.applying)
  const error = useAppUpdateStore((state) => state.error)
  const applyNow = useAppUpdateStore((state) => state.applyNow)
  const dismiss = useAppUpdateStore((state) => state.dismiss)
  const clearFailure = useAppUpdateStore((state) => state.clearFailure)

  if (!open) return null

  const required = update.kind === 'required'
  const unsupported = update.kind === 'unsupported_install'
  const failed = update.kind === 'failed' || update.status === 'rolled_back' || update.status === 'failed_manual_repair'
  const title = unsupported
    ? '当前安装位置不支持自动更新'
    : failed
      ? '应用更新未完成'
      : required
        ? '需要先升级客户端'
        : '检测到客户端更新'

  const primaryLabel =
    update.status === 'downloading' || update.status === 'staged'
      ? '正在准备更新'
      : required
        ? '更新并重启'
        : '现在更新'

  return (
    <Modal
      open={open}
      onClose={() => {
        if (!required && !unsupported && !applying) dismiss()
      }}
      title={title}
      width="520px"
      closable={!required && !unsupported && !applying}
      footer={
        <>
          {failed ? (
            <Button variant="secondary" onClick={() => void clearFailure()} disabled={applying}>
              清除失败状态
            </Button>
          ) : !required && !unsupported ? (
            <Button variant="secondary" onClick={dismiss} disabled={applying}>
              稍后再说
            </Button>
          ) : null}
          {!unsupported && !failed ? (
            <Button onClick={() => void applyNow()} loading={applying}>
              {primaryLabel}
            </Button>
          ) : null}
        </>
      }
    >
      <div className="space-y-4">
        <div
          className={`rounded-2xl border px-4 py-4 ${
            unsupported || failed
              ? 'border-amber-200 bg-amber-50 text-amber-900'
              : required
                ? 'border-rose-200 bg-rose-50 text-rose-900'
                : 'border-sky-200 bg-sky-50 text-sky-900'
          }`}
        >
          <div className="flex items-start gap-3">
            <div className="mt-0.5 rounded-xl bg-white/80 p-2 shadow-sm">
              {failed ? <RotateCcw className="h-5 w-5" /> : unsupported ? <AlertCircle className="h-5 w-5" /> : <Download className="h-5 w-5" />}
            </div>
            <div className="space-y-1">
              <div className="text-sm font-semibold">
                {unsupported
                  ? '当前安装目录不可写，自动替换已暂停。'
                  : failed
                    ? '上一次客户端更新没有成功完成。'
                    : required
                      ? '当前客户端版本低于最低要求。'
                      : '发现可安装的客户端版本。'}
              </div>
              <p className="text-sm leading-6 text-current/80">
                {unsupported
                  ? '请使用最新安装包迁移到用户目录安装版本，之后客户端就能自动完成 bugfix 更新。'
                  : failed
                    ? update.errorMessage || '如果已经回滚，当前仍可使用旧版本；请导出诊断后再重试。'
                    : '更新包会先下载、校验并解包到 staging，然后关闭应用并由 runner 完成替换。'}
              </p>
            </div>
          </div>
        </div>

        {error ? <Alert type="warning" title="更新处理未完成" message={error} /> : null}

        <div className="grid gap-3 rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
          <div className="flex items-center justify-between gap-3 text-sm">
            <span className="text-[var(--color-text-secondary)]">当前版本</span>
            <span className="font-medium text-[var(--color-text-primary)]">{update.localAppVersion || '未知'}</span>
          </div>
          <div className="flex items-center justify-between gap-3 text-sm">
            <span className="text-[var(--color-text-secondary)]">目标版本</span>
            <span className="font-medium text-[var(--color-text-primary)]">{update.remoteAppVersion || '未知'}</span>
          </div>
          {update.status ? (
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="text-[var(--color-text-secondary)]">状态</span>
              <span className="font-medium text-[var(--color-text-primary)]">{update.status}</span>
            </div>
          ) : null}
          {update.manifestSource ? (
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="text-[var(--color-text-secondary)]">来源</span>
              <span className="font-medium text-[var(--color-text-primary)]">{update.manifestSource}</span>
            </div>
          ) : null}
          {update.manifestUrl ? (
            <div className="grid gap-1 text-sm">
              <span className="text-[var(--color-text-secondary)]">清单地址</span>
              <span className="break-all font-medium text-[var(--color-text-primary)]">{update.manifestUrl}</span>
            </div>
          ) : null}
          {update.errorCode ? (
            <div className="grid gap-1 text-sm">
              <span className="text-[var(--color-text-secondary)]">错误码</span>
              <span className="break-all font-medium text-[var(--color-text-primary)]">{update.errorCode}</span>
            </div>
          ) : null}
        </div>
      </div>
    </Modal>
  )
}
