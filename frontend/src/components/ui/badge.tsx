import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2 py-0.5 text-2xs font-semibold border whitespace-nowrap',
  {
    variants: {
      variant: {
        default: 'bg-bg-hover text-fg-secondary border-border',
        accent: 'bg-accent-bg text-accent-hover border-accent-border',
        success: 'bg-success-bg text-success border-success/30',
        warning: 'bg-warning-bg text-warning border-warning/30',
        danger: 'bg-danger-bg text-danger border-danger/30',
        tcp: 'bg-accent-bg text-accent-hover border-accent-border',
        udp: 'bg-warning-bg text-warning border-warning/30',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />
}
