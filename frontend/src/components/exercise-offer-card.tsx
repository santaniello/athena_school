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
  CHALLENGE_DIFFICULTIES,
  CHALLENGE_KINDS,
  acceptOffer,
  declineOffer,
  difficultyLabel,
  exerciseErrorCode,
  exerciseErrorMessage,
  kindLabel,
  practiceSuggestions,
  type ChallengeDifficulty,
  type ChallengeKind,
} from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface ExerciseOfferCardProps {
  offerId: string
  // The difficulty the choice starts with: the profile's experience level.
  defaultDifficulty: ChallengeDifficulty
  // The session's topic, offered as a suggestion for what to practice.
  topic?: string
  // The offer is accepted: the session's exercise card takes its place.
  onAccepted: () => void
  onDeclined: () => void
  // The offer no longer exists (resolved elsewhere or gone): the chat drops it
  // and re-reads what it shows, so the composer is not left blocked.
  onGone: () => void
}

// The tutor's exercise offer, under its reply. Accepting may generate a
// challenge, so the card shows progress and locks both buttons meanwhile; a
// failure leaves the offer in place with the error, except when the offer is
// gone, which hands it back to the chat. The user picks the
// difficulty, starting from the profile's level, and the kind: an interview, or
// a practice that needs the concept to train. See
// specs/phases/phase-03-architecture-lab/14-ai-challenge-generation.md and
// 18-practice-challenges.md.
function ExerciseOfferCard({
  offerId,
  defaultDifficulty,
  topic,
  onAccepted,
  onDeclined,
  onGone,
}: ExerciseOfferCardProps) {
  const [difficulty, setDifficulty] = useState(defaultDifficulty)
  const [kind, setKind] = useState<ChallengeKind>('INTERVIEW')
  const [goal, setGoal] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { t } = useTranslation()
  const practice = kind === 'PRACTICE'
  const trimmedGoal = goal.trim()

  async function run<T>(action: () => Promise<T>, onDone: (result: T) => void) {
    setBusy(true)
    setError(null)
    try {
      onDone(await action())
    } catch (err) {
      if (exerciseErrorCode(err) === 'offer_not_found') {
        onGone()
        return
      }
      setError(exerciseErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border bg-card p-3 text-sm">
      <p className="font-medium">{t('offer.title')}</p>
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">{t('generate.difficulty')}</span>
        <Select
          value={difficulty}
          disabled={busy}
          onValueChange={(value) => setDifficulty(value as ChallengeDifficulty)}
        >
          <SelectTrigger aria-label={t('generate.difficulty')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CHALLENGE_DIFFICULTIES.map((value) => (
              <SelectItem key={value} value={value}>
                {difficultyLabel(value)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">{t('generate.type')}</span>
        <Select
          value={kind}
          disabled={busy}
          onValueChange={(value) => setKind(value as ChallengeKind)}
        >
          <SelectTrigger aria-label={t('generate.type')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CHALLENGE_KINDS.map((value) => (
              <SelectItem key={value} value={value}>
                {kindLabel(value)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {practice && (
        <div className="flex flex-col gap-2">
          <label htmlFor={`practice-goal-${offerId}`} className="text-muted-foreground">
            {t('generate.goal')}
          </label>
          <Textarea
            id={`practice-goal-${offerId}`}
            value={goal}
            disabled={busy}
            placeholder={t('generate.goalPlaceholder')}
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
      {busy && <p className="text-muted-foreground">{t('offer.preparing')}</p>}
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
          {t('offer.accept')}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={() => void run(() => declineOffer(offerId), onDeclined)}
        >
          {t('offer.decline')}
        </Button>
      </div>
    </div>
  )
}

export { ExerciseOfferCard }
