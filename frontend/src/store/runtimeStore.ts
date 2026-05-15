import { create } from 'zustand'
import {
  applyDesktopReleaseUpdate,
  checkDesktopReleaseUpdate,
  exportDesktopEnvironmentDiagnostics,
  getDesktopEnvironmentStatus,
  repairDesktopEnvironment,
} from '../modules/runtime/api'
import type { EnvironmentStatus, ReleaseUpdateState } from '../modules/runtime/types'

const checkingStatus: EnvironmentStatus = {
  state: 'checking',
  items: [],
}

interface RuntimeStoreState {
  status: EnvironmentStatus
  bootstrapped: boolean
  environmentReady: boolean
  checking: boolean
  repairing: boolean
  updating: boolean
  exporting: boolean
  updateState: ReleaseUpdateState | null
  updatePromptOpen: boolean
  updateError: string
  diagnosticsPath: string
  diagnosticsError: string
  bootstrap: () => Promise<void>
  retryCheck: () => Promise<void>
  repairNow: () => Promise<void>
  confirmUpdate: () => Promise<void>
  dismissUpdatePrompt: () => void
  exportDiagnostics: () => Promise<string>
}

async function evaluateRuntime(set: (partial: Partial<RuntimeStoreState>) => void, repair = false) {
  set({
    status: checkingStatus,
    bootstrapped: false,
    environmentReady: false,
    checking: !repair,
    repairing: repair,
    updateError: '',
    diagnosticsError: '',
  })

  const status = repair ? await repairDesktopEnvironment() : await getDesktopEnvironmentStatus()
  if (status.state !== 'pass') {
    set({
      status,
      bootstrapped: true,
      environmentReady: false,
      checking: false,
      repairing: false,
      updateState: null,
      updatePromptOpen: false,
    })
    return
  }

  try {
    const updateState = await checkDesktopReleaseUpdate()
    const requiresUpdate = updateState.kind === 'required'
    const shouldPrompt = updateState.kind === 'soft' || updateState.kind === 'required'

    set({
      status,
      bootstrapped: true,
      environmentReady: !requiresUpdate,
      checking: false,
      repairing: false,
      updateState: shouldPrompt ? updateState : null,
      updatePromptOpen: shouldPrompt,
      updateError: '',
    })
  } catch (error) {
    set({
      status,
      bootstrapped: true,
      environmentReady: true,
      checking: false,
      repairing: false,
      updateState: null,
      updatePromptOpen: false,
      updateError: error instanceof Error ? error.message : '更新检查失败',
    })
  }
}

export const useRuntimeStore = create<RuntimeStoreState>((set, get) => ({
  status: checkingStatus,
  bootstrapped: false,
  environmentReady: false,
  checking: false,
  repairing: false,
  updating: false,
  exporting: false,
  updateState: null,
  updatePromptOpen: false,
  updateError: '',
  diagnosticsPath: '',
  diagnosticsError: '',
  bootstrap: async () => {
    if (get().checking || get().repairing || get().updating) {
      return
    }
    await evaluateRuntime(set, false)
  },
  retryCheck: async () => {
    if (get().checking || get().repairing || get().updating) {
      return
    }
    await evaluateRuntime(set, false)
  },
  repairNow: async () => {
    if (get().checking || get().repairing || get().updating) {
      return
    }
    await evaluateRuntime(set, true)
  },
  confirmUpdate: async () => {
    if (get().updating) {
      return
    }

    set({ updating: true, updateError: '' })
    try {
      await applyDesktopReleaseUpdate()
      await evaluateRuntime(set, false)
    } catch (error) {
      set({
        updating: false,
        updateError: error instanceof Error ? error.message : '更新失败',
      })
      throw error
    } finally {
      set({ updating: false })
    }
  },
  dismissUpdatePrompt: () =>
    set((state) => {
      if (state.updateState?.kind === 'required') {
        return state
      }
      return {
        updatePromptOpen: false,
      }
    }),
  exportDiagnostics: async () => {
    if (get().exporting) {
      return get().diagnosticsPath
    }

    set({ exporting: true })
    try {
      const path = await exportDesktopEnvironmentDiagnostics()
      set({ diagnosticsPath: path, diagnosticsError: '' })
      return path
    } catch (error) {
      const message = error instanceof Error ? error.message : '导出诊断失败'
      set({ diagnosticsError: message })
      throw error
    } finally {
      set({ exporting: false })
    }
  },
}))
