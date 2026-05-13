export type EnvironmentState = 'checking' | 'pass' | 'repairable' | 'blocked'
export type EnvironmentSeverity = 'info' | 'warning' | 'error'
export type ReleaseUpdateKind = 'none' | 'soft' | 'required'

export interface EnvironmentFailureItem {
  code: string
  severity: EnvironmentSeverity
  message: string
  repairable: boolean
}

export interface EnvironmentStatus {
  state: EnvironmentState
  items: EnvironmentFailureItem[]
}

export interface ReleaseUpdateState {
  kind: ReleaseUpdateKind
  localAppVersion: string
  remoteAppVersion: string
  resourceVersion: string
}
