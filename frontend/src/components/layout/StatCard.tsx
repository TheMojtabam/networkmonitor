import { cn } from '@/lib/utils'

interface Props {
  label: React.ReactNode
  value: React.ReactNode
  unit?: string
  meta?: React.ReactNode
  valueColor?: string
  sparkline?: number[]
  sparklineColor?: string
  className?: string
}

/**
 * StatCard renders a numeric stat with optional unit + meta + sparkline.
 *
 * Sparkline data are normalised to fit the container — pass raw numbers
 * (e.g. last N points of a series), not pre-scaled values.
 */
export function StatCard({
  label,
  value,
  unit,
  meta,
  valueColor,
  sparkline,
  sparklineColor = 'var(--rx)',
  className,
}: Props) {
  return (
    <div
      className={cn(
        'relative overflow-hidden rounded-lg border border-border-subtle bg-bg-surface p-5',
        className,
      )}
    >
      {/* Subtle radial highlight */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(91,141,239,0.08),transparent_60%)]"
      />

      <div className="relative">
        <div className="text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
          {label}
        </div>
        <div className="mt-2 flex items-baseline gap-1.5">
          <div
            className="text-2xl font-bold tracking-tight tabular-nums"
            style={valueColor ? { color: valueColor } : undefined}
          >
            {value}
          </div>
          {unit && <div className="text-sm font-medium text-fg-secondary">{unit}</div>}
        </div>
        {meta && <div className="mt-1 text-xs text-fg-secondary">{meta}</div>}
      </div>

      {sparkline && sparkline.length > 1 && (
        <Sparkline data={sparkline} color={sparklineColor} />
      )}
    </div>
  )
}

function Sparkline({ data, color }: { data: number[]; color: string }) {
  const w = 200
  const h = 40
  const min = Math.min(...data)
  const max = Math.max(...data)
  const range = max - min || 1
  const points = data
    .map((v, i) => {
      const x = (i / (data.length - 1)) * w
      const y = h - ((v - min) / range) * (h - 4) - 2
      return `${x},${y}`
    })
    .join(' ')

  // Build closing area path
  const areaPath = `M0,${h} L${points.split(' ').join(' L')} L${w},${h} Z`

  return (
    <svg
      className="absolute bottom-0 left-0 right-0 h-10 w-full opacity-60"
      viewBox={`0 0 ${w} ${h}`}
      preserveAspectRatio="none"
      aria-hidden
    >
      <path d={areaPath} fill={color} opacity="0.15" />
      <polyline
        points={points}
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        opacity="0.7"
      />
    </svg>
  )
}
