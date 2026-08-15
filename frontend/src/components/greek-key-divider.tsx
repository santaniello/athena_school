import { cn } from '@/lib/utils'

interface GreekKeyDividerProps {
  className?: string
  orientation?: 'horizontal' | 'vertical'
}

// A tiled meander (Greek key) motif, used as a decorative divider on auth
// screens — a nod to Athena's namesake without relying on an image asset.
// Uses a fixed-size CSS mask (not an SVG `fill="currentColor"` in a
// background-image, which browsers don't reliably resolve and silently
// fall back to black) so the real bg-primary color shows through the
// stencil shape, staying crisp at any container width.
const TILE_MASK = encodeURIComponent(
  '<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><path d="M0 10 L0 4 L4 4 L4 0 L10 0 L10 6 L6 6 L6 10 Z" fill="white"/></svg>',
)
const maskImage = `url("data:image/svg+xml,${TILE_MASK}")`

function GreekKeyDivider({ className, orientation = 'horizontal' }: GreekKeyDividerProps) {
  const isVertical = orientation === 'vertical'
  return (
    <div
      role="presentation"
      className={cn(isVertical ? 'h-full w-2.5' : 'h-2.5 w-full', 'bg-primary/80', className)}
      style={{
        maskImage,
        WebkitMaskImage: maskImage,
        maskRepeat: isVertical ? 'repeat-y' : 'repeat-x',
        WebkitMaskRepeat: isVertical ? 'repeat-y' : 'repeat-x',
        maskSize: '10px 10px',
        WebkitMaskSize: '10px 10px',
      }}
    />
  )
}

export { GreekKeyDivider }
