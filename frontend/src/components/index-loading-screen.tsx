import { AthenaLogo } from '@/components/athena-logo'

// Shown for the entire application while the knowledge index's initial
// background load runs — no search or knowledge mutation can race the
// initial snapshot (see specs/phases/phase-02-knowledge-engine/04-vector-search.md).
function IndexLoadingScreen() {
  return (
    <div className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-4 bg-background">
      <AthenaLogo className="size-16 animate-pulse" />
      <p className="text-sm text-muted-foreground">Loading knowledge index...</p>
    </div>
  )
}

export { IndexLoadingScreen }
