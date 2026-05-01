import { Badge } from '@/components/ui/badge'
import type { PortStateName, Protocol } from '@/types'
import { cn } from '@/lib/utils'

export function ProtocolBadge({
  protocol,
  className,
}: {
  protocol: Protocol
  className?: string
}) {
  const label = protocol.toUpperCase()
  const variant = protocol.startsWith('udp') ? 'udp' : 'tcp'
  return (
    <Badge variant={variant} className={cn('font-mono', className)}>
      {label}
    </Badge>
  )
}

export function StateBadge({
  state,
  className,
}: {
  state: PortStateName
  className?: string
}) {
  let variant: React.ComponentProps<typeof Badge>['variant'] = 'default'
  switch (state) {
    case 'LISTEN':
      variant = 'success'
      break
    case 'ESTABLISHED':
      variant = 'accent'
      break
    case 'TIME_WAIT':
    case 'CLOSE_WAIT':
    case 'FIN_WAIT1':
    case 'FIN_WAIT2':
      variant = 'default'
      break
    case 'SYN_SENT':
    case 'SYN_RECV':
      variant = 'warning'
      break
    default:
      variant = 'default'
  }
  return (
    <Badge variant={variant} className={cn('text-2xs', className)}>
      {state}
    </Badge>
  )
}
