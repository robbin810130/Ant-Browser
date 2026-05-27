import { credentialFailureCodes, recoveryActionForState } from './statusMatrix'

export function recoveryActionFor(input: {
  reclaimPending?: boolean
  instanceRunning?: boolean
  profileExists?: boolean
  coreReady?: boolean
  sharedLoginStatus?: string
  failureCode?: string
}) {
  return recoveryActionForState(input)
}

export { credentialFailureCodes }
