import { useEffect, useState } from 'react'
import { HasLocalSession, HasOpenRouterKey, HasUserProfile } from '../wailsjs/go/desktop/App'
import type { desktop } from '../wailsjs/go/models'
import LoginScreen from './screens/LoginScreen'
import RegisterScreen from './screens/RegisterScreen'
import ResetAccountScreen from './screens/ResetAccountScreen'
import KeyGateScreen from './screens/KeyGateScreen'
import OnboardingScreen from './screens/OnboardingScreen'
import MainScreen from './screens/MainScreen'

type View = 'checking' | 'login' | 'register' | 'reset' | 'key-gate' | 'onboarding' | 'main'

// Decides what to show after authentication: the mandatory OpenRouter key
// gate first (see specs/phases/phase-01-desktop-mvp/04-onboarding.md), then
// onboarding if the profile hasn't been collected yet, else straight in.
async function resolvePostAuthView(): Promise<View> {
  if (!(await HasOpenRouterKey())) return 'key-gate'
  if (!(await HasUserProfile())) return 'onboarding'
  return 'main'
}

function App() {
  const [view, setView] = useState<View>('checking')
  const [session, setSession] = useState<desktop.LoginResult | null>(null)

  useEffect(() => {
    HasLocalSession().then(async (hasSession) => {
      setView(hasSession ? await resolvePostAuthView() : 'login')
    })
  }, [])

  async function handleAuthSuccess(result: desktop.LoginResult) {
    setSession(result)
    setView(await resolvePostAuthView())
  }

  return (
    <>
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
      {view === 'onboarding' && <OnboardingScreen onComplete={() => setView('main')} />}
      {view === 'main' && <MainScreen email={session?.email} />}
    </>
  )
}

export default App
