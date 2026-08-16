import { useEffect, useState } from 'react'
import { HasLocalSession, HasOpenRouterKey, HasUserProfile } from '../wailsjs/go/desktop/App'
import LoginScreen from './screens/LoginScreen'
import RegisterScreen from './screens/RegisterScreen'
import ResetAccountScreen from './screens/ResetAccountScreen'
import KeyGateScreen from './screens/KeyGateScreen'
import OnboardingScreen from './screens/OnboardingScreen'
import { AppShell } from './components/app-shell'
import { SplashScreen } from './components/splash-screen'

type View = 'checking' | 'login' | 'register' | 'reset' | 'key-gate' | 'onboarding' | 'app'

// The boot splash's Greek-key-frame-then-logo choreography takes ~1.05s to
// settle (see splash-screen.tsx). A local session check usually resolves
// in a few milliseconds, which would swap the splash out mid-animation —
// so its minimum time on screen is floored here regardless of how fast the
// check actually finishes.
const MIN_SPLASH_MS = 1400

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// Decides what to show after authentication: the mandatory OpenRouter key
// gate first (see specs/phases/phase-01-desktop-mvp/04-onboarding.md), then
// onboarding if the profile hasn't been collected yet, else straight into
// the app shell (see specs/phases/phase-01-desktop-mvp/03-home-screen.md).
async function resolvePostAuthView(): Promise<View> {
  if (!(await HasOpenRouterKey())) return 'key-gate'
  if (!(await HasUserProfile())) return 'onboarding'
  return 'app'
}

async function resolveInitialView(): Promise<View> {
  return (await HasLocalSession()) ? resolvePostAuthView() : 'login'
}

function App() {
  const [view, setView] = useState<View>('checking')

  useEffect(() => {
    Promise.all([resolveInitialView(), wait(MIN_SPLASH_MS)]).then(([resolvedView]) => {
      setView(resolvedView)
    })
  }, [])

  async function handleAuthSuccess() {
    setView(await resolvePostAuthView())
  }

  return (
    <>
      {view === 'checking' && <SplashScreen />}
      {view === 'login' && (
        <LoginScreen
          onSuccess={handleAuthSuccess}
          onNavigateToRegister={() => setView('register')}
          onNavigateToReset={() => setView('reset')}
        />
      )}
      {view === 'register' && (
        <RegisterScreen onSuccess={handleAuthSuccess} onNavigateToLogin={() => setView('login')} />
      )}
      {view === 'reset' && (
        <ResetAccountScreen onDone={() => setView('register')} onCancel={() => setView('login')} />
      )}
      {view === 'key-gate' && (
        <KeyGateScreen onSaved={() => void resolvePostAuthView().then(setView)} />
      )}
      {view === 'onboarding' && <OnboardingScreen onComplete={() => setView('app')} />}
      {view === 'app' && <AppShell onLogout={() => setView('login')} />}
    </>
  )
}

export default App
