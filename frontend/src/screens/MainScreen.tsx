import { GreekKeyDivider } from '@/components/greek-key-divider'
import { AthenaLogo } from '@/components/athena-logo'

interface MainScreenProps {
  email?: string
}

// Placeholder landing screen for a logged-in user. The real main screen has
// no spec yet — later phases (onboarding, study mode) replace this.
function MainScreen({ email }: MainScreenProps) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 p-6 text-center">
      <GreekKeyDivider className="max-w-xs" />
      <div className="flex items-center justify-center gap-3">
        <AthenaLogo className="h-10 w-10 shrink-0" />
        <h1 className="font-heading text-3xl font-bold tracking-[0.15em] text-primary uppercase">
          Welcome
        </h1>
      </div>
      <GreekKeyDivider className="max-w-xs scale-y-[-1]" />
      {email && <p className="mt-2 text-sm text-muted-foreground">{email}</p>}
    </div>
  )
}

export default MainScreen
