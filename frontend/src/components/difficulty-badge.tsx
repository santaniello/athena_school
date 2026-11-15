import { Badge } from '@/components/ui/badge'
import { difficultyLabel, type ChallengeDifficulty } from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

// One color per level so the difficulty reads at a glance: green is easy,
// amber is in between, red is hard. An unknown value falls back to muted.
const DIFFICULTY_VARIANTS: Record<ChallengeDifficulty, 'beginner' | 'intermediate' | 'advanced'> = {
  BEGINNER: 'beginner',
  INTERMEDIATE: 'intermediate',
  ADVANCED: 'advanced',
}

interface DifficultyBadgeProps {
  difficulty: string
}

function DifficultyBadge({ difficulty }: DifficultyBadgeProps) {
  // Subscribes the badge to the language, which difficultyLabel reads.
  useTranslation()
  const level = difficulty as ChallengeDifficulty
  return (
    <Badge variant={DIFFICULTY_VARIANTS[level] ?? 'muted'}>{difficultyLabel(difficulty)}</Badge>
  )
}

export { DifficultyBadge }
