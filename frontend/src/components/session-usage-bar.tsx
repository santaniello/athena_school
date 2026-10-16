import { Info } from 'lucide-react'
import { Progress } from '@/components/ui/progress'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { formatCost, formatTokens } from '@/lib/usage-format'
import { cn } from '@/lib/utils'
import type { StudyContextUsage, StudySessionUsage } from '@/lib/study'

// The accumulated-consumption half of StudySessionUsage; its persisted
// context snapshot is passed separately as `context`.
type UsageTotals = Omit<StudySessionUsage, 'context'>

interface SessionUsageBarProps {
  context: Pick<StudyContextUsage, 'state' | 'usedTokens' | 'contextLength' | 'estimated'>
  // null until the first fetch resolves (or when it failed).
  usage: UsageTotals | null
}

const INDICATOR_BY_STATE: Record<StudyContextUsage['state'], string> = {
  normal: '',
  warning: '[&_[data-slot=progress-indicator]]:bg-amber-500',
  blocked: '[&_[data-slot=progress-indicator]]:bg-destructive',
}

// SessionUsageBar shows the two token metrics side by side and never mixes
// them: "Context" is how full the model's window is for the next request;
// "Usage" is what the session's completed LLM calls consumed in total. See
// specs/phases/phase-02-knowledge-engine/22-usage-visibility.md.
export function SessionUsageBar({ context, usage }: SessionUsageBarProps) {
  return (
    <div
      data-slot="session-usage-bar"
      className="flex flex-wrap items-center gap-x-6 gap-y-1 text-sm text-muted-foreground"
    >
      <ContextMeter context={context} />
      <SessionUsage usage={usage} />
    </div>
  )
}

function ContextMeter({ context }: Pick<SessionUsageBarProps, 'context'>) {
  const { usedTokens, contextLength, estimated, state } = context
  const used = `${estimated ? '~' : ''}${formatTokens(usedTokens)}`

  if (contextLength > 0) {
    const percent = Math.round((usedTokens / contextLength) * 100)
    return (
      <div className="flex items-center gap-2">
        <span>Context</span>
        <Progress
          aria-label="Context"
          value={Math.min(percent, 100)}
          className={cn('w-24', INDICATOR_BY_STATE[state])}
        />
        <span>{`${used} / ${formatTokens(contextLength)} tokens · ${percent}%`}</span>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-2">
      <span>Context</span>
      <span>{usedTokens > 0 ? `${used} tokens · limit unknown` : '—'}</span>
    </div>
  )
}

function SessionUsage({ usage }: Pick<SessionUsageBarProps, 'usage'>) {
  const hasCalls = usage !== null && usage.calls > 0
  return (
    <div className="flex items-center gap-2">
      <span>Usage (completed replies)</span>
      <Tooltip>
        <TooltipTrigger aria-label="About session usage" className="cursor-help">
          <Info className="size-3.5" />
        </TooltipTrigger>
        <TooltipContent>
          Interrupted replies and failed calls are not counted. The value may be lower than your
          provider&apos;s invoice.
        </TooltipContent>
      </Tooltip>
      {hasCalls ? <UsageFigures usage={usage} /> : <span>—</span>}
    </div>
  )
}

function UsageFigures({ usage }: { usage: UsageTotals }) {
  const tokens = formatTokens(usage.inputTokens + usage.outputTokens)
  const cost = formatCost(usage.cost, usage.unpricedCalls)
  if (usage.unpricedCalls === 0) {
    return <span>{`${tokens} tokens · ${cost}`}</span>
  }
  return (
    <Tooltip>
      <TooltipTrigger className="cursor-help">{`${tokens} tokens · ${cost}`}</TooltipTrigger>
      <TooltipContent>Some calls have no reported cost</TooltipContent>
    </Tooltip>
  )
}
