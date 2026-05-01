import { cn } from '@/lib/utils'

interface Props {
  title: React.ReactNode
  subtitle?: React.ReactNode
  actions?: React.ReactNode
  className?: string
}

export function PageHeader({ title, subtitle, actions, className }: Props) {
  return (
    <div className={cn('mb-6 flex flex-wrap items-center justify-between gap-4', className)}>
      <div>
        <h1 className="text-xl font-semibold tracking-tight text-fg-primary">{title}</h1>
        {subtitle && <p className="mt-1 text-xs text-fg-secondary">{subtitle}</p>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}
