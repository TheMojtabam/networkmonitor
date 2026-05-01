import { NavLink, Outlet } from 'react-router-dom'
import {
  LayoutDashboard,
  Activity,
  Network,
  Settings,
  Bell,
  PlugZap,
  type LucideIcon,
} from 'lucide-react'
import { useApp } from '@/store/app'
import { cn } from '@/lib/utils'

const navItems = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, end: true },
  { to: '/bandwidth', label: 'Top Bandwidth', icon: Activity },
  { to: '/connections', label: 'Connections', icon: Network },
] as const

const systemItems = [
  { to: '/alerts', label: 'Alerts', icon: Bell },
  { to: '/settings', label: 'Settings', icon: Settings },
] as const

export function AppShell() {
  const wsStatus = useApp((s) => s.wsStatus)

  return (
    <div className="flex min-h-screen">
      {/* ============ Sidebar ============ */}
      <aside className="sticky top-0 hidden h-screen w-60 flex-shrink-0 flex-col border-r border-border-subtle bg-bg-surface md:flex">
        <div className="flex items-center gap-2.5 border-b border-border-subtle px-4 py-5">
          <div className="grid h-8 w-8 place-items-center rounded-md bg-gradient-to-br from-accent to-success text-sm font-bold text-white shadow-md-dark">
            P
          </div>
          <div className="flex flex-col leading-tight">
            <span className="text-sm font-semibold tracking-tight">PortSleuth</span>
            <span className="text-2xs text-fg-tertiary">v1.0.0</span>
          </div>
        </div>

        <nav className="flex-1 overflow-y-auto px-3 py-4">
          <NavSection label="Monitor" items={navItems} />
          <NavSection label="System" items={systemItems} />
        </nav>

        <div className="border-t border-border-subtle px-4 py-3 text-2xs text-fg-tertiary">
          <div className="flex items-center gap-1.5">
            <StatusDot status={wsStatus} />
            <ConnectionLabel status={wsStatus} />
          </div>
        </div>
      </aside>

      {/* ============ Main ============ */}
      <main className="min-w-0 flex-1">
        <Outlet />
      </main>
    </div>
  )
}

interface NavItemProps {
  to: string
  label: string
  icon: LucideIcon
  end?: boolean
}

function NavSection({
  label,
  items,
}: {
  label: string
  items: readonly NavItemProps[]
}) {
  return (
    <div className="mb-5">
      <div className="px-3 pb-1.5 text-2xs font-semibold uppercase tracking-wider text-fg-tertiary">
        {label}
      </div>
      <div className="flex flex-col gap-0.5">
        {items.map((item) => (
          <NavItem key={item.to} {...item} />
        ))}
      </div>
    </div>
  )
}

function NavItem({ to, label, icon: Icon, end }: NavItemProps) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cn(
          'flex items-center gap-2.5 rounded-sm px-3 py-1.5 text-sm font-medium transition-colors',
          isActive
            ? 'bg-accent-bg text-accent-hover ring-1 ring-accent-border'
            : 'text-fg-secondary hover:bg-bg-hover hover:text-fg-primary',
        )
      }
    >
      <Icon size={16} className="flex-shrink-0" />
      <span className="truncate">{label}</span>
    </NavLink>
  )
}

function StatusDot({ status }: { status: ReturnType<typeof useApp.getState>['wsStatus'] }) {
  const color =
    status === 'open'
      ? 'bg-success shadow-[0_0_8px_var(--tw-shadow-color)] shadow-success'
      : status === 'connecting'
        ? 'bg-warning'
        : 'bg-danger'
  return (
    <span
      className={cn(
        'inline-block h-2 w-2 rounded-full',
        color,
        status === 'open' && 'animate-pulse-dot',
      )}
    />
  )
}

function ConnectionLabel({
  status,
}: {
  status: ReturnType<typeof useApp.getState>['wsStatus']
}) {
  switch (status) {
    case 'open':
      return <span>Live · WebSocket connected</span>
    case 'connecting':
      return <span className="flex items-center gap-1"><PlugZap size={11} />Connecting…</span>
    case 'closed':
    case 'error':
      return <span>Disconnected · retrying…</span>
    default:
      return <span>Idle</span>
  }
}
