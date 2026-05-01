import { useEffect } from 'react'
import { Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom'
import { toast } from 'sonner'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '@/components/layout/AppShell'
import { Dashboard } from '@/pages/Dashboard'
import { Bandwidth } from '@/pages/Bandwidth'
import { Connections } from '@/pages/Connections'
import { PortDetail } from '@/pages/PortDetail'
import { AlertsPage } from '@/pages/Alerts'
import { Settings } from '@/pages/Settings'
import { Login } from '@/pages/Login'
import { useWebSocket } from '@/hooks/useWebSocket'
import { useApp } from '@/store/app'
import { auth, getToken } from '@/lib/api'

export function App() {
  const navigate = useNavigate()
  const location = useLocation()
  const setSnapshot = useApp((s) => s.setSnapshot)
  const setWsStatus = useApp((s) => s.setWsStatus)
  const pushAlert = useApp((s) => s.pushAlert)
  const setAuth = useApp((s) => s.setAuth)
  const authDisabled = useApp((s) => s.authDisabled)

  // Probe auth state on mount.
  const { data: me, isLoading: meLoading } = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: () => auth.me(),
    retry: false,
  })

  useEffect(() => {
    if (!me) return
    setAuth({
      authDisabled: !!me.authDisabled,
      username: me.user?.username ?? null,
    })
    // If auth is enabled and we don't have a token, send to /login
    if (!me.authDisabled && !me.user && !getToken() && location.pathname !== '/login') {
      navigate('/login', { replace: true })
    }
  }, [me, navigate, location.pathname, setAuth])

  // ----- live WebSocket -----
  const wsEnabled =
    !meLoading && (authDisabled || !!me?.user || !!getToken())

  const { status } = useWebSocket({
    enabled: wsEnabled,
    onSnapshot: setSnapshot,
    onAlert: (evt) => {
      pushAlert(evt)
      toast.warning(evt.ruleName, {
        description: `${evt.metric} = ${evt.value.toFixed(2)} (threshold ${evt.threshold})`,
      })
    },
  })

  useEffect(() => {
    setWsStatus(status)
  }, [status, setWsStatus])

  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<AppShell />}>
        <Route index element={<Dashboard />} />
        <Route path="bandwidth" element={<Bandwidth />} />
        <Route path="connections" element={<Connections />} />
        <Route path="ports/:protocol/:port" element={<PortDetail />} />
        <Route path="alerts" element={<AlertsPage />} />
        <Route path="settings" element={<Settings />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
