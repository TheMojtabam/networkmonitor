import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * cn merges class names with tailwind-merge so conflicting Tailwind utilities
 * resolve correctly. Use this everywhere you do conditional class names.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * formatBytes returns a human-readable byte count.
 * formatBytes(2_457_600) => "2.34 MB"
 */
export function formatBytes(bytes: number, fractionDigits = 2): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '0 B'
  if (bytes < 1024) return `${bytes.toFixed(0)} B`
  const units = ['KB', 'MB', 'GB', 'TB', 'PB']
  let value = bytes / 1024
  let unit = units[0]
  for (let i = 1; i < units.length && value >= 1024; i++) {
    value /= 1024
    unit = units[i]
  }
  return `${value.toFixed(fractionDigits)} ${unit}`
}

/**
 * formatBps takes bytes/sec and returns a per-second human-readable string.
 */
export function formatBps(bps: number, fractionDigits = 2): string {
  return `${formatBytes(bps, fractionDigits)}/s`
}

/**
 * formatNumber adds thousand separators.
 */
export function formatNumber(n: number): string {
  return n.toLocaleString('en-US')
}

/**
 * formatAge takes seconds and returns "12s", "4m 32s", "1h 12m", "2d 4h".
 */
export function formatAge(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '—'
  const s = Math.floor(seconds)
  if (s < 60) return `${s}s`
  if (s < 3600) {
    const m = Math.floor(s / 60)
    const rs = s % 60
    return rs ? `${m}m ${rs}s` : `${m}m`
  }
  if (s < 86400) {
    const h = Math.floor(s / 3600)
    const rm = Math.floor((s % 3600) / 60)
    return rm ? `${h}h ${rm}m` : `${h}h`
  }
  const d = Math.floor(s / 86400)
  const rh = Math.floor((s % 86400) / 3600)
  return rh ? `${d}d ${rh}h` : `${d}d`
}

/**
 * formatRelativeTime returns "5s ago", "2m ago", etc.
 */
export function formatRelativeTime(iso: string): string {
  const then = new Date(iso).getTime()
  const now = Date.now()
  const seconds = Math.floor((now - then) / 1000)
  if (seconds < 5) return 'just now'
  if (seconds < 60) return `${seconds}s ago`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}

/**
 * countryFlag returns the unicode flag for an ISO country code.
 * countryFlag('US') => '🇺🇸'
 */
export function countryFlag(iso?: string): string {
  if (!iso || iso.length !== 2) return ''
  const A = 0x1f1e6
  const a = 'A'.charCodeAt(0)
  const upper = iso.toUpperCase()
  return String.fromCodePoint(
    A + (upper.charCodeAt(0) - a),
    A + (upper.charCodeAt(1) - a),
  )
}
