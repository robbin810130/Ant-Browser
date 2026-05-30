import { create } from 'zustand'
import type { AuthStatus, DesktopAuthProfile } from '../modules/auth/types'

interface AuthStoreState {
  status: AuthStatus
  accessToken: string
  rememberMe: boolean
  profile: DesktopAuthProfile | null
  bootstrapReady: boolean
  signingOut: boolean
  setAuthenticating: () => void
  setAuthenticated: (payload: {
    accessToken: string
    rememberMe: boolean
    profile: DesktopAuthProfile
    bootstrapReady: boolean
  }) => void
  setAnonymous: () => void
  setSessionExpired: () => void
  setSigningOut: (value?: boolean) => void
}

const baseAnonymousState = {
  accessToken: '',
  rememberMe: false,
  profile: null,
  bootstrapReady: false,
  signingOut: false,
} as const

export const useAuthStore = create<AuthStoreState>((set) => ({
  status: 'anonymous',
  ...baseAnonymousState,
  setAuthenticating: () =>
    set({
      status: 'authenticating',
      bootstrapReady: false,
      signingOut: false,
    }),
  setAuthenticated: ({ accessToken, rememberMe, profile, bootstrapReady }) =>
    set({
      status: 'authenticated',
      accessToken: accessToken.trim(),
      rememberMe,
      profile,
      bootstrapReady,
      signingOut: false,
    }),
  setAnonymous: () =>
    set({
      status: 'anonymous',
      ...baseAnonymousState,
    }),
  setSessionExpired: () =>
    set({
      status: 'session_expired',
      ...baseAnonymousState,
    }),
  setSigningOut: (value = true) =>
    set((state) => ({
      status: value ? 'signing_out' : state.profile ? 'authenticated' : 'anonymous',
      signingOut: value,
    })),
}))
