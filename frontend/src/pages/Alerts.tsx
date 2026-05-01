import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, AlertTriangle, CheckCircle2, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { alerts as alertsAPI } from '@/lib/api'
import { PageHeader } from '@/components/layout/PageHeader'
import { StatCard } from '@/components/layout/StatCard'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { formatRelativeTime } from '@/lib/utils'
import type { AlertRule, AlertOp, AlertMetric } from '@/types'

export function AlertsPage() {
  const qc = useQueryClient()

  const { data: rulesData, isLoading: rulesLoading } = useQuery({
    queryKey: ['alerts', 'rules'],
    queryFn: () => alertsAPI.rules(),
  })

  const { data: eventsData } = useQuery({
    queryKey: ['alerts', 'events'],
    queryFn: () => alertsAPI.events(),
    refetchInterval: 10_000,
  })

  const setRules = useMutation({
    mutationFn: (rules: AlertRule[]) => alertsAPI.setRules(rules),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts', 'rules'] }),
  })

  const rules = rulesData?.rules ?? []
  const events = eventsData?.events ?? []
  const enabledCount = rules.filter((r) => r.enabled).length

  const [showForm, setShowForm] = useState(false)

  const handleToggle = (rule: AlertRule, on: boolean) => {
    const next = rules.map((r) => (r.id === rule.id ? { ...r, enabled: on } : r))
    setRules.mutate(next, {
      onSuccess: () => toast.success(`${rule.name} ${on ? 'enabled' : 'paused'}`),
      onError: () => toast.error('Failed to update rule'),
    })
  }

  const handleDelete = (rule: AlertRule) => {
    if (!confirm(`Delete rule "${rule.name}"?`)) return
    const next = rules.filter((r) => r.id !== rule.id)
    setRules.mutate(next, {
      onSuccess: () => toast.success('Rule deleted'),
    })
  }

  const handleAdd = (rule: AlertRule) => {
    const next = [...rules, rule]
    setRules.mutate(next, {
      onSuccess: () => {
        toast.success('Rule added')
        setShowForm(false)
      },
      onError: () => toast.error('Failed to save rule'),
    })
  }

  return (
    <div className="p-6 lg:p-8">
      <PageHeader
        title="Alerts"
        subtitle="Define rules and get notified via webhook, Telegram, or email"
        actions={
          <Button variant="primary" onClick={() => setShowForm(true)}>
            <Plus size={14} />
            New rule
          </Button>
        }
      />

      <div className="mb-5 grid grid-cols-1 gap-3.5 sm:grid-cols-3">
        <StatCard
          label="Active rules"
          value={enabledCount}
          meta={`${rules.length - enabledCount} paused`}
        />
        <StatCard
          label="Triggered (recent)"
          value={events.length}
          valueColor={events.length > 0 ? '#f5a623' : undefined}
          meta={
            events[0] ? `last: ${formatRelativeTime(events[0].ts)}` : 'no events yet'
          }
        />
        <StatCard
          label="Channels"
          value="—"
          meta="Configure in Settings"
        />
      </div>

      {showForm && (
        <RuleForm onSave={handleAdd} onCancel={() => setShowForm(false)} />
      )}

      <Card>
        <CardHeader>
          <CardTitle>Rules</CardTitle>
        </CardHeader>
        <CardContent>
          {rulesLoading ? (
            <Skeleton className="h-40" />
          ) : rules.length === 0 ? (
            <EmptyState
              icon={<AlertTriangle size={28} />}
              title="No alert rules yet"
              description="Create a rule to be notified when bandwidth, connection counts, or other metrics cross thresholds."
              action={
                <Button variant="primary" onClick={() => setShowForm(true)}>
                  Create first rule
                </Button>
              }
            />
          ) : (
            <div className="flex flex-col gap-2">
              {rules.map((rule) => (
                <RuleRow
                  key={rule.id}
                  rule={rule}
                  onToggle={(on) => handleToggle(rule, on)}
                  onDelete={() => handleDelete(rule)}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="mt-3.5">
        <CardHeader>
          <CardTitle>Recent events</CardTitle>
        </CardHeader>
        <CardContent>
          {events.length === 0 ? (
            <div className="py-8 text-center text-xs text-fg-tertiary">
              No events fired yet
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              {events.slice(0, 10).map((e, i) => (
                <div
                  key={`${e.ruleId}-${i}`}
                  className="flex items-center justify-between rounded-md border border-border-subtle bg-bg-elevated px-4 py-3"
                >
                  <div>
                    <div className="text-sm font-medium">{e.ruleName}</div>
                    <div className="font-mono text-2xs text-fg-tertiary">
                      {e.metric} = {e.value.toFixed(2)} (threshold {e.threshold})
                      {e.port ? ` · port ${e.port}` : ''}
                    </div>
                  </div>
                  <span className="text-2xs text-fg-secondary">
                    {formatRelativeTime(e.ts)}
                  </span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function RuleRow({
  rule,
  onToggle,
  onDelete,
}: {
  rule: AlertRule
  onToggle: (on: boolean) => void
  onDelete: () => void
}) {
  return (
    <div className="grid grid-cols-[40px_1fr_auto_auto_auto] items-center gap-3 rounded-md border border-border-subtle bg-bg-elevated px-4 py-3">
      <div
        className={
          rule.enabled
            ? 'grid h-9 w-9 place-items-center rounded-md bg-success-bg text-success'
            : 'grid h-9 w-9 place-items-center rounded-md bg-bg-hover text-fg-tertiary'
        }
      >
        {rule.enabled ? <CheckCircle2 size={18} /> : <AlertTriangle size={18} />}
      </div>
      <div className="min-w-0">
        <div className="truncate text-sm font-semibold">{rule.name}</div>
        <div className="truncate font-mono text-2xs text-fg-tertiary">
          {ruleSummary(rule)}
        </div>
      </div>
      <div className="flex flex-wrap gap-1">
        {rule.channels.map((c) => (
          <Badge key={c} variant="accent">
            {c}
          </Badge>
        ))}
      </div>
      <Switch checked={rule.enabled} onCheckedChange={onToggle} />
      <Button
        variant="ghost"
        size="icon"
        onClick={onDelete}
        aria-label="Delete rule"
      >
        <Trash2 size={14} />
      </Button>
    </div>
  )
}

function ruleSummary(r: AlertRule): string {
  let lhs = r.metric as string
  if (r.metric === 'port_bps' && r.port) lhs = `port :${r.port} bps`
  if (r.metric === 'connection_count' && r.port) lhs = `port :${r.port} connections`
  return `${lhs} ${r.operator} ${r.threshold} for ${r.forSeconds}s`
}

function RuleForm({
  onSave,
  onCancel,
}: {
  onSave: (rule: AlertRule) => void
  onCancel: () => void
}) {
  const [name, setName] = useState('')
  const [metric, setMetric] = useState<AlertMetric>('total_bps')
  const [operator, setOperator] = useState<AlertOp>('>')
  const [threshold, setThreshold] = useState('1000000')
  const [port, setPort] = useState('')
  const [forSeconds, setForSeconds] = useState('30')
  const [channels, setChannels] = useState('webhook')

  const requiresPort = metric === 'port_bps' || metric === 'connection_count'

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const rule: AlertRule = {
      id: `r-${Date.now()}`,
      name: name || 'Untitled rule',
      enabled: true,
      metric,
      operator,
      threshold: Number(threshold) || 0,
      port: requiresPort ? Number(port) || 0 : undefined,
      forSeconds: Number(forSeconds) || 30,
      channels: channels.split(',').map((c) => c.trim()).filter(Boolean),
    }
    onSave(rule)
  }

  return (
    <Card className="mb-5">
      <CardHeader>
        <CardTitle>New alert rule</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div className="md:col-span-2">
            <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
              Name
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="High traffic on :443"
              required
            />
          </div>
          <div>
            <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
              Metric
            </label>
            <Select
              value={metric}
              onChange={(e) => setMetric(e.target.value as AlertMetric)}
              className="w-full"
            >
              <option value="total_bps">Total bandwidth (bytes/s)</option>
              <option value="total_rx_bps">RX bandwidth</option>
              <option value="total_tx_bps">TX bandwidth</option>
              <option value="port_bps">Port bandwidth</option>
              <option value="connection_count">Connection count (per port)</option>
            </Select>
          </div>
          <div className="grid grid-cols-[80px_1fr] gap-2">
            <div>
              <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                Op
              </label>
              <Select
                value={operator}
                onChange={(e) => setOperator(e.target.value as AlertOp)}
                className="w-full"
              >
                <option value=">">&gt;</option>
                <option value=">=">&gt;=</option>
                <option value="<">&lt;</option>
                <option value="<=">&lt;=</option>
                <option value="==">==</option>
              </Select>
            </div>
            <div>
              <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                Threshold
              </label>
              <Input
                type="number"
                value={threshold}
                onChange={(e) => setThreshold(e.target.value)}
                required
              />
            </div>
          </div>
          {requiresPort && (
            <div>
              <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
                Port
              </label>
              <Input
                type="number"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                placeholder="443"
              />
            </div>
          )}
          <div>
            <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
              Sustained for (seconds)
            </label>
            <Input
              type="number"
              value={forSeconds}
              onChange={(e) => setForSeconds(e.target.value)}
            />
          </div>
          <div className="md:col-span-2">
            <label className="mb-1 block text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
              Channels (comma separated)
            </label>
            <Input
              value={channels}
              onChange={(e) => setChannels(e.target.value)}
              placeholder="webhook, telegram"
            />
          </div>
          <div className="md:col-span-2 flex justify-end gap-2 pt-2">
            <Button type="button" variant="default" onClick={onCancel}>
              Cancel
            </Button>
            <Button type="submit" variant="primary">
              Save rule
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
