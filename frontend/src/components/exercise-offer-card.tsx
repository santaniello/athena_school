import { useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
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
  practiceSuggestions,
  type ChallengeDifficulty,
  type ChallengeKind,
} from '@/lib/exercise'

const DIFFICULTIES = Object.keys(DIFFICULTY_LABELS) as ChallengeDifficulty[]
const KINDS: ChallengeKind[] = ['INTERVIEW', 'PRACTICE']

interface ExerciseOfferCardProps {
  offerId: string
  // The assistant language ('pt' or 'en'): the card is the one place the
  // interface speaks it.
  language: string
  // The difficulty the choice starts with: the profile's experience level.
  defaultDifficulty: ChallengeDifficulty
  // The session's topic, offered as a suggestion for what to practice.
  topic?: string
  // The offer is accepted: the session's exercise card takes its place.
  onAccepted: () => void
  onDeclined: () => void
}

// The tutor's exercise offer, under its reply. Accepting may generate a
// challenge, so the card shows progress and locks both buttons meanwhile; a
// failure leaves the offer in place with the error. The user picks the
// difficulty, starting from the profile's level, and the kind: an interview, or
// a practice that needs the concept to train. See
// specs/phases/phase-03-architecture-lab/14-ai-challenge-generation.md and
// 18-practice-challenges.md.
function ExerciseOfferCard({
  offerId,
  language,
  defaultDifficulty,
  topic,
  onAccepted,
  onDeclined,
}: ExerciseOfferCardProps) {
  const [difficulty, setDifficulty] = useState(defaultDifficulty)
  const [kind, setKind] = useState<ChallengeKind>('INTERVIEW')
  const [goal, setGoal] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const labels = offerLabels(language)
  const practice = kind === 'PRACTICE'
  const trimmedGoal = goal.trim()

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
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">{labels.kind}</span>
        <Select
          value={kind}
          disabled={busy}
          onValueChange={(value) => setKind(value as ChallengeKind)}
        >
          <SelectTrigger aria-label={labels.kind}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {KINDS.map((value) => (
              <SelectItem key={value} value={value}>
                {labels.kinds[value]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {practice && (
        <div className="flex flex-col gap-2">
          <label htmlFor={`practice-goal-${offerId}`} className="text-muted-foreground">
            {labels.goal}
          </label>
          <Textarea
            id={`practice-goal-${offerId}`}
            value={goal}
            disabled={busy}
            placeholder={labels.goalPlaceholder}
            onChange={(event) => setGoal(event.target.value)}
          />
          <div className="flex flex-wrap gap-2">
            {practiceSuggestions(topic).map((suggestion) => (
              <Button
                key={suggestion}
                type="button"
                size="xs"
                variant="outline"
                disabled={busy}
                onClick={() => setGoal(suggestion)}
              >
                {suggestion}
              </Button>
            ))}
          </div>
        </div>
      )}
      {busy && <p className="text-muted-foreground">{labels.preparing}</p>}
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="flex gap-2">
        <Button
          size="sm"
          disabled={busy || (practice && trimmedGoal === '')}
          onClick={() =>
            void run(
              () => acceptOffer(offerId, difficulty, kind, practice ? trimmedGoal : ''),
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
