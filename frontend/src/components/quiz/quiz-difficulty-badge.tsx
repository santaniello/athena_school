import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { quizDifficultyLabel } from '@/lib/quiz'

// Green is easy, amber is in between, red is hard; an unknown value is muted.
const VARIANTS: Record<string, 'beginner' | 'intermediate' | 'advanced'> = {
  EASY: 'beginner',
  MEDIUM: 'intermediate',
  HARD: 'advanced',
}

function QuizDifficultyBadge({ difficulty }: { difficulty: string }) {
  // Subscribes the badge to the language, which quizDifficultyLabel reads.
  useTranslation('quiz')
  return <Badge variant={VARIANTS[difficulty] ?? 'muted'}>{quizDifficultyLabel(difficulty)}</Badge>
}

export { QuizDifficultyBadge }
