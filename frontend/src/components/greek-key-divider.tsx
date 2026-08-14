import { cn } from '@/lib/utils'

interface GreekKeyDividerProps {
  className?: string
}

// A tiled meander (Greek key) motif, used as a decorative divider on auth
// screens — a nod to Athena's namesake without relying on an image asset.
// Rendered as a fixed-size CSS background tile (not a stretched inline SVG)
// so the stroke/fill stays crisp at any container width; `currentColor`
// resolves against the element's `color`, so Tailwind text-color utilities
// (e.g. text-primary/60) theme it.
const TILE = encodeURIComponent(
  '<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><path d="M0 10 L0 4 L4 4 L4 0 L10 0 L10 6 L6 6 L6 10 Z" fill="currentColor"/></svg>',
)

function GreekKeyDivider({ className }: GreekKeyDividerProps) {
  return (
    <div
      role="presentation"
      className={cn('h-2.5 w-full text-primary/60', className)}
      style={{
        backgroundImage: `url("data:image/svg+xml,${TILE}")`,
        backgroundRepeat: 'repeat-x',
        backgroundSize: '10px 10px',
      }}
    />
  )
}

export { GreekKeyDivider }
