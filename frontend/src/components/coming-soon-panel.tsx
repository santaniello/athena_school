import { Lock } from 'lucide-react'
import type { NavItem } from '@/lib/navigation'

interface ComingSoonPanelProps {
  item: NavItem
}

// Shown for any locked sidebar section instead of a dead click — a
// lightweight roadmap preview until the section's own phase ships. See
// specs/phases/phase-01-desktop-mvp/03-home-screen.md.
function ComingSoonPanel({ item }: ComingSoonPanelProps) {
  return (
    <div className="m-auto flex max-w-sm flex-col items-center gap-2 text-center">
      <div className="flex size-12 items-center justify-center rounded-full bg-primary/10 ring-1 ring-primary/25">
        <Lock className="size-5 text-primary" aria-hidden="true" />
      </div>
      <p className="text-xs font-bold text-primary">Planned for Phase {item.phase}</p>
      <h2 className="font-heading text-sm font-bold tracking-[0.14em] text-foreground uppercase">
        {item.label}
      </h2>
      <p className="text-sm text-muted-foreground">{item.description}</p>
    </div>
  )
}

export { ComingSoonPanel }
