import { create } from 'zustand'
import type { Snapshot, AlertEvent } from '@/types'

interface AppState {
  // ----- live data -----
  snapshot: Snapshot | null
  setSnapshot: (s: Snapshot) => void

  // ----- alerts feed (in-memory toast feed) -----
  recentAlerts: AlertEvent[]
  pushAlert: (e: AlertEvent) => void
  clearAlerts: () => void

  // ----- connection status -----
  wsStatus: 'idle' | 'connecting' | 'open' | 'closed' | 'error'
  setWsStatus: (s: AppState['wsStatus']) => void

  // ----- auth -----
  authDisabled: boolean
  username: string | null
  setAuth: (a: { authDisabled?: boolean; username?: string | null }) => void
}

export const useApp = create<AppState>((set) => ({
  snapshot: null,
  setSnapshot: (snapshot) => set({ snapshot }),

  recentAlerts: [],
  pushAlert: (alert) =>
    set((state) => ({
      recentAlerts: [alert, ...state.recentAlerts].slice(0, 50),
    })),
  clearAlerts: () => set({ recentAlerts: [] }),

  wsStatus: 'idle',
  setWsStatus: (wsStatus) => set({ wsStatus }),

  authDisabled: false,
  username: null,
  setAuth: ({ authDisabled, username }) =>
    set((state) => ({
      authDisabled: authDisabled ?? state.authDisabled,
      username: username !== undefined ? username : state.username,
    })),
}))
