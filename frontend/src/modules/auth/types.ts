export interface DesktopAuthSession {
  accessToken: string
  rememberMe: boolean
}

export interface DesktopAuthUser {
  id: string
  username: string
  displayName: string
}

export interface DesktopAuthRole {
  code: string
  name: string
}

export interface DesktopAuthProfile {
  user: DesktopAuthUser
  roles: DesktopAuthRole[]
  dataScope: string
}

export type AuthStatus =
  | 'anonymous'
  | 'authenticating'
  | 'authenticated'
  | 'session_expired'
  | 'signing_out'
