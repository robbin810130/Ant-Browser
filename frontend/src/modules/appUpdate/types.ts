export type AppUpdateKind = 'none' | 'soft' | 'required' | 'unsupported_install' | 'failed'

export type AppUpdateStatus =
  | ''
  | 'idle'
  | 'available'
  | 'downloading'
  | 'staged'
  | 'applying'
  | 'verifying'
  | 'succeeded'
  | 'rolled_back'
  | 'failed_manual_repair'

export interface AppUpdateState {
  kind: AppUpdateKind
  status: AppUpdateStatus
  localAppVersion: string
  remoteAppVersion: string
  minimumRuntimeResourceVersion: string
  manifestSource: string
  manifestUrl: string
  payloadUrl: string
  target: string
  notes: string
  errorCode: string
  errorMessage: string
  details: Record<string, string>
}
