import { useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DIFFICULTY_LABELS,
  acceptOffer,
  declineOffer,
  exerciseErrorMessage,
  offerLabels,
  type ChallengeDifficulty,
} from '@/lib/exercise'

const DIFFICULTIES = Object.keys(DIFFICULTY_LABELS) as ChallengeDifficulty[]

interface ExerciseOfferCardProps {
  offerId: string
  // The assistant language ('pt' or 'en'): the card is the one place the
  // interface speaks it.
  language: string
  // The difficulty the choice starts with: the profile's experience level.
  defaultDifficulty: ChallengeDifficulty
  // The offer is accepted: the session's exercise card takes its place.
  onAccepted: () => void
  onDeclined: () => void
}

// The tutor's exercise offer, under its reply. Accepting may generate a
// challenge, so the card shows progress and locks both buttons meanwhile; a
// failure leaves the offer in place with the error. The user picks the
// difficulty, starting from the profile's level. See
// specs/phases/phase-03-architecture-lab/14-ai-challenge-generation.md.
function ExerciseOfferCard({
  offerId,
  language,
  defaultDifficulty,
  onAccepted,
  onDeclined,
}: ExerciseOfferCardProps) {
  const [difficulty, setDifficulty] = useState(defaultDifficulty)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const labels = offerLabels(language)

  async function run<T>(action: () => Promise<T>, onDone: (result: T) => void) {
    setBusy(true)
    setError(null)
    try {
      onDone(await action())
    } catch (err) {
      setError(exerciseErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border bg-card p-3 text-sm">
      <p className="font-medium">{labels.title}</p>
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">{labels.difficulty}</span>
        <Select
          value={difficulty}
          disabled={busy}
          onValueChange={(value) => setDifficulty(value as ChallengeDifficulty)}
        >
          <SelectTrigger aria-label={labels.difficulty}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {DIFFICULTIES.map((value) => (
              <SelectItem key={value} value={value}>
                {labels.levels[value]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {busy && <p className="text-muted-foreground">{labels.preparing}</p>}
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="flex gap-2">
        <Button
          size="sm"
          disabled={busy}
          onClick={() =>
            void run(
              () => acceptOffer(offerId, difficulty),
              () => onAccepted(),
            )
          }
        >
          {labels.accept}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={() => void run(() => declineOffer(offerId), onDeclined)}
        >
          {labels.decline}
        </Button>
      </div>
    </div>
  )
}

export { ExerciseOfferCard }
