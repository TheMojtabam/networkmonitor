import { useState, useMemo } from 'react'
import { Download } from 'lucide-react'
import { useApp } from '@/store/app'
import { PageHeader } from '@/components/layout/PageHeader'
import { StatCard } from '@/components/layout/StatCard'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { ToggleGroup } from '@/components/ui/toggle-group'
import { BandwidthBars } from '@/components/charts/BandwidthBars'
import { Skeleton } from '@/components/ui/skeleton'
import { formatBps } from '@/lib/utils'

export function Bandwidth() {
  const snapshot = useApp((s) => s.snapshot)
  const [by, setBy] = useState<'total' | 'rx' | 'tx'>('total')
  const [protoFilter, setProtoFilter] = useState<'all' | 'tcp' | 'udp'>('all')
  const [ifaceFilter, setIfaceFilter] = useState<string>('all')

  const filteredPorts = useMemo(() => {
    if (!snapshot) return []
    return snapshot.ports.filter((p) => {
      if (protoFilter === 'tcp' && !p.protocol.startsWith('tcp')) return false
      if (protoFilter === 'udp' && !p.protocol.startsWith('udp')) return false
      return true
    })
  }, [snapshot, protoFilter])

  const sortedPorts = useMemo(() => {
    const list = [...filteredPorts]
    if (by === 'rx') list.sort((a, b) => b.rxBytesPerSec - a.rxBytesPerSec)
    else if (by === 'tx') list.sort((a, b) => b.txBytesPerSec - a.txBytesPerSec)
    else list.sort((a, b) => b.totalBps - a.totalBps)
    return list.slice(0, 20)
  }, [filteredPorts, by])

  const handleExport = () => {
    if (!snapshot) return
    const headers = ['protocol', 'port', 'process', 'pid', 'connections', 'rx_bps', 'tx_bps', 'total_bps']
    const rows = sortedPorts.map((p) => [
      p.protocol,
      p.localPort,
      p.process || '',
      p.pid || '',
      p.connectionCount,
      p.rxBytesPerSec.toFixed(2),
      p.txBytesPerSec.toFixed(2),
      p.totalBps.toFixed(2),
    ])
    const csv = [headers, ...rows].map((r) => r.join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `portsleuth-top-${new Date().toISOString()}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  if (!snapshot) {
    return (
      <div className="p-8">
        <PageHeader title="Top Bandwidth" subtitle="Connecting…" />
        <Skeleton className="h-96" />
      </div>
    )
  }

  const totalObserved = sortedPorts.reduce((sum, p) => sum + p.totalBps, 0)
  const peak = sortedPorts[0]
  const totalPackets =
    snapshot.rates.reduce((s, r) => s + r.rxPktsPerSec + r.txPktsPerSec, 0) | 0

  return (
    <div className="p-6 lg:p-8">
      <PageHeader
        title="Top Bandwidth"
        subtitle="Highest throughput ports across the system"
        actions={
          <>
            <ToggleGroup
              value={by}
              onChange={setBy}
              options={[
                { value: 'rx', label: 'RX' },
                { value: 'tx', label: 'TX' },
                { value: 'total', label: 'Total' },
              ]}
            />
            <Button variant="default" onClick={handleExport}>
              <Download size={14} />
              Export CSV
            </Button>
          </>
        }
      />

      <div className="mb-5 grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Total observed"
          value={formatBps(totalObserved, 1).split(' ')[0]}
          unit={formatBps(totalObserved, 1).split(' ')[1]}
          meta={`across ${sortedPorts.length} ports`}
        />
        <StatCard
          label="Peak port"
          value={peak ? `:${peak.localPort}` : '—'}
          meta={peak ? `${peak.process || 'unknown'} · ${formatBps(peak.totalBps)}` : ''}
        />
        <StatCard
          label="Active connections"
          value={snapshot.totals.activeConns}
          meta={`${snapshot.totals.establishedConn} established`}
        />
        <StatCard
          label="Packets/sec"
          value={totalPackets >= 1000 ? `${(totalPackets / 1000).toFixed(1)}k` : totalPackets}
          meta="across all interfaces"
        />
      </div>

      <Card>
        <CardHeader>
          <div>
            <CardTitle>Top 20 ports by bandwidth</CardTitle>
            <CardDescription>Live · updates with WebSocket</CardDescription>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Select
              value={protoFilter}
              onChange={(e) => setProtoFilter(e.target.value as 'all' | 'tcp' | 'udp')}
            >
              <option value="all">All protocols</option>
              <option value="tcp">TCP only</option>
              <option value="udp">UDP only</option>
            </Select>
            <Select
              value={ifaceFilter}
              onChange={(e) => setIfaceFilter(e.target.value)}
            >
              <option value="all">All interfaces</option>
              {snapshot.rates.map((r) => (
                <option key={r.name} value={r.name}>
                  {r.name}
                </option>
              ))}
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          {sortedPorts.length > 0 ? (
            <BandwidthBars ports={sortedPorts} by={by} />
          ) : (
            <div className="grid h-40 place-items-center text-xs text-fg-tertiary">
              No port traffic yet
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
