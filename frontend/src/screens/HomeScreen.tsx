import { Button } from '@/components/ui/button'
import type { ProfileDraft } from '@/lib/profile'

interface HomeScreenProps {
  profile: ProfileDraft | null
  studyLocked: boolean
  onStartStudy: () => void
  now?: Date
}

function timeOfDayGreeting(hour: number): string {
  if (hour < 12) return 'Good morning'
  if (hour < 18) return 'Good afternoon'
  return 'Good evening'
}

// The default landing content once inside the app shell. Intentionally
// minimal: a greeting and one CTA — no progress bars, knowledge counts or
// flashcard stats, since none of that exists yet in Phase 1. See
// specs/phases/phase-01-desktop-mvp/03-home-screen.md.
function HomeScreen({ profile, studyLocked, onStartStudy, now = new Date() }: HomeScreenProps) {
  if (!profile) return null

  return (
    <div className="m-auto flex max-w-md flex-col items-start gap-5">
      <div className="flex flex-col gap-1">
        <h2 className="font-heading text-2xl font-bold text-balance text-foreground">
          {timeOfDayGreeting(now.getHours())}, {profile.name}.
        </h2>
        <p className="text-muted-foreground">{profile.assistantName} is ready when you are.</p>
      </div>
      <div className="flex flex-col gap-1.5">
        <Button size="lg" onClick={onStartStudy}>
          Start a study session
        </Button>
        {studyLocked && (
          <p className="text-xs text-muted-foreground italic">Locked until Study Mode ships</p>
        )}
      </div>
    </div>
  )
}

export default HomeScreen
export { timeOfDayGreeting }
