import { useMemo, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Bell } from 'lucide-react'
import { useApp } from '@/store/app'
import { history, net } from '@/lib/api'
import { PageHeader } from '@/components/layout/PageHeader'
import { StatCard } from '@/components/layout/StatCard'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { ToggleGroup } from '@/components/ui/toggle-group'
import { Button } from '@/components/ui/button'
import { ProtocolBadge, StateBadge } from '@/components/ui/port-badges'
import { BandwidthChart } from '@/components/charts/BandwidthChart'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { formatBps, formatBytes, formatAge, countryFlag, formatRelativeTime } from '@/lib/utils'
import type { Connection, Protocol } from '@/types'

type WindowKey = '5m' | '1h' | '24h'
const WINDOW_SECONDS: Record<WindowKey, number> = {
  '5m': 300,
  '1h': 3600,
  '24h': 86400,
}

export function PortDetail() {
  const { protocol = 'tcp', port } = useParams<{ protocol: string; port: string }>()
  const portNum = Number(port)
  const proto = protocol as Protocol
  const snapshot = useApp((s) => s.snapshot)
  const [windowKey, setWindowKey] = useState<WindowKey>('1h')

  const portInfo = useMemo(
    () =>
      snapshot?.ports.find(
        (p) => p.protocol === proto && p.localPort === portNum,
      ) ?? null,
    [snapshot, proto, portNum],
  )

  const since = useMemo(
    () => new Date(Date.now() - WINDOW_SECONDS[windowKey] * 1000).toISOString(),
    [windowKey],
  )

  const { data: hist } = useQuery({
    queryKey: ['history', 'port', proto, portNum, windowKey],
    queryFn: () => history.port(portNum, proto, since),
    refetchInterval: 5_000,
  })

  const { data: connsData } = useQuery({
    queryKey: ['connections'],
    queryFn: () => net.connections(),
    refetchInterval: 5_000,
  })

  const portConns: Connection[] = useMemo(() => {
    if (!connsData) return []
    return connsData.connections.filter(
      (c) => c.localPort === portNum && c.protocol === proto,
    )
  }, [connsData, proto, portNum])

  const geoStats = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const c of portConns) {
      const k = c.country || 'Unknown'
      counts[k] = (counts[k] || 0) + 1
    }
    const total = portConns.length || 1
    return Object.entries(counts)
      .map(([country, count]) => ({
        country,
        count,
        percentage: (count / total) * 100,
      }))
      .sort((a, b) => b.count - a.count)
      .slice(0, 6)
  }, [portConns])

  if (!snapshot) {
    return (
      <div className="p-8">
        <PageHeader title="Loading port…" />
        <Skeleton className="h-96" />
      </div>
    )
  }

  if (!portInfo) {
    return (
      <div className="p-8">
        <EmptyState
          title="Port not found"
          description={`No data for ${proto.toUpperCase()} :${portNum} in the current snapshot.`}
          action={
            <Link to="/">
              <Button variant="default">← Back to dashboard</Button>
            </Link>
          }
        />
      </div>
    )
  }

  return (
    <div className="p-6 lg:p-8">
      <div className="mb-2 text-xs text-fg-secondary">
        <Link to="/" className="text-accent hover:underline">
          Dashboard
        </Link>
        <span className="mx-1.5">/</span>
        <span>Ports</span>
        <span className="mx-1.5">/</span>
        <span className="font-mono">
          :{portNum}/{proto}
        </span>
      </div>

      <PageHeader
        title={
          <span className="flex items-center gap-2.5">
            <span className="font-mono">:{portInfo.localPort}</span>
            <ProtocolBadge protocol={portInfo.protocol} />
            <StateBadge state={portInfo.stateName} />
          </span>
        }
        subtitle={`${portInfo.process || 'unknown'}${portInfo.pid ? ` · pid ${portInfo.pid}` : ''} · ${portInfo.localAddr}`}
        actions={
          <Link to="/alerts">
            <Button variant="default">
              <Bell size={14} />
              Add alert
            </Button>
          </Link>
        }
      />

      <Tabs defaultValue="overview" className="mb-5">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="connections">
            Live connections ({portConns.length})
          </TabsTrigger>
          <TabsTrigger value="history">History</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-3.5">
          <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard
              label="Current throughput"
              value={formatBps(portInfo.totalBps, 1).split(' ')[0]}
              unit={formatBps(portInfo.totalBps, 1).split(' ')[1]}
              meta={`RX ${formatBps(portInfo.rxBytesPerSec)} · TX ${formatBps(portInfo.txBytesPerSec)}`}
            />
            <StatCard
              label="Active connections"
              value={portInfo.connectionCount}
              meta={`${portConns.filter((c) => c.stateName === 'ESTABLISHED').length} established`}
            />
            <StatCard
              label="Last sample"
              value={formatRelativeTime(portInfo.ts)}
              meta={new Date(portInfo.ts).toLocaleTimeString()}
            />
            <StatCard
              label="Source"
              value={portInfo.totalBps > 0 ? 'eBPF/ss' : 'idle'}
              meta="byte counters"
            />
          </div>

          <div className="grid grid-cols-1 gap-3.5 lg:grid-cols-3">
            <Card className="lg:col-span-2">
              <CardHeader>
                <div>
                  <CardTitle>Throughput history</CardTitle>
                  <CardDescription>
                    Last {windowKey} · per-port byte counters
                  </CardDescription>
                </div>
                <ToggleGroup
                  value={windowKey}
                  onChange={setWindowKey}
                  size="sm"
                  options={[
                    { value: '5m', label: '5m' },
                    { value: '1h', label: '1h' },
                    { value: '24h', label: '24h' },
                  ]}
                />
              </CardHeader>
              <CardContent>
                {hist?.points && hist.points.length > 1 ? (
                  <BandwidthChart points={hist.points} />
                ) : (
                  <div className="grid h-60 place-items-center text-xs text-fg-tertiary">
                    No history yet — collecting…
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Geographic distribution</CardTitle>
              </CardHeader>
              <CardContent>
                {geoStats.length === 0 ? (
                  <div className="grid h-40 place-items-center text-xs text-fg-tertiary">
                    No remote connections
                  </div>
                ) : (
                  <div className="flex flex-col gap-2">
                    {geoStats.map((g) => (
                      <div key={g.country} className="flex items-center gap-2.5">
                        <span className="text-base">
                          {g.country !== 'Unknown' ? countryFlag(g.country) : '·'}
                        </span>
                        <span className="w-20 text-xs">{g.country}</span>
                        <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-bg-elevated">
                          <div
                            className="h-full rounded-full bg-accent"
                            style={{ width: `${g.percentage}%` }}
                          />
                        </div>
                        <span className="w-12 text-right font-mono text-xs tabular-nums">
                          {g.percentage.toFixed(0)}%
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="connections">
          <Card>
            <CardHeader>
              <CardTitle>
                Live connections on this port ({portConns.length})
              </CardTitle>
            </CardHeader>
            <CardContent className="px-0">
              {portConns.length === 0 ? (
                <div className="px-6 py-12 text-center text-xs text-fg-tertiary">
                  No active connections
                </div>
              ) : (
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-border-subtle">
                      <th className="px-5 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                        Remote
                      </th>
                      <th className="px-3 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                        Geo
                      </th>
                      <th className="px-3 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                        State
                      </th>
                      <th className="px-3 py-2 text-right text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                        Bytes ↓
                      </th>
                      <th className="px-3 py-2 text-right text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                        Bytes ↑
                      </th>
                      <th className="px-5 py-2 text-right text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                        Age
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {portConns.map((c, i) => (
                      <tr
                        key={`${c.remoteAddr}-${i}`}
                        className="border-b border-border-subtle text-sm last:border-b-0"
                      >
                        <td className="px-5 py-2.5 font-mono text-xs">{c.remoteAddr}</td>
                        <td className="px-3 py-2.5 text-xs">
                          {c.country ? `${countryFlag(c.country)} ${c.country}` : '—'}
                          {c.asn && <span className="ml-1 text-fg-tertiary">· {c.asn}</span>}
                        </td>
                        <td className="px-3 py-2.5">
                          <StateBadge state={c.stateName} />
                        </td>
                        <td className="px-3 py-2.5 text-right font-mono text-xs tabular-nums">
                          {c.rxBytes ? formatBytes(c.rxBytes) : '—'}
                        </td>
                        <td className="px-3 py-2.5 text-right font-mono text-xs tabular-nums">
                          {c.txBytes ? formatBytes(c.txBytes) : '—'}
                        </td>
                        <td className="px-5 py-2.5 text-right font-mono text-xs tabular-nums">
                          {c.age ? formatAge(c.age) : '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="history">
          <Card>
            <CardHeader>
              <div>
                <CardTitle>Bandwidth history</CardTitle>
                <CardDescription>RX + TX over the selected window</CardDescription>
              </div>
              <ToggleGroup
                value={windowKey}
                onChange={setWindowKey}
                size="sm"
                options={[
                  { value: '5m', label: '5m' },
                  { value: '1h', label: '1h' },
                  { value: '24h', label: '24h' },
                ]}
              />
            </CardHeader>
            <CardContent>
              {hist?.points && hist.points.length > 1 ? (
                <BandwidthChart points={hist.points} height={320} />
              ) : (
                <div className="grid h-80 place-items-center text-xs text-fg-tertiary">
                  No history yet
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
