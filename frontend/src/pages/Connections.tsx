import { useMemo } from 'react'
import { Download } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { net } from '@/lib/api'
import { PageHeader } from '@/components/layout/PageHeader'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { DataTable } from '@/components/ui/data-table'
import { ProtocolBadge, StateBadge } from '@/components/ui/port-badges'
import { formatBytes, formatAge, countryFlag } from '@/lib/utils'
import type { Connection } from '@/types'

export function Connections() {
  const { data, isLoading } = useQuery({
    queryKey: ['connections'],
    queryFn: () => net.connections(),
    refetchInterval: 5_000,
  })

  const conns = data?.connections ?? []

  const stats = useMemo(() => {
    const total = conns.length
    const established = conns.filter((c) => c.stateName === 'ESTABLISHED').length
    const listen = conns.filter((c) => c.stateName === 'LISTEN').length
    return { total, established, listen }
  }, [conns])

  const columns = useMemo<ColumnDef<Connection>[]>(
    () => [
      {
        accessorKey: 'protocol',
        header: 'Proto',
        cell: ({ row }) => <ProtocolBadge protocol={row.original.protocol} />,
        size: 70,
      },
      {
        accessorKey: 'localAddr',
        header: 'Local',
        cell: ({ row }) => (
          <span className="font-mono text-xs">{row.original.localAddr}</span>
        ),
      },
      {
        accessorKey: 'remoteAddr',
        header: 'Remote',
        cell: ({ row }) => (
          <span className="font-mono text-xs">{row.original.remoteAddr}</span>
        ),
      },
      {
        id: 'geo',
        header: 'Geo',
        cell: ({ row }) => {
          const c = row.original
          if (!c.country && !c.asn) {
            return <span className="text-xs text-fg-tertiary">—</span>
          }
          return (
            <span className="text-xs">
              {c.country && (
                <>
                  <span className="mr-1">{countryFlag(c.country)}</span>
                  {c.country}
                </>
              )}
              {c.asn && <span className="ml-1.5 text-fg-tertiary">· {c.asn}</span>}
            </span>
          )
        },
      },
      {
        accessorKey: 'stateName',
        header: 'State',
        cell: ({ row }) => <StateBadge state={row.original.stateName} />,
        size: 120,
      },
      {
        accessorKey: 'process',
        header: 'Process',
        cell: ({ row }) => (
          <span className="font-mono text-xs">{row.original.process || '—'}</span>
        ),
      },
      {
        accessorKey: 'rxBytes',
        header: () => <div className="text-right">Bytes ↓</div>,
        cell: ({ row }) => (
          <div className="text-right font-mono text-xs tabular-nums">
            {row.original.rxBytes ? formatBytes(row.original.rxBytes) : '—'}
          </div>
        ),
        size: 100,
      },
      {
        accessorKey: 'txBytes',
        header: () => <div className="text-right">Bytes ↑</div>,
        cell: ({ row }) => (
          <div className="text-right font-mono text-xs tabular-nums">
            {row.original.txBytes ? formatBytes(row.original.txBytes) : '—'}
          </div>
        ),
        size: 100,
      },
      {
        accessorKey: 'age',
        header: () => <div className="text-right">Age</div>,
        cell: ({ row }) => (
          <div className="text-right font-mono text-xs tabular-nums">
            {row.original.age ? formatAge(row.original.age) : '—'}
          </div>
        ),
        size: 90,
      },
    ],
    [],
  )

  const handleExport = () => {
    const headers = ['protocol', 'local', 'remote', 'state', 'process', 'pid', 'country', 'asn', 'rx_bytes', 'tx_bytes']
    const rows = conns.map((c) => [
      c.protocol,
      c.localAddr,
      c.remoteAddr,
      c.stateName,
      c.process || '',
      c.pid || '',
      c.country || '',
      c.asn || '',
      c.rxBytes || '',
      c.txBytes || '',
    ])
    const csv = [headers, ...rows].map((r) => r.join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `portsleuth-connections-${new Date().toISOString()}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="p-6 lg:p-8">
      <PageHeader
        title="Connections explorer"
        subtitle="All active connections · search, filter, sort"
        actions={
          <Button variant="default" onClick={handleExport} disabled={!conns.length}>
            <Download size={14} />
            Export
          </Button>
        }
      />

      <div className="mb-3 flex items-center gap-5 text-xs text-fg-secondary">
        <span>
          <strong className="mr-1 text-fg-primary">{stats.total}</strong>total
        </span>
        <span>
          <strong className="mr-1 text-fg-primary">{stats.established}</strong>established
        </span>
        <span>
          <strong className="mr-1 text-fg-primary">{stats.listen}</strong>listen
        </span>
      </div>

      <Card>
        <CardContent className="pt-5">
          {isLoading ? (
            <Skeleton className="h-96" />
          ) : (
            <DataTable
              columns={columns}
              data={conns}
              searchable
              searchPlaceholder="Search IP, port, process…"
              pageSize={25}
            />
          )}
        </CardContent>
      </Card>
    </div>
  )
}
