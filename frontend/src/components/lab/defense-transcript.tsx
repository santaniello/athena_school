import type { DefenseTurn } from '@/lib/exercise'

interface DefenseTranscriptProps {
  turns: DefenseTurn[]
}

// The tutor's questions and the user's answers, in the order they happened.
export function DefenseTranscript({ turns }: DefenseTranscriptProps) {
  return (
    <section aria-label="Tutor questions" className="flex flex-col gap-3">
      <h3 className="text-lg font-semibold text-foreground">Tutor questions</h3>
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
