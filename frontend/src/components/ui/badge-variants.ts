import { cva } from 'class-variance-authority'

export const badgeVariants = cva(
  'inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full px-1 text-[10px] leading-none font-bold tabular-nums',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground',
        muted: 'bg-muted text-muted-foreground',
        beginner: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400',
        intermediate: 'bg-amber-500/15 text-amber-700 dark:text-amber-400',
        advanced: 'bg-red-500/15 text-red-700 dark:text-red-400',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)
