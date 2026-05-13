import { HardDrive, ShieldCheck } from 'lucide-react'
import { toast } from '../../../shared/components'
import { EnvironmentStatusCard } from '../components/EnvironmentStatusCard'
import { UpdatePromptModal } from '../components/UpdatePromptModal'
import { useRuntimeStore } from '../../../store/runtimeStore'

export function EnvironmentGatePage() {
  const status = useRuntimeStore((state) => state.status)
  const checking = useRuntimeStore((state) => state.checking)
  const repairing = useRuntimeStore((state) => state.repairing)
  const exporting = useRuntimeStore((state) => state.exporting)
  const diagnosticsPath = useRuntimeStore((state) => state.diagnosticsPath)
  const updateState = useRuntimeStore((state) => state.updateState)
  const updatePromptOpen = useRuntimeStore((state) => state.updatePromptOpen)
  const updateError = useRuntimeStore((state) => state.updateError)
  const retryCheck = useRuntimeStore((state) => state.retryCheck)
  const repairNow = useRuntimeStore((state) => state.repairNow)
  const exportDiagnostics = useRuntimeStore((state) => state.exportDiagnostics)

  const updateBlocking = updatePromptOpen && updateState?.kind === 'required'

  const handleExport = async () => {
    try {
      const path = await exportDiagnostics()
      toast.success(path ? `诊断包已导出到 ${path}` : '诊断包已导出')
    } catch (error: any) {
      toast.error(error?.message || '导出诊断失败')
    }
  }

  const handleRepair = async () => {
    try {
      await repairNow()
    } catch (error: any) {
      toast.error(error?.message || '自动修复失败')
    }
  }

  const handleRetry = async () => {
    try {
      await retryCheck()
    } catch (error: any) {
      toast.error(error?.message || '重新检查失败')
    }
  }

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.14),_transparent_32%),radial-gradient(circle_at_bottom_right,_rgba(251,191,36,0.16),_transparent_28%),linear-gradient(180deg,_#f8fafc_0%,_#eef4ff_100%)] px-6 py-10">
      <div className="absolute left-[-120px] top-20 h-64 w-64 rounded-full bg-sky-200/30 blur-3xl" />
      <div className="absolute bottom-[-120px] right-[-80px] h-72 w-72 rounded-full bg-amber-200/30 blur-3xl" />

      <div className="relative mx-auto flex min-h-[calc(100vh-5rem)] max-w-6xl flex-col justify-center gap-8">
        <div className="max-w-3xl">
          <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/80 bg-white/80 px-4 py-2 text-xs font-semibold uppercase tracking-[0.24em] text-slate-500 shadow-sm">
            <ShieldCheck className="h-4 w-4" />
            Environment Bootstrap Gate
          </div>
          <h1 className="text-4xl font-semibold tracking-tight text-slate-900 md:text-5xl">
            先把桌面运行环境站稳，再放你进入工作台。
          </h1>
          <p className="mt-4 max-w-2xl text-base leading-7 text-slate-600">
            这里会统一检查运行时版本、当前指针、workspace host 连通性，以及启动更新状态。环境未就绪时，不再让你带着半坏壳子继续用。
          </p>
        </div>

        <div className="grid gap-6 xl:grid-cols-[minmax(0,1.15fr)_320px]">
          <EnvironmentStatusCard
            status={status}
            checking={checking}
            repairing={repairing}
            exporting={exporting}
            diagnosticsPath={diagnosticsPath}
            updateBlocking={updateBlocking}
            updateError={updateError}
            onRetry={() => void handleRetry()}
            onRepair={() => void handleRepair()}
            onExport={() => void handleExport()}
          />

          <div className="space-y-4">
            <div className="rounded-[28px] border border-white/80 bg-slate-950 px-5 py-6 text-white shadow-[0_24px_60px_rgba(15,23,42,0.18)]">
              <div className="flex items-center gap-3">
                <div className="rounded-2xl bg-white/10 p-2">
                  <HardDrive className="h-5 w-5" />
                </div>
                <div>
                  <div className="text-sm font-semibold">启动顺序</div>
                  <div className="text-xs text-white/60">环境先于登录，登录先于业务页</div>
                </div>
              </div>
              <div className="mt-5 space-y-3 text-sm text-white/75">
                <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
                  1. 检查本地运行时 manifest、active pointer 与 workspace host 连通性。
                </div>
                <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
                  2. 遇到可修复问题先自动修，再跑完整复检。
                </div>
                <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
                  3. 环境通过后再决定是否放行登录与主工作台。
                </div>
              </div>
            </div>

            <div className="rounded-[28px] border border-white/80 bg-white/90 px-5 py-6 shadow-[0_18px_40px_rgba(15,23,42,0.08)]">
              <div className="text-sm font-semibold text-slate-900">导出给支持团队时会包含什么</div>
              <div className="mt-3 space-y-2 text-sm leading-6 text-slate-600">
                <p>最近一次环境检查结果、错误码、关键目录状态、以及脱敏后的运行日志。</p>
                <p>访问令牌、代理密码、Cookie、Session 类字段会统一替换成 `[REDACTED]`。</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <UpdatePromptModal />
    </div>
  )
}
