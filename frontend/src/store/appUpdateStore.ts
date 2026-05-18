import { create } from 'zustand'
import {
  applyDesktopAppUpdate,
  checkDesktopAppUpdate,
  clearDesktopAppUpdateFailure,
  downloadDesktopAppUpdate,
  getDesktopAppUpdateState,
} from '../modules/appUpdate/api'
import type { AppUpdateState } from '../modules/appUpdate/types'

const noneState: AppUpdateState = {
  kind: 'none',
  status: '',
  localAppVersion: '',
  remoteAppVersion: '',
  minimumRuntimeResourceVersion: '',
  manifestSource: '',
  manifestUrl: '',
  payloadUrl: '',
  target: '',
  notes: '',
  errorCode: '',
  errorMessage: '',
  details: {},
}

interface AppUpdateStoreState {
  state: AppUpdateState
  promptOpen: boolean
  checking: boolean
  applying: boolean
  error: string
  bootstrap: () => Promise<void>
  applyNow: () => Promise<void>
  dismiss: () => void
  clearFailure: () => Promise<void>
}

function message(error: unknown, fallback: string) {
  if (typeof error === 'string') return error.trim() || fallback
  if (error instanceof Error) return error.message.trim() || fallback
  return fallback
}

function shouldPrompt(state: AppUpdateState) {
  return state.kind === 'soft' || state.kind === 'required' || state.kind === 'unsupported_install' || state.kind === 'failed'
}

export const useAppUpdateStore = create<AppUpdateStoreState>((set, get) => ({
  state: noneState,
  promptOpen: false,
  checking: false,
  applying: false,
  error: '',
  bootstrap: async () => {
    if (get().checking || get().applying) return
    set({ checking: true, error: '' })
    try {
      const persisted = await getDesktopAppUpdateState()
      if (persisted.status === 'rolled_back' || persisted.status === 'failed_manual_repair') {
        set({ state: persisted, promptOpen: true, checking: false })
        return
      }
      const state = await checkDesktopAppUpdate()
      set({
        state,
        promptOpen: shouldPrompt(state),
        checking: false,
      })
    } catch (error) {
      set({ state: noneState, promptOpen: false, checking: false, error: message(error, '应用更新检查失败') })
    }
  },
  applyNow: async () => {
    if (get().applying) return
    set({ applying: true, error: '' })
    try {
      const staged = await downloadDesktopAppUpdate()
      set({ state: staged, promptOpen: true })
      if (staged.kind === 'failed' || staged.kind === 'unsupported_install') {
        set({ applying: false, error: staged.errorMessage || '应用更新准备失败' })
        return
      }
      const state = await applyDesktopAppUpdate()
      set({ state, promptOpen: true, applying: false })
    } catch (error) {
      set({ applying: false, error: message(error, '应用更新启动失败') })
      throw error
    }
  },
  dismiss: () =>
    set((current) => {
      if (current.state.kind === 'required' || current.state.kind === 'unsupported_install') return current
      return { promptOpen: false, error: '' }
    }),
  clearFailure: async () => {
    await clearDesktopAppUpdateFailure()
    set({ state: noneState, promptOpen: false, error: '' })
  },
}))
