import type {
  AlertChannel,
  AlertEvent,
  AlertRule,
  AuthMe,
  Connection,
  HistoryPoint,
  InterfaceRate,
  InterfaceStats,
  Port,
  Protocol,
  Snapshot,
} from '@/types'

const TOKEN_KEY = 'portsleuth.token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

class APIError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  headers.set('Accept', 'application/json')
  if (init?.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(path, { ...init, headers })

  if (res.status === 401) {
    // Token expired or invalid — clear and bubble up.
    setToken(null)
  }
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new APIError(res.status, text || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

// ----- auth -----

export const auth = {
  login: (username: string, password: string) =>
    request<{ token: string; authDisabled?: boolean }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  me: () => request<AuthMe>('/api/auth/me'),
}

// ----- snapshot -----

export const snapshot = {
  get: () => request<Snapshot>('/api/snapshot'),
}

// ----- net -----

export const net = {
  interfaces: () =>
    request<{ interfaces: InterfaceStats[]; rates: InterfaceRate[] }>(
      '/api/net/interfaces',
    ),
  ports: () => request<{ ports: Port[] }>('/api/net/ports'),
  top: (by: 'bandwidth' | 'rx' | 'tx' | 'connections' = 'bandwidth', limit = 20) =>
    request<{ by: string; limit: number; ports: Port[] }>(
      `/api/net/top?by=${by}&limit=${limit}`,
    ),
  connections: () =>
    request<{ connections: Connection[] }>('/api/net/connections'),
}

// ----- history -----

export const history = {
  totals: (since?: string) =>
    request<{ points: HistoryPoint[] }>(
      `/api/history/totals${since ? `?since=${encodeURIComponent(since)}` : ''}`,
    ),
  iface: (name: string, since?: string) =>
    request<{ points: HistoryPoint[] }>(
      `/api/history/interface?name=${encodeURIComponent(name)}${
        since ? `&since=${encodeURIComponent(since)}` : ''
      }`,
    ),
  port: (port: number, protocol: Protocol = 'tcp', since?: string) =>
    request<{ points: HistoryPoint[] }>(
      `/api/history/port?port=${port}&protocol=${protocol}${
        since ? `&since=${encodeURIComponent(since)}` : ''
      }`,
    ),
}

// ----- alerts -----

export const alerts = {
  rules: () => request<{ rules: AlertRule[] }>('/api/alerts/rules'),
  setRules: (rules: AlertRule[]) =>
    request<{ ok: true }>('/api/alerts/rules', {
      method: 'PUT',
      body: JSON.stringify({ rules }),
    }),
  channels: () => request<{ channels: AlertChannel[] }>('/api/alerts/channels'),
  setChannels: (channels: AlertChannel[]) =>
    request<{ ok: true }>('/api/alerts/channels', {
      method: 'PUT',
      body: JSON.stringify({ channels }),
    }),
  events: () => request<{ events: AlertEvent[] }>('/api/alerts/events'),
}

export { APIError }
