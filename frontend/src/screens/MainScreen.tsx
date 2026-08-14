interface MainScreenProps {
  email?: string
}

// Placeholder landing screen for a logged-in user. The real main screen has
// no spec yet — later phases (onboarding, study mode) replace this.
function MainScreen({ email }: MainScreenProps) {
  return (
    <div className="auth-screen">
      <h1>Bem-vindo(a)</h1>
      {email && <p>{email}</p>}
    </div>
  )
}

export default MainScreen
