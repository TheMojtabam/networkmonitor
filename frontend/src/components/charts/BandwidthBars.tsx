import { cn, formatBps } from '@/lib/utils'
import type { Port } from '@/types'

interface Props {
  ports: Port[]
  /** Which value to use for the bar width. */
  by?: 'total' | 'rx' | 'tx'
  className?: string
  /** When true, the bar is rendered in TX color. Otherwise it follows `by`. */
  tone?: 'rx' | 'tx' | 'auto'
}

/**
 * BandwidthBars renders a list of horizontal bars for top-N ports.
 * Bar width is normalized to the highest value in the list.
 */
export function BandwidthBars({ ports, by = 'total', className, tone = 'auto' }: Props) {
  const values = ports.map((p) =>
    by === 'rx' ? p.rxBytesPerSec : by === 'tx' ? p.txBytesPerSec : p.totalBps,
  )
  const max = Math.max(...values, 1)

  return (
    <div className={cn('flex flex-col', className)}>
      {ports.map((p, i) => {
        const v = values[i]
        const width = (v / max) * 100
        const barTone =
          tone === 'auto' ? (by === 'tx' ? 'tx' : by === 'rx' ? 'rx' : 'mixed') : tone

        return (
          <div
            key={`${p.protocol}-${p.localPort}-${i}`}
            className="grid grid-cols-[100px_1fr_110px] items-center gap-3 border-b border-border-subtle py-2.5 last:border-b-0"
          >
            <div className="min-w-0">
              <div className="font-mono text-xs font-semibold text-fg-primary">
                :{p.localPort}
              </div>
              <div className="font-mono text-2xs text-fg-tertiary truncate">
                {p.protocol.toUpperCase()} · {p.process || '—'}
              </div>
            </div>

            <div className="relative h-5 overflow-hidden rounded border border-border-subtle bg-bg-elevated">
              <div
                className={cn(
                  'h-full rounded-sm transition-[width] duration-300 ease-out',
                  barTone === 'rx' && 'bg-gradient-to-r from-rx to-rx/80',
                  barTone === 'tx' && 'bg-gradient-to-r from-tx to-tx/80',
                  barTone === 'mixed' && 'bg-gradient-to-r from-accent to-accent-hover',
                )}
                style={{ width: `${width}%` }}
              />
            </div>

            <div className="text-right font-mono text-xs font-semibold text-fg-primary tabular-nums">
              {formatBps(v)}
            </div>
          </div>
        )
      })}
    </div>
  )
}
