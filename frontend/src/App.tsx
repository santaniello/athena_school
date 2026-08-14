import { useEffect, useState } from 'react'
import { HasLocalSession } from '../wailsjs/go/desktop/App'
import type { desktop } from '../wailsjs/go/models'
import LoginScreen from './screens/LoginScreen'
import RegisterScreen from './screens/RegisterScreen'
import ResetAccountScreen from './screens/ResetAccountScreen'
import MainScreen from './screens/MainScreen'

type View = 'checking' | 'login' | 'register' | 'reset' | 'main'

function App() {
  const [view, setView] = useState<View>('checking')
  const [session, setSession] = useState<desktop.LoginResult | null>(null)

  useEffect(() => {
    HasLocalSession().then((hasSession) => setView(hasSession ? 'main' : 'login'))
  }, [])

  function handleAuthSuccess(result: desktop.LoginResult) {
    setSession(result)
    setView('main')
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
      {view === 'main' && <MainScreen email={session?.email} />}
    </>
  )
}

export default App
