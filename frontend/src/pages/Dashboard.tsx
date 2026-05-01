import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowDown, ArrowUp, RefreshCw } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useApp } from '@/store/app'
import { history } from '@/lib/api'
import { PageHeader } from '@/components/layout/PageHeader'
import { StatCard } from '@/components/layout/StatCard'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { ToggleGroup } from '@/components/ui/toggle-group'
import { ProtocolBadge } from '@/components/ui/port-badges'
import { BandwidthChart } from '@/components/charts/BandwidthChart'
import { BandwidthBars } from '@/components/charts/BandwidthBars'
import { Skeleton } from '@/components/ui/skeleton'
import { formatBps, formatRelativeTime } from '@/lib/utils'

type WindowKey = '1m' | '5m' | '1h' | '24h'

const WINDOW_SECONDS: Record<WindowKey, number> = {
  '1m': 60,
  '5m': 300,
  '1h': 3600,
  '24h': 86400,
}

export function Dashboard() {
  const navigate = useNavigate()
  const snapshot = useApp((s) => s.snapshot)
  const [windowKey, setWindowKey] = useState<WindowKey>('5m')
  const [topBy, setTopBy] = useState<'total' | 'rx' | 'tx'>('total')

  const since = useMemo(() => {
    return new Date(Date.now() - WINDOW_SECONDS[windowKey] * 1000).toISOString()
  }, [windowKey])

  const { data: hist, refetch, isFetching } = useQuery({
    queryKey: ['history', 'totals', windowKey],
    queryFn: () => history.totals(since),
    refetchInterval: 5_000,
  })

  if (!snapshot) {
    return (
      <div className="p-8">
        <PageHeader title="Dashboard" subtitle="Connecting to live data…" />
        <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-4">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-28" />
          ))}
        </div>
      </div>
    )
  }

  const points = hist?.points ?? []
  const sparklineRx = points.slice(-30).map((p) => p.rxBytesPerSec)
  const sparklineTx = points.slice(-30).map((p) => p.txBytesPerSec)

  const topPorts = (() => {
    const list = [...snapshot.topPorts]
    if (topBy === 'rx') list.sort((a, b) => b.rxBytesPerSec - a.rxBytesPerSec)
    else if (topBy === 'tx') list.sort((a, b) => b.txBytesPerSec - a.txBytesPerSec)
    else list.sort((a, b) => b.totalBps - a.totalBps)
    return list.slice(0, 8)
  })()

  return (
    <div className="p-6 lg:p-8">
      <PageHeader
        title="Dashboard"
        subtitle={
          <span>
            Real-time network overview · last updated{' '}
            <span className="font-mono">{formatRelativeTime(snapshot.ts)}</span>
          </span>
        }
        actions={
          <>
            <ToggleGroup
              value={windowKey}
              onChange={setWindowKey}
              options={[
                { value: '1m', label: '1m' },
                { value: '5m', label: '5m' },
                { value: '1h', label: '1h' },
                { value: '24h', label: '24h' },
              ]}
            />
            <Button
              variant="default"
              size="icon"
              onClick={() => refetch()}
              disabled={isFetching}
              aria-label="Refresh"
            >
              <RefreshCw size={14} className={isFetching ? 'animate-spin' : ''} />
            </Button>
          </>
        }
      />

      {/* Stat cards */}
      <div className="mb-5 grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label={
            <span className="flex items-center gap-1">
              <ArrowDown size={11} strokeWidth={2.5} />
              RX (download)
            </span>
          }
          value={formatBps(snapshot.totals.rxBytesPerSec, 1).split(' ')[0]}
          unit={formatBps(snapshot.totals.rxBytesPerSec, 1).split(' ')[1]}
          valueColor="#3ec28f"
          sparkline={sparklineRx}
          sparklineColor="#3ec28f"
          meta={`across ${snapshot.rates.length} interfaces`}
        />
        <StatCard
          label={
            <span className="flex items-center gap-1">
              <ArrowUp size={11} strokeWidth={2.5} />
              TX (upload)
            </span>
          }
          value={formatBps(snapshot.totals.txBytesPerSec, 1).split(' ')[0]}
          unit={formatBps(snapshot.totals.txBytesPerSec, 1).split(' ')[1]}
          valueColor="#f5a623"
          sparkline={sparklineTx}
          sparklineColor="#f5a623"
        />
        <StatCard
          label="Active connections"
          value={snapshot.totals.activeConns}
          meta={`${snapshot.totals.establishedConn} established`}
        />
        <StatCard
          label="Listening ports"
          value={snapshot.totals.listeningPorts}
          meta={`${snapshot.ports.filter((p) => p.protocol.startsWith('tcp')).length} TCP · ${snapshot.ports.filter((p) => p.protocol.startsWith('udp')).length} UDP`}
        />
      </div>

      {/* Main bandwidth chart */}
      <Card className="mb-3.5">
        <CardHeader>
          <div>
            <CardTitle>Bandwidth over time</CardTitle>
            <CardDescription>
              All interfaces combined · {windowKey} window
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          {points.length > 1 ? (
            <BandwidthChart points={points} />
          ) : (
            <div className="grid h-60 place-items-center text-xs text-fg-tertiary">
              Collecting data…
            </div>
          )}
        </CardContent>
      </Card>

      {/* Two-column section */}
      <div className="grid grid-cols-1 gap-3.5 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <div>
              <CardTitle>Top bandwidth (live)</CardTitle>
              <CardDescription>Top ports by current throughput</CardDescription>
            </div>
            <ToggleGroup
              value={topBy}
              onChange={setTopBy}
              size="sm"
              options={[
                { value: 'total', label: 'Total' },
                { value: 'rx', label: 'RX' },
                { value: 'tx', label: 'TX' },
              ]}
            />
          </CardHeader>
          <CardContent>
            {topPorts.length > 0 ? (
              <BandwidthBars ports={topPorts} by={topBy} />
            ) : (
              <div className="grid h-40 place-items-center text-xs text-fg-tertiary">
                No port traffic yet — eBPF/fallback warming up
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Interfaces</CardTitle>
          </CardHeader>
          <CardContent className="px-0">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border-subtle">
                  <th className="px-5 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                    Iface
                  </th>
                  <th className="px-3 py-2 text-right text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                    RX
                  </th>
                  <th className="px-5 py-2 text-right text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                    TX
                  </th>
                </tr>
              </thead>
              <tbody>
                {snapshot.rates.map((r) => (
                  <tr
                    key={r.name}
                    className="border-b border-border-subtle last:border-b-0 text-sm"
                  >
                    <td className="px-5 py-2 font-mono font-semibold">{r.name}</td>
                    <td className="px-3 py-2 text-right font-mono tabular-nums text-rx">
                      {formatBps(r.rxBytesPerSec)}
                    </td>
                    <td className="px-5 py-2 text-right font-mono tabular-nums text-tx">
                      {formatBps(r.txBytesPerSec)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      </div>

      {/* Listening ports */}
      <Card className="mt-3.5">
        <CardHeader>
          <div>
            <CardTitle>Listening ports</CardTitle>
            <CardDescription>Click any row for detail</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="px-0">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border-subtle">
                <th className="px-5 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                  Proto
                </th>
                <th className="px-3 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                  Address
                </th>
                <th className="px-3 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                  Port
                </th>
                <th className="px-3 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                  Process
                </th>
                <th className="px-3 py-2 text-right text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                  Connections
                </th>
                <th className="px-5 py-2 text-right text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                  Throughput
                </th>
              </tr>
            </thead>
            <tbody>
              {snapshot.ports.slice(0, 15).map((p) => (
                <tr
                  key={`${p.protocol}-${p.localAddr}`}
                  onClick={() => navigate(`/ports/${p.protocol}/${p.localPort}`)}
                  className="cursor-pointer border-b border-border-subtle text-sm transition-colors last:border-b-0 hover:bg-bg-hover"
                >
                  <td className="px-5 py-2.5">
                    <ProtocolBadge protocol={p.protocol} />
                  </td>
                  <td className="px-3 py-2.5 font-mono text-fg-secondary">{p.localIp}</td>
                  <td className="px-3 py-2.5 font-mono font-semibold">{p.localPort}</td>
                  <td className="px-3 py-2.5 font-mono">{p.process || '—'}</td>
                  <td className="px-3 py-2.5 text-right font-mono tabular-nums">
                    {p.connectionCount || '—'}
                  </td>
                  <td className="px-5 py-2.5 text-right font-mono tabular-nums">
                    {p.totalBps > 0 ? formatBps(p.totalBps) : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </CardContent>
      </Card>
    </div>
  )
}
