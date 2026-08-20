import { Button } from '@/components/ui/button'

interface IndexFailedScreenProps {
  lastError: string
  retrying: boolean
  onRetry: () => void
  onContinue: () => void
}

// Shown for the entire application when the knowledge index's initial
// background load fails outright (no prior snapshot to fall back to). A
// failed/unpublished index is unavailable, not empty — see
// specs/phases/phase-02-knowledge-engine/04-vector-search.md.
function IndexFailedScreen({ lastError, retrying, onRetry, onContinue }: IndexFailedScreenProps) {
  return (
    <div className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-4 bg-background px-6 text-center">
      <p className="text-sm font-medium text-foreground">Knowledge index could not be loaded.</p>
      {lastError && <p className="max-w-sm text-xs text-muted-foreground">{lastError}</p>}
      <div className="flex gap-2 pt-2">
        <Button onClick={onRetry} disabled={retrying}>
          Retry
        </Button>
        <Button variant="outline" onClick={onContinue} disabled={retrying}>
          Continue without local search
        </Button>
      </div>
    </div>
  )
}

export { IndexFailedScreen }
