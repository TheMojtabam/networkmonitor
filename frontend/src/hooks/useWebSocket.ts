import { useEffect, useRef, useState } from 'react'
import type { Snapshot, WSMessage, AlertEvent } from '@/types'
import { getToken } from '@/lib/api'

export type WSStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

interface Options {
  /** Called on every snapshot. */
  onSnapshot?: (snap: Snapshot) => void
  /** Called when an alert fires. */
  onAlert?: (evt: AlertEvent) => void
  /** Disable connection (useful for SSR / tests). */
  enabled?: boolean
}

const MAX_BACKOFF_MS = 30_000
const INITIAL_BACKOFF_MS = 1_000

/**
 * useWebSocket connects to /ws and keeps a live Snapshot in state.
 *
 * It auto-reconnects with exponential backoff (capped at 30s) and
 * exposes the connection status so the UI can show a banner.
 *
 * The hook owns the lifecycle — components mount and unmount freely;
 * the connection follows the component tree.
 */
export function useWebSocket(opts: Options = {}) {
  const { onSnapshot, onAlert, enabled = true } = opts
  const [status, setStatus] = useState<WSStatus>('idle')
  const [latestSnapshot, setLatestSnapshot] = useState<Snapshot | null>(null)

  // Hold the latest callbacks in refs so the connect closure doesn't go stale.
  const cbSnapshot = useRef(onSnapshot)
  const cbAlert = useRef(onAlert)
  cbSnapshot.current = onSnapshot
  cbAlert.current = onAlert

  useEffect(() => {
    if (!enabled) return
    let ws: WebSocket | null = null
    let backoff = INITIAL_BACKOFF_MS
    let timer: number | undefined
    let cancelled = false

    function connect() {
      if (cancelled) return
      setStatus('connecting')

      // ws:// or wss:// based on page protocol
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const token = getToken()
      const url = `${proto}://${location.host}/ws${token ? `?token=${encodeURIComponent(token)}` : ''}`

      ws = new WebSocket(url)

      ws.onopen = () => {
        setStatus('open')
        backoff = INITIAL_BACKOFF_MS
      }

      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data) as WSMessage
          if (msg.type === 'snapshot') {
            setLatestSnapshot(msg.data)
            cbSnapshot.current?.(msg.data)
          } else if (msg.type === 'alert') {
            cbAlert.current?.(msg.data)
          }
        } catch {
          // ignore malformed frame
        }
      }

      ws.onerror = () => setStatus('error')

      ws.onclose = () => {
        setStatus('closed')
        if (cancelled) return
        timer = window.setTimeout(connect, backoff)
        backoff = Math.min(backoff * 2, MAX_BACKOFF_MS)
      }
    }

    connect()

    return () => {
      cancelled = true
      if (timer) window.clearTimeout(timer)
      if (ws) {
        ws.onclose = null
        ws.close()
      }
    }
  }, [enabled])

  return { status, latestSnapshot }
}
