import { useEffect, useMemo, useState } from 'react'
import { apiGet } from './lib/api'
import type { NetInterface, NetPort } from './lib/api'

function Card({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/30 p-4">
      <div className="text-xs text-slate-400">{title}</div>
      <div className="mt-1 text-2xl font-semibold tracking-tight">{value}</div>
    </div>
  )
}

function fmtBps(n: number) {
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export default function App() {
  const [ifs, setIfs] = useState<NetInterface[]>([])
  const [ports, setPorts] = useState<NetPort[]>([])
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let t: any
    const tick = async () => {
      try {
        setErr(null)
        const [a, b] = await Promise.all([
          apiGet<{ interfaces: any[]; rates?: any[] }>('/api/net/interfaces'),
          apiGet<{ ports: any[] }>('/api/net/ports'),
        ])
        // Backend returns {interfaces:[...], rates:[...]} where rates items carry bytes/sec.
        // Merge by interface name.
        const rateMap = new Map<string, any>()
        ;(a.rates ?? []).forEach((r) => rateMap.set(r.name, r))
        const merged = (a.interfaces ?? []).map((x) => {
          const r = rateMap.get(x.name) || {}
          return {
            name: x.name,
            rxBytes: x.rxBytes,
            txBytes: x.txBytes,
            rxBps: r.rxBytesPerSec ?? 0,
            txBps: r.txBytesPerSec ?? 0,
          } as NetInterface
        })
        setIfs(merged)
        setPorts(
          (b.ports ?? []).map((p) => ({
            proto: p.protocol,
            localAddr: p.localAddr,
            localPort: p.localPort,
            state: String(p.state),
            connections: p.connectionCount ?? 0,
          }))
        )
      } catch (e: any) {
        setErr(e?.message ?? String(e))
      }
      t = setTimeout(tick, 1000)
    }
    tick()
    return () => clearTimeout(t)
  }, [])

  const totals = useMemo(() => {
    const rx = ifs.reduce((s, x) => s + (x.rxBps || 0), 0)
    const tx = ifs.reduce((s, x) => s + (x.txBps || 0), 0)
    return { rx, tx }
  }, [ifs])

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="sticky top-0 border-b border-slate-800 bg-slate-950/70 backdrop-blur px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-xs text-slate-400">PortSleuth</div>
            <div className="text-lg font-semibold">Network Monitor</div>
          </div>
          <div className="text-xs text-slate-400">{err ? `Error: ${err}` : 'Live'}</div>
        </div>
      </header>

      <main className="p-6 space-y-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <Card title="Interfaces" value={String(ifs.length)} />
          <Card title="Listening ports" value={String(ports.length)} />
          <Card title="RX total" value={fmtBps(totals.rx)} />
          <Card title="TX total" value={fmtBps(totals.tx)} />
        </div>

        <section className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <div className="rounded-2xl border border-slate-800 bg-slate-900/20 p-4">
            <div className="text-sm font-semibold">Interfaces</div>
            <div className="mt-3 overflow-auto">
              <table className="w-full text-sm">
                <thead className="text-slate-400">
                  <tr>
                    <th className="text-left py-2">Name</th>
                    <th className="text-right py-2">RX</th>
                    <th className="text-right py-2">TX</th>
                  </tr>
                </thead>
                <tbody>
                  {ifs.map((x) => (
                    <tr key={x.name} className="border-t border-slate-800/60">
                      <td className="py-2 font-medium">{x.name}</td>
                      <td className="py-2 text-right tabular-nums">{fmtBps(x.rxBps || 0)}</td>
                      <td className="py-2 text-right tabular-nums">{fmtBps(x.txBps || 0)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <div className="rounded-2xl border border-slate-800 bg-slate-900/20 p-4">
            <div className="text-sm font-semibold">Ports</div>
            <div className="mt-3 overflow-auto">
              <table className="w-full text-sm">
                <thead className="text-slate-400">
                  <tr>
                    <th className="text-left py-2">Proto</th>
                    <th className="text-left py-2">Bind</th>
                    <th className="text-right py-2">Port</th>
                    <th className="text-right py-2">Conns</th>
                  </tr>
                </thead>
                <tbody>
                  {ports
                    .slice()
                    .sort((a, b) => (b.connections || 0) - (a.connections || 0))
                    .slice(0, 50)
                    .map((p, i) => (
                      <tr key={`${p.proto}-${p.localAddr}-${p.localPort}-${i}`} className="border-t border-slate-800/60">
                        <td className="py-2 font-medium">{p.proto}</td>
                        <td className="py-2 text-slate-300">{p.localAddr}</td>
                        <td className="py-2 text-right tabular-nums">{p.localPort}</td>
                        <td className="py-2 text-right tabular-nums">{p.connections ?? 0}</td>
                      </tr>
                    ))}
                </tbody>
              </table>
              <div className="mt-2 text-xs text-slate-500">Showing top 50 by connections</div>
            </div>
          </div>
        </section>
      </main>
    </div>
  )
}
