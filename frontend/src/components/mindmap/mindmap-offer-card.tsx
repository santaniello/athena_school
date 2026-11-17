import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  acceptMindMapOffer,
  declineMindMapOffer,
  mindMapErrorMessage,
  type MindMapSummary,
} from '@/lib/mindmap'

interface MindMapOfferCardProps {
  offerId: string
  // The offer is accepted: the mind map was drawn and its card takes this one's place.
  onAccepted: (mindMap: MindMapSummary) => void
  onDeclined: () => void
}

// The tutor's mind map offer, under its reply. Accepting draws the map, so the
// card shows progress and locks both buttons meanwhile; a failure leaves the
// offer in place with the error. It speaks the profile's language, like the
// rest of the mind map screens. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
function MindMapOfferCard({ offerId, onAccepted, onDeclined }: MindMapOfferCardProps) {
  const { t } = useTranslation('mindmap')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function run<T>(action: () => Promise<T>, onDone: (result: T) => void) {
    setBusy(true)
    setError(null)
    try {
      onDone(await action())
    } catch (err) {
      setError(mindMapErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border bg-card p-3 text-sm">
      <p className="font-medium">{t('offer.title')}</p>
      {busy && <p className="text-muted-foreground">{t('offer.preparing')}</p>}
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="flex gap-2">
        <Button
          size="sm"
          disabled={busy}
          onClick={() => void run(() => acceptMindMapOffer(offerId), onAccepted)}
        >
          {t('offer.accept')}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={() => void run(() => declineMindMapOffer(offerId), onDeclined)}
        >
          {t('offer.decline')}
        </Button>
      </div>
    </div>
  )
}

export { MindMapOfferCard }
