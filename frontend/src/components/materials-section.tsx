import { useId } from 'react'
import type { ReactNode } from 'react'
import { ChevronRight } from 'lucide-react'
import { cn } from '@/lib/utils'

interface MaterialsSectionProps {
  title: string
  count: number
  open: boolean
  onToggle: () => void
  // Buttons shown at the right of the header, outside the toggle.
  actions?: ReactNode
  children: ReactNode
}

// One collapsible section of the Session materials panel. An open section
// takes an equal share of the panel's height and scrolls on its own; a closed
// one takes only its header. See
// specs/phases/phase-04-tutor/05-session-materials-panel.md.
function MaterialsSection({
  title,
  count,
  open,
  onToggle,
  actions,
  children,
}: MaterialsSectionProps) {
  const bodyId = useId()

  return (
    <section
      className={cn(
        'flex min-h-0 flex-col border-b border-border',
        open ? 'min-h-40 flex-1 basis-0' : 'flex-none',
      )}
    >
      <div className="flex shrink-0 items-center gap-2 pr-3 hover:bg-accent/30">
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={open}
          aria-controls={bodyId}
          className="flex min-w-0 flex-1 items-center gap-2 px-3 py-2.5 text-left"
        >
          <ChevronRight
            className={cn(
              'size-3.5 shrink-0 text-muted-foreground transition-transform',
              open && 'rotate-90',
            )}
            aria-hidden="true"
          />
          <span className="truncate text-sm font-semibold text-foreground">{title}</span>
          <span className="rounded-full border border-border px-1.5 text-[11px] text-muted-foreground tabular-nums">
            {count}
          </span>
        </button>
        {actions}
      </div>
      {open && (
        <div
          id={bodyId}
          role="region"
          aria-label={title}
          className="thin-scroll min-h-0 flex-1 overflow-y-auto px-1.5 pb-2"
        >
          {children}
        </div>
      )}
    </section>
  )
}

export { MaterialsSection }
