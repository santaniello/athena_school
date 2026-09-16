import { cva } from 'class-variance-authority'

export const badgeVariants = cva(
  'inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full px-1 text-[10px] leading-none font-bold tabular-nums',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground',
        muted: 'bg-muted text-muted-foreground',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)
