import type { DefenseTurn } from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface DefenseTranscriptProps {
  turns: DefenseTurn[]
}

// The tutor's questions and the user's answers, in the order they happened.
export function DefenseTranscript({ turns }: DefenseTranscriptProps) {
  const { t } = useTranslation()
  return (
    <section aria-label={t('result.tutorQuestions')} className="flex flex-col gap-3">
      <h3 className="text-lg font-semibold text-foreground">{t('result.tutorQuestions')}</h3>
      <ol className="flex flex-col gap-3">
        {turns.map((turn) => (
          <li
            key={turn.position}
            className="flex flex-col gap-1 rounded-md border border-border p-3"
          >
            <p className="text-sm font-medium text-foreground">
              {turn.position}. {turn.question}
            </p>
            <p className="text-sm whitespace-pre-line text-muted-foreground">{turn.answer}</p>
          </li>
        ))}
      </ol>
    </section>
  )
}
