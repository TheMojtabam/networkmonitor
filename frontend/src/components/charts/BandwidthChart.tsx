import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
} from 'recharts'
import type { HistoryPoint } from '@/types'
import { formatBps } from '@/lib/utils'

interface Props {
  points: HistoryPoint[]
  height?: number
  showLegend?: boolean
  /** When true, only show RX (single-series mode for per-port pages). */
  singleLine?: boolean
}

/**
 * BandwidthChart renders an RX/TX area chart with the project's dark theme.
 * Pass an array of HistoryPoint — empty array renders an empty grid.
 */
export function BandwidthChart({
  points,
  height = 240,
  showLegend = true,
  singleLine = false,
}: Props) {
  // Recharts wants serialisable data; convert ISO timestamps to short hh:mm:ss labels.
  const data = points.map((p) => ({
    ts: new Date(p.ts).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }),
    rx: p.rxBytesPerSec,
    tx: p.txBytesPerSec,
  }))

  return (
    <div style={{ width: '100%', height }}>
      <ResponsiveContainer>
        <AreaChart data={data} margin={{ top: 4, right: 4, left: 4, bottom: 4 }}>
          <defs>
            <linearGradient id="gradRx" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#3ec28f" stopOpacity={0.35} />
              <stop offset="100%" stopColor="#3ec28f" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="gradTx" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#f5a623" stopOpacity={0.3} />
              <stop offset="100%" stopColor="#f5a623" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="#1f2532" strokeDasharray="2 4" vertical={false} />
          <XAxis
            dataKey="ts"
            stroke="#6b7280"
            tick={{ fill: '#6b7280', fontSize: 10 }}
            tickLine={false}
            axisLine={false}
            minTickGap={32}
          />
          <YAxis
            stroke="#6b7280"
            tick={{ fill: '#6b7280', fontSize: 10 }}
            tickFormatter={(v) => formatBps(Number(v), 0)}
            tickLine={false}
            axisLine={false}
            width={70}
          />
          <Tooltip
            content={<CustomTooltip />}
            cursor={{ stroke: '#3a4256', strokeWidth: 1, strokeDasharray: '3 3' }}
          />
          {showLegend && !singleLine && (
            <Legend
              verticalAlign="top"
              height={28}
              iconType="circle"
              iconSize={8}
              wrapperStyle={{ fontSize: 11, color: '#9aa3b2' }}
            />
          )}
          <Area
            type="monotone"
            dataKey="rx"
            name="RX"
            stroke="#3ec28f"
            strokeWidth={2}
            fill="url(#gradRx)"
            isAnimationActive={false}
          />
          {!singleLine && (
            <Area
              type="monotone"
              dataKey="tx"
              name="TX"
              stroke="#f5a623"
              strokeWidth={2}
              fill="url(#gradTx)"
              isAnimationActive={false}
            />
          )}
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

interface TooltipProps {
  active?: boolean
  payload?: Array<{ name: string; value: number; color: string; dataKey: string }>
  label?: string
}

function CustomTooltip({ active, payload, label }: TooltipProps) {
  if (!active || !payload?.length) return null
  return (
    <div className="rounded-md border border-border-subtle bg-bg-elevated p-2.5 shadow-lg-dark">
      <div className="mb-1 font-mono text-2xs text-fg-tertiary">{label}</div>
      {payload.map((p) => (
        <div key={p.dataKey} className="flex items-center gap-2 text-xs">
          <span
            className="inline-block h-2 w-2 rounded-full"
            style={{ background: p.color }}
          />
          <span className="text-fg-secondary">{p.name}:</span>
          <span className="font-mono font-semibold tabular-nums" style={{ color: p.color }}>
            {formatBps(p.value)}
          </span>
        </div>
      ))}
    </div>
  )
}
