import { Rocket, ShieldAlert } from 'lucide-react'
import { Alert, Button, Modal } from '../../../shared/components'
import { useRuntimeStore } from '../../../store/runtimeStore'

export function UpdatePromptModal() {
  const updateState = useRuntimeStore((state) => state.updateState)
  const open = useRuntimeStore((state) => state.updatePromptOpen)
  const updating = useRuntimeStore((state) => state.updating)
  const updateError = useRuntimeStore((state) => state.updateError)
  const dismiss = useRuntimeStore((state) => state.dismissUpdatePrompt)
  const confirmUpdate = useRuntimeStore((state) => state.confirmUpdate)

  if (!updateState) {
    return null
  }

  const required = updateState.kind === 'required'

  return (
    <Modal
      open={open}
      onClose={() => {
        if (!required) {
          dismiss()
        }
      }}
      title={required ? '需要先完成运行时更新' : '检测到可用更新'}
      width="520px"
      closable={!required && !updating}
      footer={
        <>
          {!required ? (
            <Button variant="secondary" onClick={dismiss} disabled={updating}>
              稍后再说
            </Button>
          ) : null}
          <Button onClick={() => void confirmUpdate()} loading={updating}>
            {required ? '立即更新并继续' : '现在更新'}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className={`rounded-2xl border px-4 py-4 ${
          required
            ? 'border-amber-200 bg-amber-50 text-amber-900'
            : 'border-sky-200 bg-sky-50 text-sky-900'
        }`}>
          <div className="flex items-start gap-3">
            <div className="mt-0.5 rounded-xl bg-white/80 p-2 shadow-sm">
              {required ? <ShieldAlert className="h-5 w-5" /> : <Rocket className="h-5 w-5" />}
            </div>
            <div className="space-y-1">
              <div className="text-sm font-semibold">
                {required ? '当前版本需要先升级运行时资源。' : '发现一个可手动确认的启动更新。'}
              </div>
              <p className="text-sm leading-6 text-current/80">
                {required
                  ? '在升级完成前，应用不会放行进入正常工作台，避免使用旧运行时造成半可用状态。'
                  : '这是一个软更新。你可以现在更新，也可以先继续进入应用，稍后再处理。'}
              </p>
            </div>
          </div>
        </div>

        {updateError ? (
          <Alert
            type="warning"
            title={required ? '更新未完成，仍需先升级' : '更新未完成'}
            message={updateError}
          />
        ) : null}

        <div className="grid gap-3 rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
          <div className="flex items-center justify-between gap-3 text-sm">
            <span className="text-[var(--color-text-secondary)]">当前应用版本</span>
            <span className="font-medium text-[var(--color-text-primary)]">{updateState.localAppVersion || '未知'}</span>
          </div>
          <div className="flex items-center justify-between gap-3 text-sm">
            <span className="text-[var(--color-text-secondary)]">远端应用版本</span>
            <span className="font-medium text-[var(--color-text-primary)]">{updateState.remoteAppVersion || '未知'}</span>
          </div>
          <div className="flex items-center justify-between gap-3 text-sm">
            <span className="text-[var(--color-text-secondary)]">目标运行时资源</span>
            <span className="font-medium text-[var(--color-text-primary)]">{updateState.resourceVersion || '未知'}</span>
          </div>
          {updateState.manifestSource ? (
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="text-[var(--color-text-secondary)]">更新清单来源</span>
              <span className="font-medium text-[var(--color-text-primary)]">{updateState.manifestSource}</span>
            </div>
          ) : null}
          {updateState.manifestUrl ? (
            <div className="grid gap-1 text-sm">
              <span className="text-[var(--color-text-secondary)]">更新清单地址</span>
              <span className="break-all font-medium text-[var(--color-text-primary)]">{updateState.manifestUrl}</span>
            </div>
          ) : null}
        </div>
      </div>
    </Modal>
  )
}
