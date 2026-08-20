import { Alert, AlertAction, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import type { IndexStatus } from '@/lib/knowledge-index'

interface IndexStatusBannerProps {
  status: IndexStatus
  // Set once the user picks "Continue without local search" on the failed
  // screen — the backend status stays "failed", but the app is no longer
  // gated behind it.
  continuedWithoutSearch: boolean
  retrying: boolean
  onRetry: () => void
  onReview: () => void
}

// The persistent, non-blocking warning shown once the app is open: isolated
// content after a partial load (ready_with_warnings), an unavailable index
// the user chose to continue past, or a retry currently rebuilding the
// snapshot. Renders nothing when the index is fully ready and idle.
function IndexStatusBanner({
  status,
  continuedWithoutSearch,
  retrying,
  onRetry,
  onReview,
}: IndexStatusBannerProps) {
  if (retrying) {
    return (
      <Alert>
        <AlertDescription>Rebuilding knowledge index...</AlertDescription>
      </Alert>
    )
  }

  if (status.state === 'ready_with_warnings') {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          {status.issues.length} item{status.issues.length === 1 ? '' : 's'} need attention and were
          left out of local search.
        </AlertDescription>
        <AlertAction>
          <Button size="sm" variant="outline" onClick={onReview}>
            Review
          </Button>
        </AlertAction>
      </Alert>
    )
  }

  if (status.state === 'failed' && continuedWithoutSearch) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Local search is unavailable. Existing content is unaffected.
        </AlertDescription>
        <AlertAction>
          <Button size="sm" variant="outline" onClick={onRetry}>
            Retry
          </Button>
        </AlertAction>
      </Alert>
    )
  }

  return null
}

export { IndexStatusBanner }
