import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { DONT_KNOW_ANSWER, MAX_ANSWER_LENGTH, type DefenseQuestion } from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface DefenseStageProps {
  phase:
    'questioning' | 'questionFailed' | 'answering' | 'sendingAnswer' | 'judging' | 'judgingFailed'
  // The question being answered; null while one is being written.
  question: DefenseQuestion | null
  // Why the last step failed.
  message: string | null
  onSend: (answer: string) => void
  onRetry: () => void
}

// The tutor's defense, one question at a time. It never shows a score, a level
// or whether an answer was right: that is only in the final outcome. See
// specs/phases/phase-03-architecture-lab/12-architecture-defense.md.
export function DefenseStage({ phase, question, message, onSend, onRetry }: DefenseStageProps) {
  const { t } = useTranslation()
  if (phase === 'questioning' || phase === 'judging') {
    return (
      <Progress text={phase === 'questioning' ? t('defense.preparing') : t('defense.judging')} />
    )
  }

  if (phase === 'questionFailed' || phase === 'judgingFailed' || !question) {
    return (
      <div className="flex flex-col gap-3 p-4">
        <p role="alert" className="text-sm text-destructive">
          {message}
        </p>
        <div>
          <Button type="button" onClick={onRetry}>
            {t('common.tryAgain')}
          </Button>
        </div>
      </div>
    )
  }

  // A new key gives every question an empty answer box.
  return (
    <AnswerForm
      key={question.position}
      question={question}
      sending={phase === 'sendingAnswer'}
      message={message}
      onSend={onSend}
    />
  )
}

interface AnswerFormProps {
  question: DefenseQuestion
  sending: boolean
  message: string | null
  onSend: (answer: string) => void
}

function AnswerForm({ question, sending, message, onSend }: AnswerFormProps) {
  const { t } = useTranslation()
  const [answer, setAnswer] = useState('')

  return (
    <section className="flex flex-col gap-3 p-4">
      <h2 className="text-sm font-semibold text-foreground">
        {t('defense.question', { position: question.position, total: question.total })}
      </h2>
      <p className="text-sm text-foreground">{question.text}</p>
      <Textarea
        aria-label={t('defense.answerLabel')}
        value={answer}
        maxLength={MAX_ANSWER_LENGTH}
        disabled={sending}
        onChange={(event) => setAnswer(event.target.value)}
      />
      <p className="text-right text-xs text-muted-foreground">
        {answer.length} / {MAX_ANSWER_LENGTH}
      </p>
      {message && (
        <p role="alert" className="text-sm text-destructive">
          {message}
        </p>
      )}
      <div className="flex gap-2">
        <Button
          type="button"
          disabled={sending || answer.trim() === ''}
          onClick={() => onSend(answer)}
        >
          {sending ? t('defense.sending') : t('defense.send')}
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={sending}
          onClick={() => onSend(DONT_KNOW_ANSWER)}
        >
          {t('defense.dontKnow')}
        </Button>
      </div>
    </section>
  )
}

function Progress({ text }: { text: string }) {
  return (
    <div className="flex flex-col items-center gap-3 p-6 text-center">
      <Loader2 className="size-5 animate-spin text-muted-foreground" aria-hidden="true" />
      <p role="status" className="text-sm text-muted-foreground">
        {text}
      </p>
    </div>
  )
}
