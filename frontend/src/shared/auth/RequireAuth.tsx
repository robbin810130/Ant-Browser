import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { Loading } from '../components'
import { useAuthStore } from '../../store/authStore'

export function RequireAuth() {
  const location = useLocation()
  const status = useAuthStore((state) => state.status)
  const bootstrapReady = useAuthStore((state) => state.bootstrapReady)

  if (status === 'authenticating') {
    return (
      <div className="flex min-h-[240px] items-center justify-center py-10">
        <Loading text="正在校验登录状态..." />
      </div>
    )
  }

  if (status !== 'authenticated' || !bootstrapReady) {
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  return <Outlet />
}
