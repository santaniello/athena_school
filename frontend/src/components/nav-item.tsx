import { Lock } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import type { AppSection, NavItem as NavItemData } from '@/lib/navigation'

interface NavItemProps {
  item: NavItemData
  active: boolean
  onSelect: (id: AppSection) => void
}

// A single sidebar row. Locked rows (sections from an unbuilt phase) stay
// fully clickable — routing to a ComingSoonPanel — instead of a dead click,
// per specs/phases/phase-01-desktop-mvp/03-home-screen.md.
function NavItem({ item, active, onSelect }: NavItemProps) {
  const locked = item.status === 'locked'
  const Icon = item.icon

  const button = (
    <button
      type="button"
      aria-current={active ? 'page' : undefined}
      onClick={() => onSelect(item.id)}
      className={cn(
        'flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium text-muted-foreground transition-colors',
        locked ? 'opacity-60 hover:bg-accent/40' : 'hover:bg-accent hover:text-accent-foreground',
        active && 'bg-primary/10 text-foreground',
      )}
    >
      <Icon className="size-4 shrink-0" aria-hidden="true" />
      <span className="flex-1 text-left">{item.label}</span>
      {locked && <Lock className="size-3.5 shrink-0" aria-hidden="true" />}
    </button>
  )

  if (!locked) return button

  return (
    <Tooltip>
      <TooltipTrigger asChild>{button}</TooltipTrigger>
      <TooltipContent side="right">Planned for Phase {item.phase}</TooltipContent>
    </Tooltip>
  )
}

export { NavItem }
