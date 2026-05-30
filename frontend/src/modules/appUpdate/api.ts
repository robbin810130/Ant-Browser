import type { AppUpdateState } from './types'

type AppUpdateBindings = {
  CheckDesktopAppUpdate?: () => Promise<any>
  DownloadDesktopAppUpdate?: () => Promise<any>
  ApplyDesktopAppUpdate?: () => Promise<any>
  GetDesktopAppUpdateState?: () => Promise<any>
  ClearDesktopAppUpdateFailure?: () => Promise<void>
}

async function getBindings(): Promise<AppUpdateBindings | null> {
  const fallback = ((window as any)?.go?.main?.App as AppUpdateBindings | undefined) ?? null
  try {
    const module: any = await import('../../wailsjs/go/main/App')
    return { ...(fallback || {}), ...(module as AppUpdateBindings) }
  } catch {
    return fallback
  }
}

function normalizeKind(kind: string): AppUpdateState['kind'] {
  if (kind === 'soft' || kind === 'required' || kind === 'unsupported_install' || kind === 'failed' || kind === 'none') {
    return kind
  }
  return 'none'
}

function normalizeStatus(status: string): AppUpdateState['status'] {
  if (
    status === 'idle' ||
    status === 'available' ||
    status === 'downloading' ||
    status === 'staged' ||
    status === 'applying' ||
    status === 'verifying' ||
    status === 'succeeded' ||
    status === 'rolled_back' ||
    status === 'failed_manual_repair'
  ) {
    return status
  }
  return ''
}

function normalize(input: any): AppUpdateState {
  const details =
    input?.details && typeof input.details === 'object'
      ? Object.fromEntries(Object.entries(input.details).map(([key, value]) => [String(key), String(value ?? '')]))
      : {}

  return {
    kind: normalizeKind(String(input?.kind || 'none')),
    status: normalizeStatus(String(input?.status || '')),
    localAppVersion: String(input?.localAppVersion || ''),
    remoteAppVersion: String(input?.remoteAppVersion || ''),
    minimumRuntimeResourceVersion: String(input?.minimumRuntimeResourceVersion || ''),
    manifestSource: String(input?.manifestSource || ''),
    manifestUrl: String(input?.manifestUrl || ''),
    payloadUrl: String(input?.payloadUrl || ''),
    target: String(input?.target || ''),
    notes: Array.isArray(input?.notes) ? input.notes.map((item: any) => String(item)).join('\n') : String(input?.notes || ''),
    errorCode: String(input?.errorCode || ''),
    errorMessage: String(input?.errorMessage || ''),
    details,
  }
}

async function requireBinding<K extends keyof AppUpdateBindings>(name: K): Promise<NonNullable<AppUpdateBindings[K]>> {
  const bindings = await getBindings()
  const fn = bindings?.[name]
  if (!fn) {
    throw new Error(`当前环境缺少 ${String(name)} 绑定`)
  }
  return fn as NonNullable<AppUpdateBindings[K]>
}

export async function checkDesktopAppUpdate(): Promise<AppUpdateState> {
  const fn = await requireBinding('CheckDesktopAppUpdate')
  return normalize(await fn())
}

export async function downloadDesktopAppUpdate(): Promise<AppUpdateState> {
  const fn = await requireBinding('DownloadDesktopAppUpdate')
  return normalize(await fn())
}

export async function applyDesktopAppUpdate(): Promise<AppUpdateState> {
  const fn = await requireBinding('ApplyDesktopAppUpdate')
  return normalize(await fn())
}

export async function getDesktopAppUpdateState(): Promise<AppUpdateState> {
  const fn = await requireBinding('GetDesktopAppUpdateState')
  return normalize(await fn())
}

export async function clearDesktopAppUpdateFailure(): Promise<void> {
  const fn = await requireBinding('ClearDesktopAppUpdateFailure')
  await fn()
}
