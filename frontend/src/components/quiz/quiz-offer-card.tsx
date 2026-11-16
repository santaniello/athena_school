import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DEFAULT_QUESTIONS,
  MAX_QUESTIONS,
  MIN_QUESTIONS,
  QUIZ_DIFFICULTIES,
  acceptQuizOffer,
  declineQuizOffer,
  parseQuestionCount,
  quizDifficultyLabel,
  quizErrorMessage,
  type QuizDifficulty,
  type QuizSummary,
} from '@/lib/quiz'

interface QuizOfferCardProps {
  offerId: string
  // The difficulty the choice starts with: the profile's experience level.
  defaultDifficulty: QuizDifficulty
  // The offer is accepted: the quiz was written and its card takes this one's place.
  onAccepted: (quiz: QuizSummary) => void
  onDeclined: () => void
}

// The tutor's quiz offer, under its reply. Accepting writes the quiz, so the
// card shows progress and locks everything meanwhile; a failure leaves the
// offer in place with the error. It speaks the profile's language, like the
// rest of the quiz screens. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
function QuizOfferCard({ offerId, defaultDifficulty, onAccepted, onDeclined }: QuizOfferCardProps) {
  const { t } = useTranslation('quiz')
  const [countText, setCountText] = useState(String(DEFAULT_QUESTIONS))
  const [difficulty, setDifficulty] = useState(defaultDifficulty)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const count = parseQuestionCount(countText)

  async function run<T>(action: () => Promise<T>, onDone: (result: T) => void) {
    setBusy(true)
    setError(null)
    try {
      onDone(await action())
    } catch (err) {
      setError(quizErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border bg-card p-3 text-sm">
      <p className="font-medium">{t('offer.title')}</p>
      <div className="flex items-center gap-2">
        <label htmlFor={`quiz-offer-count-${offerId}`} className="text-muted-foreground">
          {t('offer.questions')}
        </label>
        <Input
          id={`quiz-offer-count-${offerId}`}
          type="number"
          min={MIN_QUESTIONS}
          max={MAX_QUESTIONS}
          className="w-24"
          value={countText}
          disabled={busy}
          onChange={(event) => setCountText(event.target.value)}
        />
      </div>
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">{t('offer.difficulty')}</span>
        <Select
          value={difficulty}
          disabled={busy}
          onValueChange={(value) => setDifficulty(value as QuizDifficulty)}
        >
          <SelectTrigger aria-label={t('offer.difficulty')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {QUIZ_DIFFICULTIES.map((value) => (
              <SelectItem key={value} value={value}>
                {quizDifficultyLabel(value)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {busy && <p className="text-muted-foreground">{t('offer.preparing')}</p>}
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="flex gap-2">
        <Button
          size="sm"
          disabled={busy || count === null}
          onClick={() => {
            if (count !== null)
              void run(() => acceptQuizOffer(offerId, count, difficulty), onAccepted)
          }}
        >
          {t('offer.accept')}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={() => void run(() => declineQuizOffer(offerId), onDeclined)}
        >
          {t('offer.decline')}
        </Button>
      </div>
    </div>
  )
}

export { QuizOfferCard }
