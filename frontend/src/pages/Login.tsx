import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import { auth, setToken } from '@/lib/api'
import { useApp } from '@/store/app'

export function Login() {
  const navigate = useNavigate()
  const authDisabled = useApp((s) => s.authDisabled)
  const setAuth = useApp((s) => s.setAuth)
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  // If auth is disabled server-side, jump straight in.
  useEffect(() => {
    if (authDisabled) navigate('/', { replace: true })
  }, [authDisabled, navigate])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    try {
      const res = await auth.login(username, password)
      if (res.authDisabled) {
        navigate('/', { replace: true })
        return
      }
      setToken(res.token)
      setAuth({ username })
      toast.success('Welcome back')
      navigate('/', { replace: true })
    } catch {
      toast.error('Invalid credentials')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid min-h-screen place-items-center bg-bg-base p-6">
      <Card className="w-full max-w-sm">
        <CardContent className="p-6">
          <div className="mb-6 flex items-center gap-3">
            <div className="grid h-10 w-10 place-items-center rounded-md bg-gradient-to-br from-accent to-success font-bold text-white shadow-md-dark">
              P
            </div>
            <div>
              <div className="text-sm font-semibold">PortSleuth</div>
              <div className="text-xs text-fg-secondary">Sign in to continue</div>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="flex flex-col gap-3">
            <div>
              <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                Username
              </label>
              <Input
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                autoFocus
                required
              />
            </div>
            <div>
              <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                Password
              </label>
              <Input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            <Button type="submit" variant="primary" disabled={busy} className="mt-2">
              {busy ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
