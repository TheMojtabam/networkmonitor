import * as React from 'react'
import { ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {}

/**
 * Native HTML <select> styled to match the dark theme. Used for filters
 * where Radix's full <Select> primitive would be overkill.
 */
export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  ({ className, children, ...props }, ref) => (
    <div className="relative inline-block">
      <select
        ref={ref}
        className={cn(
          'h-8 appearance-none rounded-sm border border-border bg-bg-elevated pl-3 pr-8 text-sm',
          'text-fg-primary focus:border-accent focus:outline-none',
          'disabled:cursor-not-allowed disabled:opacity-50',
          className,
        )}
        {...props}
      >
        {children}
      </select>
      <ChevronDown
        className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-fg-tertiary"
        size={14}
      />
    </div>
  ),
)
Select.displayName = 'Select'
