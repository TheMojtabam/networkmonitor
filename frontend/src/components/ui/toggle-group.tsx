import * as React from 'react'
import { cn } from '@/lib/utils'

export interface ToggleOption<T extends string> {
  value: T
  label: React.ReactNode
}

interface ToggleGroupProps<T extends string> {
  options: ToggleOption<T>[]
  value: T
  onChange: (v: T) => void
  className?: string
  size?: 'sm' | 'default'
}

/**
 * Segmented control. Use for binary or 3–4-way mutually exclusive choices.
 * For longer lists prefer <Select>.
 */
export function ToggleGroup<T extends string>({
  options,
  value,
  onChange,
  className,
  size = 'default',
}: ToggleGroupProps<T>) {
  const sizeClass = size === 'sm' ? 'h-6 px-2 text-xs' : 'h-7 px-2.5 text-xs'
  return (
    <div
      className={cn(
        'inline-flex rounded-sm border border-border bg-bg-elevated p-0.5',
        className,
      )}
    >
      {options.map((opt) => {
        const active = opt.value === value
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={cn(
              'inline-flex items-center justify-center rounded-sm font-medium transition-colors',
              sizeClass,
              active
                ? 'bg-accent-bg text-accent-hover'
                : 'text-fg-secondary hover:text-fg-primary',
            )}
          >
            {opt.label}
          </button>
        )
      })}
    </div>
  )
}
